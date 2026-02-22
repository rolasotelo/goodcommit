package pluginruntime

import "fmt"

// BuildLockfileFromResolved creates a reproducible lockfile from current plugin resolution.
func BuildLockfileFromResolved(resolved []ResolvedPlugin) (Lockfile, error) {
	lf := NewLockfile()
	for _, rp := range resolved {
		sum, err := FileSHA256(rp.ManifestPath)
		if err != nil {
			return Lockfile{}, fmt.Errorf("checksum manifest %s: %w", rp.ManifestPath, err)
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
		sum, err := FileSHA256(rp.ManifestPath)
		if err != nil {
			return fmt.Errorf("checksum manifest %s: %w", rp.ManifestPath, err)
		}
		if locked.ManifestChecksum != sum {
			return fmt.Errorf("manifest checksum mismatch for plugin %s", rp.Runtime.Manifest.ID)
		}
		if locked.Source.Type != "" && rp.Source.Type != "" && locked.Source.Type != rp.Source.Type {
			return fmt.Errorf("source type mismatch for plugin %s", rp.Runtime.Manifest.ID)
		}
	}
	return nil
}
