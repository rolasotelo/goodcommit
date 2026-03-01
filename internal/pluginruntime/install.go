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

	if err := buildPluginArtifact(artifactPath, target); err != nil {
		return "", "", fmt.Errorf("build plugin %s: %w", rp.Runtime.Manifest.ID, err)
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

func buildPluginArtifact(artifactPath, target string) error {
	stderr, err := runGoBuild("", artifactPath, target)
	if err == nil {
		return nil
	}
	if isModuleImportTarget(target) && isMissingGoModuleContext(stderr) {
		tempStderr, tempErr := buildInTempModule(artifactPath, target)
		if tempErr == nil {
			return nil
		}
		return fmt.Errorf("%w stderr=%s (fallback stderr=%s)", err, strings.TrimSpace(stderr), strings.TrimSpace(tempStderr))
	}
	return fmt.Errorf("%w stderr=%s", err, strings.TrimSpace(stderr))
}

func runGoBuild(dir, artifactPath, target string) (string, error) {
	cmd := exec.Command("go", "build", "-o", artifactPath, target)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

func buildInTempModule(artifactPath, target string) (string, error) {
	tempDir, err := os.MkdirTemp("", "goodcommit-plugin-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp module dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmdInit := exec.Command("go", "mod", "init", "goodcommit-temp-build")
	cmdInit.Dir = tempDir
	var initErr bytes.Buffer
	cmdInit.Stderr = &initErr
	if err := cmdInit.Run(); err != nil {
		return initErr.String(), fmt.Errorf("init temp module: %w", err)
	}

	cmdGet := exec.Command("go", "get", target+"@latest")
	cmdGet.Dir = tempDir
	var getErr bytes.Buffer
	cmdGet.Stderr = &getErr
	if err := cmdGet.Run(); err != nil {
		return getErr.String(), fmt.Errorf("resolve module target: %w", err)
	}

	buildErr, err := runGoBuild(tempDir, artifactPath, target)
	if err != nil {
		return buildErr, err
	}
	return "", nil
}

func isModuleImportTarget(target string) bool {
	target = strings.TrimSpace(target)
	return target != "" &&
		!strings.HasPrefix(target, ".") &&
		!strings.HasPrefix(target, "/") &&
		strings.Contains(target, ".")
}

func isMissingGoModuleContext(stderr string) bool {
	return strings.Contains(stderr, "go.mod file not found in current directory or any parent directory")
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
