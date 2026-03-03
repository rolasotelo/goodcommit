package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func gatherRequestContext() (plugins.RequestContext, error) {
	staged := []string{}
	if out, err := gitOutput("diff", "--cached", "--name-only"); err == nil && out != "" {
		staged = strings.Split(strings.TrimSpace(out), "\n")
	}
	repoRoot, err := gitRequiredOutput("repo root", "rev-parse", "--show-toplevel")
	if err != nil {
		return plugins.RequestContext{}, err
	}
	branch := gitOptionalOutput("rev-parse", "--abbrev-ref", "HEAD")
	head := gitOptionalOutput("rev-parse", "HEAD")
	name, _ := gitOutput("config", "--get", "user.name")
	email, _ := gitOutput("config", "--get", "user.email")

	return plugins.RequestContext{
		RepoRoot:     repoRoot,
		Branch:       branch,
		Head:         head,
		StagedFiles:  staged,
		GitUserName:  strings.TrimSpace(name),
		GitUserEmail: strings.TrimSpace(email),
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func gitOptionalOutput(args ...string) string {
	out, err := gitOutput(args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func gitRequiredOutput(label string, args ...string) (string, error) {
	out, err := gitOutput(args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return "", fmt.Errorf("%s: empty output from git %s", label, strings.Join(args, " "))
	}
	return trimmed, nil
}

func retryMessageFilePath() string {
	if out, err := gitOutput("rev-parse", "--git-dir"); err == nil {
		gitDir := strings.TrimSpace(out)
		if gitDir != "" {
			return filepath.Join(gitDir, "goodcommit", "last_failed_commit_message.txt")
		}
	}
	return filepath.Join(".git", "goodcommit", "last_failed_commit_message.txt")
}

func retryMessageFilePathForRead() string {
	return retryMessageFilePath()
}

func cleanupRetryMessageFiles() {
	p := retryMessageFilePath()
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Warning: could not remove stale retry message file %s: %s\n", p, err)
	}
}

func runGitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\nOutput:\n%s", err, string(output))
	}
	return nil
}
