package pluginruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

const projectBinDir = ".goodcommit/plugins/bin"
const gobinLockPrefix = "gobin:"

// BuildLockfileWithArtifacts writes lock metadata plus locally built plugin executables.
func BuildLockfileWithArtifacts(resolved []ResolvedPlugin, lockPath, binDirOverride string) (Lockfile, error) {
	lf, err := BuildLockfileFromResolved(resolved)
	if err != nil {
		return Lockfile{}, err
	}

	lockDir := filepath.Dir(lockPath)
	lockDirAbs, err := filepath.Abs(lockDir)
	if err != nil {
		return Lockfile{}, fmt.Errorf("resolve lock directory: %w", err)
	}
	binDir, err := resolvePluginBinDir(lockDirAbs, binDirOverride)
	if err != nil {
		return Lockfile{}, err
	}
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
		execRel, execSum, err := installExecutable(lockDirAbs, binDir, resolvedPlugin)
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
	if suffix := artifactNameSuffix(rp); suffix != "" {
		artifactName += "-" + suffix
	}
	if goruntime.GOOS == "windows" {
		artifactName += ".exe"
	}
	artifactPath := filepath.Join(binDir, artifactName)
	artifactPath, err := filepath.Abs(artifactPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve plugin artifact path %s: %w", artifactPath, err)
	}

	if err := buildPluginArtifact(artifactPath, target, moduleRefForBuild(rp)); err != nil {
		return "", "", fmt.Errorf("build plugin %s: %w", rp.Runtime.Manifest.ID, err)
	}

	sum, err := FileSHA256(artifactPath)
	if err != nil {
		return "", "", fmt.Errorf("checksum plugin executable %s: %w", rp.Runtime.Manifest.ID, err)
	}
	storedPath, err := pathForLockfile(lockDir, binDir, artifactPath)
	if err != nil {
		return "", "", fmt.Errorf("path for plugin executable %s: %w", rp.Runtime.Manifest.ID, err)
	}
	return storedPath, sum, nil
}

func buildPluginArtifact(artifactPath, target, moduleRef string) error {
	stderr, err := runGoBuild("", artifactPath, target)
	if err == nil {
		return nil
	}
	if isModuleImportTarget(target) {
		tempStderr, tempErr := buildInTempModule(artifactPath, target, moduleRef)
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

func buildInTempModule(artifactPath, target, moduleRef string) (string, error) {
	moduleRef = strings.TrimSpace(moduleRef)
	if moduleRef == "" {
		return "", fmt.Errorf("deterministic module ref required for %s (set source.ref)", target)
	}

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

	cmdGet := exec.Command("go", "get", target+"@"+moduleRef)
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

func resolvePluginBinDir(lockDir, override string) (string, error) {
	trimmed := strings.TrimSpace(override)
	if trimmed == "" || strings.EqualFold(trimmed, "gobin") {
		gobin, err := discoverGoBin()
		if err != nil {
			return "", fmt.Errorf("resolve GOBIN: %w", err)
		}
		return gobin, nil
	}
	if strings.EqualFold(trimmed, "project") {
		return filepath.Join(lockDir, projectBinDir), nil
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	return filepath.Join(lockDir, trimmed), nil
}

func discoverGoBin() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GOBIN")); v != "" {
		return v, nil
	}

	cmd := exec.Command("go", "env", "GOBIN")
	if out, err := cmd.Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v, nil
		}
	}

	cmd = exec.Command("go", "env", "GOPATH")
	if out, err := cmd.Output(); err == nil {
		gopath := strings.TrimSpace(string(out))
		if gopath != "" {
			paths := strings.Split(gopath, string(os.PathListSeparator))
			if len(paths) > 0 && strings.TrimSpace(paths[0]) != "" {
				return filepath.Join(strings.TrimSpace(paths[0]), "bin"), nil
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("unable to determine go bin directory")
	}
	return filepath.Join(home, "go", "bin"), nil
}

func pathForLockfile(lockDir, binDir, artifactPath string) (string, error) {
	rel, err := filepath.Rel(lockDir, artifactPath)
	if err == nil {
		isOutside := rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
		if !isOutside {
			return filepath.ToSlash(rel), nil
		}
	}

	if samePath(binDir, artifactPath) || pathWithin(binDir, artifactPath) {
		if gobin, gobinErr := discoverGoBin(); gobinErr == nil && samePath(gobin, binDir) {
			return gobinLockPrefix + filepath.ToSlash(filepath.Base(artifactPath)), nil
		}
	}

	return filepath.ToSlash(artifactPath), nil
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

func artifactNameSuffix(rp ResolvedPlugin) string {
	if sum := strings.TrimSpace(rp.ManifestSHA); sum != "" {
		sum = strings.TrimPrefix(strings.ToLower(sum), "sha256:")
		sum = normalizeAlphaNum(sum)
		if len(sum) >= 12 {
			return sum[:12]
		}
		return sum
	}
	seed := strings.TrimSpace(strings.Join([]string{
		rp.Runtime.Manifest.Version,
		rp.Source.Type,
		rp.Source.Repo,
		rp.Source.Ref,
		rp.Source.Path,
		rp.Source.Checksum,
	}, "|"))
	if seed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:12]
}

func normalizeAlphaNum(in string) string {
	var b strings.Builder
	for _, r := range in {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func moduleRefForBuild(rp ResolvedPlugin) string {
	if ref := strings.TrimSpace(rp.Source.Ref); ref != "" {
		return ref
	}
	if _, isBuiltin := builtinByID(rp.Runtime.Manifest.ID); isBuiltin {
		if ref := builtinSourceRef(); ref != "" {
			return ref
		}
	}
	return ""
}

func samePath(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr != nil || bErr != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	if goruntime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(aAbs), filepath.Clean(bAbs))
	}
	return filepath.Clean(aAbs) == filepath.Clean(bAbs)
}

func pathWithin(base, target string) bool {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(target) == "" {
		return false
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	isOutside := rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	return !isOutside
}
