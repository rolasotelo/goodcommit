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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
	"golang.org/x/term"
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

	reqCtx := gatherRequestContext()
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

func runPluginPhases(ctx context.Context, runner *plugins.Runner, runtimePlugins []plugins.RuntimePlugin, reqCtx plugins.RequestContext, draft *plugins.CommitDraft, phases []plugins.HookPhase) ([]plugins.Invocation, error) {
	all := []plugins.Invocation{}
	for _, phase := range phases {
		invocations, err := runner.RunPhase(ctx, phase, draft, reqCtx, runtimePlugins)
		all = append(all, invocations...)
		if err != nil {
			return all, err
		}
		for _, inv := range invocations {
			if inv.Response.BlockCommit {
				reason := inv.Response.BlockReason
				if reason == "" {
					reason = "blocked by plugin"
				}
				return all, fmt.Errorf("commit blocked by %s: %s", inv.PluginID, reason)
			}
			if inv.Response.Fatal {
				return all, fmt.Errorf("fatal plugin response from %s", inv.PluginID)
			}
		}
	}
	return all, nil
}

func printPluginInvocations(invocations []plugins.Invocation) {
	for _, inv := range invocations {
		for _, d := range inv.Response.Diagnostics {
			fmt.Printf("[plugin:%s][%s][%s] %s\n", inv.PluginID, inv.Hook, d.Level, d.Message)
		}
		if inv.Stderr != "" {
			fmt.Printf("[plugin:%s][stderr] %s\n", inv.PluginID, strings.TrimSpace(inv.Stderr))
		}
	}
}

func makePromptResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) plugins.PromptHandler {
	return func(pluginID string, promptRequests []plugins.PromptRequest) (map[string]interface{}, error) {
		return resolvePromptRequests(pluginID, promptRequests, predefined, autoByPlugin, accessible)
	}
}

func makeUIResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) plugins.UIHandler {
	return func(pluginID string, forms []plugins.UIRequest) (map[string]interface{}, error) {
		return resolveUIRequests(pluginID, forms, predefined, autoByPlugin, accessible)
	}
}

func makeGroupedUIResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool, detailed bool) plugins.GroupedUIHandler {
	return func(groupID string, requests []plugins.PluginUIBatchRequest) (map[string]map[string]interface{}, error) {
		return resolveGroupedUIRequests(groupID, requests, predefined, autoByPlugin, accessible, detailed)
	}
}

