package main

import (
	"fmt"
	"os"

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

	if req.Hook != plugins.HookCollect {
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "breaking plugin runs only on collect")
		return pluginutil.WriteResponse(resp)
	}

	commitType := metaString(req.Draft.Metadata, "gc.type")
	if commitType != "feat" && commitType != "fix" {
		pluginutil.AddInfo(&resp, "BREAKING_SKIP", "breaking prompt only applies to feat/fix")
		return pluginutil.WriteResponse(resp)
	}

	if pluginutil.Submitted(req, "breaking") {
		isBreaking, _ := pluginutil.GetAnswerBool(req, "is_breaking")
		resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{"gc.breaking": isBreaking}}
		pluginutil.AddInfo(&resp, "BREAKING_CAPTURED", "captured breaking change flag")
		return pluginutil.WriteResponse(resp)
	}

	resp.UIRequests = []plugins.UIRequest{{
		ID:    "breaking",
		Title: "Does this commit introduce a breaking change?",
		Fields: []plugins.UIField{
			{ID: "is_breaking", Type: "confirm", Title: "Breaking change"},
		},
	}}
	pluginutil.AddInfo(&resp, "BREAKING_PROMPT", "prompting breaking change confirmation")
	return pluginutil.WriteResponse(resp)
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
