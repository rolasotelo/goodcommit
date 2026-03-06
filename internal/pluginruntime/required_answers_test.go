package pluginruntime

import (
	"strings"
	"testing"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

func TestApplyRequiredAnswerOverrides(t *testing.T) {
	manifest := Manifest{
		ID: "example/plugin",
		Contract: &api.PluginContract{
			Answers: []api.AIAnswerSpec{
				{Key: "commit_description", Type: "string"},
				{Key: "commit_body", Type: "string"},
			},
		},
	}

	updated, err := applyRequiredAnswerOverrides(manifest, []string{"commit_body"})
	if err != nil {
		t.Fatalf("applyRequiredAnswerOverrides() error = %v", err)
	}
	if updated.Contract == nil || len(updated.Contract.Answers) != 2 {
		t.Fatalf("updated manifest contract missing answers")
	}
	if updated.Contract.Answers[0].Required {
		t.Fatalf("commit_description should remain optional")
	}
	if !updated.Contract.Answers[1].Required {
		t.Fatalf("commit_body should be required after override")
	}
}

func TestApplyRequiredAnswerOverridesUnknownKey(t *testing.T) {
	manifest := Manifest{
		ID: "example/plugin",
		Contract: &api.PluginContract{
			Answers: []api.AIAnswerSpec{
				{Key: "commit_description", Type: "string"},
			},
		},
	}

	_, err := applyRequiredAnswerOverrides(manifest, []string{"commit_body"})
	if err == nil {
		t.Fatalf("expected error for unknown required_answers key")
	}
	if !strings.Contains(err.Error(), "unknown answer key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRequiredAnswersProvided(t *testing.T) {
	rp := RuntimePlugin{
		Manifest: Manifest{
			ID: "example/plugin",
			Contract: &api.PluginContract{
				Answers: []api.AIAnswerSpec{
					{Key: "commit_body", Type: "string", Required: true},
					{Key: "is_breaking", Type: "bool", Required: true},
				},
			},
		},
	}

	if err := validateRequiredAnswersProvided(HookCollect, rp, map[string]interface{}{
		"commit_body": "details",
		"is_breaking": false,
	}); err != nil {
		t.Fatalf("validateRequiredAnswersProvided() unexpected error: %v", err)
	}

	err := validateRequiredAnswersProvided(HookCollect, rp, map[string]interface{}{
		"commit_body": "",
		"is_breaking": true,
	})
	if err == nil || !strings.Contains(err.Error(), "commit_body") {
		t.Fatalf("expected missing commit_body error, got: %v", err)
	}
	if !isRequiredAnswerError(err) {
		t.Fatalf("expected required-answer classification")
	}
}

func TestValidateRequiredAnswersProvidedSkipsNonCollectHooks(t *testing.T) {
	rp := RuntimePlugin{
		Manifest: Manifest{
			ID: "example/plugin",
			Contract: &api.PluginContract{
				Answers: []api.AIAnswerSpec{
					{Key: "commit_scopes", Type: "[]string", Required: true},
				},
			},
		},
	}

	if err := validateRequiredAnswersProvided(HookEnrich, rp, nil); err != nil {
		t.Fatalf("validateRequiredAnswersProvided() should skip non-collect hooks: %v", err)
	}
}
