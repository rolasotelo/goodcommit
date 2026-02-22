package main

import (
	"fmt"
	"os"

	"github.com/rolasotelo/goodcommit/internal/pluginutil"
	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

type typeItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Emoji string `json:"emoji"`
}

type typeConfig struct {
	Types []typeItem `json:"types"`
}

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
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "types plugin runs only on collect")
		return pluginutil.WriteResponse(resp)
	}

	path := pluginutil.ConfigString(req.PluginConfig, "path", "./configs/commit-types.json")
	cfg := typeConfig{}
	if err := pluginutil.ReadJSONFile(path, &cfg); err != nil {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "TYPE_CONFIG_ERROR", err.Error())
		return pluginutil.WriteResponse(resp)
	}

	if pluginutil.Submitted(req, "types") {
		value, _ := pluginutil.GetAnswerString(req, "commit_type")
		if value == "" {
			resp.OK = false
			resp.Fatal = true
			pluginutil.AddError(&resp, "TYPE_REQUIRED", "commit type is required")
			return pluginutil.WriteResponse(resp)
		}
		resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{"gc.type": value}}
		pluginutil.AddInfo(&resp, "TYPE_SELECTED", "commit type selected")
		return pluginutil.WriteResponse(resp)
	}

	opts := make([]plugins.UIOption, 0, len(cfg.Types))
	for _, item := range cfg.Types {
		label := item.Emoji + " " + item.Name
		if item.Title != "" {
			label += " - " + item.Title
		}
		opts = append(opts, plugins.UIOption{Label: label, Value: item.ID})
	}
	resp.UIRequests = []plugins.UIRequest{{
		ID:          "types",
		Title:       "Select Commit Type",
		Description: "Following Conventional Commits.",
		Fields: []plugins.UIField{
			{ID: "commit_type", Type: "select", Title: "Type", Required: true, Options: opts},
		},
	}}
	pluginutil.AddInfo(&resp, "TYPE_PROMPT", "prompting commit type")
	return pluginutil.WriteResponse(resp)
}
