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

	if req.Hook != plugins.HookCollect {
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "description plugin runs only on collect")
		return pluginutil.WriteResponse(resp)
	}

	if pluginutil.Submitted(req, "description") {
		desc, _ := pluginutil.GetAnswerString(req, "commit_description")
		desc = strings.TrimSpace(strings.TrimSuffix(desc, "."))
		if desc == "" {
			resp.OK = false
			resp.Fatal = true
			pluginutil.AddError(&resp, "DESCRIPTION_REQUIRED", "description is required")
			return pluginutil.WriteResponse(resp)
		}
		desc = pluginutil.LowercaseFirstRune(desc)
		resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{"gc.description": desc}}
		pluginutil.AddInfo(&resp, "DESCRIPTION_SET", "commit description captured")
		return pluginutil.WriteResponse(resp)
	}

	resp.UIRequests = []plugins.UIRequest{{
		ID:          "description",
		Title:       "Write Commit Description",
		Description: "Briefly describe changes (max 72 chars).",
		Fields: []plugins.UIField{
			{ID: "commit_description", Type: "input", Title: "Description", Required: true, CharLimit: 72},
		},
	}}
	pluginutil.AddInfo(&resp, "DESCRIPTION_PROMPT", "prompting commit description")
	return pluginutil.WriteResponse(resp)
}
