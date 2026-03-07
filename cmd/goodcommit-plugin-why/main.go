package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rolasotelo/goodcommit/internal/pluginutil"
	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "plugin error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	req, err := pluginutil.ReadRequest()
	if err != nil {
		return err
	}
	resp := pluginutil.NewResponse(req)

	switch req.Hook {
	case plugins.HookCollect:
		required := pluginutil.ConfigBool(req.PluginConfig, "required", false)
		if pluginutil.Submitted(req, "why") {
			why, _ := pluginutil.GetAnswerString(req, "commit_why")
			why = normalizeSentence(why)
			if required && why == "" {
				resp.OK = false
				resp.Fatal = true
				pluginutil.AddError(&resp, "WHY_REQUIRED", "reason is required")
				return pluginutil.WriteResponse(resp)
			}
			if resp.Mutations == nil {
				resp.Mutations = &plugins.Mutations{}
			}
			resp.Mutations.MetadataPatch = map[string]interface{}{"gc.why": why}
			pluginutil.AddInfo(&resp, "WHY_CAPTURED", "captured change reasoning")
			return pluginutil.WriteResponse(resp)
		}

		resp.UIRequests = []plugins.UIRequest{{
			ID:          "why",
			Title:       "Why was this change needed?",
			Description: "Optional but recommended context.",
			Fields: []plugins.UIField{
				{ID: "commit_why", Type: "input", Title: "Reason", Required: required, CharLimit: 180},
			},
		}}
		pluginutil.AddInfo(&resp, "WHY_PROMPT", "prompting change reason")
		return pluginutil.WriteResponse(resp)

	case plugins.HookEnrich:
		why := metaString(req.Draft.Metadata, "gc.why")
		if why == "" {
			return pluginutil.WriteResponse(resp)
		}
		resp.Mutations = &plugins.Mutations{PrependBody: "WHY: " + why + "\n\n"}
		pluginutil.AddInfo(&resp, "WHY_ENRICHED", "prepended WHY section")
		return pluginutil.WriteResponse(resp)

	default:
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "why plugin runs on collect/enrich")
		return pluginutil.WriteResponse(resp)
	}
}

func normalizeSentence(s string) string {
	s = strings.TrimSpace(strings.TrimSuffix(s, "."))
	if s == "" {
		return ""
	}
	s = pluginutil.UppercaseFirstRune(s)
	return s + "."
}

func metaString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
