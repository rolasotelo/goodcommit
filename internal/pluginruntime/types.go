package pluginruntime

import (
	"time"

	api "github.com/rolasotelo/goodcommit/pkg/pluginapi"
)

type HookPhase = api.HookPhase

type FailureMode = api.FailureMode

const (
	HookCollect    = api.HookCollect
	HookValidate   = api.HookValidate
	HookEnrich     = api.HookEnrich
	HookFinalize   = api.HookFinalize
	HookPreCommit  = api.HookPreCommit
	HookPostCommit = api.HookPostCommit

	FailClosed = api.FailClosed
	FailOpen   = api.FailOpen

	ProtocolVersionV1 = api.ProtocolVersionV1
)

type Trailer = api.Trailer

type CommitDraft = api.CommitDraft

type RequestContext = api.RequestContext

type Request = api.Request

type Diagnostic = api.Diagnostic

type PromptOption = api.PromptOption

type PromptRequest = api.PromptRequest

type UIOption = api.UIOption

type UIField = api.UIField

type UIRequest = api.UIRequest

type Mutations = api.Mutations

type Response = api.Response

type EntryPoint = api.EntryPoint

type Permissions = api.Permissions

type Manifest = api.Manifest

// RuntimePlugin configures execution details for one enabled plugin instance.
type RuntimePlugin struct {
	Manifest    Manifest
	Config      map[string]interface{}
	AIHints     *api.AIHints
	AIAuto      map[string]interface{}
	Order       int
	FailureMode FailureMode
	Timeout     time.Duration
}

// Invocation captures details from a plugin execution.
type Invocation struct {
	PluginID string
	Hook     HookPhase
	Response Response
	Stderr   string
	Duration time.Duration
}
