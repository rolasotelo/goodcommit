package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rolasotelo/goodcommit/internal/pluginutil"
	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

type scopeItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Emoji       string   `json:"emoji"`
	Conditional []string `json:"conditional"`
}

type scopeConfig struct {
	Scopes []scopeItem `json:"scopes"`
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

	path := pluginutil.ConfigString(req.PluginConfig, "path", "./configs/commit-scopes.json")
	cfg := scopeConfig{}
	if err := pluginutil.ReadJSONFile(path, &cfg); err != nil {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "SCOPE_CONFIG_ERROR", err.Error())
		return pluginutil.WriteResponse(resp)
	}

	switch req.Hook {
	case plugins.HookCollect:
		commitType := metaString(req.Draft.Metadata, "gc.type")
		if commitType == "" {
			pluginutil.AddInfo(&resp, "SCOPE_SKIP", "type not selected, skipping scopes")
			return pluginutil.WriteResponse(resp)
		}
		available := filterScopes(cfg.Scopes, commitType)
		if len(available) == 0 {
			pluginutil.AddInfo(&resp, "SCOPE_SKIP", "no scopes available for selected type")
			return pluginutil.WriteResponse(resp)
		}

		if pluginutil.Submitted(req, "scopes") {
			selected, _ := pluginutil.GetAnswerStringSlice(req, "commit_scopes")
			names := []string{}
			emojis := ""
			for _, id := range selected {
				for _, item := range available {
					if item.ID == id {
						names = append(names, item.Name)
						emojis += item.Emoji
					}
				}
			}
			resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{
				"gc.scope_names":  names,
				"gc.scope_emojis": emojis,
			}}
			pluginutil.AddInfo(&resp, "SCOPE_SELECTED", "commit scopes selected")
			return pluginutil.WriteResponse(resp)
		}

		opts := make([]plugins.UIOption, 0, len(available))
		for _, item := range available {
			label := item.Emoji + " " + item.Name
			if item.Description != "" {
				label += " - " + item.Description
			}
			opts = append(opts, plugins.UIOption{Label: label, Value: item.ID})
		}
		resp.UIRequests = []plugins.UIRequest{{
			ID:          "scopes",
			Title:       "Select Commit Scopes",
			Description: "Additional contextual information. Multiple selections allowed.",
			Fields: []plugins.UIField{
				{ID: "commit_scopes", Type: "multiselect", Title: "Scopes", Options: opts},
			},
		}}
		pluginutil.AddInfo(&resp, "SCOPE_PROMPT", "prompting commit scopes")
		return pluginutil.WriteResponse(resp)

	case plugins.HookEnrich:
		names := metaStringSlice(req.Draft.Metadata, "gc.scope_names")
		if len(names) == 0 {
			return pluginutil.WriteResponse(resp)
		}
		header := "SCOPE: " + strings.Join(names, " / ")
		if len(names) > 1 {
			header = "SCOPES: " + strings.Join(names, " / ")
		}
		resp.Mutations = &plugins.Mutations{PrependBody: header + "\n\n"}
		pluginutil.AddInfo(&resp, "SCOPE_ENRICH", "prepended scope section to body")
		return pluginutil.WriteResponse(resp)

	default:
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "scopes plugin runs on collect/enrich")
		return pluginutil.WriteResponse(resp)
	}
}

func filterScopes(items []scopeItem, commitType string) []scopeItem {
	out := []scopeItem{}
	for _, item := range items {
		for _, c := range item.Conditional {
			if c == commitType {
				out = append(out, item)
				break
			}
		}
	}
	return out
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

func metaStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if ok {
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			s, ok := item.(string)
			if ok {
				out = append(out, s)
			}
		}
		return out
	}
	ss, ok := v.([]string)
	if ok {
		return ss
	}
	return nil
}
