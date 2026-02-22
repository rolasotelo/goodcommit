/*
Goodcommit is a plugin-first tool for creating consistent commit messages.

Usage:

	goodcommit [flags]
	goodcommit plugin lock [flags]
	goodcommit plugin verify [flags]
	goodcommit plugin context [flags]

Flags:
*/
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func main() {
	fmt.Println("hello world")
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		if err := runPluginSubcommand(os.Args[2:]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		return
	}
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
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *pluginsConfigPath == "" {
			return errors.New("--plugins-config is required")
		}
		resolved, err := plugins.LoadResolvedPlugins(*pluginsConfigPath)
		if err != nil {
			return err
		}
		lf, err := plugins.BuildLockfileFromResolved(resolved)
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
		contextPayload := plugins.BuildAIContext(resolved)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(contextPayload)
	default:
		return fmt.Errorf("unknown plugin subcommand %q", args[0])
	}
}
