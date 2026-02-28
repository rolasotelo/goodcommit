package pluginruntime

import (
	"fmt"
	"sort"
	"strings"
)

// AIAnswerHint describes one answer key an agent may provide.
type AIAnswerHint struct {
	Key         string `json:"key"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
	Strategy    string `json:"strategy,omitempty"`
	FromPlugin  string `json:"from_plugin,omitempty"`
}

// AIPluginHint is a concise plugin instruction block for agents.
type AIPluginHint struct {
	ID           string                 `json:"id"`
	Purpose      string                 `json:"purpose,omitempty"`
	Instructions string                 `json:"instructions,omitempty"`
	AutoAnswers  map[string]interface{} `json:"auto_answers,omitempty"`
}

// AIContext is a simplified, non-interactive context payload for agents.
type AIContext struct {
	Version                  string         `json:"version"`
	Goal                     string         `json:"goal"`
	RequiredAnswers          []AIAnswerHint `json:"required_answers,omitempty"`
	OptionalAnswers          []AIAnswerHint `json:"optional_answers,omitempty"`
	PluginHints              []AIPluginHint `json:"plugin_hints,omitempty"`
	ExampleNonInteractiveRun string         `json:"example_non_interactive_run,omitempty"`
}

// BuildAIContext returns minimal guidance to run goodcommit successfully.
func BuildAIContext(resolved []ResolvedPlugin) AIContext {
	sorted := append([]ResolvedPlugin(nil), resolved...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Runtime.Order == sorted[j].Runtime.Order {
			return sorted[i].Runtime.Manifest.ID < sorted[j].Runtime.Manifest.ID
		}
		return sorted[i].Runtime.Order < sorted[j].Runtime.Order
	})

	required := []AIAnswerHint{}
	optional := []AIAnswerHint{}
	pluginHints := []AIPluginHint{}

	for _, rp := range sorted {
		hint := AIPluginHint{
			ID:          rp.Runtime.Manifest.ID,
			AutoAnswers: rp.Runtime.AIAuto,
		}
		if rp.Runtime.AIHints != nil {
			hint.Purpose = strings.TrimSpace(rp.Runtime.AIHints.Purpose)
			hint.Instructions = strings.TrimSpace(rp.Runtime.AIHints.Instructions)
		}
		if hint.Purpose != "" || hint.Instructions != "" || len(hint.AutoAnswers) > 0 {
			pluginHints = append(pluginHints, hint)
		}

		if rp.Runtime.Manifest.Contract == nil {
			continue
		}
		for _, a := range rp.Runtime.Manifest.Contract.Answers {
			item := AIAnswerHint{
				Key:         strings.TrimSpace(a.Key),
				Type:        strings.TrimSpace(a.Type),
				Required:    a.Required,
				Description: strings.TrimSpace(a.Description),
				Strategy:    strings.TrimSpace(a.Strategy),
				FromPlugin:  rp.Runtime.Manifest.ID,
			}
			if item.Key == "" {
				continue
			}
			if item.Required {
				required = append(required, item)
			} else {
				optional = append(optional, item)
			}
		}
	}

	ctx := AIContext{
		Version:         "v1",
		Goal:            "Generate a valid commit message with goodcommit using non-interactive plugin answers.",
		RequiredAnswers: required,
		OptionalAnswers: optional,
		PluginHints:     pluginHints,
	}
	ctx.ExampleNonInteractiveRun = buildExampleNonInteractiveCommand(required, optional)
	return ctx
}

func buildExampleNonInteractiveCommand(required, optional []AIAnswerHint) string {
	parts := []string{"goodcommit", "-m"}
	for _, a := range required {
		parts = append(parts, "--plugin-answer", fmt.Sprintf("%s=%s", a.Key, placeholderValueForType(a.Type)))
	}
	for i := 0; i < len(optional) && i < 1; i++ {
		parts = append(parts, "--plugin-answer", fmt.Sprintf("%s=%s", optional[i].Key, placeholderValueForType(optional[i].Type)))
	}
	return strings.Join(parts, " ")
}

func placeholderValueForType(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "bool", "boolean":
		return "<true|false>"
	case "[]string", "string[]", "array":
		return "<item1,item2>"
	default:
		return "<value>"
	}
}