func renderDraft(d plugins.CommitDraft) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(d.Title))
	if d.Body != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(d.Body, "\n"))
	}
	if len(d.Trailers) > 0 {
		b.WriteString("\n\n")
		for i, t := range d.Trailers {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimSpace(t.Key))
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(t.Value))
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func resolveUIRequests(pluginID string, forms []plugins.UIRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) (map[string]interface{}, error) {
	answers := map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	for _, ui := range forms {
		if !isTTY {
			for _, f := range ui.Fields {
				if f.Type == "note" {
					continue
				}
				raw, ok := findPredefined(predefined, ui.ID, f.ID)
				if !ok {
					if autoVal, found := findAutoAnswer(autoByPlugin, pluginID, ui.ID, f.ID); found {
						answers[f.ID] = autoVal
						continue
					}
					if f.Required {
						return nil, fmt.Errorf("form field %s.%s requires TTY (or pass --plugin-answer %s.%s=...)", ui.ID, f.ID, ui.ID, f.ID)
					}
					continue
				}
				value, err := parsePredefinedAnswer(raw, f.Type)
				if err != nil {
					return nil, fmt.Errorf("plugin %s field %s.%s invalid predefined answer: %w", pluginID, ui.ID, f.ID, err)
				}
				answers[f.ID] = value
			}
			answers[ui.ID+".__submitted"] = true
			continue
		}

		var fields []huh.Field
		local := map[string]interface{}{}
		for _, f := range ui.Fields {
			switch f.Type {
			case "note":
				fields = append(fields, huh.NewNote().Title(f.Title).Description(f.Description))
			case "input":
				v := f.Value
				input := huh.NewInput().Title(f.Title).Description(f.Description).Placeholder(f.Placeholder).Value(&v)
				if f.CharLimit > 0 {
					input = input.CharLimit(f.CharLimit)
				}
				if f.Required {
					input = input.Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("value is required")
						}
						return nil
					})
				}
				fields = append(fields, input)
				local[f.ID] = &v
			case "text":
				v := f.Value
				text := huh.NewText().Title(f.Title).Description(f.Description).Placeholder(f.Placeholder).Value(&v)
				if f.Editor {
					text = text.Editor("vim")
				}
				fields = append(fields, text)
				local[f.ID] = &v
			case "confirm":
				var v bool
				fields = append(fields, huh.NewConfirm().Title(f.Title).Description(f.Description).Value(&v))
				local[f.ID] = &v
			case "select":
				opts := make([]huh.Option[string], 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, huh.NewOption(o.Label, o.Value))
				}
				v := ""
				fields = append(fields, huh.NewSelect[string]().Title(f.Title).Description(f.Description).Options(opts...).Value(&v))
				local[f.ID] = &v
			case "multiselect":
				opts := make([]huh.Option[string], 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, huh.NewOption(o.Label, o.Value))
				}
				v := []string{}
				fields = append(fields, huh.NewMultiSelect[string]().Title(f.Title).Description(f.Description).Options(opts...).Value(&v))
				local[f.ID] = &v
			default:
				return nil, fmt.Errorf("plugin %s field %s.%s has unsupported type %q", pluginID, ui.ID, f.ID, f.Type)
			}
		}
		form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(accessible)
		if err := form.Run(); err != nil {
			return nil, fmt.Errorf("plugin %s form %s failed: %w", pluginID, ui.ID, err)
		}
		for id, ref := range local {
			switch x := ref.(type) {
			case *string:
				answers[id] = *x
			case *bool:
				answers[id] = *x
			case *[]string:
				answers[id] = *x
			}
		}
		answers[ui.ID+".__submitted"] = true
	}

	return answers, nil
}

