package engine

import "context"

// Expert group extension contract (I009 R001 §3.4). The Runtime never queries
// storage: it receives an injected ExpertGroupResolver (same pattern as
// PersonaResolver) implemented by the expertgroup package at the assembly
// layer. Callers (diagnosis/inspection) complete the full four-level binding
// resolution before building a Request and pass the resolved group ID here.

// ExpertGroupSeat is one resolved member seat: the persona identity, its seat,
// scene tags and the resolved published persona version.
type ExpertGroupSeat struct {
	PersonaID string   `json:"persona_id"`
	Seat      string   `json:"seat"`
	SceneTags []string `json:"scene_tags,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	Name      string   `json:"name,omitempty"`
	Role      string   `json:"role,omitempty"`
	Domain    string   `json:"domain,omitempty"`
	Version   int      `json:"version,omitempty"`
}

// ExpertGroupResolution is the frozen member snapshot for one execution.
type ExpertGroupResolution struct {
	GroupID                 string            `json:"group_id"`
	Name                    string            `json:"name,omitempty"`
	ScopeID                 string            `json:"scope_id"`
	Scene                   string            `json:"scene,omitempty"`
	Lead                    ExpertGroupSeat   `json:"lead"`
	Assistants              []ExpertGroupSeat `json:"assistants,omitempty"`
	IgnoredWorkerPersonaIDs []string          `json:"ignored_worker_persona_ids,omitempty"`
}

// ExpertGroupResolver maps a group ID to its member seats for one execution
// scope. Implementations must re-validate seat roles and persona visibility;
// they never widen the caller's scope.
type ExpertGroupResolver interface {
	ResolveExpertGroup(context.Context, string, string) (ExpertGroupResolution, error)
}

// EventExpertGroupResolved is the platform lifecycle event emitted when an
// execution freezes an expert group. It is registered as a platform-extension
// event family (see adapter.go) and is never emitted by a runtime adapter.
const EventExpertGroupResolved = "expert_group.resolved"
