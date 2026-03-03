package main

import (
	"context"
	"fmt"
	"strings"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func runPluginPhases(ctx context.Context, runner *plugins.Runner, runtimePlugins []plugins.RuntimePlugin, reqCtx plugins.RequestContext, draft *plugins.CommitDraft, phases []plugins.HookPhase) ([]plugins.Invocation, error) {
	all := []plugins.Invocation{}
	for _, phase := range phases {
		invocations, err := runner.RunPhase(ctx, phase, draft, reqCtx, runtimePlugins)
		all = append(all, invocations...)
		if err != nil {
			return all, err
		}
		for _, inv := range invocations {
			if inv.Response.BlockCommit {
				reason := inv.Response.BlockReason
				if reason == "" {
					reason = "blocked by plugin"
				}
				return all, fmt.Errorf("commit blocked by %s: %s", inv.PluginID, reason)
			}
			if inv.Response.Fatal {
				return all, fmt.Errorf("fatal plugin response from %s", inv.PluginID)
			}
		}
	}
	return all, nil
}

func printPluginInvocations(invocations []plugins.Invocation) {
	for _, inv := range invocations {
		for _, d := range inv.Response.Diagnostics {
			fmt.Printf("[plugin:%s][%s][%s] %s\n", inv.PluginID, inv.Hook, d.Level, d.Message)
		}
		if inv.Stderr != "" {
			fmt.Printf("[plugin:%s][stderr] %s\n", inv.PluginID, strings.TrimSpace(inv.Stderr))
		}
	}
}

func buildAutoAnswersByPlugin(runtimePlugins []plugins.RuntimePlugin) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	for _, rp := range runtimePlugins {
		if len(rp.AIAuto) == 0 {
			continue
		}
		out[rp.Manifest.ID] = rp.AIAuto
	}
	return out
}
