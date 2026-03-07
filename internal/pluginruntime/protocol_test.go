package pluginruntime

import (
	"context"
	"strings"
	"testing"
)

func TestParseManifestRejectsUnsupportedProtocol(t *testing.T) {
	raw := []byte(`{
  "api_version": "goodcommit.io/v1",
  "kind": "Plugin",
  "id": "test/protocol",
  "version": "1.0.0",
  "protocol_versions": ["2.0"],
  "entrypoint": {
    "type": "exec",
    "command": "echo"
  },
  "hooks": ["collect"]
}`)

	_, err := ParseManifest(raw)
	if err == nil {
		t.Fatalf("expected ParseManifest() error")
	}
	if !strings.Contains(err.Error(), "does not support protocol version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvokeRejectsUnsupportedProtocolWhenManifestConstructedProgrammatically(t *testing.T) {
	runner := NewRunner()
	rp := RuntimePlugin{
		Manifest: Manifest{
			APIVersion:       "goodcommit.io/v1",
			Kind:             "Plugin",
			ID:               "test/protocol",
			Version:          "1.0.0",
			ProtocolVersions: []string{"2.0"},
			Entrypoint: EntryPoint{
				Type:    "exec",
				Command: "echo",
			},
			Hooks: []HookPhase{HookCollect},
		},
	}
	req := Request{
		ProtocolVersion: ProtocolVersionV1,
		RequestID:       "req-1",
		PluginID:        "test/protocol",
		Hook:            HookCollect,
		Context: RequestContext{
			RepoRoot: t.TempDir(),
		},
		Draft: CommitDraft{Metadata: map[string]interface{}{}},
	}

	_, err := runner.Invoke(context.Background(), rp, req)
	if err == nil {
		t.Fatalf("expected Invoke() error")
	}
	if !strings.Contains(err.Error(), "does not support protocol version") {
		t.Fatalf("unexpected error: %v", err)
	}
}
