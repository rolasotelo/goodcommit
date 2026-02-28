package pluginruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"os"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

// ConfigFile defines project plugin configuration.
type ConfigFile struct {
	Plugins []PluginConfig `json:"plugins"`
}

// SourceConfig declares where plugin artifacts come from.
type SourceConfig struct {
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// PluginConfig is one plugin entry in plugins config file.
type PluginConfig struct {
	ID                   string                 `json:"id"`
	Enabled              bool                   `json:"enabled"`
	Manifest             string                 `json:"manifest"`
	Source               SourceConfig           `json:"source"`
	UIGroup              string                 `json:"ui_group,omitempty"`
	Order                int                    `json:"order"`
	FailureMode          FailureMode            `json:"failure_mode"`
	TimeoutMS            int                    `json:"timeout_ms"`
	Config               map[string]interface{} `json:"config,omitempty"`
	AIAuto               map[string]interface{} `json:"ai_auto_answers,omitempty"`
	AIInstructionsAppend string                 `json:"ai_instructions_append,omitempty"`
	Hooks                []HookPhase            `json:"hooks,omitempty"`    // deprecated/unsupported override
	AIHints              *api.AIHints           `json:"ai_hints,omitempty"` // deprecated/unsupported override
}

// ResolvedPlugin carries runtime data and source/manifest metadata.
type ResolvedPlugin struct {
	Runtime      RuntimePlugin
	ManifestPath string
	ManifestSHA  string
	Source       LockedSource
}

func resolvePath(baseDir, p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

// LoadResolvedPlugins parses and validates plugin config and manifests.
func LoadResolvedPlugins(configPath string) ([]ResolvedPlugin, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read plugins config: %w", err)
	}

	var cfg ConfigFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse plugins config: %w", err)
	}

	baseDir := filepath.Dir(configPath)
	resolved := make([]ResolvedPlugin, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		if !p.Enabled {
			continue
		}
		builtinDef, isBuiltin := builtinByID(p.ID)

		var (
			manifestPath string
			manifestSHA  string
			manifest     Manifest
			err          error
		)

		if p.Manifest != "" {
			manifestPath = resolvePath(baseDir, p.Manifest)
			var manifestRaw []byte
			manifest, manifestRaw, err = ReadManifestWithRaw(manifestPath)
			if err != nil {
				return nil, fmt.Errorf("plugin %q manifest error: %w", p.ID, err)
			}
			manifestSHA = BytesSHA256(manifestRaw)
		} else if isBuiltin {
			manifest, manifestSHA, err = builtinManifest(p.ID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("plugin %q missing manifest path", p.ID)
		}
		if p.ID != "" && p.ID != manifest.ID {
			return nil, fmt.Errorf("plugin id mismatch config=%q manifest=%q", p.ID, manifest.ID)
		}
		if p.Source.Type == "" && isBuiltin {
			p.Source = builtinDef.DefaultSource
		}
		if p.Source.Type == "" && p.Source.Path != "" {
			p.Source.Type = "path"
		}
		if len(p.Hooks) > 0 {
			return nil, fmt.Errorf("plugin %q: overriding manifest hooks from config is not allowed", manifest.ID)
		}
		if p.AIHints != nil {
			return nil, fmt.Errorf("plugin %q: overriding manifest ai_hints from config is not allowed (use ai_instructions_append)", manifest.ID)
		}
		aiHints := manifest.AIHints
		if strings.TrimSpace(p.AIInstructionsAppend) != "" {
			aiHints = mergeAIHints(aiHintsOrDefault(aiHints), p.AIInstructionsAppend)
		}
		if p.FailureMode == "" {
			p.FailureMode = FailClosed
		}
		timeout := time.Duration(p.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = defaultPluginTimeout
		}

		rp := RuntimePlugin{
			Manifest:    manifest,
			Config:      p.Config,
			AIHints:     aiHints,
			AIAuto:      p.AIAuto,
			UIGroup:     strings.TrimSpace(p.UIGroup),
			Order:       p.Order,
			FailureMode: p.FailureMode,
			Timeout:     timeout,
		}

		resolved = append(resolved, ResolvedPlugin{
			Runtime:      rp,
			ManifestPath: manifestPath,
			ManifestSHA:  manifestSHA,
			Source: LockedSource{
				Type:     p.Source.Type,
				Repo:     p.Source.Repo,
				Ref:      p.Source.Ref,
				Path:     p.Source.Path,
				Checksum: p.Source.Checksum,
			},
		})
	}

	return resolved, nil
}

func aiHintsOrDefault(h *api.AIHints) *api.AIHints {
	if h != nil {
		return h
	}
	return &api.AIHints{}
}

func mergeAIHints(base *api.AIHints, appendInstructions string) *api.AIHints {
	out := *base
	appendInstructions = strings.TrimSpace(appendInstructions)
	if appendInstructions == "" {
		return &out
	}
	if strings.TrimSpace(out.Instructions) == "" {
		out.Instructions = appendInstructions
		return &out
	}
	out.Instructions = strings.TrimSpace(out.Instructions) + "\n" + appendInstructions
	return &out
}

func RuntimePlugins(resolved []ResolvedPlugin) []RuntimePlugin {
	out := make([]RuntimePlugin, 0, len(resolved))
	for _, rp := range resolved {
		out = append(out, rp.Runtime)
	}
	return out
}
