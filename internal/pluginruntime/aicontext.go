package pluginruntime

import (
	"sort"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

// AIPluginContext is a compact machine-readable instruction block for an agent.
type AIPluginContext struct {
	ID             string                 `json:"id"`
	Hooks          []HookPhase            `json:"hooks"`
	Order          int                    `json:"order"`
	FailureMode    FailureMode            `json:"failure_mode"`
	TimeoutMS      int                    `json:"timeout_ms"`
	Purpose        string                 `json:"purpose,omitempty"`
	Instructions   string                 `json:"instructions,omitempty"`
	AnswerSpecs    []api.AIAnswerSpec     `json:"answer_specs,omitempty"`
	MetadataReads  []api.MetadataSpec     `json:"metadata_reads,omitempty"`
	MetadataWrites []api.MetadataSpec     `json:"metadata_writes,omitempty"`
	DependsOn      []string               `json:"depends_on,omitempty"`
	AutoAnswers    map[string]interface{} `json:"auto_answers,omitempty"`
}

// BuildAIContext returns sorted agent-oriented plugin execution guidance.
func BuildAIContext(resolved []ResolvedPlugin) []AIPluginContext {
	writers := map[string][]string{}
	for _, rp := range resolved {
		if rp.Runtime.Manifest.Contract == nil {
			continue
		}
		for _, w := range rp.Runtime.Manifest.Contract.MetadataWrites {
			writers[w.Key] = append(writers[w.Key], rp.Runtime.Manifest.ID)
		}
	}

	out := make([]AIPluginContext, 0, len(resolved))
	for _, rp := range resolved {
		ctx := AIPluginContext{
			ID:          rp.Runtime.Manifest.ID,
			Hooks:       rp.Runtime.Manifest.Hooks,
			Order:       rp.Runtime.Order,
			FailureMode: rp.Runtime.FailureMode,
			TimeoutMS:   int(rp.Runtime.Timeout.Milliseconds()),
			AutoAnswers: rp.Runtime.AIAuto,
		}
		if rp.Runtime.AIHints != nil {
			ctx.Purpose = rp.Runtime.AIHints.Purpose
			ctx.Instructions = rp.Runtime.AIHints.Instructions
		}
		if rp.Runtime.Manifest.Contract != nil {
			ctx.AnswerSpecs = rp.Runtime.Manifest.Contract.Answers
			ctx.MetadataReads = rp.Runtime.Manifest.Contract.MetadataReads
			ctx.MetadataWrites = rp.Runtime.Manifest.Contract.MetadataWrites
			deps := map[string]struct{}{}
			for _, r := range rp.Runtime.Manifest.Contract.MetadataReads {
				for _, writerID := range writers[r.Key] {
					if writerID == rp.Runtime.Manifest.ID {
						continue
					}
					deps[writerID] = struct{}{}
				}
			}
			for dep := range deps {
				ctx.DependsOn = append(ctx.DependsOn, dep)
			}
			sort.Strings(ctx.DependsOn)
		}
		out = append(out, ctx)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order == out[j].Order {
			return out[i].ID < out[j].ID
		}
		return out[i].Order < out[j].Order
	})
	return out
}
