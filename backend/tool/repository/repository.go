package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"opsseek/backend/config"
	"opsseek/backend/gitrepo"
)

type ConnectionInput struct {
	Path           string        `json:"path,omitempty"`
	URL            string        `json:"url,omitempty"`
	Branch         string        `json:"branch,omitempty"`
	Username       string        `json:"username,omitempty"`
	Password       string        `json:"password,omitempty"`
	TimeoutSeconds int           `json:"timeout_seconds,omitempty"`
	StorageBackend string        `json:"storage_backend,omitempty"`
	StorageKey     string        `json:"storage_key,omitempty"`
	S3Endpoint     string        `json:"s3_endpoint,omitempty"`
	S3Bucket       string        `json:"s3_bucket,omitempty"`
	S3Prefix       string        `json:"s3_prefix,omitempty"`
	S3AccessKey    string        `json:"s3_access_key,omitempty"`
	S3SecretKey    string        `json:"s3_secret_key,omitempty"`
	S3UseSSL       bool          `json:"s3_use_ssl,omitempty"`
	Postgres       *pgxpool.Pool `json:"-"`
	// Directory scopes application source queries to a relative subdirectory
	// of the repository. It is supplied by the server-side application binding.
	Directory string `json:"-"`
	// PinnedRevision is set by the execution-scoped repository snapshot. Model
	// supplied revisions cannot move a source-level diagnosis to another commit.
	PinnedRevision string `json:"-"`
}

type BranchRevisions struct {
	Branch      string           `json:"branch"`
	Commits     []string         `json:"commits"`
	CommitTimes map[string]int64 `json:"commit_times"`
	IsDefault   bool             `json:"is_default"`
}

// Preview is the repository state needed by the repository resource UI.
type Preview struct {
	Branch   string
	CommitID string
	Branches []BranchRevisions
	Entries  []TreeEntry
}
type ToolInfo struct{ Name, Description string }

// Snapshot lazily materializes one repository for one Engine execution. The
// first repository tool pins the source; later tools reuse the same checkout.
type Snapshot struct {
	input    ConnectionInput
	once     sync.Once
	mu       sync.Mutex
	prepared ConnectionInput
	cleanup  func() error
	err      error
}

func NewSnapshot(input ConnectionInput) *Snapshot { return &Snapshot{input: input} }

func (s *Snapshot) resolve(ctx context.Context) (ConnectionInput, error) {
	s.once.Do(func() {
		prepared, cleanup, err := Prepare(ctx, s.input)
		s.mu.Lock()
		s.prepared, s.cleanup, s.err = prepared, cleanup, err
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepared, s.err
}

func (s *Snapshot) Close() error {
	s.mu.Lock()
	cleanup := s.cleanup
	s.cleanup = nil
	s.mu.Unlock()
	if cleanup == nil {
		return nil
	}
	return cleanup()
}

func (s *Snapshot) Branches(ctx context.Context) ([]string, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Branches(ctx, in)
}

func (s *Snapshot) Status(ctx context.Context) (map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Status(ctx, in)
}

func (s *Snapshot) Tree(ctx context.Context, path string, limit int, revision string) ([]TreeEntry, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return TreeAt(ctx, in, path, limit, revision)
}

func (s *Snapshot) TreePage(ctx context.Context, path string, offset, limit int, revision string) (TreePage, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return TreePage{}, err
	}
	return TreePageAt(ctx, in, path, offset, limit, revision)
}

func (s *Snapshot) File(ctx context.Context, path string, max int64, revision string) (string, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return "", err
	}
	return FileAt(ctx, in, path, max, revision)
}

func (s *Snapshot) FileRange(ctx context.Context, path string, offset, max int64, revision string) (FileRange, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return FileRange{}, err
	}
	return FileRangeAt(ctx, in, path, offset, max, revision)
}

func (s *Snapshot) Search(ctx context.Context, query string, limit int, revision string) ([]map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return SearchAt(ctx, in, query, limit, revision)
}

