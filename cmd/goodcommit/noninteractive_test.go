package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoodcommitHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_GOODCOMMIT_HELPER") != "1" {
		return
	}

	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 {
		os.Exit(2)
	}

	os.Args = append([]string{"goodcommit"}, os.Args[sep+1:]...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	main()
	os.Exit(0)
}

func TestGoodcommitNonInteractiveSimpleFlow(t *testing.T) {
	repoDir := setupGoodcommitRepo(t)
	writeJSONFile(t, filepath.Join(repoDir, "configs", "commit-types.json"), map[string]any{
		"types": []map[string]any{
			{"id": "feat", "name": "Feat", "title": "Feature", "emoji": "✨"},
		},
	})

	typesPlugin := builtinPluginConfig(t, "types", 10, "fail_closed", map[string]any{"path": "./configs/commit-types.json"})
	descriptionPlugin := builtinPluginConfig(t, "description", 20, "fail_closed", map[string]any{})
	bodyPlugin := builtinPluginConfig(t, "body", 30, "fail_closed", map[string]any{})
	bodyPlugin["required_answers"] = []string{"commit_body"}
	titlePlugin := builtinPluginConfig(t, "conventional-title", 90, "fail_closed", map[string]any{})
	signoffPlugin := builtinPluginConfig(t, "signedoffby", 100, "fail_closed", map[string]any{"trailer_key": "Signed-off-by"})

	writeJSONFile(t, filepath.Join(repoDir, "configs", "goodcommit.plugins.json"), map[string]any{
		"plugins": []map[string]any{
			typesPlugin,
			descriptionPlugin,
			bodyPlugin,
			titlePlugin,
			signoffPlugin,
		},
	})

	lockGoodcommitPlugins(t, repoDir)

	stdout, stderr, code := runGoodcommit(t, repoDir,
		"--plugins-config", "./configs/goodcommit.plugins.json",
		"--plugins-lockfile", "./goodcommit.plugins.lock",
		"-m",
		"--plugin-answer", "commit_type=feat",
		"--plugin-answer", "commit_description=Ñormalize protocol handling",
		"--plugin-answer", "commit_body=áccent aware normalization",
	)
	if code != 0 {
		t.Fatalf("goodcommit exited with %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	got := finalMessageFromOutput(t, stdout)
	want := "feat: ñormalize protocol handling\n\nÁccent aware normalization.\n\nSigned-off-by: Test User <test@example.com>\n"
	if got != want {
		t.Fatalf("final message mismatch\nstdout:\n%s\nwant:\n%q\ngot:\n%q", stdout, want, got)
	}
}

func TestGoodcommitNonInteractiveGroupedBreakingFlow(t *testing.T) {
	repoDir := setupGoodcommitRepo(t)
	writeFile(t, filepath.Join(repoDir, "configs", "logo.ascii.txt"), "GOODCOMMIT\n")
	writeJSONFile(t, filepath.Join(repoDir, "configs", "commit-types.json"), map[string]any{
		"types": []map[string]any{
			{"id": "feat", "name": "Feat", "title": "Feature", "emoji": "✨"},
		},
	})
	writeJSONFile(t, filepath.Join(repoDir, "configs", "commit-scopes.json"), map[string]any{
		"scopes": []map[string]any{
			{"id": "api", "name": "API", "description": "External interface", "emoji": "🚀", "conditional": []string{"feat"}},
			{"id": "cli", "name": "CLI", "description": "Command-line UX", "emoji": "🖥", "conditional": []string{"feat"}},
		},
	})
	writeJSONFile(t, filepath.Join(repoDir, "configs", "commit-coauthors.json"), map[string]any{
		"coauthors": []map[string]any{
			{"id": "test@example.com", "name": "Test User", "emoji": "🧑"},
			{"id": "pair@example.com", "name": "Pair Programmer", "emoji": "🤝"},
		},
	})

	logoPlugin := builtinPluginConfig(t, "logo", 1, "fail_open", map[string]any{"path": "./configs/logo.ascii.txt"})
	logoPlugin["ui_group"] = "intro"
	typesPlugin := builtinPluginConfig(t, "types", 10, "fail_closed", map[string]any{"path": "./configs/commit-types.json"})
	typesPlugin["ui_group"] = "intro"
	scopesPlugin := builtinPluginConfig(t, "scopes", 20, "fail_closed", map[string]any{"path": "./configs/commit-scopes.json"})
	descriptionPlugin := builtinPluginConfig(t, "description", 30, "fail_closed", map[string]any{})
	descriptionPlugin["ui_group"] = "compose_message"
	whyPlugin := builtinPluginConfig(t, "why", 40, "fail_closed", map[string]any{"required": false})
	whyPlugin["ui_group"] = "compose_message"
	bodyPlugin := builtinPluginConfig(t, "body", 50, "fail_closed", map[string]any{})
	bodyPlugin["ui_group"] = "compose_message"
	bodyPlugin["required_answers"] = []string{"commit_body"}
	breakingPlugin := builtinPluginConfig(t, "breaking", 60, "fail_closed", map[string]any{})
	breakingPlugin["ui_group"] = "compose_message"
	breakingMsgPlugin := builtinPluginConfig(t, "breakingmsg", 70, "fail_closed", map[string]any{})
	coauthorsPlugin := builtinPluginConfig(t, "coauthors", 80, "fail_closed", map[string]any{"path": "./configs/commit-coauthors.json"})
	titlePlugin := builtinPluginConfig(t, "conventional-title", 90, "fail_closed", map[string]any{})
	signoffPlugin := builtinPluginConfig(t, "signedoffby", 100, "fail_closed", map[string]any{"trailer_key": "Signed-off-by"})

	writeJSONFile(t, filepath.Join(repoDir, "configs", "goodcommit.plugins.json"), map[string]any{
		"plugins": []map[string]any{
			logoPlugin,
			typesPlugin,
			scopesPlugin,
			descriptionPlugin,
			whyPlugin,
			bodyPlugin,
			breakingPlugin,
			breakingMsgPlugin,
			coauthorsPlugin,
			titlePlugin,
			signoffPlugin,
		},
	})

	lockGoodcommitPlugins(t, repoDir)

	stdout, stderr, code := runGoodcommit(t, repoDir,
		"--plugins-config", "./configs/goodcommit.plugins.json",
		"--plugins-lockfile", "./goodcommit.plugins.lock",
		"-m",
		"--plugin-answer", "builtin/types.types.commit_type=feat",
		"--plugin-answer", "commit_scopes=api,cli",
		"--plugin-answer", "commit_description=Introduce protocol support",
		"--plugin-answer", "commit_why=avoid confusing runtime failures",
		"--plugin-answer", "builtin/body.body.commit_body=document plugin compatibility changes",
		"--plugin-answer", "builtin/breaking.breaking.is_breaking=true",
		"--plugin-answer", "breaking_message=clients must regenerate lockfiles before upgrading",
		"--plugin-answer", "coauthors_selected=pair@example.com",
	)
	if code != 0 {
		t.Fatalf("goodcommit exited with %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	got := finalMessageFromOutput(t, stdout)
	want := strings.Join([]string{
		"feat(🚀🖥)!: introduce protocol support",
		"",
		"WHY: Avoid confusing runtime failures.",
		"",
		"SCOPES: API / CLI",
		"",
		"Document plugin compatibility changes.",
		"",
		"BREAKING CHANGE: Clients must regenerate lockfiles before upgrading.",
		"",
		"🧑 🤝",
		"",
		"Co-authored-by: Pair Programmer <pair@example.com>",
		"Signed-off-by: Test User <test@example.com>",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("final message mismatch\nstdout:\n%s\nwant:\n%q\ngot:\n%q", stdout, want, got)
	}
}

func TestGoodcommitNonInteractiveMissingRequiredAnswerFails(t *testing.T) {
	repoDir := setupGoodcommitRepo(t)
	writeJSONFile(t, filepath.Join(repoDir, "configs", "commit-types.json"), map[string]any{
		"types": []map[string]any{
			{"id": "feat", "name": "Feat", "title": "Feature", "emoji": "✨"},
		},
	})

	typesPlugin := builtinPluginConfig(t, "types", 10, "fail_closed", map[string]any{"path": "./configs/commit-types.json"})
	descriptionPlugin := builtinPluginConfig(t, "description", 20, "fail_closed", map[string]any{})
	descriptionPlugin["ui_group"] = "compose_message"
	bodyPlugin := builtinPluginConfig(t, "body", 30, "fail_closed", map[string]any{})
	bodyPlugin["ui_group"] = "compose_message"
	bodyPlugin["required_answers"] = []string{"commit_body"}
	titlePlugin := builtinPluginConfig(t, "conventional-title", 90, "fail_closed", map[string]any{})

	writeJSONFile(t, filepath.Join(repoDir, "configs", "goodcommit.plugins.json"), map[string]any{
		"plugins": []map[string]any{
			typesPlugin,
			descriptionPlugin,
			bodyPlugin,
			titlePlugin,
		},
	})

	lockGoodcommitPlugins(t, repoDir)

	stdout, stderr, code := runGoodcommit(t, repoDir,
		"--plugins-config", "./configs/goodcommit.plugins.json",
		"--plugins-lockfile", "./goodcommit.plugins.lock",
		"-m",
		"--plugin-answer", "commit_type=feat",
		"--plugin-answer", "commit_description=Handle missing body",
	)
	if code == 0 {
		t.Fatalf("expected goodcommit failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "commit_body") || !strings.Contains(combined, "missing required answers") {
		t.Fatalf("unexpected failure output:\n%s", combined)
	}
}

func setupGoodcommitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	writeFile(t, filepath.Join(repoDir, "README.md"), "staged change\n")
	runGit(t, repoDir, "add", "README.md")
	return repoDir
}

func lockGoodcommitPlugins(t *testing.T, repoDir string) {
	t.Helper()

	stdout, stderr, code := runGoodcommit(t, repoDir,
		"plugin", "lock",
		"--plugins-config", "./configs/goodcommit.plugins.json",
		"--plugins-lockfile", "./goodcommit.plugins.lock",
		"--plugins-bin-dir", "project",
	)
	if code != 0 {
		t.Fatalf("plugin lock failed with %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	stdout, stderr, code = runGoodcommit(t, repoDir,
		"plugin", "verify",
		"--plugins-config", "./configs/goodcommit.plugins.json",
		"--plugins-lockfile", "./goodcommit.plugins.lock",
	)
	if code != 0 {
		t.Fatalf("plugin verify failed with %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func runGoodcommit(t *testing.T, repoDir string, args ...string) (string, string, int) {
	t.Helper()

	cmdArgs := append([]string{"-test.run=TestGoodcommitHelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GO_WANT_GOODCOMMIT_HELPER=1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("runGoodcommit() unexpected error: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func runGit(t *testing.T, repoDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func writeJSONFile(t *testing.T, path string, value interface{}) {
	t.Helper()

	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(%s) error = %v", path, err)
	}
	writeFile(t, path, string(append(raw, '\n')))
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func finalMessageFromOutput(t *testing.T, stdout string) string {
	t.Helper()

	const marker = "Final commit message:\n"
	idx := strings.Index(stdout, marker)
	if idx == -1 {
		t.Fatalf("final message marker not found in output:\n%s", stdout)
	}
	message := stdout[idx+len(marker):]
	return strings.TrimSuffix(message, "\n")
}

func builtinPluginConfig(t *testing.T, name string, order int, failureMode string, config map[string]any) map[string]any {
	t.Helper()

	return map[string]any{
		"id":           "builtin/" + name,
		"enabled":      true,
		"source":       map[string]any{"type": "path", "path": filepath.Join(repoRoot(t), "cmd", "goodcommit-plugin-"+name)},
		"order":        order,
		"failure_mode": failureMode,
		"timeout_ms":   10000,
		"config":       config,
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
