package pluginruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	defaultPluginTimeout = 5 * time.Second
	defaultMaxOutputSize = 1 << 20 // 1MB
)

// Runner executes plugin processes and validates protocol behavior.
type Runner struct {
	DefaultTimeout       time.Duration
	MaxOutputBytes       int
	PromptHandler        PromptHandler
	UIHandler            UIHandler
	GroupedUIHandler     GroupedUIHandler
	MaxPromptRounds      int
	AllowPluginNetwork   bool
	AllowPluginGitWrite  bool
	AllowFilesystemWrite bool
	AllowPluginSecrets   bool
}

// PromptHandler resolves plugin prompt requests into answer values by prompt ID.
type PromptHandler func(pluginID string, prompts []PromptRequest) (map[string]interface{}, error)

// UIHandler resolves declarative UI requests into answer values by field ID.
type UIHandler func(pluginID string, forms []UIRequest) (map[string]interface{}, error)

// PluginUIBatchRequest groups one plugin's forms for same-page rendering.
type PluginUIBatchRequest struct {
	PluginID string
	Forms    []UIRequest
}

// GroupedUIHandler resolves answers for a group of plugins shown on one page.
type GroupedUIHandler func(groupID string, requests []PluginUIBatchRequest) (map[string]map[string]interface{}, error)

func NewRunner() *Runner {
	return &Runner{
		DefaultTimeout:       defaultPluginTimeout,
		MaxOutputBytes:       defaultMaxOutputSize,
		MaxPromptRounds:      2,
		AllowPluginNetwork:   false,
		AllowPluginGitWrite:  false,
		AllowFilesystemWrite: false,
		AllowPluginSecrets:   false,
	}
}

// RunPhase executes all plugins that support a hook in deterministic order.
func (r *Runner) RunPhase(ctx context.Context, hook HookPhase, draft *CommitDraft, reqCtx RequestContext, plugins []RuntimePlugin) ([]Invocation, error) {
	sorted := append([]RuntimePlugin(nil), plugins...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Order == sorted[j].Order {
			return sorted[i].Manifest.ID < sorted[j].Manifest.ID
		}
		return sorted[i].Order < sorted[j].Order
	})

	results := make([]Invocation, 0, len(sorted))
	for i := 0; i < len(sorted); i++ {
		rp := sorted[i]
		if !supportsHook(rp.Manifest.Hooks, hook) {
			continue
		}

		if r.GroupedUIHandler != nil && strings.TrimSpace(rp.UIGroup) != "" {
			group := []RuntimePlugin{rp}
			j := i + 1
			for ; j < len(sorted); j++ {
				next := sorted[j]
				if next.UIGroup != rp.UIGroup {
					break
				}
				if supportsHook(next.Manifest.Hooks, hook) {
					group = append(group, next)
				}
			}
			if len(group) > 1 {
				groupInvocations, stop, err := r.runGroupedUIPlugins(ctx, hook, draft, reqCtx, rp.UIGroup, group)
				results = append(results, groupInvocations...)
				if err != nil {
					return results, err
				}
				if stop {
					break
				}
				i = j - 1
				continue
			}
		}

		request := newPluginRequest(hook, reqCtx, *draft, rp)
		invocation, err := r.invokeWithPrompts(ctx, rp, request)
		if err != nil {
			if rp.FailureMode == FailOpen && !isRequiredAnswerError(err) {
				appendFailOpenInvocation(&results, rp, hook, request.RequestID, err)
				continue
			}
			return results, err
		}

		if invocation.Response.Mutations != nil {
			ApplyMutations(draft, *invocation.Response.Mutations)
		}
		results = append(results, invocation)

		if invocation.Response.Fatal || invocation.Response.BlockCommit {
			break
		}
	}

	return results, nil
}