func (s *Snapshot) SearchPage(ctx context.Context, options SearchOptions) (SearchPage, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return SearchPage{}, err
	}
	return SearchPageWithOptionsAt(ctx, in, options)
}

func (s *Snapshot) Metadata(ctx context.Context) (map[string]any, error) {
	in, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return Metadata(ctx, in)
}

var tools = []ToolInfo{
	{"repository_branches", "List repository branches."},
	{"repository_status", "Read the resolved repository revision and worktree status when applicable."},
	{"repository_tree", "List tracked files at the execution-pinned revision, optionally under a repository-relative directory. Results are paginated; follow next_offset until done=true before treating the listing as complete."},
	{"repository_file", "Read a tracked text file at the execution-pinned revision by byte range. Follow next_offset until done=true before treating the file as fully read; a truncated range is not the whole file."},
	{"repository_search", "Search tracked text files at the execution-pinned revision using literal text by default or grep-style extended regular expressions when requested. Results may be paginated or truncated; use next_offset to continue until done=true and complete=true before treating the search as complete."},
	{"repository_metadata", "Read repository remotes and recent commits at the resolved revision."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }

// Prepare opens one immutable, execution-scoped repository snapshot. The
// returned input points at the materialized local repository, so every tool in
// the execution reads the same files even when the remote repository changes.
func Prepare(ctx context.Context, in ConnectionInput) (ConnectionInput, func() error, error) {
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	sourceInput := configuredBundleStorage(in)
	root, cleanup, selectedRevision, err := openRepository(ctx, sourceInput, strings.TrimSpace(in.PinnedRevision))
	if err != nil {
		return ConnectionInput{}, func() error { return nil }, err
	}
	if strings.EqualFold(strings.TrimSpace(sourceInput.StorageBackend), "local") && strings.TrimSpace(sourceInput.StorageKey) != "" {
		dir, cloneErr := os.MkdirTemp("", "opsseek-repository-snapshot-")
		if cloneErr != nil {
			cleanup()
			return ConnectionInput{}, func() error { return nil }, cloneErr
		}
		snapshotRoot := filepath.Join(dir, "repo.git")
		if cloneErr := cloneRepository(ctx, ConnectionInput{}, root, snapshotRoot, true); cloneErr != nil {
			cleanup()
			_ = os.RemoveAll(dir)
			return ConnectionInput{}, func() error { return nil }, fmt.Errorf("repository snapshot: %w", cloneErr)
		}
		previousCleanup := cleanup
		root = snapshotRoot
		cleanup = func() {
			previousCleanup()
			_ = os.RemoveAll(dir)
		}
	}
	prepared := in
	prepared.Path = root
	prepared.URL = ""
	prepared.StorageBackend = ""
	prepared.StorageKey = ""
	prepared.S3Endpoint = ""
	prepared.S3Bucket = ""
	prepared.S3Prefix = ""
	prepared.S3AccessKey = ""
	prepared.S3SecretKey = ""
	prepared.S3UseSSL = false
	if selectedRevision != "" {
		prepared.PinnedRevision = selectedRevision
	} else if revision := snapshotRevision(ctx, prepared, root); revision != "" {
		prepared.PinnedRevision = revision
	}
	return prepared, func() error {
		cleanup()
		return nil
	}, nil
}

func snapshotRevision(ctx context.Context, in ConnectionInput, root string) string {
	return nativeSnapshotRevision(root)
}

func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"path": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"}, "branch": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}, "storage_backend": map[string]any{"type": "string", "enum": []string{"local", "postgres", "s3"}}, "storage_key": map[string]any{"type": "string"}, "s3_endpoint": map[string]any{"type": "string"}, "s3_bucket": map[string]any{"type": "string"}, "s3_prefix": map[string]any{"type": "string"}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}
func openRepository(ctx context.Context, in ConnectionInput, revision string) (string, func(), string, error) {
	in = configuredBundleStorage(in)
	if len(revision) > 256 || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\r\n:") || strings.Contains(revision, "..") || strings.Contains(revision, "@{") {
		return "", func() {}, "", errors.New("invalid repository revision")
	}
	root := strings.TrimSpace(in.Path)
	cleanup := func() {}
	backend := strings.ToLower(strings.TrimSpace(in.StorageBackend))
	if backend == "s3" {
		var err error
		root, cleanup, err = materializeS3(ctx, in)
		if err != nil {
			return "", func() {}, "", err
		}
	}
	if backend == "postgres" {
		var err error
		root, cleanup, err = materializePostgres(ctx, in)
		if err != nil {
			return "", func() {}, "", err
		}
	}
	if root == "" && strings.TrimSpace(in.URL) != "" {
		parsed, err := transport.NewEndpoint(strings.TrimSpace(in.URL))
		if err != nil {
			return "", func() {}, "", errors.New("invalid repository URL")
		}
		if (parsed.Protocol == "http" || parsed.Protocol == "https") && (parsed.User != "" || parsed.Password != "") {
			return "", func() {}, "", errors.New("repository URL must not contain embedded credentials")
		}
		d, err := os.MkdirTemp("", "opsseek-repository-")
		if err != nil {
			return "", func() {}, "", err
		}
		cleanup = func() { _ = os.RemoveAll(d) }
		if err := cloneRepository(ctx, in, in.URL, d, false); err != nil {
			cleanup()
			return "", func() {}, "", fmt.Errorf("clone repository: %w", err)
		}
		root = d
	}
	if root == "" {
		return "", func() {}, "", errors.New("repository path or url is required")
	}
	clean, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		cleanup()
		return "", func() {}, "", err
	}
	return clean, cleanup, revision, nil
}

