package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestGatherRequestContextAllowsUnbornHead(t *testing.T) {
	repoDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v output=%s", err, string(out))
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	ctx, err := gatherRequestContext()
	if err != nil {
		t.Fatalf("gatherRequestContext() error = %v", err)
	}
	gotInfo, err := os.Stat(ctx.RepoRoot)
	if err != nil {
		t.Fatalf("stat repo_root %q: %v", ctx.RepoRoot, err)
	}
	wantInfo, err := os.Stat(repoDir)
	if err != nil {
		t.Fatalf("stat repoDir %q: %v", repoDir, err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("repo_root = %q does not resolve to expected repo %q", ctx.RepoRoot, repoDir)
	}
	if ctx.Head != "" {
		t.Fatalf("head should be empty for unborn HEAD repo, got %q", ctx.Head)
	}
}