func (r *Runner) runGroupedUIPlugins(ctx context.Context, hook HookPhase, draft *CommitDraft, reqCtx RequestContext, groupID string, group []RuntimePlugin) ([]Invocation, bool, error) {
	results := []Invocation{}

	type groupedPending struct {
		Plugin RuntimePlugin
		Req    Request
		Forms  []UIRequest
	}

	pending := make([]groupedPending, 0, len(group))
	requests := make([]PluginUIBatchRequest, 0, len(group))

	for _, rp := range group {
		req := newPluginRequest(hook, reqCtx, *draft, rp)
		invocation, err := r.Invoke(ctx, rp, req)
		if err != nil {
			if rp.FailureMode == FailOpen && !isRequiredAnswerError(err) {
				appendFailOpenInvocation(&results, rp, hook, req.RequestID, err)
				continue
			}
			return results, false, err
		}

		if len(invocation.Response.PromptRequests) > 0 {
			// Legacy prompt requests are handled with the normal mediation flow.
			invocation, err = r.invokeWithPrompts(ctx, rp, req)
			if err != nil {
				if rp.FailureMode == FailOpen && !isRequiredAnswerError(err) {
					appendFailOpenInvocation(&results, rp, hook, req.RequestID, err)
					continue
				}
				return results, false, err
			}
			if invocation.Response.Mutations != nil {
				ApplyMutations(draft, *invocation.Response.Mutations)
			}
			results = append(results, invocation)
			if invocation.Response.Fatal || invocation.Response.BlockCommit {
				return results, true, nil
			}
			continue
		}

		if len(invocation.Response.UIRequests) == 0 {
			if err := validateRequiredAnswersProvided(hook, rp, req.Answers); err != nil {
				return results, false, err
			}
			if invocation.Response.Mutations != nil {
				ApplyMutations(draft, *invocation.Response.Mutations)
			}
			results = append(results, invocation)
			if invocation.Response.Fatal || invocation.Response.BlockCommit {
				return results, true, nil
			}
			continue
		}

		pending = append(pending, groupedPending{
			Plugin: rp,
			Req:    req,
			Forms:  invocation.Response.UIRequests,
		})
		requests = append(requests, PluginUIBatchRequest{
			PluginID: rp.Manifest.ID,
			Forms:    invocation.Response.UIRequests,
		})
	}

	if len(pending) == 0 {
		return results, false, nil
	}

	answersByPlugin, err := r.GroupedUIHandler(groupID, requests)
	if err != nil {
		return results, false, fmt.Errorf("grouped prompt mediation failed for %s: %w", groupID, err)
	}

	for _, item := range pending {
		req := item.Req
		if answers, ok := answersByPlugin[item.Plugin.Manifest.ID]; ok && len(answers) > 0 {
			req.Answers = answers
		}
		invocation, err := r.invokeWithPrompts(ctx, item.Plugin, req)
		if err != nil {
			if item.Plugin.FailureMode == FailOpen && !isRequiredAnswerError(err) {
				appendFailOpenInvocation(&results, item.Plugin, hook, req.RequestID, err)
				continue
			}
			return results, false, err
		}

		if invocation.Response.Mutations != nil {
			ApplyMutations(draft, *invocation.Response.Mutations)
		}
		results = append(results, invocation)

		if invocation.Response.Fatal || invocation.Response.BlockCommit {
			return results, true, nil
		}
	}

	return results, false, nil
}