func resolveGroupedUIRequests(groupID string, requests []plugins.PluginUIBatchRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool, detailed bool) (map[string]map[string]interface{}, error) {
	answersByPlugin := map[string]map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	for _, req := range requests {
		if _, ok := answersByPlugin[req.PluginID]; !ok {
			answersByPlugin[req.PluginID] = map[string]interface{}{}
		}
	}

	if !isTTY {
		for _, req := range requests {
			for _, ui := range req.Forms {
				for _, f := range ui.Fields {
					if f.Type == "note" {
						continue
					}
					raw, ok := findGroupedPredefined(predefined, req.PluginID, ui.ID, f.ID)
					if !ok {
						if autoVal, found := findAutoAnswer(autoByPlugin, req.PluginID, ui.ID, f.ID); found {
							answersByPlugin[req.PluginID][f.ID] = autoVal
							continue
						}
						if f.Required {
							return nil, fmt.Errorf("group %s field %s.%s.%s requires TTY (or pass --plugin-answer %s.%s.%s=...)", groupID, req.PluginID, ui.ID, f.ID, req.PluginID, ui.ID, f.ID)
						}
						continue
					}
					value, err := parsePredefinedAnswer(raw, f.Type)
					if err != nil {
						return nil, fmt.Errorf("plugin %s field %s.%s invalid predefined answer: %w", req.PluginID, ui.ID, f.ID, err)
					}
					answersByPlugin[req.PluginID][f.ID] = value
				}
				answersByPlugin[req.PluginID][ui.ID+".__submitted"] = true
			}
		}
		return answersByPlugin, nil
	}

	type localFieldRef struct {
		pluginID string
		fieldID  string
		ref      interface{}
	}
	local := []localFieldRef{}
	fields := []huh.Field{}

	for _, req := range requests {
		for _, ui := range req.Forms {
			if detailed {
				sectionTitle := ui.Title
				if strings.TrimSpace(sectionTitle) == "" {
					sectionTitle = ui.ID
				}
				sectionDesc := ui.Description
				if sectionDesc == "" {
					sectionDesc = "Fill required fields and submit."
				}
				fields = append(fields, huh.NewNote().Title(fmt.Sprintf("%s - %s", req.PluginID, sectionTitle)).Description(sectionDesc))
			}

			firstPromptField := true
			for _, f := range ui.Fields {
				fieldDesc := f.Description
				if !detailed && firstPromptField && f.Type != "note" && strings.TrimSpace(fieldDesc) == "" {
					fieldDesc = strings.TrimSpace(ui.Description)
				}
				switch f.Type {
				case "note":
					fields = append(fields, huh.NewNote().Title(f.Title).Description(fieldDesc))
				case "input":
					v := f.Value
					input := huh.NewInput().Title(f.Title).Description(fieldDesc).Placeholder(f.Placeholder).Value(&v)
					if f.CharLimit > 0 {
						input = input.CharLimit(f.CharLimit)
					}
					if f.Required {
						input = input.Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("value is required")
							}
							return nil
						})
					}
					fields = append(fields, input)
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "text":
					v := f.Value
					text := huh.NewText().Title(f.Title).Description(fieldDesc).Placeholder(f.Placeholder).Value(&v)
					if f.Editor {
						text = text.Editor("vim")
					}
					fields = append(fields, text)
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "confirm":
					var v bool
					fields = append(fields, huh.NewConfirm().Title(f.Title).Description(fieldDesc).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "select":
					opts := make([]huh.Option[string], 0, len(f.Options))
					for _, o := range f.Options {
						opts = append(opts, huh.NewOption(o.Label, o.Value))
					}
					v := ""
					fields = append(fields, huh.NewSelect[string]().Title(f.Title).Description(fieldDesc).Options(opts...).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "multiselect":
					opts := make([]huh.Option[string], 0, len(f.Options))
					for _, o := range f.Options {
						opts = append(opts, huh.NewOption(o.Label, o.Value))
					}
					v := []string{}
					fields = append(fields, huh.NewMultiSelect[string]().Title(f.Title).Description(fieldDesc).Options(opts...).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				default:
					return nil, fmt.Errorf("plugin %s field %s.%s has unsupported type %q", req.PluginID, ui.ID, f.ID, f.Type)
				}
			}
		}
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("grouped form %s failed: %w", groupID, err)
	}

	for _, item := range local {
		switch x := item.ref.(type) {
		case *string:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		case *bool:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		case *[]string:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		}
	}
	for _, req := range requests {
		for _, ui := range req.Forms {
			answersByPlugin[req.PluginID][ui.ID+".__submitted"] = true
		}
	}

	return answersByPlugin, nil
}

