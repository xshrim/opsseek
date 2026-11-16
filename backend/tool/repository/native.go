package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	gitSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func init() {
	// The default go-git file transport invokes git-upload-pack; use its Go server instead.
	client.InstallProtocol("file", server.NewClient(localRepositoryLoader{}))
}

type localRepositoryLoader struct{}

type timedSSHAuth struct {
	gitSSH.AuthMethod
	ctx     context.Context
	timeout time.Duration
}

func (a timedSSHAuth) ClientConfig() (*ssh.ClientConfig, error) {
	config, err := a.AuthMethod.ClientConfig()
	if err != nil {
		return nil, err
	}
	config.Timeout = a.timeout
	if deadline, ok := a.ctx.Deadline(); ok && time.Until(deadline) < config.Timeout {
		config.Timeout = time.Until(deadline)
	}
	if err := a.ctx.Err(); err != nil {
		return nil, err
	}
	return config, nil
}

func (localRepositoryLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	r, err := git.PlainOpen(ep.Path)
	if err != nil {
		return nil, err
	}
	return r.Storer, nil
}

func cloneRepository(ctx context.Context, in ConnectionInput, source, target string, bare bool) error {
	ep, err := transport.NewEndpoint(source)
	if err != nil {
		return err
	}
	var auth transport.AuthMethod
	switch ep.Protocol {
	case "http", "https":
		if in.Username != "" || in.Password != "" {
			user := in.Username
			if user == "" {
				user = "git"
			}
			auth = &gitHTTP.BasicAuth{Username: user, Password: in.Password}
		}
	case "ssh":
		// Host verification uses known_hosts; never accept unknown keys silently.
		user := in.Username
		if user == "" {
			user = ep.User
		}
		if user == "" {
			user = "git"
		}
		if in.Password != "" {
			auth = &gitSSH.Password{User: user, Password: in.Password}
		} else {
			for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				keyPath := filepath.Join(home, ".ssh", name)
				if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
					continue
				}
				key, err := gitSSH.NewPublicKeysFromFile(user, keyPath, "")
				if err != nil {
					return err
				}
				auth = key
				break
			}
			if auth == nil {
				auth, err = gitSSH.NewSSHAgentAuth(user)
				if err != nil {
					return fmt.Errorf("SSH credentials unavailable: %w", err)
				}
			}
		}
	case "file", "git":
	default:
		return fmt.Errorf("unsupported repository transport %q", ep.Protocol)
	}
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if ep.Protocol == "ssh" {
		auth = timedSSHAuth{auth.(gitSSH.AuthMethod), ctx, time.Duration(timeout) * time.Second}
	}
	options := &git.CloneOptions{URL: source, Auth: auth, Tags: git.NoTags, Mirror: bare}
	if in.Branch != "" {
		options.ReferenceName = plumbing.NewBranchReferenceName(in.Branch)
	}
	_, err = git.PlainCloneContext(ctx, target, bare, options)
	return err
}

func resolveCommit(r *git.Repository, revision string) (*object.Commit, error) {
	hash, err := r.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("resolve repository revision %q: %w", revision, err)
	}
	return r.CommitObject(*hash)
}

func branchRefs(r *git.Repository) ([]*plumbing.Reference, error) {
	refs, err := r.References()
	if err != nil {
		return nil, err
	}
	var result []*plumbing.Reference
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference && (ref.Name().IsBranch() || strings.HasPrefix(ref.Name().String(), "refs/remotes/origin/")) {
			result = append(result, ref)
		}
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result, err
}

func nativeSnapshotRevision(root string) string {
	r, err := git.PlainOpen(root)
	if err != nil {
		return ""
	}
	if head, err := r.Head(); err == nil {
		return head.Hash().String()
	}
	refs, err := branchRefs(r)
	if err != nil {
		return ""
	}
	var latest *object.Commit
	for _, ref := range refs {
		c, err := r.CommitObject(ref.Hash())
		if err == nil && (latest == nil || c.Committer.When.After(latest.Committer.When)) {
			latest = c
		}
	}
	if latest != nil {
		return latest.Hash.String()
	}
	return ""
}

