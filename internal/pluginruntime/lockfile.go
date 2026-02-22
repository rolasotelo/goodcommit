package pluginruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

const LockfileVersion = 1

// LockedSource captures the resolved source of a plugin.
type LockedSource struct {
	Type     string `json:"type"`
	Repo     string `json:"repo,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Path     string `json:"path,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// LockedPlugin captures reproducible plugin resolution data.
type LockedPlugin struct {
	ID               string                 `json:"id"`
	Version          string                 `json:"version,omitempty"`
	Source           LockedSource           `json:"source"`
	ManifestChecksum string                 `json:"manifest_checksum"`
	Hooks            []HookPhase            `json:"hooks,omitempty"`
	Order            int                    `json:"order,omitempty"`
	FailureMode      FailureMode            `json:"failure_mode,omitempty"`
	TimeoutMS        int                    `json:"timeout_ms,omitempty"`
	AIHints          *api.AIHints           `json:"ai_hints,omitempty"`
	Contract         *api.PluginContract    `json:"contract,omitempty"`
	AIAuto           map[string]interface{} `json:"ai_auto_answers,omitempty"`
	UpdatedAtUTC     string                 `json:"updated_at_utc"`
}

// Lockfile defines plugin source pinning data.
type Lockfile struct {
	Version int            `json:"version"`
	Plugins []LockedPlugin `json:"plugins"`
}

func NewLockfile() Lockfile {
	return Lockfile{Version: LockfileVersion, Plugins: []LockedPlugin{}}
}

func ReadLockfile(path string) (Lockfile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Lockfile{}, err
	}
	var lf Lockfile
	if err := json.Unmarshal(raw, &lf); err != nil {
		return Lockfile{}, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Version != LockfileVersion {
		return Lockfile{}, fmt.Errorf("unsupported lockfile version %d", lf.Version)
	}
	return lf, nil
}

func WriteLockfile(path string, lf Lockfile) error {
	if lf.Version == 0 {
		lf.Version = LockfileVersion
	}
	sort.Slice(lf.Plugins, func(i, j int) bool {
		return lf.Plugins[i].ID < lf.Plugins[j].ID
	})
	raw, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

func (lf *Lockfile) UpsertPlugin(p LockedPlugin) {
	if p.UpdatedAtUTC == "" {
		p.UpdatedAtUTC = time.Now().UTC().Format(time.RFC3339)
	}
	for i := range lf.Plugins {
		if lf.Plugins[i].ID == p.ID {
			lf.Plugins[i] = p
			return
		}
	}
	lf.Plugins = append(lf.Plugins, p)
}

func (lf *Lockfile) FindPlugin(id string) (LockedPlugin, bool) {
	for _, p := range lf.Plugins {
		if p.ID == id {
			return p, true
		}
	}
	return LockedPlugin{}, false
}
