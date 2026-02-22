package pluginruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

const (
	defaultPluginTimeout = 5 * time.Second
	defaultMaxOutputSize = 1 << 20 // 1MB
)

// Runner executes plugin processes and validates protocol behavior.
type Runner struct {
	DefaultTimeout  time.Duration
	MaxOutputBytes  int
	PromptHandler   PromptHandler
	UIHandler       UIHandler
	MaxPromptRounds int
}

// PromptHandler resolves plugin prompt requests into answer values by prompt ID.
type PromptHandler func(pluginID string, prompts []PromptRequest) (map[string]interface{}, error)

// UIHandler resolves declarative UI requests into answer values by field ID.
type UIHandler func(pluginID string, forms []UIRequest) (map[string]interface{}, error)

func NewRunner() *Runner {
	return &Runner{
		DefaultTimeout:  defaultPluginTimeout,
		MaxOutputBytes:  defaultMaxOutputSize,
		MaxPromptRounds: 2,
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
	for _, rp := range sorted {
		if !supportsHook(rp.Manifest.Hooks, hook) {
			continue
		}
		request := Request{
			ProtocolVersion: ProtocolVersionV1,
			RequestID:       fmt.Sprintf("%s:%d", rp.Manifest.ID, time.Now().UnixNano()),
			PluginID:        rp.Manifest.ID,
			Hook:            hook,
			PluginConfig:    rp.Config,
			Context:         reqCtx,
			Draft:           *draft,
		}

		invocation, err := r.invokeWithPrompts(ctx, rp, request)
		if err != nil {
			if rp.FailureMode == FailOpen {
				results = append(results, Invocation{
					PluginID: rp.Manifest.ID,
					Hook:     hook,
					Response: Response{
						RequestID: request.RequestID,
						OK:        false,
						Diagnostics: []Diagnostic{{
							Level:   "warn",
							Message: fmt.Sprintf("plugin failed in fail_open mode: %v", err),
							Code:    "PLUGIN_FAIL_OPEN",
						}},
					},
				})
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

func (r *Runner) invokeWithPrompts(ctx context.Context, rp RuntimePlugin, req Request) (Invocation, error) {
	maxRounds := r.MaxPromptRounds
	if maxRounds <= 0 {
		maxRounds = 1
	}

	answers := map[string]interface{}{}
	for round := 0; round <= maxRounds; round++ {
		if len(answers) > 0 {
			req.Answers = answers
		}

		invocation, err := r.Invoke(ctx, rp, req)
		if err != nil {
			return Invocation{}, err
		}

		if len(invocation.Response.PromptRequests) == 0 && len(invocation.Response.UIRequests) == 0 {
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
	cmd.Env = append([]string{}, os.Environ()...)
	for k, v := range rp.Manifest.Entrypoint.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

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
