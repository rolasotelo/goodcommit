package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func runInitSubcommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	pluginsConfigPath := fs.String("plugins-config", "./configs/goodcommit.plugins.json", "Path to write plugin config")
	typesConfigPath := fs.String("types-config", "./configs/commit-types.json", "Path to write commit types config")
	pluginsLockfilePath := fs.String("plugins-lockfile", "goodcommit.plugins.lock", "Path to write plugin lockfile")
	pluginsBinDir := fs.String("plugins-bin-dir", "gobin", "Directory for built plugin executables (default: GOBIN)")
	force := fs.Bool("force", false, "Overwrite scaffold files if they already exist")
	withLock := fs.Bool("lock", true, "Generate plugin lockfile after scaffolding")
	strictLock := fs.Bool("strict-lock", false, "Fail init if lockfile generation fails")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected args: %s", strings.Join(fs.Args(), " "))
	}

	configDir := filepath.Dir(*pluginsConfigPath)
	typesConstraintPath, err := filepath.Rel(configDir, *typesConfigPath)
	if err != nil {
		typesConstraintPath = *typesConfigPath
	}
	typesConstraintPath = filepath.ToSlash(typesConstraintPath)

	pluginsCfg := map[string]interface{}{
		"plugins": []map[string]interface{}{
			{
				"id":      "builtin/types",
				"enabled": true,
				"ai_constraints": map[string]interface{}{
					"commit_type": map[string]interface{}{
						"allowed_values_from_json": map[string]interface{}{
							"path":      typesConstraintPath,
							"array_key": "types",
							"value_key": "id",
						},
					},
				},
				"order":        10,
				"failure_mode": "fail_closed",
				"timeout_ms":   10000,
				"config": map[string]interface{}{
					"path": *typesConfigPath,
				},
			},
			{
				"id":           "builtin/description",
				"enabled":      true,
				"ui_group":     "compose_message",
				"order":        30,
				"failure_mode": "fail_closed",
				"timeout_ms":   10000,
				"config":       map[string]interface{}{},
			},
			{
				"id":           "builtin/body",
				"enabled":      true,
				"ui_group":     "compose_message",
				"order":        50,
				"failure_mode": "fail_open",
				"timeout_ms":   10000,
				"config":       map[string]interface{}{},
			},
			{
				"id":           "builtin/conventional-title",
				"enabled":      true,
				"order":        90,
				"failure_mode": "fail_closed",
				"timeout_ms":   10000,
				"config":       map[string]interface{}{},
			},
		},
	}
	pluginsCfgRaw, err := json.MarshalIndent(pluginsCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugins config: %w", err)
	}
	pluginsCfgRaw = append(pluginsCfgRaw, '\n')

	typesCfg := map[string]interface{}{
		"types": []map[string]interface{}{
			{
				"id":    "feat",
				"name":  "Feat",
				"title": "New commit introduces a new feature",
				"emoji": "✨",
			},
			{
				"id":    "fix",
				"name":  "Fix",
				"title": "This commit patches a bug",
				"emoji": "🐞",
			},
			{
				"id":    "chore",
				"name":  "Chore",
				"title": "For all other tasks",
				"emoji": "🧰",
			},
		},
	}
	typesCfgRaw, err := json.MarshalIndent(typesCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal types config: %w", err)
	}
	typesCfgRaw = append(typesCfgRaw, '\n')

	created := []string{}
	lockCreated := false
	var lockErr error
	if err := writeScaffoldFile(*pluginsConfigPath, pluginsCfgRaw, *force); err != nil {
		return err
	}
	created = append(created, *pluginsConfigPath)

	if err := writeScaffoldFile(*typesConfigPath, typesCfgRaw, *force); err != nil {
		return err
	}
	created = append(created, *typesConfigPath)

	if *withLock {
		resolved, err := plugins.LoadResolvedPlugins(*pluginsConfigPath)
		if err != nil {
			lockErr = fmt.Errorf("load plugins config: %w", err)
		} else if lf, err := plugins.BuildLockfileWithArtifacts(resolved, *pluginsLockfilePath, *pluginsBinDir); err != nil {
			lockErr = fmt.Errorf("build lockfile artifacts: %w", err)
		} else if err := plugins.WriteLockfile(*pluginsLockfilePath, lf); err != nil {
			lockErr = fmt.Errorf("write lockfile: %w", err)
		} else {
			created = append(created, *pluginsLockfilePath)
			lockCreated = true
		}
	}

	fmt.Println("Initialized goodcommit scaffold:")
	for _, p := range created {
		fmt.Printf("- %s\n", p)
	}
	if lockErr != nil {
		fmt.Printf("Warning: lock step failed: %v\n", lockErr)
		fmt.Println("Hint: if your environment blocks GOBIN writes, retry lock using project-local binaries:")
		fmt.Println("  goodcommit plugin lock --plugins-config " + *pluginsConfigPath + " --plugins-lockfile " + *pluginsLockfilePath + " --plugins-bin-dir project")
		if *strictLock {
			return fmt.Errorf("scaffold created, but lock step failed in strict mode: %w", lockErr)
		}
	}
	if !*withLock || !lockCreated || lockErr != nil {
		fmt.Println("Next: run `goodcommit plugin lock --plugins-config " + *pluginsConfigPath + " --plugins-lockfile " + *pluginsLockfilePath + "`")
	}
	return nil
}

