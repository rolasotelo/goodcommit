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

	if req.Hook != plugins.HookFinalize {
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "conventional-title plugin runs only on finalize")
		return pluginutil.WriteResponse(resp)
	}

	typeID := metaString(req.Draft.Metadata, "gc.type")
	desc := metaString(req.Draft.Metadata, "gc.description")
	if typeID == "" || desc == "" {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "TITLE_FIELDS_REQUIRED", "type and description are required to build title")
		return pluginutil.WriteResponse(resp)
	}

	scope := metaString(req.Draft.Metadata, "gc.scope_emojis")
	breaking := metaBool(req.Draft.Metadata, "gc.breaking")

	title := typeID
	if scope != "" {
		title += "(" + scope + ")"
	}
	if breaking {
		title += "!"
	}
	title += ": " + desc

	resp.Mutations = &plugins.Mutations{SetTitle: strings.TrimSpace(title)}
	pluginutil.AddInfo(&resp, "TITLE_COMPOSED", "composed conventional commit title")
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
