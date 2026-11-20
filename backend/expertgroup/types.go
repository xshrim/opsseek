// Package expertgroup owns the expert group domain: reusable expert teams
// made of persona seats, their application/scope bindings and the four-level
// binding resolution consumed by diagnosis and inspection callers.
package expertgroup

import (
	"errors"
	"time"
)

var (
	// ErrNotFound is returned when a group, binding or referenced object does
	// not exist.
	ErrNotFound = errors.New("expert group not found")
	// ErrConflict is returned on duplicate identifiers or binding conflicts.
	ErrConflict = errors.New("expert group conflicts with existing data")
)

// ValidationError marks caller-fixable input errors.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }

// Seats (R001 §2.1). lead requires a manager persona, assistant requires a
// worker persona; oncall is a reserved enum value rejected by the API.
const (
	SeatLead      = "lead"
	SeatAssistant = "assistant"
	SeatOncall    = "oncall"
)

// SceneTags mirror the engine scope binding vocabulary. general is only used
// by bindings; member scene tags use the three execution scenes.
var SceneTags = []string{"general", "diagnosis", "inspection", "workflow"}

const MaxMembers = 20

// Group is the managed expert team configuration.
type Group struct {
	ID          string    `json:"id"`
	ScopeID     string    `json:"scope_id"`
	Identifier  string    `json:"identifier"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Members     []Member  `json:"members,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Member is one seat inside a group. It references a persona; the persona is
// never copied and versions are resolved at execution time (Q1: no pinning).
type Member struct {
	ID        string    `json:"id,omitempty"`
	GroupID   string    `json:"group_id,omitempty"`
	PersonaID string    `json:"persona_id"`
	Seat      string    `json:"seat"`
	SceneTags []string  `json:"scene_tags"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Binding binds a group to an application (ApplicationID set) or to a
// scope+scene default (ApplicationID empty). For application bindings ScopeID
// redundantly stores the application's project scope at write time so orphan
// bindings are detectable at resolution (m4).
type Binding struct {
	ScopeID       string    `json:"scope_id"`
	SceneTag      string    `json:"scene_tag"`
	GroupID       string    `json:"group_id"`
	ApplicationID string    `json:"application_id,omitempty"`
	CreatedBy     string    `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// MemberSnapshot is the frozen, auditable projection of one member at
// execution time: the resolved persona identity plus its published version.
type MemberSnapshot struct {
	PersonaID string   `json:"persona_id"`
	Seat      string   `json:"seat"`
	SceneTags []string `json:"scene_tags"`
	Priority  int      `json:"priority"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Domain    string   `json:"domain,omitempty"`
	Version   int      `json:"version"`
}

// Snapshot is the complete frozen execution audit record (Q1 companion).
type Snapshot struct {
	GroupID                 string           `json:"group_id"`
	Name                    string           `json:"name"`
	ScopeID                 string           `json:"scope_id"`
	Scene                   string           `json:"scene"`
	Lead                    MemberSnapshot   `json:"lead"`
	Assistants              []MemberSnapshot `json:"assistants"`
	IgnoredWorkerPersonaIDs []string         `json:"ignored_worker_persona_ids,omitempty"`
}

// LeadPersonaID returns the frozen lead persona for engine.Request wiring.
func (s Snapshot) LeadPersonaID() string { return s.Lead.PersonaID }

// AssistantPersonaIDs returns the frozen assistant persona IDs in priority
// order, filtered for the frozen scene.
func (s Snapshot) AssistantPersonaIDs() []string {
	ids := make([]string, 0, len(s.Assistants))
	for _, member := range s.Assistants {
		ids = append(ids, member.PersonaID)
	}
	return ids
}

// ResolutionWarning records a non-fatal degradation observed while resolving
// (disabled group, orphan application binding) so callers can log an
// expert_group.* warning event (R001 §3.4.8).
type ResolutionWarning struct {
	Kind    string `json:"kind"`
	GroupID string `json:"group_id,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// ResolvedGroup is the resolution result handed to callers.
type ResolvedGroup struct {
	Snapshot
	Warnings []ResolutionWarning `json:"warnings,omitempty"`
}