func resolvePromptRequests(pluginID string, promptRequests []plugins.PromptRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) (map[string]interface{}, error) {
	answers := map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	for _, p := range promptRequests {
		if predefined != nil {
			if raw, ok := predefined[p.ID]; ok {
				value, err := parsePredefinedAnswer(raw, p.Type)
				if err != nil {
					return nil, fmt.Errorf("plugin %s prompt %s invalid predefined answer: %w", pluginID, p.ID, err)
				}
				answers[p.ID] = value
				continue
			}
		}
		if autoVal, ok := findAutoAnswer(autoByPlugin, pluginID, "", p.ID); ok {
			answers[p.ID] = autoVal
			continue
		}
		if !isTTY {
			return nil, fmt.Errorf("prompt %s requires TTY (or pass --plugin-answer %s=...)", p.ID, p.ID)
		}
		switch p.Type {
		case "input":
			var value string
			field := huh.NewInput().Title(p.Title).Description(p.Description).Value(&value)
			if p.Required {
				field = field.Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("value is required")
					}
					return nil
				})
			}
			form := huh.NewForm(huh.NewGroup(field)).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "confirm":
			var value bool
			form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(p.Title).Description(p.Description).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "select":
			if len(p.Options) == 0 {
				return nil, fmt.Errorf("plugin %s prompt %s has no options", pluginID, p.ID)
			}
			opts := make([]huh.Option[string], 0, len(p.Options))
			for _, o := range p.Options {
				opts = append(opts, huh.NewOption(o.Label, o.Value))
			}
			var value string
			form := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(p.Title).Description(p.Description).Options(opts...).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "multiselect":
			if len(p.Options) == 0 {
				return nil, fmt.Errorf("plugin %s prompt %s has no options", pluginID, p.ID)
			}
			opts := make([]huh.Option[string], 0, len(p.Options))
			for _, o := range p.Options {
				opts = append(opts, huh.NewOption(o.Label, o.Value))
			}
			var value []string
			form := huh.NewForm(huh.NewGroup(huh.NewMultiSelect[string]().Title(p.Title).Description(p.Description).Options(opts...).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		default:
			return nil, fmt.Errorf("plugin %s prompt %s has unsupported type %q", pluginID, p.ID, p.Type)
		}
	}
	return answers, nil
}

func findAutoAnswer(autoByPlugin map[string]map[string]interface{}, pluginID, formID, fieldID string) (interface{}, bool) {
	if autoByPlugin == nil {
		return nil, false
	}
	answers, ok := autoByPlugin[pluginID]
	if !ok || answers == nil {
		return nil, false
	}
	if formID != "" {
		if v, ok := answers[formID+"."+fieldID]; ok {
			return v, true
		}
	}
	v, ok := answers[fieldID]
	return v, ok
}

var trailerLineRe = regexp.MustCompile(`^[A-Za-z0-9-]+:\s+.+$`)

func draftFromMessage(message string) plugins.CommitDraft {
	msg := strings.ReplaceAll(message, "\r\n", "\n")
	msg = strings.TrimRight(msg, "\n")
	parts := strings.SplitN(msg, "\n", 2)
	title := ""
	rest := ""
	if len(parts) > 0 {
		title = parts[0]
	}
	if len(parts) == 2 {
		rest = strings.TrimLeft(parts[1], "\n")
	}

	body := rest
	trailers := []plugins.Trailer{}
	if rest != "" {
		lines := strings.Split(rest, "\n")
		i := len(lines) - 1
		for i >= 0 && strings.TrimSpace(lines[i]) == "" {
			i--
		}
		start := i
		for start >= 0 && trailerLineRe.MatchString(lines[start]) {
			start--
		}
		start++
		if start <= i && start >= 0 {
			for _, tl := range lines[start : i+1] {
				kv := strings.SplitN(tl, ":", 2)
				if len(kv) == 2 {
					trailers = append(trailers, plugins.Trailer{Key: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])})
				}
			}
			body = strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
		}
	}

	return plugins.CommitDraft{Title: title, Body: body, Trailers: trailers, Metadata: map[string]interface{}{}}
}

func parsePredefinedAnswer(raw, promptType string) (interface{}, error) {
	switch promptType {
	case "input", "select", "text":
		return raw, nil
	case "confirm":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected boolean, got %q", raw)
		}
		return v, nil
	case "multiselect":
		if strings.TrimSpace(raw) == "" {
			return []string{}, nil
		}
		items := strings.Split(raw, ",")
		out := make([]string, 0, len(items))
		for _, i := range items {
			out = append(out, strings.TrimSpace(i))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported prompt type %q", promptType)
	}
}

