package gitrepo

import (
	byteutil "bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestBundleRoundTripWithoutExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ctx := context.Background()
	source := t.TempDir()
	r, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := w.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.test", When: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "repo.bundle")
	if err := ExportBundle(ctx, source, path); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := Init(target); err != nil {
		t.Fatal(err)
	}
	if err := ImportBundle(ctx, target, path, true); err != nil {
		t.Fatal(err)
	}
	imported, err := git.PlainOpen(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imported.CommitObject(hash); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v3 := byteutil.Replace(bytes, []byte("# v2 git bundle\n"), []byte("# v3 git bundle\n@object-format=sha1\n"), 1)
	if err := os.WriteFile(path, v3, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportBundle(ctx, target, path, true); err != nil {
		t.Fatalf("v3: %v", err)
	}
	unsupported := byteutil.Replace(v3, []byte("@object-format=sha1"), []byte("@object-format=sha256"), 1)
	if err := os.WriteFile(path, unsupported, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportBundle(ctx, target, path, true); err == nil || !strings.Contains(err.Error(), "unsupported bundle capability") {
		t.Fatalf("SHA-256 accepted: %v", err)
	}
	bytes[len(bytes)-1] ^= 1
	if err := os.WriteFile(path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportBundle(ctx, target, path, true); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt pack accepted: %v", err)
	}
}
