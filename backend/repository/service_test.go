package repository

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"opsseek/backend/gitrepo"
	"opsseek/backend/resource"
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
	// The API container's working directory is not itself a Git repository.
	t.Chdir(root)
	out, err := service.UploadBundle(ctx, resources.item.ID, bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	wantEndpoint := "repository-id [main·" + commitID + "]"
	if out.CommitID != commitID || out.StorageBackend != "local" || resources.updated.Config["endpoint"] != wantEndpoint {
		t.Fatalf("upload = %+v, endpoint = %#v, want %s", out, resources.updated.Config["endpoint"], wantEndpoint)
	}
	runGitTest(t, repoDir, "commit", "--allow-empty", "-m", "update")
	nextCommit := string(runGitTest(t, repoDir, "rev-parse", "HEAD"))
	incrementalPath := filepath.Join(root, "incremental.bundle")
	runGitTest(t, repoDir, "bundle", "create", incrementalPath, commitID+"..main")
	incremental, err := os.ReadFile(incrementalPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UploadBundle(ctx, resources.item.ID, bytes.NewReader(incremental), int64(len(incremental)))
	if err != nil {
		t.Fatalf("upload incremental bundle to existing repository: %v", err)
	}
	if updated.CommitID != nextCommit || len(updated.ArchivedRefs) == 0 {
		t.Fatalf("existing repository update = %+v", updated)
	}
	if resources.updated.Config["endpoint"] != wantEndpoint || resources.updated.Config["default_branch"] != "main" {
		t.Fatalf("upload changed saved selection: %#v", resources.updated.Config)
	}
	t.Setenv("OPSK_STORAGE_BACKEND", "local")
	t.Setenv("OPSK_REPOSITORY_LOCAL_ROOT", filepath.Join(root, "storage"))
	preview, err := service.PreviewRevision(ctx, resources.item.ID, updated.DefaultBranch, updated.CommitID)
	if err != nil || preview.CommitID != nextCommit {
		t.Fatalf("uploaded revision preview = %+v, err=%v", preview, err)
	}
	currentPreview, err := service.Preview(ctx, resources.item.ID)
	if err != nil || currentPreview.CommitID != commitID {
		t.Fatalf("saved revision preview = %+v, err=%v", currentPreview, err)
	}
	runGitTest(t, repoDir, "checkout", "-b", "release")
	runGitTest(t, repoDir, "commit", "--allow-empty", "-m", "release")
	releaseCommit := string(runGitTest(t, repoDir, "rev-parse", "HEAD"))
	releasePath := filepath.Join(root, "release.bundle")
	runGitTest(t, repoDir, "bundle", "create", releasePath, "release")
	releaseBundle, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatal(err)
	}
	release, err := service.UploadBundle(ctx, resources.item.ID, bytes.NewReader(releaseBundle), int64(len(releaseBundle)))
	if err != nil || release.DefaultBranch != "release" || release.CommitID != releaseCommit {
		t.Fatalf("new branch upload = %+v, err=%v", release, err)
	}
	if resources.updated.Config["endpoint"] != wantEndpoint || resources.updated.Config["default_branch"] != "main" {
		t.Fatalf("new branch upload changed saved selection: %#v", resources.updated.Config)
	}
	invalid := []byte("invalid bundle")
	if _, err := service.UploadBundle(ctx, resources.item.ID, bytes.NewReader(invalid), int64(len(invalid))); err == nil {
		t.Fatal("invalid bundle must be rejected")
	}
	if current := string(runGitTest(t, filepath.Join(root, "storage", resources.item.ID), "rev-parse", "main")); current != nextCommit {
		t.Fatalf("failed upload changed current commit: %s", current)
	}
	exported := filepath.Join(root, "exported.bundle")
	if err := gitrepo.ExportBundle(ctx, filepath.Join(root, "storage", resources.item.ID), exported); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repoDir, "bundle", "verify", exported)
	cloned := filepath.Join(root, "exported-clone")
	runGitTest(t, repoDir, "clone", "--bare", exported, cloned)
	if got := string(runGitTest(t, cloned, "rev-parse", "release")); got != releaseCommit {
		t.Fatalf("export lost release: %s", got)
	}
}

func TestFirstBundleUploadOutsideGitRepository(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	source := filepath.Join(root, "source")
	runGitTest(t, root, "init", "-b", "main", source)
	runGitTest(t, source, "config", "user.name", "Test")
	runGitTest(t, source, "config", "user.email", "test@example.test")
	runGitTest(t, source, "commit", "--allow-empty", "-m", "initial")
	base := string(runGitTest(t, source, "rev-parse", "HEAD"))
	runGitTest(t, source, "commit", "--allow-empty", "-m", "next")
	commit := string(runGitTest(t, source, "rev-parse", "HEAD"))
	resources := &bundleTestResources{item: resource.Resource{
		ID: "fresh-repository", Kind: "Repository", Subtype: "Bundle",
		Config: map[string]any{"default_branch": "main"},
	}}
	service := NewService(filepath.Join(root, "storage"), 1<<20, resources)
	for _, test := range []struct {
		name      string
		refs      string
		wantError bool
	}{
		{"missing prerequisites", base + "..main", true},
		{"complete bundle", "--all", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, "upload.bundle")
			runGitTest(t, source, "bundle", "create", path, test.refs)
			bundle, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			out, err := service.UploadBundle(context.Background(), resources.item.ID, bytes.NewReader(bundle), int64(len(bundle)))
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "prerequisite") {
					t.Fatalf("first incremental upload should report missing prerequisites: %v", err)
				}
				if resources.updated.ID != "" {
					t.Fatal("failed first upload must not publish resource metadata")
				}
				return
			}
			if err != nil || out.CommitID != commit || out.DefaultBranch != "main" {
				t.Fatalf("first complete upload = %+v, err=%v", out, err)
			}
		})
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