// Bundle storage is selected when the process starts. Git resources have no
// storage key and continue to use their remote URL, so their connection input
// is left untouched.
func configuredBundleStorage(in ConnectionInput) ConnectionInput {
	if strings.TrimSpace(in.StorageKey) == "" {
		return in
	}
	backend := strings.TrimSpace(os.Getenv("OPSK_STORAGE_BACKEND"))
	if backend == "" {
		backend = "postgres"
	}
	in.StorageBackend = strings.ToLower(backend)
	if in.StorageBackend == "local" {
		if root := strings.TrimSpace(os.Getenv("OPSK_REPOSITORY_LOCAL_ROOT")); root != "" {
			in.Path = filepath.Join(root, in.StorageKey)
		}
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_ENDPOINT")); value != "" {
		in.S3Endpoint = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_BUCKET")); value != "" {
		in.S3Bucket = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_ACCESS_KEY")); value != "" {
		in.S3AccessKey = value
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_SECRET_KEY")); value != "" {
		in.S3SecretKey = value
	}
	if prefix := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_PREFIX")); prefix != "" {
		in.S3Prefix = prefix
	}
	if value := strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_USE_SSL")); value != "" {
		in.S3UseSSL = strings.EqualFold(value, "true") || value == "1"
	}
	return in
}

