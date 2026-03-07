package pluginruntime

import (
	"os"
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

func TestBuildLockfileWithArtifactsLocksDirectExecutablePathPlugins(t *testing.T) {
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "goodcommit.plugins.lock")
	pluginPath := filepath.Join(lockDir, "plugins", "my-plugin.sh")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved := []ResolvedPlugin{{
		Runtime: RuntimePlugin{
			Manifest: Manifest{
				APIVersion: "goodcommit.io/v1",
				Kind:       "Plugin",
				ID:         "test/direct-exec",
				Version:    "1.0.0",
				Entrypoint: EntryPoint{
					Type:    "exec",
					Command: "./plugins/my-plugin.sh",
					Args:    []string{"serve"},
				},
				Hooks:            []HookPhase{HookCollect},
				ProtocolVersions: []string{ProtocolVersionV1},
			},
		},
		Source: LockedSource{
			Type: "path",
			Path: "./plugins/my-plugin.sh",
		},
		ManifestSHA: "sha256:0123456789abcdef",
	}}

	lf, err := BuildLockfileWithArtifacts(resolved, lockPath, "project")
	if err != nil {
		t.Fatalf("BuildLockfileWithArtifacts() error = %v", err)
	}
	if len(lf.Plugins) != 1 {
		t.Fatalf("lockfile plugins = %d, want 1", len(lf.Plugins))
	}
	if lf.Plugins[0].ExecutablePath == "" {
		t.Fatalf("ExecutablePath should be populated for direct exec path plugins")
	}
	if lf.Plugins[0].ExecutableChecksum == "" {
		t.Fatalf("ExecutableChecksum should be populated for direct exec path plugins")
	}
	if err := WriteLockfile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}
	if err := VerifyResolvedPlugins(resolved, lockPath); err != nil {
		t.Fatalf("VerifyResolvedPlugins() error = %v", err)
	}

	runtimePlugins, err := RuntimePluginsFromLock(resolved, lockPath)
	if err != nil {
		t.Fatalf("RuntimePluginsFromLock() error = %v", err)
	}
	if got := runtimePlugins[0].Manifest.Entrypoint.Command; got != pluginPath {
		t.Fatalf("runtime command = %q, want %q", got, pluginPath)
	}
	if len(runtimePlugins[0].Manifest.Entrypoint.Args) != 1 || runtimePlugins[0].Manifest.Entrypoint.Args[0] != "serve" {
		t.Fatalf("runtime args = %#v, want preserved direct-exec args", runtimePlugins[0].Manifest.Entrypoint.Args)
	}
}

func TestVerifyResolvedPluginsRejectsTamperedDirectExecutablePathPlugins(t *testing.T) {
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "goodcommit.plugins.lock")
	pluginPath := filepath.Join(lockDir, "plugins", "my-plugin.sh")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved := []ResolvedPlugin{{
		Runtime: RuntimePlugin{
			Manifest: Manifest{
				APIVersion: "goodcommit.io/v1",
				Kind:       "Plugin",
				ID:         "test/direct-exec",
				Version:    "1.0.0",
				Entrypoint: EntryPoint{
					Type:    "exec",
					Command: "./plugins/my-plugin.sh",
				},
				Hooks:            []HookPhase{HookCollect},
				ProtocolVersions: []string{ProtocolVersionV1},
			},
		},
		Source: LockedSource{
			Type: "path",
			Path: "./plugins/my-plugin.sh",
		},
		ManifestSHA: "sha256:0123456789abcdef",
	}}

	lf, err := BuildLockfileWithArtifacts(resolved, lockPath, "project")
	if err != nil {
		t.Fatalf("BuildLockfileWithArtifacts() error = %v", err)
	}
	if err := WriteLockfile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho changed\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = VerifyResolvedPlugins(resolved, lockPath)
	if err == nil {
		t.Fatalf("expected VerifyResolvedPlugins() error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyResolvedPluginsRejectsUnpinnedDirectExecutablePathPlugins(t *testing.T) {
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "goodcommit.plugins.lock")
	resolved := []ResolvedPlugin{{
		Runtime: RuntimePlugin{
			Manifest: Manifest{
				APIVersion: "goodcommit.io/v1",
				Kind:       "Plugin",
				ID:         "test/direct-exec",
				Version:    "1.0.0",
				Entrypoint: EntryPoint{
					Type:    "exec",
					Command: "./plugins/my-plugin.sh",
				},
				Hooks:            []HookPhase{HookCollect},
				ProtocolVersions: []string{ProtocolVersionV1},
			},
		},
		Source: LockedSource{
			Type: "path",
			Path: "./plugins/my-plugin.sh",
		},
		ManifestSHA: "sha256:0123456789abcdef",
	}}

	lf := NewLockfile()
	lf.UpsertPlugin(LockedPlugin{
		ID:               "test/direct-exec",
		Version:          "1.0.0",
		Source:           LockedSource{Type: "path", Path: "./plugins/my-plugin.sh"},
		ManifestChecksum: "sha256:0123456789abcdef",
	})
	if err := WriteLockfile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}

	err := VerifyResolvedPlugins(resolved, lockPath)
	if err == nil {
		t.Fatalf("expected VerifyResolvedPlugins() error")
	}
	if !strings.Contains(err.Error(), "missing executable path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