func writeScaffoldFile(path string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func runPluginSubcommand(args []string) error {
	if len(args) == 0 {
		return errors.New("expected one of: lock, verify, context")
	}

	switch args[0] {
	case "lock":
		fs := flag.NewFlagSet("plugin lock", flag.ContinueOnError)
		pluginsConfigPath := fs.String("plugins-config", "", "Path to a plugin configuration file")
		pluginsLockfilePath := fs.String("plugins-lockfile", "goodcommit.plugins.lock", "Path to a plugin lockfile")
		pluginsBinDir := fs.String("plugins-bin-dir", "gobin", "Directory for built plugin executables (default: GOBIN)")
		if err := fs.Parse(args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if *pluginsConfigPath == "" {
			return errors.New("--plugins-config is required")
		}
		resolved, err := plugins.LoadResolvedPlugins(*pluginsConfigPath)
		if err != nil {
			return err
		}
		lf, err := plugins.BuildLockfileWithArtifacts(resolved, *pluginsLockfilePath, *pluginsBinDir)
		if err != nil {
			return err
		}
		if err := plugins.WriteLockfile(*pluginsLockfilePath, lf); err != nil {
			return err
		}
		fmt.Printf("Wrote lockfile with %d plugin(s): %s\n", len(lf.Plugins), *pluginsLockfilePath)
		return nil
	case "verify":
		fs := flag.NewFlagSet("plugin verify", flag.ContinueOnError)
		pluginsConfigPath := fs.String("plugins-config", "", "Path to a plugin configuration file")
		pluginsLockfilePath := fs.String("plugins-lockfile", "goodcommit.plugins.lock", "Path to a plugin lockfile")
		if err := fs.Parse(args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if *pluginsConfigPath == "" {
			return errors.New("--plugins-config is required")
		}
		resolved, err := plugins.LoadResolvedPlugins(*pluginsConfigPath)
		if err != nil {
			return err
		}
		if err := plugins.VerifyResolvedPlugins(resolved, *pluginsLockfilePath); err != nil {
			return err
		}
		fmt.Printf("Plugin verification successful for %d plugin(s) using %s\n", len(resolved), *pluginsLockfilePath)
		return nil
	case "context":
		fs := flag.NewFlagSet("plugin context", flag.ContinueOnError)
		pluginsConfigPath := fs.String("plugins-config", "", "Path to a plugin configuration file")
		pluginsLockfilePath := fs.String("plugins-lockfile", "goodcommit.plugins.lock", "Path to a plugin lockfile")
		if err := fs.Parse(args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if *pluginsConfigPath == "" {
			return errors.New("--plugins-config is required")
		}
		resolved, err := plugins.LoadResolvedPlugins(*pluginsConfigPath)
		if err != nil {
			return err
		}
		if err := plugins.VerifyResolvedPlugins(resolved, *pluginsLockfilePath); err != nil {
			return err
		}
		contextPayload, err := plugins.BuildAIContext(resolved)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(contextPayload)
	default:
		return fmt.Errorf("unknown plugin subcommand %q", args[0])
	}
}