func gatherRequestContext() plugins.RequestContext {
	staged := []string{}
	if out, err := gitOutput("diff", "--cached", "--name-only"); err == nil && out != "" {
		staged = strings.Split(strings.TrimSpace(out), "\n")
	}
	repoRoot, _ := gitOutput("rev-parse", "--show-toplevel")
	branch, _ := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	head, _ := gitOutput("rev-parse", "HEAD")
	name, _ := gitOutput("config", "--get", "user.name")
	email, _ := gitOutput("config", "--get", "user.email")

	return plugins.RequestContext{
		RepoRoot:     strings.TrimSpace(repoRoot),
		Branch:       strings.TrimSpace(branch),
		Head:         strings.TrimSpace(head),
		StagedFiles:  staged,
		GitUserName:  strings.TrimSpace(name),
		GitUserEmail: strings.TrimSpace(email),
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
	}
}

func findPredefined(predefined map[string]string, formID, fieldID string) (string, bool) {
	if predefined == nil {
		return "", false
	}
	if v, ok := predefined[formID+"."+fieldID]; ok {
		return v, true
	}
	v, ok := predefined[fieldID]
	return v, ok
}

func findGroupedPredefined(predefined map[string]string, pluginID, formID, fieldID string) (string, bool) {
	if predefined == nil {
		return "", false
	}
	if v, ok := predefined[pluginID+"."+formID+"."+fieldID]; ok {
		return v, true
	}
	if v, ok := predefined[pluginID+"."+fieldID]; ok {
		return v, true
	}
	return findPredefined(predefined, formID, fieldID)
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func retryMessageFilePath() string {
	if out, err := gitOutput("rev-parse", "--git-dir"); err == nil {
		gitDir := strings.TrimSpace(out)
		if gitDir != "" {
			return filepath.Join(gitDir, "goodcommit", "last_failed_commit_message.txt")
		}
	}
	return filepath.Join(".git", "goodcommit", "last_failed_commit_message.txt")
}

func retryMessageFilePathForRead() string {
	return retryMessageFilePath()
}

func cleanupRetryMessageFiles() {
	p := retryMessageFilePath()
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Warning: could not remove stale retry message file %s: %s\n", p, err)
	}
}

func runInitSubcommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	pluginsConfigPath := fs.String("plugins-config", "./configs/goodcommit.plugins.json", "Path to write plugin config")
	typesConfigPath := fs.String("types-config", "./configs/commit-types.json", "Path to write commit types config")
	pluginsLockfilePath := fs.String("plugins-lockfile", "goodcommit.plugins.lock", "Path to write plugin lockfile")
	force := fs.Bool("force", false, "Overwrite scaffold files if they already exist")
	withLock := fs.Bool("lock", true, "Generate plugin lockfile after scaffolding")
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
			fmt.Printf("Warning: scaffold created but lock step failed to load plugins config: %v\n", err)
		} else if lf, err := plugins.BuildLockfileWithArtifacts(resolved, *pluginsLockfilePath); err != nil {
			fmt.Printf("Warning: scaffold created but lock step failed: %v\n", err)
		} else if err := plugins.WriteLockfile(*pluginsLockfilePath, lf); err != nil {
			fmt.Printf("Warning: scaffold created but lockfile write failed: %v\n", err)
		} else {
			created = append(created, *pluginsLockfilePath)
			lockCreated = true
		}
	}

	fmt.Println("Initialized goodcommit scaffold:")
	for _, p := range created {
		fmt.Printf("- %s\n", p)
	}
	if !*withLock || !lockCreated {
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
		lf, err := plugins.BuildLockfileWithArtifacts(resolved, *pluginsLockfilePath)
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

func runGitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\nOutput:\n%s", err, string(output))
	}
	return nil
}

func buildAutoAnswersByPlugin(runtimePlugins []plugins.RuntimePlugin) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	for _, rp := range runtimePlugins {
		if len(rp.AIAuto) == 0 {
			continue
		}
		out[rp.Manifest.ID] = rp.AIAuto
	}
	return out
}
