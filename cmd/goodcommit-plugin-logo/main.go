package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rolasotelo/goodcommit/internal/pluginutil"
	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

const defaultLogo = `┌─────────────────────────────────────┐
│  You're gonna like this commit...   │
└─────────────────────────────────────┘`

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
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "logo plugin runs only on collect")
		return pluginutil.WriteResponse(resp)
	}

	if pluginutil.Submitted(req, "logo") {
		if resp.Mutations == nil {
			resp.Mutations = &plugins.Mutations{}
		}
		resp.Mutations.MetadataPatch = map[string]interface{}{"gc.logo_seen": true}
		pluginutil.AddInfo(&resp, "LOGO_SEEN", "logo acknowledged")
		return pluginutil.WriteResponse(resp)
	}

	logo, err := resolveLogoText(req.PluginConfig)
	if err != nil {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "LOGO_CONFIG_ERROR", err.Error())
		return pluginutil.WriteResponse(resp)
	}
	resp.UIRequests = []plugins.UIRequest{{
		ID:    "logo",
		Title: "Goodcommit",
		Fields: []plugins.UIField{
			{ID: "banner", Type: "note", Title: logo},
		},
	}}
	pluginutil.AddInfo(&resp, "LOGO_RENDER", "rendering logo banner")
	return pluginutil.WriteResponse(resp)
}

func resolveLogoText(cfg map[string]interface{}) (string, error) {
	path := strings.TrimSpace(pluginutil.ConfigString(cfg, "path", ""))
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read logo file %s: %w", path, err)
		}
		content := strings.TrimSpace(string(raw))
		if content == "" {
			return "", fmt.Errorf("logo file %s is empty", path)
		}
		return content, nil
	}

	text := strings.TrimSpace(pluginutil.ConfigString(cfg, "text", defaultLogo))
	if text == "" {
		return defaultLogo, nil
	}
	return text, nil
}