func (r *Runner) invokeWithPrompts(ctx context.Context, rp RuntimePlugin, req Request) (Invocation, error) {
	maxRounds := r.MaxPromptRounds
	if maxRounds <= 0 {
		maxRounds = 1
	}

	answers := map[string]interface{}{}
	for k, v := range req.Answers {
		answers[k] = v
	}
	for round := 0; round <= maxRounds; round++ {
		if len(answers) > 0 {
			req.Answers = answers
		}

		invocation, err := r.Invoke(ctx, rp, req)
		if err != nil {
			return Invocation{}, err
		}

		if len(invocation.Response.PromptRequests) == 0 && len(invocation.Response.UIRequests) == 0 {
			if err := validateRequiredAnswersProvided(req.Hook, rp, req.Answers); err != nil {
				return Invocation{}, err
			}
			return invocation, nil
		}
		if round == maxRounds {
			return Invocation{}, fmt.Errorf("plugin %s exceeded max prompt rounds", rp.Manifest.ID)
		}

		newAnswers, err := r.handlePluginUIRequests(rp.Manifest.ID, invocation.Response)
		if err != nil {
			return Invocation{}, fmt.Errorf("prompt mediation failed for %s: %w", rp.Manifest.ID, err)
		}
		for k, v := range newAnswers {
			answers[k] = v
		}
	}

	return Invocation{}, fmt.Errorf("unreachable prompt mediation state")
}

func newPluginRequest(hook HookPhase, reqCtx RequestContext, draft CommitDraft, rp RuntimePlugin) Request {
	return Request{
		ProtocolVersion: ProtocolVersionV1,
		RequestID:       fmt.Sprintf("%s:%d", rp.Manifest.ID, time.Now().UnixNano()),
		PluginID:        rp.Manifest.ID,
		Hook:            hook,
		PluginConfig:    rp.Config,
		Context:         reqCtx,
		Draft:           draft,
	}
}

func appendFailOpenInvocation(results *[]Invocation, rp RuntimePlugin, hook HookPhase, requestID string, err error) {
	*results = append(*results, Invocation{
		PluginID: rp.Manifest.ID,
		Hook:     hook,
		Response: Response{
			RequestID: requestID,
			OK:        false,
			Diagnostics: []Diagnostic{{
				Level:   "warn",
				Message: fmt.Sprintf("plugin failed in fail_open mode: %v", err),
				Code:    "PLUGIN_FAIL_OPEN",
			}},
		},
	})
}

func (r *Runner) handlePluginUIRequests(pluginID string, resp Response) (map[string]interface{}, error) {
	if len(resp.UIRequests) > 0 {
		if r.UIHandler == nil {
			return nil, fmt.Errorf("plugin %s requested UI forms but no UI handler is configured", pluginID)
		}
		return r.UIHandler(pluginID, resp.UIRequests)
	}
	if len(resp.PromptRequests) > 0 {
		if r.PromptHandler == nil {
			return nil, fmt.Errorf("plugin %s requested prompts but no prompt handler is configured", pluginID)
		}
		return r.PromptHandler(pluginID, resp.PromptRequests)
	}
	return map[string]interface{}{}, nil
}

// Invoke executes a single plugin process.
func (r *Runner) Invoke(ctx context.Context, rp RuntimePlugin, req Request) (Invocation, error) {
	if err := validateManifest(rp.Manifest); err != nil {
		return Invocation{}, fmt.Errorf("manifest validation failed for %s: %w", rp.Manifest.ID, err)
	}
	if !supportsHook(rp.Manifest.Hooks, req.Hook) {
		return Invocation{}, fmt.Errorf("plugin %s does not support hook %s", rp.Manifest.ID, req.Hook)
	}
	if err := validateRequest(req); err != nil {
		return Invocation{}, fmt.Errorf("request validation failed: %w", err)
	}
	if err := r.checkPermissions(rp.Manifest.ID, rp.Manifest.Permissions); err != nil {
		return Invocation{}, err
	}

	rawReq, err := json.Marshal(req)
	if err != nil {
		return Invocation{}, fmt.Errorf("marshal request: %w", err)
	}

	timeout := rp.Timeout
	if timeout <= 0 {
		timeout = r.DefaultTimeout
		if timeout <= 0 {
			timeout = defaultPluginTimeout
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, rp.Manifest.Entrypoint.Command, rp.Manifest.Entrypoint.Args...)
	cmd.Stdin = bytes.NewReader(rawReq)
	cmd.Dir = req.Context.RepoRoot
	cmd.Env = buildPluginEnv(rp.Manifest.Permissions.Secrets, rp.Manifest.Entrypoint.Env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Invocation{}, fmt.Errorf("plugin %s timed out after %s", rp.Manifest.ID, timeout)
		}
		return Invocation{}, fmt.Errorf("plugin %s failed: %w stderr=%s", rp.Manifest.ID, err, stderr.String())
	}

	maxOut := r.MaxOutputBytes
	if maxOut <= 0 {
		maxOut = defaultMaxOutputSize
	}
	if stdout.Len() > maxOut {
		return Invocation{}, fmt.Errorf("plugin %s output exceeded max size %d", rp.Manifest.ID, maxOut)
	}

	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return Invocation{}, fmt.Errorf("invalid plugin response from %s: %w stderr=%s", rp.Manifest.ID, err, stderr.String())
	}
	if err := validateResponse(resp, req); err != nil {
		return Invocation{}, fmt.Errorf("response validation failed for %s: %w", rp.Manifest.ID, err)
	}

	return Invocation{
		PluginID: rp.Manifest.ID,
		Hook:     req.Hook,
		Response: resp,
		Stderr:   stderr.String(),
		Duration: duration,
	}, nil
}

