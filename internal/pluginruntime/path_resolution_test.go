package pluginruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathForLockfileRelativeWithinLockDir(t *testing.T) {
	lockDir := t.TempDir()
	binDir := filepath.Join(lockDir, ".goodcommit", "plugins", "bin")
	artifact := filepath.Join(binDir, "builtin-types-abc123")

	got, err := pathForLockfile(lockDir, binDir, artifact)
	if err != nil {
		t.Fatalf("pathForLockfile() error = %v", err)
	}
	want := filepath.ToSlash(filepath.Join(".goodcommit", "plugins", "bin", "builtin-types-abc123"))
	if got != want {
		t.Fatalf("pathForLockfile() = %q, want %q", got, want)
	}
}

func TestPathForLockfileGobinToken(t *testing.T) {
	lockDir := t.TempDir()
	gobin := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobin)

	artifact := filepath.Join(gobin, "builtin-description-123456")
	got, err := pathForLockfile(lockDir, gobin, artifact)
	if err != nil {
		t.Fatalf("pathForLockfile() error = %v", err)
	}
	want := gobinLockPrefix + "builtin-description-123456"
	if got != want {
		t.Fatalf("pathForLockfile() = %q, want %q", got, want)
	}
}

func TestResolveExecutablePathGobinToken(t *testing.T) {
	gobin := filepath.Join(t.TempDir(), "bin")
	t.Setenv("GOBIN", gobin)

	got, err := resolveExecutablePath(t.TempDir(), gobinLockPrefix+"builtin-logo-abcdef")
	if err != nil {
		t.Fatalf("resolveExecutablePath() error = %v", err)
	}
	want := filepath.Join(gobin, "builtin-logo-abcdef")
	if got != want {
		t.Fatalf("resolveExecutablePath() = %q, want %q", got, want)
	}
}

func TestArtifactNameSuffixNormalizesManifestHash(t *testing.T) {
	rp := ResolvedPlugin{ManifestSHA: "sha256:ABCDEF1234567890"}
	got := artifactNameSuffix(rp)
	if got != "abcdef123456" {
		t.Fatalf("artifactNameSuffix() = %q, want %q", got, "abcdef123456")
	}
}

func TestResolveExecutablePathRelative(t *testing.T) {
	lockDir := t.TempDir()
	got, err := resolveExecutablePath(lockDir, "plugins/bin/my-plugin")
	if err != nil {
		t.Fatalf("resolveExecutablePath() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("plugins", "bin", "my-plugin")) {
		t.Fatalf("resolveExecutablePath() unexpected suffix: %q", got)
	}
}
