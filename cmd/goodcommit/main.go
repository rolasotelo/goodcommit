/*
Goodcommit is a plugin-first tool for creating consistent commit messages.

Usage:

	goodcommit [flags]
	goodcommit init [flags]
	goodcommit plugin lock [flags]
	goodcommit plugin verify [flags]
	goodcommit plugin context [flags]

Flags:

	--accessible           Enable accessible mode for forms
	--detailed-ui          Show detailed grouped plugin headings/instructions
	--message              Use an initial commit message before plugin phases
	--plugins-config       Path to a plugin configuration file
	--plugins-lockfile     Path to a plugin lockfile (default: goodcommit.plugins.lock)
	--plugins-skip-verify  Skip plugin lockfile verification
	--allow-plugin-network Allow plugins that request network permission
	--allow-plugin-git-write Allow plugins that request git_write permission
	--allow-plugin-filesystem-write Allow plugins that request filesystem_write permission
	--allow-plugin-secrets Allow plugins that request secrets permission
	--plugin-answer        Provide answer for plugin prompts/forms as key=value (repeatable)
	--retry                Retry commit with the last saved commit message
	--edit                 Edit the last saved commit message
	-m                     Dry run mode, do not execute commit
	-h                     Show this help message
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

type pluginAnswerFlag map[string]string

func (f *pluginAnswerFlag) String() string {
	if f == nil {
		return ""
	}
	parts := make([]string, 0, len(*f))
	for k, v := range *f {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (f *pluginAnswerFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("invalid --plugin-answer value %q, expected key=value", value)
	}
	if *f == nil {
		*f = map[string]string{}
	}
	(*f)[strings.TrimSpace(parts[0])] = parts[1]
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInitSubcommand(os.Args[2:]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		if err := runPluginSubcommand(os.Args[2:]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		return
	}

	pluginsConfigPath := os.Getenv("GOODCOMMIT_PLUGINS_CONFIG_PATH")
	if pluginsConfigPath == "" {
		pluginsConfigPath = "./configs/goodcommit.plugins.json"
	}
	flag.StringVar(&pluginsConfigPath, "plugins-config", pluginsConfigPath, "Path to a plugin configuration file")

	pluginsLockfilePath := os.Getenv("GOODCOMMIT_PLUGINS_LOCKFILE")
	if pluginsLockfilePath == "" {
		pluginsLockfilePath = "goodcommit.plugins.lock"
	}
	flag.StringVar(&pluginsLockfilePath, "plugins-lockfile", pluginsLockfilePath, "Path to a plugin lockfile")

	pluginsSkipVerify := flag.Bool("plugins-skip-verify", false, "Skip plugin lockfile verification")
	allowPluginNetwork := flag.Bool("allow-plugin-network", false, "Allow plugins that request network permission")
	allowPluginGitWrite := flag.Bool("allow-plugin-git-write", false, "Allow plugins that request git_write permission")
	allowPluginFilesystemWrite := flag.Bool("allow-plugin-filesystem-write", false, "Allow plugins that request filesystem_write permission")
	allowPluginSecrets := flag.Bool("allow-plugin-secrets", false, "Allow plugins that request secrets permission")
	messageOverride := flag.String("message", "", "Use an initial commit message before plugin phases")
	var pluginAnswers pluginAnswerFlag
	flag.Var(&pluginAnswers, "plugin-answer", "Provide answer for plugin prompts/forms as key=value (repeatable)")

	accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	flag.BoolVar(&accessible, "accessible", accessible, "Enable accessible mode")
	detailedUI, _ := strconv.ParseBool(os.Getenv("GOODCOMMIT_DETAILED_UI"))
	flag.BoolVar(&detailedUI, "detailed-ui", detailedUI, "Show detailed grouped plugin headings/instructions")

	dryRun := flag.Bool("m", false, "Dry run mode, do not execute commit")
	retry := flag.Bool("retry", false, "Retry commit with the last saved commit message")
	help := flag.Bool("h", false, "Show this help message")
	edit := flag.Bool("edit", false, "Edit the last saved commit message")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *edit {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		msgPath := retryMessageFilePathForRead()
		if err := os.MkdirAll(filepath.Dir(msgPath), 0o755); err != nil {
			fmt.Printf("Error preparing temp message path: %s\n", err)
			os.Exit(1)
		}
		cmd := exec.Command(editor, msgPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error opening editor: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Commit message edited, now run 'goodcommit --retry' to commit.")
		os.Exit(0)
	}

	if *retry && *dryRun {
		fmt.Println("Error: -m and --retry cannot be used together.")
		os.Exit(1)
	}

	if *retry {
		msgPath := retryMessageFilePathForRead()
		messageBytes, err := os.ReadFile(msgPath)
		if err != nil {
			fmt.Printf("Error reading saved commit message: %s\n", err)
			os.Exit(1)
		}
		message := string(messageBytes)

		var confirm bool
		err = huh.NewConfirm().Title("Commit with the following message?").Description(message).Value(&confirm).Run()
		if err != nil {
			fmt.Printf("Error during confirmation: %s\n", err)
			os.Exit(1)
		}
		if !confirm {
			fmt.Println("Commit canceled.")
			os.Exit(0)
		}

		if err := runGitCommit(message); err != nil {
			fmt.Printf("Error executing commit command: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Commit successful with the last saved commit message.")
		cleanupRetryMessageFiles()
		os.Exit(0)
	}

	resolvedPlugins, err := plugins.LoadResolvedPlugins(pluginsConfigPath)
	if err != nil {
		fmt.Println("Error loading plugins config:", err)
		os.Exit(1)
	}
	runtimePlugins := plugins.RuntimePlugins(resolvedPlugins)
	if !*pluginsSkipVerify {
		if err := plugins.VerifyResolvedPlugins(resolvedPlugins, pluginsLockfilePath); err != nil {
			fmt.Printf("Plugin lockfile verification failed (%s): %v\n", pluginsLockfilePath, err)
			fmt.Println("Run: goodcommit plugin lock --plugins-config <path> --plugins-lockfile <path>")
			os.Exit(1)
		}
		runtimePlugins, err = plugins.RuntimePluginsFromLock(resolvedPlugins, pluginsLockfilePath)
		if err != nil {
			fmt.Printf("Error loading plugin executables from lockfile (%s): %v\n", pluginsLockfilePath, err)
			fmt.Println("Run: goodcommit plugin lock --plugins-config <path> --plugins-lockfile <path>")
			os.Exit(1)
		}
	}
	autoAnswersByPlugin := buildAutoAnswersByPlugin(runtimePlugins)

	draft := plugins.CommitDraft{Metadata: map[string]interface{}{}}
	if *messageOverride != "" {
		draft = draftFromMessage(*messageOverride)
	}

	reqCtx, err := gatherRequestContext()
	if err != nil {
		fmt.Println("Error gathering git context:", err)
		os.Exit(1)
	}
	runner := plugins.NewRunner()
	runner.PromptHandler = makePromptResolver(pluginAnswers, autoAnswersByPlugin, accessible)
	runner.UIHandler = makeUIResolver(pluginAnswers, autoAnswersByPlugin, accessible)
	runner.GroupedUIHandler = makeGroupedUIResolver(pluginAnswers, autoAnswersByPlugin, accessible, detailedUI)
	runner.AllowPluginNetwork = *allowPluginNetwork
	runner.AllowPluginGitWrite = *allowPluginGitWrite
	runner.AllowFilesystemWrite = *allowPluginFilesystemWrite
	runner.AllowPluginSecrets = *allowPluginSecrets

	invocations, err := runPluginPhases(context.Background(), runner, runtimePlugins, reqCtx, &draft, []plugins.HookPhase{
		plugins.HookCollect,
		plugins.HookValidate,
		plugins.HookEnrich,
		plugins.HookFinalize,
		plugins.HookPreCommit,
	})
	printPluginInvocations(invocations)
	if err != nil {
		fmt.Println("Plugin execution failed:", err)
		os.Exit(1)
	}

	message := renderDraft(draft)
	if *dryRun {
		fmt.Println("Dry run mode, commit not executed.")
		fmt.Println("Final commit message:")
		fmt.Println(message)
		return
	}

	if err := runGitCommit(message); err != nil {
		msgPath := retryMessageFilePath()
		if errMkdir := os.MkdirAll(filepath.Dir(msgPath), 0o755); errMkdir != nil {
			fmt.Printf("Error preparing temp message path ('goodcommit --retry' won't work): %s\n", errMkdir)
		}
		errSave := os.WriteFile(msgPath, []byte(message), 0o644)
		if errSave != nil {
			fmt.Printf("Error saving commit message at %s ('goodcommit --retry' won't work): %s\n", msgPath, errSave)
		}
		fmt.Printf("Error executing command: %s\n", err)
		os.Exit(1)
	}
	cleanupRetryMessageFiles()

	postInvocations, postErr := runPluginPhases(context.Background(), runner, runtimePlugins, reqCtx, &draft, []plugins.HookPhase{plugins.HookPostCommit})
	printPluginInvocations(postInvocations)
	if postErr != nil {
		fmt.Println("Post-commit plugin execution failed:", postErr)
	}
}
