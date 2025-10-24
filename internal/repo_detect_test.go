package ghprcomments

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectRepositoriesMultiple(t *testing.T) {
	t.Setenv("GH_REPO", "")

	tmpDir := t.TempDir()

	makeRepo := func(owner, name string) string {
		repoPath := filepath.Join(tmpDir, name)
		if err := os.Mkdir(repoPath, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", repoPath, err)
		}
		runGit(t, repoPath, "init")
		runGit(t, repoPath, "remote", "add", "origin", "git@github.com:"+owner+"/"+name+".git")
		return repoPath
	}

	alphaPath := makeRepo("octo", "alpha")
	betaPath := makeRepo("octo", "beta")

	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}

	repos, err := DetectRepositories(context.Background())
	if err != nil {
		t.Fatalf("DetectRepositories: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	repoLookup := make(map[string]Repository)
	for _, repo := range repos {
		repoLookup[repo.Name] = repo
	}

	if got, ok := repoLookup["alpha"]; !ok {
		t.Fatalf("alpha repo missing from detection")
	} else {
		if got.Owner != "octo" {
			t.Fatalf("alpha owner = %s, want octo", got.Owner)
		}
		if normalizePath(t, got.Path) != normalizePath(t, alphaPath) {
			t.Fatalf("alpha path = %s, want %s", got.Path, alphaPath)
		}
	}

	if got, ok := repoLookup["beta"]; !ok {
		t.Fatalf("beta repo missing from detection")
	} else {
		if got.Owner != "octo" {
			t.Fatalf("beta owner = %s, want octo", got.Owner)
		}
		if normalizePath(t, got.Path) != normalizePath(t, betaPath) {
			t.Fatalf("beta path = %s, want %s", got.Path, betaPath)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func normalizePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}
