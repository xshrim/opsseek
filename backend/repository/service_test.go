package repository

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"opskeeper/backend/resource"
)

type bundleTestResources struct {
	item    resource.Resource
	updated resource.Resource
}

func (r *bundleTestResources) Get(_ context.Context, id string) (resource.Resource, error) {
	if r.updated.ID == id {
		return r.updated, nil
	}
	return r.item, nil
}

func (r *bundleTestResources) Update(_ context.Context, _ string, input resource.UpdateInput) (resource.Resource, error) {
	r.updated = r.item
	r.updated.Config = *input.Config
	return r.updated, nil
}

func TestUploadBundleRecordsLatestCommitInEndpoint(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repoDir := filepath.Join(root, "source")
	if err := os.Mkdir(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repoDir, "init", "-b", "main")
	runGitTest(t, repoDir, "config", "user.name", "Test")
	runGitTest(t, repoDir, "config", "user.email", "test@example.test")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("bundle"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repoDir, "add", "README.md")
	runGitTest(t, repoDir, "commit", "-m", "initial")
	commitID := string(runGitTest(t, repoDir, "rev-parse", "HEAD"))
	bundlePath := filepath.Join(root, "repo.bundle")
	runGitTest(t, repoDir, "bundle", "create", bundlePath, "--all")
	bundle, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	resources := &bundleTestResources{item: resource.Resource{
		ID: "repository-id", Kind: "Repository", Subtype: "Bundle",
		Config: map[string]any{"default_branch": "main"},
	}}
	service := NewService(filepath.Join(root, "storage"), 1<<20, resources)
	out, err := service.UploadBundle(ctx, resources.item.ID, bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	wantEndpoint := "repository-id [main·" + commitID + "]"
	if out.CommitID != commitID || out.StorageBackend != "local" || resources.updated.Config["endpoint"] != wantEndpoint {
		t.Fatalf("upload = %+v, endpoint = %#v, want %s", out, resources.updated.Config["endpoint"], wantEndpoint)
	}
}

func TestRepositoryEndpointFormatsGitAndBundleRevisions(t *testing.T) {
	commit := "aabbccdd00112233445566778899aabbccddeeff"
	git := resource.Resource{ID: "git-id", Subtype: "Git", Config: map[string]any{"url": "https://git.example/team/repo.git"}}
	bundle := resource.Resource{ID: "bundle-id", Subtype: "Bundle"}
	if got := repositoryEndpoint(git, "release", commit); got != "https://git.example/team/repo.git [release·"+commit+"]" {
		t.Fatalf("Git endpoint = %q", got)
	}
	if got := repositoryEndpoint(bundle, "release", commit); got != "bundle-id [release·"+commit+"]" {
		t.Fatalf("Bundle endpoint = %q", got)
	}
}

func TestPreviewRevisionDoesNotPersistBundleSelection(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "repository-id")
	runGitTest(t, root, "init", "--bare", bare)
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, work, "init", "-b", "main")
	runGitTest(t, work, "config", "user.name", "Test")
	runGitTest(t, work, "config", "user.email", "test@example.test")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("selected"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, work, "add", ".")
	runGitTest(t, work, "commit", "-m", "selected")
	commitID := string(runGitTest(t, work, "rev-parse", "HEAD"))
	runGitTest(t, work, "remote", "add", "origin", bare)
	runGitTest(t, work, "push", "origin", "main")
	runGitTest(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	t.Setenv("OPSK_STORAGE_BACKEND", "local")
	t.Setenv("OPSK_REPOSITORY_LOCAL_ROOT", root)
	resources := &bundleTestResources{item: resource.Resource{
		ID: "repository-id", Kind: "Repository", Subtype: "Bundle",
		Config: map[string]any{"storage_key": "repository-id"},
	}}
	service := NewServiceWithStorage(StorageConfig{Backend: "local", Root: root}, 1<<20, resources)
	preview, err := service.PreviewRevision(context.Background(), "repository-id", "main", commitID)
	if err != nil || preview.CommitID != commitID {
		t.Fatalf("temporary preview = %+v, %v", preview, err)
	}
	if resources.updated.ID != "" {
		t.Fatal("preview persisted the selection")
	}
	if _, err := service.PreviewRevision(context.Background(), "repository-id", "missing", commitID); err == nil {
		t.Fatal("preview accepted missing branch")
	}
	if resources.updated.ID != "" {
		t.Fatalf("preview persisted resource config: %#v", resources.updated.Config)
	}
}

func runGitTest(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return bytes.TrimSpace(output)
}
