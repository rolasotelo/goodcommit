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
		pluginutil.AddInfo(&resp, "SKIP_HOOK", "signedoffby plugin runs only on finalize")
		return pluginutil.WriteResponse(resp)
	}

	name := strings.TrimSpace(pluginutil.ConfigString(req.PluginConfig, "name", req.Context.GitUserName))
	email := strings.TrimSpace(pluginutil.ConfigString(req.PluginConfig, "email", req.Context.GitUserEmail))
	if name == "" || email == "" {
		resp.OK = false
		resp.Fatal = true
		pluginutil.AddError(&resp, "SIGNEDOFFBY_IDENTITY_REQUIRED", "git user.name and user.email are required")
		return pluginutil.WriteResponse(resp)
	}
	key := pluginutil.ConfigString(req.PluginConfig, "trailer_key", "Signed-off-by")
	resp.Mutations = &plugins.Mutations{AddTrailers: []plugins.Trailer{{Key: key, Value: fmt.Sprintf("%s <%s>", name, email)}}}
	pluginutil.AddInfo(&resp, "SIGNEDOFFBY_ADDED", "added signed-off-by trailer")
	return pluginutil.WriteResponse(resp)
}
