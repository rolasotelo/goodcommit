package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
	"golang.org/x/term"
)

func makePromptResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) plugins.PromptHandler {
	return func(pluginID string, promptRequests []plugins.PromptRequest) (map[string]interface{}, error) {
		return resolvePromptRequests(pluginID, promptRequests, predefined, autoByPlugin, accessible)
	}
}

func makeUIResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) plugins.UIHandler {
	return func(pluginID string, forms []plugins.UIRequest) (map[string]interface{}, error) {
		return resolveUIRequests(pluginID, forms, predefined, autoByPlugin, accessible)
	}
}

func makeGroupedUIResolver(predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool, detailed bool) plugins.GroupedUIHandler {
	return func(groupID string, requests []plugins.PluginUIBatchRequest) (map[string]map[string]interface{}, error) {
		return resolveGroupedUIRequests(groupID, requests, predefined, autoByPlugin, accessible, detailed)
	}
}

func resolveUIRequests(pluginID string, forms []plugins.UIRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) (map[string]interface{}, error) {
	answers := map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	for _, ui := range forms {
		if !isTTY {
			for _, f := range ui.Fields {
				if f.Type == "note" {
					continue
				}
				raw, ok := findPredefined(predefined, ui.ID, f.ID)
				if !ok {
					if autoVal, found := findAutoAnswer(autoByPlugin, pluginID, ui.ID, f.ID); found {
						answers[f.ID] = autoVal
						continue
					}
					if f.Required {
						return nil, fmt.Errorf("form field %s.%s requires TTY (or pass --plugin-answer %s.%s=...)", ui.ID, f.ID, ui.ID, f.ID)
					}
					continue
				}
				value, err := parsePredefinedAnswer(raw, f.Type)
				if err != nil {
					return nil, fmt.Errorf("plugin %s field %s.%s invalid predefined answer: %w", pluginID, ui.ID, f.ID, err)
				}
				answers[f.ID] = value
			}
			answers[ui.ID+".__submitted"] = true
			continue
		}

		var fields []huh.Field
		local := map[string]interface{}{}
		for _, f := range ui.Fields {
			switch f.Type {
			case "note":
				fields = append(fields, huh.NewNote().Title(f.Title).Description(f.Description))
			case "input":
				v := f.Value
				input := huh.NewInput().Title(f.Title).Description(f.Description).Placeholder(f.Placeholder).Value(&v)
				if f.CharLimit > 0 {
					input = input.CharLimit(f.CharLimit)
				}
				if f.Required {
					input = input.Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("value is required")
						}
						return nil
					})
				}
				fields = append(fields, input)
				local[f.ID] = &v
			case "text":
				v := f.Value
				text := huh.NewText().Title(f.Title).Description(f.Description).Placeholder(f.Placeholder).Value(&v)
				if f.Editor {
					text = text.Editor("vim")
				}
				fields = append(fields, text)
				local[f.ID] = &v
			case "confirm":
				var v bool
				fields = append(fields, huh.NewConfirm().Title(f.Title).Description(f.Description).Value(&v))
				local[f.ID] = &v
			case "select":
				opts := make([]huh.Option[string], 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, huh.NewOption(o.Label, o.Value))
				}
				v := ""
				fields = append(fields, huh.NewSelect[string]().Title(f.Title).Description(f.Description).Options(opts...).Value(&v))
				local[f.ID] = &v
			case "multiselect":
				opts := make([]huh.Option[string], 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, huh.NewOption(o.Label, o.Value))
				}
				v := []string{}
				fields = append(fields, huh.NewMultiSelect[string]().Title(f.Title).Description(f.Description).Options(opts...).Value(&v))
				local[f.ID] = &v
			default:
				return nil, fmt.Errorf("plugin %s field %s.%s has unsupported type %q", pluginID, ui.ID, f.ID, f.Type)
			}
		}
		form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(accessible)
		if err := form.Run(); err != nil {
			return nil, fmt.Errorf("plugin %s form %s failed: %w", pluginID, ui.ID, err)
		}
		for id, ref := range local {
			switch x := ref.(type) {
			case *string:
				answers[id] = *x
			case *bool:
				answers[id] = *x
			case *[]string:
				answers[id] = *x
			}
		}
		answers[ui.ID+".__submitted"] = true
	}

	return answers, nil
}