func (r *Runner) checkPermissions(pluginID string, p Permissions) error {
	if p.Network && !r.AllowPluginNetwork {
		return fmt.Errorf("plugin %s requires network permission; re-run with --allow-plugin-network", pluginID)
	}
	if p.GitWrite && !r.AllowPluginGitWrite {
		return fmt.Errorf("plugin %s requires git_write permission; re-run with --allow-plugin-git-write", pluginID)
	}
	if len(p.FilesystemWrite) > 0 && !r.AllowFilesystemWrite {
		return fmt.Errorf("plugin %s requires filesystem_write permission; re-run with --allow-plugin-filesystem-write", pluginID)
	}
	if len(p.Secrets) > 0 && !r.AllowPluginSecrets {
		return fmt.Errorf("plugin %s requires secrets permission; re-run with --allow-plugin-secrets", pluginID)
	}
	return nil
}

func buildPluginEnv(requestedSecrets []string, manifestEnv map[string]string) []string {
	allowlist := []string{
		"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "USER", "LOGNAME", "SHELL",
		"LANG", "LC_ALL", "LC_CTYPE", "TERM",
	}
	env := make([]string, 0, len(allowlist)+len(requestedSecrets)+len(manifestEnv))
	added := map[string]bool{}

	addVar := func(name, value string) {
		if name == "" || added[name] {
			return
		}
		added[name] = true
		env = append(env, fmt.Sprintf("%s=%s", name, value))
	}

	for _, name := range allowlist {
		if v, ok := os.LookupEnv(name); ok {
			addVar(name, v)
		}
	}
	for _, name := range requestedSecrets {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		if v, ok := os.LookupEnv(key); ok {
			addVar(key, v)
		}
	}
	for k, v := range manifestEnv {
		addVar(k, v)
	}

	return env
}

// ApplyMutations applies plugin mutations to a draft in a deterministic order.
func ApplyMutations(draft *CommitDraft, m Mutations) {
	if draft.Metadata == nil {
		draft.Metadata = map[string]interface{}{}
	}
	if m.SetTitle != "" {
		draft.Title = m.SetTitle
	}
	if m.SetBody != "" {
		draft.Body = m.SetBody
	}
	if m.PrependBody != "" {
		draft.Body = m.PrependBody + draft.Body
	}
	if m.AppendBody != "" {
		draft.Body += m.AppendBody
	}
	if len(m.AddTrailers) > 0 {
		draft.Trailers = append(draft.Trailers, m.AddTrailers...)
	}
	for k, v := range m.MetadataPatch {
		draft.Metadata[k] = v
	}
}
