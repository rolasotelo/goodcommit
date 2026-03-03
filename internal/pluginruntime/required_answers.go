package pluginruntime

import (
	"errors"
	"fmt"
	"strings"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

type requiredAnswerError struct {
	pluginID string
	missing  []string
}

func (e *requiredAnswerError) Error() string {
	return fmt.Sprintf("plugin %s missing required answers: %s", e.pluginID, strings.Join(e.missing, ", "))
}

func isRequiredAnswerError(err error) bool {
	var target *requiredAnswerError
	return errors.As(err, &target)
}

func applyRequiredAnswerOverrides(manifest Manifest, requiredAnswers []string) (Manifest, error) {
	if len(requiredAnswers) == 0 {
		return manifest, nil
	}
	if manifest.Contract == nil || len(manifest.Contract.Answers) == 0 {
		return Manifest{}, fmt.Errorf("plugin has no contract.answers to override")
	}

	contract := *manifest.Contract
	contract.Answers = append([]api.AIAnswerSpec(nil), manifest.Contract.Answers...)

	indexByKey := make(map[string]int, len(contract.Answers))
	available := make([]string, 0, len(contract.Answers))
	for i, answer := range contract.Answers {
		key := strings.TrimSpace(answer.Key)
		if key == "" {
			continue
		}
		if _, exists := indexByKey[key]; !exists {
			indexByKey[key] = i
			available = append(available, key)
		}
	}

	for _, rawKey := range requiredAnswers {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return Manifest{}, fmt.Errorf("required_answers contains an empty key")
		}
		idx, ok := indexByKey[key]
		if !ok {
			return Manifest{}, fmt.Errorf("unknown answer key %q (available: %s)", key, strings.Join(available, ", "))
		}
		contract.Answers[idx].Required = true
	}

	manifest.Contract = &contract
	return manifest, nil
}

func validateRequiredAnswersProvided(rp RuntimePlugin, answers map[string]interface{}) error {
	if rp.Manifest.Contract == nil {
		return nil
	}
	missing := []string{}
	for _, spec := range rp.Manifest.Contract.Answers {
		if !spec.Required {
			continue
		}
		if !answerValuePresent(answers, spec.Key, spec.Type) {
			missing = append(missing, spec.Key)
		}
	}
	if len(missing) > 0 {
		return &requiredAnswerError{pluginID: rp.Manifest.ID, missing: missing}
	}
	return nil
}

func answerValuePresent(answers map[string]interface{}, key, kind string) bool {
	if answers == nil {
		return false
	}
	value, ok := answers[key]
	if !ok || value == nil {
		return false
	}

	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "bool", "boolean":
		_, isBool := value.(bool)
		if isBool {
			return true
		}
		if s, ok := value.(string); ok {
			s = strings.TrimSpace(strings.ToLower(s))
			return s == "true" || s == "false"
		}
		return false
	case "[]string", "string[]", "array":
		switch v := value.(type) {
		case []string:
			return len(v) > 0
		case []interface{}:
			return len(v) > 0
		case string:
			return strings.TrimSpace(v) != ""
		default:
			return false
		}
	case "string":
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s) != ""
		}
		return false
	default:
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s) != ""
		}
		return true
	}
}