func resolveGroupedUIRequests(groupID string, requests []plugins.PluginUIBatchRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool, detailed bool) (map[string]map[string]interface{}, error) {
	answersByPlugin := map[string]map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	for _, req := range requests {
		if _, ok := answersByPlugin[req.PluginID]; !ok {
			answersByPlugin[req.PluginID] = map[string]interface{}{}
		}
	}

	if !isTTY {
		for _, req := range requests {
			for _, ui := range req.Forms {
				for _, f := range ui.Fields {
					if f.Type == "note" {
						continue
					}
					raw, ok := findGroupedPredefined(predefined, req.PluginID, ui.ID, f.ID)
					if !ok {
						if autoVal, found := findAutoAnswer(autoByPlugin, req.PluginID, ui.ID, f.ID); found {
							answersByPlugin[req.PluginID][f.ID] = autoVal
							continue
						}
						if f.Required {
							return nil, fmt.Errorf("group %s field %s.%s.%s requires TTY (or pass --plugin-answer %s.%s.%s=...)", groupID, req.PluginID, ui.ID, f.ID, req.PluginID, ui.ID, f.ID)
						}
						continue
					}
					value, err := parsePredefinedAnswer(raw, f.Type)
					if err != nil {
						return nil, fmt.Errorf("plugin %s field %s.%s invalid predefined answer: %w", req.PluginID, ui.ID, f.ID, err)
					}
					answersByPlugin[req.PluginID][f.ID] = value
				}
				answersByPlugin[req.PluginID][ui.ID+".__submitted"] = true
			}
		}
		return answersByPlugin, nil
	}

	type localFieldRef struct {
		pluginID string
		fieldID  string
		ref      interface{}
	}
	local := []localFieldRef{}
	fields := []huh.Field{}

	for _, req := range requests {
		for _, ui := range req.Forms {
			if detailed {
				sectionTitle := ui.Title
				if strings.TrimSpace(sectionTitle) == "" {
					sectionTitle = ui.ID
				}
				sectionDesc := ui.Description
				if sectionDesc == "" {
					sectionDesc = "Fill required fields and submit."
				}
				fields = append(fields, huh.NewNote().Title(fmt.Sprintf("%s - %s", req.PluginID, sectionTitle)).Description(sectionDesc))
			}

			firstPromptField := true
			for _, f := range ui.Fields {
				fieldDesc := f.Description
				if !detailed && firstPromptField && f.Type != "note" && strings.TrimSpace(fieldDesc) == "" {
					fieldDesc = strings.TrimSpace(ui.Description)
				}
				switch f.Type {
				case "note":
					fields = append(fields, huh.NewNote().Title(f.Title).Description(fieldDesc))
				case "input":
					v := f.Value
					input := huh.NewInput().Title(f.Title).Description(fieldDesc).Placeholder(f.Placeholder).Value(&v)
					if f.CharLimit > 0 {
						input = input.CharLimit(f.CharLimit)
					}
					if f.Required {
						input = input.Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("value is required")
							}
							return nil
						})
					}
					fields = append(fields, input)
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "text":
					v := f.Value
					text := huh.NewText().Title(f.Title).Description(fieldDesc).Placeholder(f.Placeholder).Value(&v)
					if f.Editor {
						text = text.Editor("vim")
					}
					fields = append(fields, text)
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "confirm":
					var v bool
					fields = append(fields, huh.NewConfirm().Title(f.Title).Description(fieldDesc).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "select":
					opts := make([]huh.Option[string], 0, len(f.Options))
					for _, o := range f.Options {
						opts = append(opts, huh.NewOption(o.Label, o.Value))
					}
					v := ""
					fields = append(fields, huh.NewSelect[string]().Title(f.Title).Description(fieldDesc).Options(opts...).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				case "multiselect":
					opts := make([]huh.Option[string], 0, len(f.Options))
					for _, o := range f.Options {
						opts = append(opts, huh.NewOption(o.Label, o.Value))
					}
					v := []string{}
					fields = append(fields, huh.NewMultiSelect[string]().Title(f.Title).Description(fieldDesc).Options(opts...).Value(&v))
					local = append(local, localFieldRef{pluginID: req.PluginID, fieldID: f.ID, ref: &v})
					firstPromptField = false
				default:
					return nil, fmt.Errorf("plugin %s field %s.%s has unsupported type %q", req.PluginID, ui.ID, f.ID, f.Type)
				}
			}
		}
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("grouped form %s failed: %w", groupID, err)
	}

	for _, item := range local {
		switch x := item.ref.(type) {
		case *string:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		case *bool:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		case *[]string:
			answersByPlugin[item.pluginID][item.fieldID] = *x
		}
	}
	for _, req := range requests {
		for _, ui := range req.Forms {
			answersByPlugin[req.PluginID][ui.ID+".__submitted"] = true
		}
	}

	return answersByPlugin, nil
}

