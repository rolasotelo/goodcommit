package pluginruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildLockfileFromResolved creates a reproducible lockfile from current plugin resolution.
func BuildLockfileFromResolved(resolved []ResolvedPlugin) (Lockfile, error) {
	lf := NewLockfile()
	for _, rp := range resolved {
		sum := rp.ManifestSHA
		if sum == "" {
			if rp.ManifestPath == "" {
				return Lockfile{}, fmt.Errorf("manifest checksum missing for plugin %s", rp.Runtime.Manifest.ID)
			}
			var err error
			sum, err = FileSHA256(rp.ManifestPath)
			if err != nil {
				return Lockfile{}, fmt.Errorf("checksum manifest %s: %w", rp.ManifestPath, err)
			}
		}
		lf.UpsertPlugin(LockedPlugin{
			ID:               rp.Runtime.Manifest.ID,
			Version:          rp.Runtime.Manifest.Version,
			Source:           rp.Source,
			ManifestChecksum: sum,
			Hooks:            rp.Runtime.Manifest.Hooks,
			Order:            rp.Runtime.Order,
			FailureMode:      rp.Runtime.FailureMode,
			TimeoutMS:        int(rp.Runtime.Timeout.Milliseconds()),
			AIHints:          rp.Runtime.AIHints,
			Contract:         rp.Runtime.Manifest.Contract,
			AIAuto:           rp.Runtime.AIAuto,
		})
	}
	return lf, nil
}

// VerifyResolvedPlugins checks lockfile presence and manifest checksum for all enabled plugins.
func VerifyResolvedPlugins(resolved []ResolvedPlugin, lockPath string) error {
	lf, err := ReadLockfile(lockPath)
	if err != nil {
		return fmt.Errorf("read lockfile: %w", err)
	}

	for _, rp := range resolved {
		locked, ok := lf.FindPlugin(rp.Runtime.Manifest.ID)
		if !ok {
			return fmt.Errorf("plugin %s missing from lockfile", rp.Runtime.Manifest.ID)
		}
		sum := rp.ManifestSHA
		if sum == "" {
			if rp.ManifestPath == "" {
				return fmt.Errorf("manifest checksum missing for plugin %s", rp.Runtime.Manifest.ID)
			}
			sum, err = FileSHA256(rp.ManifestPath)
			if err != nil {
				return fmt.Errorf("checksum manifest %s: %w", rp.ManifestPath, err)
			}
		}
		if locked.ManifestChecksum != sum {
			return fmt.Errorf("manifest checksum mismatch for plugin %s", rp.Runtime.Manifest.ID)
		}
		if !sameSource(locked.Source, rp.Source) {
			return fmt.Errorf("source mismatch for plugin %s", rp.Runtime.Manifest.ID)
		}
		if locked.ExecutablePath == "" && requiresExecutablePin(rp) {
			return fmt.Errorf("lockfile missing executable path for plugin %s; re-run plugin lock", rp.Runtime.Manifest.ID)
		}
		if locked.ExecutablePath != "" {
			execPath, err := resolveExecutablePath(filepath.Dir(lockPath), locked.ExecutablePath)
			if err != nil {
				return fmt.Errorf("resolve executable path for %s: %w", rp.Runtime.Manifest.ID, err)
			}
			if _, err := os.Stat(execPath); err != nil {
				return fmt.Errorf("missing plugin executable for %s: %w", rp.Runtime.Manifest.ID, err)
			}
			if locked.ExecutableChecksum != "" {
				if err := VerifyFileChecksum(execPath, locked.ExecutableChecksum); err != nil {
					return fmt.Errorf("plugin executable checksum mismatch for %s: %w", rp.Runtime.Manifest.ID, err)
				}
			}
		}
	}
	return nil
}

func RuntimePluginsFromLock(resolved []ResolvedPlugin, lockPath string) ([]RuntimePlugin, error) {
	lf, err := ReadLockfile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("read lockfile: %w", err)
	}

	lockDir := filepath.Dir(lockPath)
	out := make([]RuntimePlugin, 0, len(resolved))
	for _, rp := range resolved {
		runtimePlugin := rp.Runtime
		locked, ok := lf.FindPlugin(runtimePlugin.Manifest.ID)
		if !ok {
			return nil, fmt.Errorf("plugin %s missing from lockfile", runtimePlugin.Manifest.ID)
		}
		if locked.ExecutablePath == "" && requiresExecutablePin(rp) {
			return nil, fmt.Errorf("lockfile missing executable path for plugin %s; re-run plugin lock", runtimePlugin.Manifest.ID)
		}
		if locked.ExecutablePath != "" {
			execPath, err := resolveExecutablePath(lockDir, locked.ExecutablePath)
			if err != nil {
				return nil, fmt.Errorf("resolve executable path for %s: %w", runtimePlugin.Manifest.ID, err)
			}
			runtimePlugin.Manifest.Entrypoint.Command = execPath
			if buildTarget(rp) != "" {
				runtimePlugin.Manifest.Entrypoint.Args = nil
			}
		}
		out = append(out, runtimePlugin)
	}

	return out, nil
}

func sameSource(a, b LockedSource) bool {
	return a.Type == b.Type &&
		a.Repo == b.Repo &&
		a.Ref == b.Ref &&
		a.Path == b.Path &&
		a.Checksum == b.Checksum
}

func resolveExecutablePath(lockDir, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, gobinLockPrefix) {
		artifact := strings.TrimPrefix(trimmed, gobinLockPrefix)
		artifact = filepath.Base(filepath.FromSlash(artifact))
		if artifact == "." || artifact == string(filepath.Separator) || artifact == "" {
			return "", fmt.Errorf("invalid gobin executable value %q", raw)
		}
		gobin, err := discoverGoBin()
		if err != nil {
			return "", fmt.Errorf("discover GOBIN: %w", err)
		}
		return filepath.Join(gobin, artifact), nil
	}
	if filepath.IsAbs(trimmed) {
		return filepath.FromSlash(trimmed), nil
	}
	return filepath.Join(lockDir, filepath.FromSlash(trimmed)), nil
}
