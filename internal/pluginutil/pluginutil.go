package pluginutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	plugins "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

func ReadRequest() (plugins.Request, error) {
	var req plugins.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return plugins.Request{}, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func WriteResponse(resp plugins.Response) error {
	return json.NewEncoder(os.Stdout).Encode(resp)
}

func NewResponse(req plugins.Request) plugins.Response {
	return plugins.Response{
		RequestID:   req.RequestID,
		OK:          true,
		Diagnostics: []plugins.Diagnostic{},
	}
}

func AddInfo(resp *plugins.Response, code, message string) {
	resp.Diagnostics = append(resp.Diagnostics, plugins.Diagnostic{Level: "info", Code: code, Message: message})
}

func AddError(resp *plugins.Response, code, message string) {
	resp.Diagnostics = append(resp.Diagnostics, plugins.Diagnostic{Level: "error", Code: code, Message: message})
}

func ReadJSONFile(path string, v interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func ConfigString(cfg map[string]interface{}, key, fallback string) string {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return fallback
	}
	return s
}

func ConfigBool(cfg map[string]interface{}, key string, fallback bool) bool {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

func ConfigInt(cfg map[string]interface{}, key string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return fallback
	}
}

func ConfigStringSlice(cfg map[string]interface{}, key string) []string {
	if cfg == nil {
		return nil
	}
	v, ok := cfg[key]
	if !ok {
		return nil
	}
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func GetAnswerString(req plugins.Request, key string) (string, bool) {
	if req.Answers == nil {
		return "", false
	}
	v, ok := req.Answers[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func GetAnswerBool(req plugins.Request, key string) (bool, bool) {
	if req.Answers == nil {
		return false, false
	}
	v, ok := req.Answers[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if ok {
		return b, true
	}
	if s, ok := v.(string); ok {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			return parsed, true
		}
	}
	return false, false
}

func GetAnswerStringSlice(req plugins.Request, key string) ([]string, bool) {
	if req.Answers == nil {
		return nil, false
	}
	v, ok := req.Answers[key]
	if !ok {
		return nil, false
	}
	slice, ok := v.([]interface{})
	if ok {
		out := make([]string, 0, len(slice))
		for _, item := range slice {
			s, ok := item.(string)
			if ok {
				out = append(out, s)
			}
		}
		return out, true
	}
	arr, ok := v.([]string)
	if ok {
		return arr, true
	}
	return nil, false
}

func Submitted(req plugins.Request, formID string) bool {
	v, ok := GetAnswerBool(req, formID+".__submitted")
	return ok && v
}

func AppendBodySection(body, section string) string {
	body = strings.TrimSpace(body)
	section = strings.TrimSpace(section)
	if body == "" {
		return section
	}
	if section == "" {
		return body
	}
	return section + "\n\n" + body
}