func resolvePromptRequests(pluginID string, promptRequests []plugins.PromptRequest, predefined map[string]string, autoByPlugin map[string]map[string]interface{}, accessible bool) (map[string]interface{}, error) {
	answers := map[string]interface{}{}
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	for _, p := range promptRequests {
		if predefined != nil {
			if raw, ok := predefined[p.ID]; ok {
				value, err := parsePredefinedAnswer(raw, p.Type)
				if err != nil {
					return nil, fmt.Errorf("plugin %s prompt %s invalid predefined answer: %w", pluginID, p.ID, err)
				}
				answers[p.ID] = value
				continue
			}
		}
		if autoVal, ok := findAutoAnswer(autoByPlugin, pluginID, "", p.ID); ok {
			answers[p.ID] = autoVal
			continue
		}
		if !isTTY {
			return nil, fmt.Errorf("prompt %s requires TTY (or pass --plugin-answer %s=...)", p.ID, p.ID)
		}
		switch p.Type {
		case "input":
			var value string
			field := huh.NewInput().Title(p.Title).Description(p.Description).Value(&value)
			if p.Required {
				field = field.Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("value is required")
					}
					return nil
				})
			}
			form := huh.NewForm(huh.NewGroup(field)).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "confirm":
			var value bool
			form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(p.Title).Description(p.Description).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "select":
			if len(p.Options) == 0 {
				return nil, fmt.Errorf("plugin %s prompt %s has no options", pluginID, p.ID)
			}
			opts := make([]huh.Option[string], 0, len(p.Options))
			for _, o := range p.Options {
				opts = append(opts, huh.NewOption(o.Label, o.Value))
			}
			var value string
			form := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(p.Title).Description(p.Description).Options(opts...).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		case "multiselect":
			if len(p.Options) == 0 {
				return nil, fmt.Errorf("plugin %s prompt %s has no options", pluginID, p.ID)
			}
			opts := make([]huh.Option[string], 0, len(p.Options))
			for _, o := range p.Options {
				opts = append(opts, huh.NewOption(o.Label, o.Value))
			}
			var value []string
			form := huh.NewForm(huh.NewGroup(huh.NewMultiSelect[string]().Title(p.Title).Description(p.Description).Options(opts...).Value(&value))).WithAccessible(accessible)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("plugin %s prompt %s failed: %w", pluginID, p.ID, err)
			}
			answers[p.ID] = value
		default:
			return nil, fmt.Errorf("plugin %s prompt %s has unsupported type %q", pluginID, p.ID, p.Type)
		}
	}
	return answers, nil
}

func findAutoAnswer(autoByPlugin map[string]map[string]interface{}, pluginID, formID, fieldID string) (interface{}, bool) {
	if autoByPlugin == nil {
		return nil, false
	}
	answers, ok := autoByPlugin[pluginID]
	if !ok || answers == nil {
		return nil, false
	}
	if formID != "" {
		if v, ok := answers[formID+"."+fieldID]; ok {
			return v, true
		}
	}
	v, ok := answers[fieldID]
	return v, ok
}

func parsePredefinedAnswer(raw, promptType string) (interface{}, error) {
	switch promptType {
	case "input", "select", "text":
		return raw, nil
	case "confirm":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected boolean, got %q", raw)
		}
		return v, nil
	case "multiselect":
		if strings.TrimSpace(raw) == "" {
			return []string{}, nil
		}
		items := strings.Split(raw, ",")
		out := make([]string, 0, len(items))
		for _, i := range items {
			out = append(out, strings.TrimSpace(i))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported prompt type %q", promptType)
	}
}

func findPredefined(predefined map[string]string, formID, fieldID string) (string, bool) {
	if predefined == nil {
		return "", false
	}
	if v, ok := predefined[formID+"."+fieldID]; ok {
		return v, true
	}
	v, ok := predefined[fieldID]
	return v, ok
}

func findGroupedPredefined(predefined map[string]string, pluginID, formID, fieldID string) (string, bool) {
	if predefined == nil {
		return "", false
	}
	if v, ok := predefined[pluginID+"."+formID+"."+fieldID]; ok {
		return v, true
	}
	if v, ok := predefined[pluginID+"."+fieldID]; ok {
		return v, true
	}
	return findPredefined(predefined, formID, fieldID)
}
