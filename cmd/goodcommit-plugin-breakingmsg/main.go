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
		if !metaBool(req.Draft.Metadata, "gc.breaking") {
			pluginutil.AddInfo(&resp, "BREAKINGMSG_SKIP", "no breaking change selected")
			return pluginutil.WriteResponse(resp)
		}
		if pluginutil.Submitted(req, "breakingmsg") {
			msg, _ := pluginutil.GetAnswerString(req, "breaking_message")
			msg = normalizeSentence(msg)
			if msg == "" {
				resp.OK = false
				resp.Fatal = true
				pluginutil.AddError(&resp, "BREAKINGMSG_REQUIRED", "breaking change details are required")
				return pluginutil.WriteResponse(resp)
			}
			resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{"gc.breaking_msg": msg}}
			pluginutil.AddInfo(&resp, "BREAKINGMSG_CAPTURED", "captured breaking change details")
			return pluginutil.WriteResponse(resp)
		}

		resp.UIRequests = []plugins.UIRequest{{
			ID:          "breakingmsg",
			Title:       "Breaking Change Details",
			Description: "Provide details about compatibility impact.",
			Fields: []plugins.UIField{
				{ID: "breaking_message", Type: "text", Title: "Details", Required: true, Editor: true},
			},
		}}
		pluginutil.AddInfo(&resp, "BREAKINGMSG_PROMPT", "prompting breaking change details")
		return pluginutil.WriteResponse(resp)

	case plugins.HookEnrich:
		msg := metaString(req.Draft.Metadata, "gc.breaking_msg")
		if msg == "" {
			return pluginutil.WriteResponse(resp)
		}
		append := "\n\nBREAKING CHANGE: " + msg
		resp.Mutations = &plugins.Mutations{AppendBody: append}
		pluginutil.AddInfo(&resp, "BREAKINGMSG_ENRICH", "appended BREAKING CHANGE section")
		return pluginutil.WriteResponse(resp)

	default:
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "breakingmsg plugin runs on collect/enrich")
		return pluginutil.WriteResponse(resp)
	}
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

func metaBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func normalizeSentence(s string) string {
	s = strings.TrimSpace(strings.TrimSuffix(s, "."))
	if s == "" {
		return ""
	}
	s = pluginutil.UppercaseFirstRune(s)
	return s + "."
}
