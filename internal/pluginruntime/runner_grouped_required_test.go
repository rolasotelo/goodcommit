package pluginruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

func TestRunGroupedUIPluginsEnforcesRequiredAnswersWhenNoUIReturned(t *testing.T) {
	repoDir := t.TempDir()
	pluginScript := filepath.Join(t.TempDir(), "plugin-no-ui.sh")
	script := `#!/bin/sh
req="$(cat)"
id=$(printf "%s" "$req" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
printf '{"request_id":"%s","ok":true,"diagnostics":[]}\n' "$id"
`
	if err := os.WriteFile(pluginScript, []byte(script), 0o755); err != nil {
		t.Fatalf("write plugin script: %v", err)
	}

	runner := NewRunner()
	draft := CommitDraft{Metadata: map[string]interface{}{}}
	reqCtx := RequestContext{RepoRoot: repoDir}
	rp := RuntimePlugin{
		Manifest: Manifest{
			APIVersion: "goodcommit.io/v1",
			Kind:       "Plugin",
			ID:         "test/no-ui",
			Version:    "1.0.0",
			Entrypoint: EntryPoint{
				Type:    "exec",
				Command: pluginScript,
			},
			Hooks: []HookPhase{HookCollect},
			Contract: &api.PluginContract{
				Answers: []api.AIAnswerSpec{
					{Key: "commit_body", Type: "string", Required: true},
				},
			},
		},
		FailureMode: FailOpen,
	}

	results, stop, err := runner.runGroupedUIPlugins(context.Background(), HookCollect, &draft, reqCtx, "compose", []RuntimePlugin{rp})
	if err == nil {
		t.Fatalf("expected missing required answers error")
	}
	if !isRequiredAnswerError(err) {
		t.Fatalf("expected required-answer error type, got: %v", err)
	}
	if !strings.Contains(err.Error(), "commit_body") {
		t.Fatalf("expected missing commit_body in error, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no successful results, got %d", len(results))
	}
	if stop {
		t.Fatalf("expected stop=false on error path")
	}
}
