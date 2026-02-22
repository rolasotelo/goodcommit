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

	switch req.Hook {
	case plugins.HookCollect:
		if pluginutil.Submitted(req, "body") {
			body, _ := pluginutil.GetAnswerString(req, "commit_body")
			resp.Mutations = &plugins.Mutations{SetBody: normalizeBody(body)}
			pluginutil.AddInfo(&resp, "BODY_SET", "commit body captured")
			return pluginutil.WriteResponse(resp)
		}

		resp.UIRequests = []plugins.UIRequest{{
			ID:          "body",
			Title:       "Write Commit Body",
			Description: "Provide a detailed description.",
			Fields: []plugins.UIField{
				{ID: "commit_body", Type: "text", Title: "Body", Editor: true, Value: req.Draft.Body},
			},
		}}
		pluginutil.AddInfo(&resp, "BODY_PROMPT", "prompting commit body")
		return pluginutil.WriteResponse(resp)
	default:
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "body plugin runs only on collect")
		return pluginutil.WriteResponse(resp)
	}
}

func normalizeBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	first := strings.ToUpper(body[:1])
	body = first + body[1:]
	if !strings.HasSuffix(body, ".") && !strings.HasSuffix(body, "!") && !strings.HasSuffix(body, "?") {
		body += "."
	}
	return body
}
