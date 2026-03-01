package pluginruntime

import (
	"fmt"
)

func isValidHook(h HookPhase) bool {
	switch h {
	case HookCollect, HookValidate, HookEnrich, HookFinalize, HookPreCommit, HookPostCommit:
		return true
	default:
		return false
	}
}

func supportsHook(hooks []HookPhase, hook HookPhase) bool {
	for _, h := range hooks {
		if h == hook {
			return true
		}
	}
	return false
}

func validateManifest(m Manifest) error {
	if m.APIVersion == "" || m.Kind == "" || m.ID == "" || m.Version == "" {
		return fmt.Errorf("manifest missing required fields")
	}
	if m.Entrypoint.Type != "exec" || m.Entrypoint.Command == "" {
		return fmt.Errorf("manifest entrypoint must be exec with command")
	}
	if len(m.Hooks) == 0 {
		return fmt.Errorf("manifest must define at least one hook")
	}
	for _, h := range m.Hooks {
		if !isValidHook(h) {
			return fmt.Errorf("manifest has invalid hook %q", h)
		}
	}
	return nil
}

func validateRequest(r Request) error {
	if r.ProtocolVersion != ProtocolVersionV1 {
		return fmt.Errorf("unsupported protocol version %q", r.ProtocolVersion)
	}
	if r.RequestID == "" || r.PluginID == "" {
		return fmt.Errorf("request_id and plugin_id are required")
	}
	if !isValidHook(r.Hook) {
		return fmt.Errorf("invalid hook %q", r.Hook)
	}
	if r.Context.RepoRoot == "" {
		return fmt.Errorf("context.repo_root is required")
	}
	if r.Draft.Metadata == nil {
		r.Draft.Metadata = map[string]interface{}{}
	}
	return nil
}

func validateResponse(resp Response, req Request) error {
	if resp.RequestID == "" {
		return fmt.Errorf("response.request_id is required")
	}
	if resp.RequestID != req.RequestID {
		return fmt.Errorf("response.request_id does not match request_id")
	}
	for _, d := range resp.Diagnostics {
		switch d.Level {
		case "info", "warn", "error":
		default:
			return fmt.Errorf("invalid diagnostic level %q", d.Level)
		}
		if d.Message == "" {
			return fmt.Errorf("diagnostic message is required")
		}
	}
	for _, p := range resp.PromptRequests {
		if p.ID == "" || p.Title == "" || p.Type == "" {
			return fmt.Errorf("prompt_request id/type/title are required")
		}
		switch p.Type {
		case "input", "confirm", "select", "multiselect":
		default:
			return fmt.Errorf("invalid prompt_request type %q", p.Type)
		}
	}
	for _, ui := range resp.UIRequests {
		if ui.ID == "" || ui.Title == "" {
			return fmt.Errorf("ui_request id/title are required")
		}
		if len(ui.Fields) == 0 {
			return fmt.Errorf("ui_request %q must include at least one field", ui.ID)
		}
		for _, f := range ui.Fields {
			if f.ID == "" || f.Type == "" || f.Title == "" {
				return fmt.Errorf("ui_request %q fields require id/type/title", ui.ID)
			}
			switch f.Type {
			case "note", "input", "text", "confirm", "select", "multiselect":
			default:
				return fmt.Errorf("ui_request %q has invalid field type %q", ui.ID, f.Type)
			}
			if (f.Type == "select" || f.Type == "multiselect") && len(f.Options) == 0 {
				return fmt.Errorf("ui_request %q field %q requires options", ui.ID, f.ID)
			}
		}
	}
	return nil
}
