package pluginruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// AIAnswerHint describes one answer key an agent may provide.
type AIAnswerHint struct {
	Key           string   `json:"key"`
	Type          string   `json:"type,omitempty"`
	Required      bool     `json:"required,omitempty"`
	Description   string   `json:"description,omitempty"`
	Strategy      string   `json:"strategy,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	FromPlugin    string   `json:"from_plugin,omitempty"`
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
func BuildAIContext(resolved []ResolvedPlugin) (AIContext, error) {
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
			allowedValues, err := resolveAllowedValues(rp.Runtime.AIConstraints[a.Key])
			if err != nil {
				return AIContext{}, fmt.Errorf("plugin %s answer %s constraints: %w", rp.Runtime.Manifest.ID, a.Key, err)
			}
			item := AIAnswerHint{
				Key:           strings.TrimSpace(a.Key),
				Type:          strings.TrimSpace(a.Type),
				Required:      a.Required,
				Description:   strings.TrimSpace(a.Description),
				Strategy:      strings.TrimSpace(a.Strategy),
				AllowedValues: allowedValues,
				FromPlugin:    rp.Runtime.Manifest.ID,
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
	return ctx, nil
}

func resolveAllowedValues(c AIAnswerConstraint) ([]string, error) {
	values := []string{}
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		values = append(values, v)
	}

	for _, v := range c.AllowedValues {
		add(v)
	}

	if c.AllowedValuesFromJSON != nil {
		src := c.AllowedValuesFromJSON
		if strings.TrimSpace(src.Path) == "" {
			return nil, fmt.Errorf("allowed_values_from_json.path is required")
		}
		raw, err := os.ReadFile(src.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", src.Path, err)
		}

		var decoded interface{}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("parse %s: %w", src.Path, err)
		}

		var items []interface{}
		if strings.TrimSpace(src.ArrayKey) == "" {
			arr, ok := decoded.([]interface{})
			if !ok {
				return nil, fmt.Errorf("expected root array in %s when array_key is empty", src.Path)
			}
			items = arr
		} else {
			obj, ok := decoded.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected root object in %s", src.Path)
			}
			rawItems, ok := obj[src.ArrayKey]
			if !ok {
				return nil, fmt.Errorf("array_key %q not found in %s", src.ArrayKey, src.Path)
			}
			arr, ok := rawItems.([]interface{})
			if !ok {
				return nil, fmt.Errorf("array_key %q in %s is not an array", src.ArrayKey, src.Path)
			}
			items = arr
		}

		valueKey := strings.TrimSpace(src.ValueKey)
		for _, it := range items {
			if valueKey == "" {
				s, ok := it.(string)
				if !ok {
					return nil, fmt.Errorf("array item is not string in %s (set value_key to extract from objects)", src.Path)
				}
				add(s)
				continue
			}
			obj, ok := it.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("array item is not object in %s for value_key %q", src.Path, valueKey)
			}
			rawValue, ok := obj[valueKey]
			if !ok {
				return nil, fmt.Errorf("value_key %q missing in array item from %s", valueKey, src.Path)
			}
			s, ok := rawValue.(string)
			if !ok {
				return nil, fmt.Errorf("value_key %q is not string in %s", valueKey, src.Path)
			}
			add(s)
		}
	}

	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
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
