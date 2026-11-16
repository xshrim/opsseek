// Package gitrepo implements repository storage without external executables.
package gitrepo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
)

type Bundle struct {
	Refs          []*plumbing.Reference
	Prerequisites []plumbing.Hash
}

func readHeader(reader *bufio.Reader) (Bundle, error) {
	var bundle Bundle
	line, err := reader.ReadString('\n')
	if err != nil || (line != "# v2 git bundle\n" && line != "# v3 git bundle\n") {
		return bundle, errors.New("invalid Git bundle header (expected v2 or v3)")
	}
	for size := len(line); ; {
		lineBytes, readErr := reader.ReadSlice('\n')
		line, err = string(lineBytes), readErr
		size += len(line)
		if err != nil || size > 1<<20 {
			return bundle, errors.New("invalid or oversized Git bundle header")
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "@") {
			if line != "@object-format=sha1" {
				return bundle, fmt.Errorf("unsupported bundle capability %q", line)
			}
			continue
		}
		prerequisite := strings.HasPrefix(line, "-")
		line = strings.TrimPrefix(line, "-")
		hash, name, ok := strings.Cut(line, " ")
		if !ok || !plumbing.IsHash(hash) {
			return bundle, errors.New("invalid bundle object ID")
		}
		if prerequisite {
			bundle.Prerequisites = append(bundle.Prerequisites, plumbing.NewHash(hash))
			continue
		}
		ref := plumbing.ReferenceName(name)
		if name != "HEAD" {
			if !strings.HasPrefix(name, "refs/") || ref.Validate() != nil {
				return bundle, errors.New("invalid bundle reference")
			}
		}
		bundle.Refs = append(bundle.Refs, plumbing.NewHashReference(ref, plumbing.NewHash(hash)))
	}
	if len(bundle.Refs) == 0 {
		return bundle, errors.New("bundle contains no references")
	}
	return bundle, nil
}

func BundleHeader(path string) (Bundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return Bundle{}, err
	}
	defer f.Close()
	return readHeader(bufio.NewReader(f))
}

func Init(path string) error {
	_, err := git.PlainInit(path, true)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return nil
	}
	return err
}

// ImportBundle validates prerequisites and the pack before publishing any refs.
func ImportBundle(ctx context.Context, path, bundlePath string, publish bool) error {
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	f, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	bundle, err := readHeader(reader)
	if err != nil {
		return err
	}
	for _, hash := range bundle.Prerequisites {
		if _, err := r.CommitObject(hash); err != nil {
			return fmt.Errorf("missing bundle prerequisite %s: %w", hash, err)
		}
	}
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(&contextReader{ctx, reader}), r.Storer)
	if err != nil {
		return fmt.Errorf("read bundle pack: %w", err)
	}
	if _, err = parser.Parse(); err != nil {
		return fmt.Errorf("invalid bundle pack: %w", err)
	}
	for _, ref := range bundle.Refs {
		if _, err := r.Storer.EncodedObject(plumbing.AnyObject, ref.Hash()); err != nil {
			return fmt.Errorf("bundle reference %s: %w", ref.Name(), err)
		}
		if ref.Name().IsBranch() {
			if _, err := r.CommitObject(ref.Hash()); err != nil {
				return fmt.Errorf("bundle branch %s does not point to a commit: %w", ref.Name(), err)
			}
		}
	}
	if !publish {
		return nil
	}
	for _, ref := range bundle.Refs {
		if ref.Name() == plumbing.HEAD {
			continue
		}
		if err := r.Storer.SetReference(ref); err != nil {
			return err
		}
	}
	return nil
}

func ExportBundle(ctx context.Context, path, target string) (err error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if _, err = io.WriteString(f, "# v2 git bundle\n"); err != nil {
		return err
	}
	refs, err := r.References()
	if err != nil {
		return err
	}
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference || !strings.HasPrefix(ref.Name().String(), "refs/") {
			return nil
		}
		_, err := fmt.Fprintf(f, "%s %s\n", ref.Hash(), ref.Name())
		return err
	})
	if err != nil {
		return err
	}
	if _, err = io.WriteString(f, "\n"); err != nil {
		return err
	}
	objects, err := r.Storer.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return err
	}
	defer objects.Close()
	var hashes []plumbing.Hash
	err = objects.ForEach(func(obj plumbing.EncodedObject) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		hashes = append(hashes, obj.Hash())
		return nil
	})
	if err != nil {
		return err
	}
	_, err = packfile.NewEncoder(&contextWriter{ctx, f}, r.Storer, false).Encode(hashes, 0)
	return err
}

type contextReader struct {
	context.Context
	io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

type contextWriter struct {
	context.Context
	io.Writer
}

func (w *contextWriter) Write(p []byte) (int, error) {
	if err := w.Err(); err != nil {
		return 0, err
	}
	return w.Writer.Write(p)
}
