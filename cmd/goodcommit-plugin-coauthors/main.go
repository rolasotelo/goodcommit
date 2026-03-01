package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rolasotelo/goodcommit/internal/pluginutil"
	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

type coauthorItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

type coauthorConfig struct {
	Coauthors []coauthorItem `json:"coauthors"`
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

	path := pluginutil.ConfigString(req.PluginConfig, "path", "./configs/commit-coauthors.json")
	cfg := coauthorConfig{}
	if err := pluginutil.ReadJSONFile(path, &cfg); err != nil {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "COAUTHORS_CONFIG_ERROR", err.Error())
		return pluginutil.WriteResponse(resp)
	}

	switch req.Hook {
	case plugins.HookCollect:
		userEmail := strings.TrimSpace(req.Context.GitUserEmail)
		available := []coauthorItem{}
		for _, item := range cfg.Coauthors {
			if strings.EqualFold(item.ID, userEmail) {
				continue
			}
			available = append(available, item)
		}
		if len(available) == 0 {
			pluginutil.AddInfo(&resp, "COAUTHORS_SKIP", "no co-authors configured")
			return pluginutil.WriteResponse(resp)
		}

		if pluginutil.Submitted(req, "coauthors") {
			selectedIDs, _ := pluginutil.GetAnswerStringSlice(req, "coauthors_selected")
			names := []string{}
			emojis := []string{}
			trailers := []string{}
			for _, id := range selectedIDs {
				for _, item := range available {
					if item.ID == id {
						names = append(names, item.Name)
						emojis = append(emojis, item.Emoji)
						trailers = append(trailers, fmt.Sprintf("%s <%s>", item.Name, item.ID))
					}
				}
			}
			resp.Mutations = &plugins.Mutations{MetadataPatch: map[string]interface{}{
				"gc.coauthors":        trailers,
				"gc.coauthors_names":  names,
				"gc.coauthors_emojis": emojis,
			}}
			pluginutil.AddInfo(&resp, "COAUTHORS_CAPTURED", "captured co-authors")
			return pluginutil.WriteResponse(resp)
		}

		opts := make([]plugins.UIOption, 0, len(available))
		for _, item := range available {
			opts = append(opts, plugins.UIOption{Label: item.Name + " - " + item.ID, Value: item.ID})
		}
		resp.UIRequests = []plugins.UIRequest{{
			ID:          "coauthors",
			Title:       "Select Co-Authors",
			Description: "Choose collaborators for this commit.",
			Fields: []plugins.UIField{
				{ID: "coauthors_selected", Type: "multiselect", Title: "Co-authors", Options: opts},
			},
		}}
		pluginutil.AddInfo(&resp, "COAUTHORS_PROMPT", "prompting co-author selection")
		return pluginutil.WriteResponse(resp)

	case plugins.HookFinalize:
		coauthors := metaStringSlice(req.Draft.Metadata, "gc.coauthors")
		emojis := metaStringSlice(req.Draft.Metadata, "gc.coauthors_emojis")
		if len(coauthors) == 0 {
			return pluginutil.WriteResponse(resp)
		}
		addTrailers := make([]plugins.Trailer, 0, len(coauthors))
		for _, c := range coauthors {
			addTrailers = append(addTrailers, plugins.Trailer{Key: "Co-authored-by", Value: c})
		}
		appendBody := ""
		if len(emojis) > 0 {
			authorEmoji := findAuthorEmoji(cfg.Coauthors, req.Context.GitUserEmail)
			appendBody = "\n\n" + strings.TrimSpace(authorEmoji+" "+strings.Join(emojis, " "))
		}
		resp.Mutations = &plugins.Mutations{AddTrailers: addTrailers, AppendBody: appendBody}
		pluginutil.AddInfo(&resp, "COAUTHORS_FINALIZED", "added co-author trailers")
		return pluginutil.WriteResponse(resp)

	default:
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "coauthors plugin runs on collect/finalize")
		return pluginutil.WriteResponse(resp)
	}
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

func findAuthorEmoji(items []coauthorItem, email string) string {
	for _, item := range items {
		if strings.EqualFold(item.ID, email) {
			return item.Emoji
		}
	}
	return ""
}