func repositoryContext(ctx context.Context, in ConnectionInput) (context.Context, context.CancelFunc) {
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func materializePostgres(ctx context.Context, in ConnectionInput) (string, func(), error) {
	pool := in.Postgres
	closePool := func() {}
	if pool == nil {
		cfg, configErr := config.Load()
		if configErr != nil || strings.TrimSpace(in.StorageKey) == "" {
			return "", func() {}, errors.New("PostgreSQL repository database configuration and storage key are required")
		}
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return "", func() {}, err
		}
		closePool = pool.Close
	}
	if strings.TrimSpace(in.StorageKey) == "" {
		closePool()
		return "", func() {}, errors.New("PostgreSQL repository storage key is required")
	}
	var bundle []byte
	err := pool.QueryRow(ctx, `SELECT bundle FROM repository_bundles WHERE resource_id=$1::uuid`, in.StorageKey).Scan(&bundle)
	if err != nil {
		closePool()
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp("", "opsseek-repository-postgres-")
	if err != nil {
		closePool()
		return "", func() {}, err
	}
	cleanup := func() { closePool(); _ = os.RemoveAll(dir) }
	bundlePath := filepath.Join(dir, "repository.bundle")
	if err := os.WriteFile(bundlePath, bundle, 0600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	repo := filepath.Join(dir, "repo.git")
	if err := gitrepo.Init(repo); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := gitrepo.ImportBundle(ctx, repo, bundlePath, true); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return repo, cleanup, nil
}
func materializeS3(ctx context.Context, in ConnectionInput) (string, func(), error) {
	if in.S3Endpoint == "" {
		in.S3Endpoint = os.Getenv("OPSK_STORAGE_S3_ENDPOINT")
	}
	if in.S3Bucket == "" {
		in.S3Bucket = os.Getenv("OPSK_STORAGE_S3_BUCKET")
	}
	if in.S3AccessKey == "" {
		in.S3AccessKey = os.Getenv("OPSK_STORAGE_S3_ACCESS_KEY")
	}
	if in.S3SecretKey == "" {
		in.S3SecretKey = os.Getenv("OPSK_STORAGE_S3_SECRET_KEY")
	}
	if in.S3Endpoint == "" || in.S3Bucket == "" || in.StorageKey == "" {
		return "", func() {}, errors.New("S3 repository endpoint, bucket and storage key are required")
	}
	endpoint := strings.TrimSpace(in.S3Endpoint)
	if parsed, e := url.Parse(endpoint); e == nil && parsed.Host != "" {
		endpoint = parsed.Host
		if parsed.Scheme == "https" {
			in.S3UseSSL = true
		}
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(in.S3AccessKey, in.S3SecretKey, ""), Secure: in.S3UseSSL, BucketLookup: minio.BucketLookupPath})
	if err != nil {
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp("", "opsseek-repository-s3-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	bundle := filepath.Join(dir, "repository.bundle")
	obj, err := client.GetObject(ctx, in.S3Bucket, in.StorageKey, minio.GetObjectOptions{})
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	f, err := os.Create(bundle)
	if err == nil {
		_, err = io.Copy(f, obj)
		_ = f.Close()
	}
	_ = obj.Close()
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	repo := filepath.Join(dir, "repo.git")
	if err = gitrepo.Init(repo); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err = gitrepo.ImportBundle(ctx, repo, bundle, true); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return repo, cleanup, nil
}
func Branches(ctx context.Context, in ConnectionInput) ([]string, error) {
	revisions, err := RepositoryRevisions(ctx, in, 1)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, r := range revisions {
		result = append(result, r.Branch)
	}
	sort.Strings(result)
	return result, nil
}

func RepositoryRevisions(ctx context.Context, in ConnectionInput, limit int) ([]BranchRevisions, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, _, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	return nativeRevisions(ctx, r, limit)
}

func ResolveBranchCommit(ctx context.Context, in ConnectionInput, branch, commitID string) (string, error) {
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, _, err := openRepository(ctx, in, "")
	if err != nil {
		return "", err
	}
	defer cleanup()
	r, err := git.PlainOpen(root)
	if err != nil {
		return "", err
	}
	return resolveBranchCommitInRepository(ctx, r, branch, commitID)
}

func resolveBranchCommitInRepository(ctx context.Context, r *git.Repository, branch, commitID string) (string, error) {
	branch = strings.TrimSpace(branch)
	commitID = strings.TrimSpace(commitID)
	if branch == "" || strings.HasPrefix(branch, "-") || plumbing.NewBranchReferenceName(branch).Validate() != nil {
		return "", errors.New("invalid repository branch")
	}
	if !validCommitID(commitID) || len(commitID) != 40 {
		return "", errors.New("invalid or unsupported commit ID (SHA-1 required)")
	}
	head, err := resolveCommit(r, "refs/heads/"+branch)
	if err != nil {
		head, err = resolveCommit(r, "refs/remotes/origin/"+branch)
	}
	if err != nil {
		return "", errors.New("repository branch not found")
	}
	commit, err := r.CommitObject(plumbing.NewHash(commitID))
	if err != nil {
		return "", errors.New("repository commit not found")
	}
	iter, err := r.Log(&git.LogOptions{From: head.Hash})
	if err != nil {
		return "", err
	}
	defer iter.Close()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		c, err := iter.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if c.Hash == commit.Hash {
			return commit.Hash.String(), nil
		}
	}
	return "", errors.New("commit is not available on the selected branch")
}

// PreviewRepository opens and reads the repository once for the resource preview.
func PreviewRepository(ctx context.Context, in ConnectionInput, branch, commitID string) (Preview, error) {
	path, err := scopedPath(in.Directory, "")
	if err != nil {
		return Preview{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, _, err := openRepository(ctx, in, "")
	if err != nil {
		return Preview{}, err
	}
	defer cleanup()
	r, err := git.PlainOpen(root)
	if err != nil {
		return Preview{}, err
	}
	branches, err := nativeRevisions(ctx, r, 20)
	if err != nil {
		return Preview{}, err
	}
	branch = strings.TrimSpace(branch)
	commitID = strings.TrimSpace(commitID)
	if branch != "" {
		found := false
		for _, revision := range branches {
			if revision.Branch == branch {
				found = true
				break
			}
		}
		if !found {
			return Preview{}, errors.New("repository branch not found")
		}
	}
	if commitID != "" && branch == "" {
		return Preview{}, errors.New("repository branch is required for commit selection")
	}
	if branch == "" {
		for _, revision := range branches {
			if revision.IsDefault && len(revision.Commits) > 0 {
				branch, commitID = revision.Branch, revision.Commits[0]
				break
			}
		}
		if branch == "" && len(branches) > 0 && len(branches[0].Commits) > 0 {
			branch, commitID = branches[0].Branch, branches[0].Commits[0]
		}
	}
	if commitID == "" {
		for _, revision := range branches {
			if revision.Branch == branch && len(revision.Commits) > 0 {
				commitID = revision.Commits[0]
				break
			}
		}
	}
	if branch == "" || commitID == "" {
		return Preview{}, errors.New("repository has no selectable branch commit")
	}
	commitID, err = resolveBranchCommitInRepository(ctx, r, branch, commitID)
	if err != nil {
		return Preview{}, err
	}
	entries, err := nativeTreeFromRepository(ctx, r, commitID, path, in.Directory)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Branch: branch, CommitID: commitID, Branches: branches, Entries: entries}, nil
}

func ParseEndpointSelection(subtype, remote, endpoint string) (string, string) {
	selection := strings.TrimSpace(endpoint)
	if strings.EqualFold(subtype, "Git") {
		prefix := strings.TrimSpace(remote) + " ["
		if strings.HasPrefix(selection, prefix) && strings.HasSuffix(selection, "]") {
			selection = strings.TrimSuffix(strings.TrimPrefix(selection, prefix), "]")
			branch, commitID, ok := strings.Cut(selection, "·")
			if ok && validCommitID(commitID) {
				return branch, commitID
			}
			return "", ""
		}
		// Accept the pre-0094 representation while reading resources during rollout.
		legacy := strings.TrimSpace(remote) + " ("
		if strings.HasPrefix(selection, legacy) && strings.HasSuffix(selection, ")") {
			selection = strings.TrimSuffix(strings.TrimPrefix(selection, legacy), ")")
		}
	} else if open := strings.LastIndex(selection, " ["); open >= 0 && strings.HasSuffix(selection, "]") {
		selection = strings.TrimSuffix(selection[open+2:], "]")
		branch, commitID, ok := strings.Cut(selection, "·")
		if ok && validCommitID(commitID) {
			return branch, commitID
		}
		return "", ""
	}
	branch, commitID, ok := strings.Cut(selection, ":")
	if !ok || branch == "" || !validCommitID(commitID) {
		return "", ""
	}
	return branch, commitID
}

func validCommitID(commitID string) bool {
	if len(commitID) != 40 && len(commitID) != 64 {
		return false
	}
	for _, char := range commitID {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

func Checkout(ctx context.Context, in ConnectionInput, branch string) (map[string]any, error) {
	if branch == "" || plumbing.NewBranchReferenceName(branch).Validate() != nil {
		return nil, errors.New("invalid branch")
	}
	root, cleanup, _, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	ref := plumbing.NewBranchReferenceName(branch)
	if _, err = r.Reference(ref, true); err != nil {
		return nil, err
	}
	if err = r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, ref)); err != nil {
		return nil, err
	}
	return map[string]any{"branch": branch}, nil
}

func Status(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, selected, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	revision := readRevision(ctx, in, root, selected)
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	commit, err := resolveCommit(r, revision)
	if err != nil {
		return nil, err
	}
	revision = commit.Hash.String()
	branch := ""
	if head, err := r.Head(); err == nil && head.Hash() == commit.Hash && head.Name().IsBranch() {
		branch = head.Name().Short()
	}
	if branch == "" {
		refs, err := branchRefs(r)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if ref.Hash() == commit.Hash {
				branch = strings.TrimPrefix(ref.Name().Short(), "origin/")
				break
			}
		}
	}
	worktree, err := r.Worktree()
	bare := errors.Is(err, git.ErrIsBareRepository)
	if err != nil && !bare {
		return nil, err
	}
	result := map[string]any{"head": revision, "branch": branch, "resolved_revision": revision, "has_worktree": !bare, "clean": nil, "changed_files": nil, "worktree_status": "not_applicable"}
	if !bare {
		status, err := worktree.Status()
		if err != nil {
			return nil, err
		}
		changed := 0
		for _, s := range status {
			if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
				changed++
			}
		}
		result["clean"] = changed == 0
		result["changed_files"] = changed
		result["worktree_status"] = "available"
	}
	return result, nil
}

func TreePageAt(ctx context.Context, in ConnectionInput, path string, offset, limit int, revision string) (TreePage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	entries, err := treeAt(ctx, in, path, 0, revision)
	if err != nil {
		return TreePage{}, err
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return TreePage{Items: entries[offset:end], Offset: offset, NextOffset: end, Total: len(entries), Done: end == len(entries)}, nil
}

func FileRangeAt(ctx context.Context, in ConnectionInput, path string, offset int64, max int64, revision string) (FileRange, error) {
	if offset < 0 {
		return FileRange{}, errors.New("file offset must not be negative")
	}
	if max <= 0 || max > 1<<20 {
		max = 1 << 20
	}
	cleanPath, err := scopedPath(in.Directory, path)
	if err != nil {
		return FileRange{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return FileRange{}, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	file, err := nativeFile(root, selectedRevision, cleanPath)
	if err != nil {
		return FileRange{}, err
	}
	totalBytes := file.Size
	if offset >= totalBytes {
		return FileRange{Path: displayScopedPath(in.Directory, cleanPath), Offset: totalBytes, NextOffset: totalBytes, TotalBytes: totalBytes, Done: true}, nil
	}
	stdout, err := file.Reader()
	if err != nil {
		return FileRange{}, err
	}
	defer stdout.Close()
	if _, err := io.CopyN(io.Discard, stdout, offset); err != nil {
		return FileRange{}, err
	}
	chunk := make([]byte, int(max)+4)
	n, readErr := io.ReadFull(stdout, chunk)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return FileRange{}, readErr
	}
	chunk = chunk[:n]
	start := 0
	for start < len(chunk) && chunk[start]&0xC0 == 0x80 {
		start++
	}
	end := start + int(max)
	if end > len(chunk) {
		end = len(chunk)
	}
	for end < len(chunk) && end > start && chunk[end]&0xC0 == 0x80 {
		end--
	}
	content := chunk[start:end]
	if !utf8ish(content) || !utf8.Valid(content) {
		return FileRange{}, errors.New("file is not UTF-8 text")
	}
	nextOffset := offset + int64(end)
	done := nextOffset >= totalBytes
	return FileRange{Path: displayScopedPath(in.Directory, cleanPath), Content: string(content), Offset: offset + int64(start), NextOffset: nextOffset, TotalBytes: totalBytes, Done: done}, nil
}

func SearchPageAt(ctx context.Context, in ConnectionInput, query, path string, offset, limit int, revision string) (SearchPage, error) {
	return SearchPageWithOptionsAt(ctx, in, SearchOptions{Pattern: query, Path: path, Offset: offset, Limit: limit, Revision: revision})
}

func SearchPageWithOptionsAt(ctx context.Context, in ConnectionInput, options SearchOptions) (SearchPage, error) {
	query, path, offset, limit, revision := options.Pattern, options.Path, options.Offset, options.Limit, options.Revision
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 256 {
		return SearchPage{}, errors.New("query is required and must be at most 256 characters")
	}
	path, err := scopedPath(in.Directory, path)
	if err != nil {
		return SearchPage{}, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return SearchPage{}, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	options.Pattern, options.Offset, options.Limit = query, offset, limit
	return nativeSearch(ctx, root, selectedRevision, path, in.Directory, options)
}

func readRevision(ctx context.Context, in ConnectionInput, root, selected string) string {
	if in.PinnedRevision != "" {
		return in.PinnedRevision
	}
	if selected != "" {
		return selected
	}
	if branch := strings.TrimSpace(in.Branch); branch != "" {
		return branch
	}
	if revision := snapshotRevision(ctx, in, root); revision != "" {
		return revision
	}
	return "HEAD"
}

func Tree(ctx context.Context, in ConnectionInput, path string, limit int) ([]TreeEntry, error) {
	return TreeAt(ctx, in, path, limit, "")
}

func TreeAt(ctx context.Context, in ConnectionInput, path string, limit int, revision string) ([]TreeEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return treeAt(ctx, in, path, limit, revision)
}

// TreeAll returns every file in the selected repository path for UI previews.
func TreeAll(ctx context.Context, in ConnectionInput, path string) ([]TreeEntry, error) {
	page, err := TreeAllAt(ctx, in, path, "")
	return page.Items, err
}

// TreeAllAt returns a complete repository file inventory from one resolved revision.
func TreeAllAt(ctx context.Context, in ConnectionInput, path, revision string) (TreePage, error) {
	entries, err := treeAt(ctx, in, path, 0, revision)
	if err != nil {
		return TreePage{}, err
	}
	return TreePage{Items: entries, Offset: 0, NextOffset: len(entries), Total: len(entries), Done: true}, nil
}

func treeAt(ctx context.Context, in ConnectionInput, path string, limit int, revision string) ([]TreeEntry, error) {
	path, err := scopedPath(in.Directory, path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, err := openRepository(ctx, in, revision)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	entries, err := nativeTree(ctx, root, selectedRevision, path, in.Directory)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}
func File(ctx context.Context, in ConnectionInput, path string, max int64) (string, error) {
	return FileAt(ctx, in, path, max, "")
}

func FileAt(ctx context.Context, in ConnectionInput, path string, max int64, revision string) (string, error) {
	path, e := scopedPath(in.Directory, path)
	if e != nil {
		return "", e
	}
	if max <= 0 || max > 4<<20 {
		max = 4 << 20
	}
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	if in.PinnedRevision != "" {
		revision = ""
	}
	root, cleanup, selectedRevision, e := openRepository(ctx, in, revision)
	if e != nil {
		return "", e
	}
	defer cleanup()
	selectedRevision = readRevision(ctx, in, root, selectedRevision)
	file, e := nativeFile(root, selectedRevision, path)
	if e != nil {
		return "", e
	}
	if file.Size > max {
		return "", fmt.Errorf("file exceeds %d bytes", max)
	}
	reader, e := file.Reader()
	if e != nil {
		return "", e
	}
	defer reader.Close()
	out, e := io.ReadAll(io.LimitReader(reader, max+1))
	if e != nil {
		return "", e
	}
	if int64(len(out)) > max {
		return "", fmt.Errorf("file exceeds %d bytes", max)
	}
	if !utf8ish(out) {
		return "", errors.New("file is not UTF-8 text")
	}
	return string(out), nil
}
func Search(ctx context.Context, in ConnectionInput, q string, limit int) ([]map[string]any, error) {
	return SearchAt(ctx, in, q, limit, "")
}

func SearchAt(ctx context.Context, in ConnectionInput, q string, limit int, revision string) ([]map[string]any, error) {
	page, err := SearchPageAt(ctx, in, q, "", 0, limit, revision)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

type limitedBuffer struct {
	bytes.Buffer
	max       int
	Truncated bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.max - b.Len()
	if remaining > 0 {
		if len(value) > remaining {
			_, _ = b.Buffer.Write(value[:remaining])
			b.Truncated = true
		} else {
			_, _ = b.Buffer.Write(value)
		}
	} else if len(value) > 0 {
		b.Truncated = true
	}
	return len(value), nil
}
func Metadata(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	in = configuredBundleStorage(in)
	ctx, cancel := repositoryContext(ctx, in)
	defer cancel()
	root, cleanup, selected, err := openRepository(ctx, in, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	revision := readRevision(ctx, in, root, selected)
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, err
	}
	commit, err := resolveCommit(r, revision)
	if err != nil {
		return nil, err
	}
	remotes, err := r.Remotes()
	if err != nil {
		return nil, err
	}
	var rem []string
	for _, remote := range remotes {
		for _, u := range remote.Config().URLs {
			rem = append(rem, remote.Config().Name+"\t"+u+" (fetch)", remote.Config().Name+"\t"+u+" (push)")
		}
	}
	sort.Strings(rem)
	iter, err := r.Log(&git.LogOptions{From: commit.Hash, Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	logs := []string{}
	for len(logs) < 20 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c, err := iter.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		logs = append(logs, fmt.Sprintf("%s\t%s\t%s\t%s", c.Hash, c.Author.When.Format(time.RFC3339), c.Author.Name, strings.SplitN(c.Message, "\n", 2)[0]))
	}
	return map[string]any{"remotes": strings.Join(rem, "\n"), "has_remote": len(rem) > 0, "resolved_revision": commit.Hash.String(), "commits": logs}, nil
}
func safePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) || strings.ContainsRune(p, '\x00') {
		return "", errors.New("path must be relative")
	}
	c := filepath.ToSlash(filepath.Clean(p))
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", errors.New("path escapes repository")
	}
	return c, nil
}

func scopedPath(directory, path string) (string, error) {
	directory, err := safePath(directory)
	if err != nil {
		return "", err
	}
	path, err = safePath(path)
	if err != nil {
		return "", err
	}
	if directory == "" {
		return path, nil
	}
	if path == "" {
		return directory, nil
	}
	return filepath.ToSlash(filepath.Join(directory, path)), nil
}

func displayScopedPath(directory, path string) string {
	directory = strings.Trim(strings.TrimSpace(filepath.ToSlash(directory)), "/")
	path = filepath.ToSlash(path)
	if directory == "" {
		return path
	}
	return strings.TrimPrefix(path, directory+"/")
}
func utf8ish(b []byte) bool { return bytes.IndexByte(b, 0) < 0 }
