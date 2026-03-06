package pluginruntime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

func TestRunPhaseSkipsRequiredAnswersOnNonCollectHooks(t *testing.T) {
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
			ID:         "test/scopes-like",
			Version:    "1.0.0",
			Entrypoint: EntryPoint{
				Type:    "exec",
				Command: pluginScript,
			},
			Hooks: []HookPhase{HookCollect, HookEnrich},
			Contract: &api.PluginContract{
				Answers: []api.AIAnswerSpec{
					{Key: "commit_scopes", Type: "[]string", Required: true},
				},
			},
		},
		FailureMode: FailClosed,
	}

	results, err := runner.RunPhase(context.Background(), HookEnrich, &draft, reqCtx, []RuntimePlugin{rp})
	if err != nil {
		t.Fatalf("RunPhase(HookEnrich) unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("RunPhase(HookEnrich) results = %d, want 1", len(results))
	}
}