func nativeRevisions(ctx context.Context, r *git.Repository, limit int) ([]BranchRevisions, error) {
	refs, err := branchRefs(r)
	if err != nil {
		return nil, err
	}
	head, _ := r.Head()
	defaultBranch := ""
	if head != nil {
		defaultBranch = head.Name().Short()
	}
	if ref, err := r.Reference("refs/remotes/origin/HEAD", false); err == nil {
		defaultBranch = strings.TrimPrefix(ref.Target().String(), "refs/remotes/origin/")
	}
	byBranch := map[string]*plumbing.Reference{}
	for _, ref := range refs {
		name := strings.TrimPrefix(ref.Name().Short(), "origin/")
		if name == "HEAD" {
			continue
		}
		if byBranch[name] == nil || ref.Name().IsBranch() {
			byBranch[name] = ref
		}
	}
	result := []BranchRevisions{}
	for name, ref := range byBranch {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		iter, err := r.Log(&git.LogOptions{From: ref.Hash(), Order: git.LogOrderCommitterTime})
		if err != nil {
			return nil, err
		}
		item := BranchRevisions{Branch: name, Commits: []string{}, CommitTimes: map[string]int64{}, IsDefault: name == defaultBranch}
		for len(item.Commits) < limit {
			if err := ctx.Err(); err != nil {
				iter.Close()
				return nil, err
			}
			c, err := iter.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				iter.Close()
				return nil, err
			}
			item.Commits = append(item.Commits, c.Hash.String())
			item.CommitTimes[c.Hash.String()] = c.Committer.When.Unix()
		}
		iter.Close()
		sort.SliceStable(item.Commits, func(i, j int) bool { return item.CommitTimes[item.Commits[i]] > item.CommitTimes[item.Commits[j]] })
		if len(item.Commits) > 0 {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		x, y := a.CommitTimes[a.Commits[0]], b.CommitTimes[b.Commits[0]]
		if x == y {
			return a.Branch < b.Branch
		}
		return x > y
	})
	return result, nil
}

func nativeTreeFromRepository(ctx context.Context, r *git.Repository, revision, path, directory string) ([]TreeEntry, error) {
	c, err := resolveCommit(r, revision)
	if err != nil {
		return nil, err
	}
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}
	entries := []TreeEntry{}
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	for {
		name, entry, err := walker.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.Mode == filemode.Dir {
			continue
		}
		if path != "" && path != "." && name != path && !strings.HasPrefix(name, path+"/") {
			continue
		}
		var size int64
		if entry.Mode != filemode.Submodule {
			blob, err := r.BlobObject(entry.Hash)
			if err != nil {
				return nil, err
			}
			size = blob.Size
		}
		entries = append(entries, TreeEntry{Path: displayScopedPath(directory, name), Type: "file", Size: size})
	}
	return entries, nil
}

func nativeTree(ctx context.Context, root, revision, path, directory string) ([]TreeEntry, error) {
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	return nativeTreeFromRepository(ctx, r, revision, path, directory)
}

func nativeFile(root, revision, path string) (*object.File, error) {
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	return nativeFileFromRepository(r, revision, path)
}

func nativeFileFromRepository(r *git.Repository, revision, path string) (*object.File, error) {
	c, err := resolveCommit(r, revision)
	if err != nil {
		return nil, err
	}
	return c.File(path)
}

func nativeSearch(ctx context.Context, root, revision, path, directory string, options SearchOptions) (SearchPage, error) {
	query := options.Pattern
	if !options.RegularExpression {
		query = regexp.QuoteMeta(query)
	}
	if options.IgnoreCase {
		query = "(?i)" + query
	}
	re, err := regexp.Compile(query)
	if err != nil {
		return SearchPage{}, fmt.Errorf("invalid regular expression: %w", err)
	}
	r, err := git.PlainOpen(root)
	if err != nil {
		return SearchPage{}, err
	}
	entries, err := nativeTreeFromRepository(ctx, r, revision, path, "")
	if err != nil {
		return SearchPage{}, err
	}
	page := SearchPage{Items: []map[string]any{}, Offset: options.Offset, Complete: true}
	var budget int64 = 64 << 20
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return SearchPage{}, err
		}
		file, err := nativeFileFromRepository(r, revision, entry.Path)
		if err != nil {
			return SearchPage{}, err
		}
		reader, err := file.Reader()
		if err != nil {
			return SearchPage{}, err
		}
		// Git's binary heuristic checks the initial 8000 bytes.
		buf := bufio.NewReaderSize(io.LimitReader(reader, budget+1), 8192)
		prefix, _ := buf.Peek(8000)
		if strings.ContainsRune(string(prefix), 0) {
			reader.Close()
			continue
		}
		line := 0
		for {
			text, readErr := buf.ReadString('\n')
			if len(text) > 0 {
				line++
				if int64(len(text)) > budget {
					page.Complete = false
					page.Truncated = true
					break
				}
				budget -= int64(len(text))
				text = strings.TrimSuffix(text, "\n")
				if re.MatchString(text) {
					if page.ObservedMatches >= options.Offset && len(page.Items) < options.Limit {
						item := map[string]any{"path": displayScopedPath(directory, entry.Path), "line": line, "text": text}
						if len(text) > 4096 {
							end := 4096
							for end > 0 && text[end]&0xc0 == 0x80 {
								end--
							}
							item["text"] = text[:end]
							item["text_truncated"] = true
						}
						page.Items = append(page.Items, item)
					}
					page.ObservedMatches++
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					reader.Close()
					return SearchPage{}, readErr
				}
				break
			}
			if err := ctx.Err(); err != nil {
				reader.Close()
				return SearchPage{}, err
			}
		}
		reader.Close()
		if page.Truncated {
			break
		}
	}
	page.NextOffset = page.Offset + len(page.Items)
	page.Done = page.Complete && page.NextOffset >= page.ObservedMatches
	return page, nil
}
