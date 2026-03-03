package pluginruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvedPluginsRequiredAnswersOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "plugins.json")
	raw := `{
  "plugins": [
    {
      "id": "builtin/body",
      "enabled": true,
      "required_answers": ["commit_body"],
      "order": 50,
      "failure_mode": "fail_open",
      "timeout_ms": 10000,
      "config": {}
    }
  ]
}`
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := LoadResolvedPlugins(configPath)
	if err != nil {
		t.Fatalf("LoadResolvedPlugins() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolved plugins = %d, want 1", len(resolved))
	}
	answers := resolved[0].Runtime.Manifest.Contract.Answers
	if len(answers) != 1 {
		t.Fatalf("contract answers = %d, want 1", len(answers))
	}
	if answers[0].Key != "commit_body" || !answers[0].Required {
		t.Fatalf("expected commit_body required=true, got key=%q required=%v", answers[0].Key, answers[0].Required)
	}
}

func TestLoadResolvedPluginsRequiredAnswersUnknownKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "plugins.json")
	raw := `{
  "plugins": [
    {
      "id": "builtin/body",
      "enabled": true,
      "required_answers": ["not_a_real_key"],
      "order": 50,
      "failure_mode": "fail_open",
      "timeout_ms": 10000,
      "config": {}
    }
  ]
}`
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadResolvedPlugins(configPath)
	if err == nil {
		t.Fatalf("expected error for invalid required_answers key")
	}
	if !strings.Contains(err.Error(), "unknown answer key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
