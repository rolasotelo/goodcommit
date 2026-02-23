package pluginruntime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

const projectBinDir = ".goodcommit/plugins/bin"

// BuildLockfileWithArtifacts writes lock metadata plus locally built plugin executables.
func BuildLockfileWithArtifacts(resolved []ResolvedPlugin, lockPath string) (Lockfile, error) {
	lf, err := BuildLockfileFromResolved(resolved)
	if err != nil {
		return Lockfile{}, err
	}

	lockDir := filepath.Dir(lockPath)
	binDir := filepath.Join(lockDir, projectBinDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return Lockfile{}, fmt.Errorf("create plugin bin directory: %w", err)
	}

	resolvedByID := map[string]ResolvedPlugin{}
	for _, rp := range resolved {
		resolvedByID[rp.Runtime.Manifest.ID] = rp
	}

	for i := range lf.Plugins {
		resolvedPlugin, ok := resolvedByID[lf.Plugins[i].ID]
		if !ok {
			continue
		}
		execRel, execSum, err := installExecutable(lockDir, binDir, resolvedPlugin)
		if err != nil {
			return Lockfile{}, err
		}
		lf.Plugins[i].ExecutablePath = execRel
		lf.Plugins[i].ExecutableChecksum = execSum
	}

	return lf, nil
}

func installExecutable(lockDir, binDir string, rp ResolvedPlugin) (string, string, error) {
	target := buildTarget(rp)
	if target == "" {
		return "", "", nil
	}

	artifactName := sanitizePluginID(rp.Runtime.Manifest.ID)
	if goruntime.GOOS == "windows" {
		artifactName += ".exe"
	}
	artifactPath := filepath.Join(binDir, artifactName)

	cmd := exec.Command("go", "build", "-o", artifactPath, target)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("build plugin %s: %w stderr=%s", rp.Runtime.Manifest.ID, err, strings.TrimSpace(stderr.String()))
	}

	sum, err := FileSHA256(artifactPath)
	if err != nil {
		return "", "", fmt.Errorf("checksum plugin executable %s: %w", rp.Runtime.Manifest.ID, err)
	}
	rel, err := filepath.Rel(lockDir, artifactPath)
	if err != nil {
		return "", "", fmt.Errorf("relative path for plugin executable %s: %w", rp.Runtime.Manifest.ID, err)
	}
	return filepath.ToSlash(rel), sum, nil
}

func buildTarget(rp ResolvedPlugin) string {
	if def, ok := builtinByID(rp.Runtime.Manifest.ID); ok {
		if rp.Source.Type == "path" && strings.TrimSpace(rp.Source.Path) != "" {
			return rp.Source.Path
		}
		return def.DefaultSource.Path
	}

	if rp.Source.Type == "path" &&
		rp.Runtime.Manifest.Entrypoint.Command == "go" &&
		len(rp.Runtime.Manifest.Entrypoint.Args) >= 2 &&
		rp.Runtime.Manifest.Entrypoint.Args[0] == "run" {
		return rp.Runtime.Manifest.Entrypoint.Args[1]
	}

	return ""
}

func sanitizePluginID(id string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-", "@", "-")
	return replacer.Replace(id)
}
