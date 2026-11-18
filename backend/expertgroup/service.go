package expertgroup

import (
	"context"
	"sort"
	"strings"

	"opsseek/backend/authorization"
	"opsseek/backend/engine"
)

// Store persists groups, members and bindings.
type Store interface {
	CreateGroup(context.Context, Group) (Group, error)
	GetGroup(context.Context, string) (Group, error)
	ListGroups(context.Context, string) ([]Group, error)
	UpdateGroup(context.Context, string, Group) (Group, error)
	ListMembers(context.Context, string) ([]Member, error)
	ReplaceMembers(context.Context, string, []Member) ([]Member, error)
	ListBindings(context.Context, string) ([]Binding, error)
	SetBinding(context.Context, string, Binding) (Binding, error)
	RemoveBinding(context.Context, Binding) error
	ApplicationBinding(context.Context, string, string) (Binding, bool, error)
	ScopeChainBinding(context.Context, string, string) (Binding, bool, error)
}

// PersonaResolver resolves a persona (with its published version) for one
// execution scope. persona.PersonaResolver satisfies this interface.
type PersonaResolver interface {
	Resolve(context.Context, string, string) (engine.Persona, error)
}

// Service is the management and resolution boundary for expert groups.
type Service struct {
	store    Store
	personas PersonaResolver
}

func NewService(store Store, personas PersonaResolver) *Service {
	return &Service{store: store, personas: personas}
}

// NewResolver exposes the service as the engine-side injection contract.
func NewResolver(store Store, personas PersonaResolver) *Service {
	return NewService(store, personas)
}

type GroupInput struct {
	ScopeID     string
	Identifier  string
	Name        string
	Description string
	Status      string
}

type GroupPatch struct {
	Name        *string
	Description *string
	Status      *string
}

func (s *Service) ListGroups(ctx context.Context, scopeID string) ([]Group, error) {
	scopeID = strings.TrimSpace(scopeID)
	if s == nil || s.store == nil || !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	groups, err := s.store.ListGroups(ctx, scopeID)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		members, memberErr := s.store.ListMembers(ctx, groups[index].ID)
		if memberErr != nil {
			return nil, memberErr
		}
		groups[index].Members = members
	}
	return groups, nil
}

func (s *Service) GetGroup(ctx context.Context, id string) (Group, error) {
	if s == nil || s.store == nil {
		return Group{}, invalid("expert group service is unavailable")
	}
	group, err := s.store.GetGroup(ctx, strings.TrimSpace(id))
	if err != nil {
		return Group{}, err
	}
	if !allowsScope(ctx, group.ScopeID) {
		return Group{}, authorization.ErrForbidden
	}
	group.Members, err = s.store.ListMembers(ctx, group.ID)
	return group, err
}

func (s *Service) CreateGroup(ctx context.Context, input GroupInput) (Group, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.Identifier = strings.TrimSpace(input.Identifier)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.ScopeID == "" || input.Identifier == "" || input.Name == "" {
		return Group{}, invalid("scope_id, identifier and name are required")
	}
	if len(strings.Fields(input.Identifier)) != 1 || len(strings.Fields(input.Name)) != 1 {
		return Group{}, invalid("expert group identifier and name must not contain whitespace")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return Group{}, invalid("expert group status is invalid")
	}
	if !allowsExactScope(ctx, input.ScopeID) {
		return Group{}, authorization.ErrForbidden
	}
	return s.store.CreateGroup(ctx, Group{ScopeID: input.ScopeID, Identifier: input.Identifier, Name: input.Name, Description: input.Description, Status: input.Status})
}

func (s *Service) UpdateGroup(ctx context.Context, id string, patch GroupPatch) (Group, error) {
	id = strings.TrimSpace(id)
	group, err := s.GetGroup(ctx, id)
	if err != nil {
		return Group{}, err
	}
	if !allowsExactScope(ctx, group.ScopeID) {
		return Group{}, authorization.ErrForbidden
	}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len(strings.Fields(name)) != 1 {
			return Group{}, invalid("expert group name is invalid")
		}
		group.Name = name
	}
	if patch.Description != nil {
		group.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Status != nil {
		status := strings.TrimSpace(*patch.Status)
		if status != "active" && status != "disabled" {
			return Group{}, invalid("expert group status is invalid")
		}
		group.Status = status
	}
	return s.store.UpdateGroup(ctx, group.ID, Group{Name: group.Name, Description: group.Description, Status: group.Status})
}

// ReplaceMembers validates and fully replaces the member seats (m2: physical
// replacement, no soft delete). Seat role constraints are hard checks against
// the resolved persona role (B2).
func (s *Service) ReplaceMembers(ctx context.Context, groupID string, members []Member) ([]Member, error) {
	group, err := s.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !allowsExactScope(ctx, group.ScopeID) {
		return nil, authorization.ErrForbidden
	}
	if group.Status != "active" {
		return nil, invalid("expert group is disabled")
	}
	if len(members) == 0 {
		return nil, invalid("expert group requires at least one member")
	}
	if len(members) > MaxMembers {
		return nil, invalid("expert group allows at most 20 members")
	}
	seen := make(map[string]struct{}, len(members))
	lead := 0
	for index := range members {
		member := &members[index]
		member.PersonaID = strings.TrimSpace(member.PersonaID)
		member.Seat = strings.TrimSpace(member.Seat)
		if member.Seat == "" {
			member.Seat = SeatAssistant
		}
		member.SceneTags = normalizeSceneTags(member.SceneTags)
		if member.Priority == 0 {
			member.Priority = 100
		}
		if member.PersonaID == "" {
			return nil, invalid("expert group member persona_id is required")
		}
		if _, duplicate := seen[member.PersonaID]; duplicate {
			return nil, invalid("expert group members must be unique")
		}
		seen[member.PersonaID] = struct{}{}
		switch member.Seat {
		case SeatLead:
			lead++
		case SeatAssistant:
		case SeatOncall:
			// Reserved enum (Q4): the API rejects saving it in this version.
			return nil, invalid("oncall seat is reserved and cannot be saved yet")
		default:
			return nil, invalid("expert group seat is invalid")
		}
		tags, tagErr := validateMemberSceneTags(member.SceneTags)
		if tagErr != nil {
			return nil, tagErr
		}
		member.SceneTags = tags
	}
	if lead != 1 {
		return nil, invalid("expert group requires exactly one lead member")
	}
	if s.personas != nil {
		roles := make([]string, len(members))
		for index := range members {
			profile, resolveErr := s.personas.Resolve(ctx, group.ScopeID, members[index].PersonaID)
			if resolveErr != nil {
				return nil, resolveErr
			}
			roles[index] = profile.Role
		}
		for index := range members {
			if err := validateSeatRole(members[index].Seat, roles[index]); err != nil {
				return nil, err
			}
		}
	}
	saved, err := s.store.ReplaceMembers(ctx, group.ID, members)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func validateSeatRole(seat, role string) error {
	if seat == SeatLead && role != "manager" {
		return invalid("lead seat requires a manager persona")
	}
	if seat == SeatAssistant && role != "worker" {
		return invalid("assistant seat requires a worker persona")
	}
	return nil
}

func normalizeSceneTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, duplicate := seen[tag]; duplicate {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func validateMemberSceneTags(tags []string) ([]string, error) {
	for _, tag := range tags {
		valid := false
		for _, allowed := range SceneTags {
			if tag == allowed && tag != "general" {
				valid = true
			}
		}
		if !valid {
			return nil, invalid("expert group member scene tags are invalid")
		}
	}
	return tags, nil
}

// SetBinding validates and upserts one binding. Application bindings store the
// application's current project scope in ScopeID; the caller supplies it
// (scopeID) so this package stays independent of the application domain.
func (s *Service) SetBinding(ctx context.Context, actorID string, binding Binding) (Binding, error) {
	binding.ScopeID = strings.TrimSpace(binding.ScopeID)
	binding.SceneTag = strings.TrimSpace(binding.SceneTag)
	binding.GroupID = strings.TrimSpace(binding.GroupID)
	binding.ApplicationID = strings.TrimSpace(binding.ApplicationID)
	if binding.ScopeID == "" || binding.GroupID == "" || !validSceneTag(binding.SceneTag) {
		return Binding{}, invalid("binding scope_id, scene_tag and group_id are required")
	}
	if !allowsExactScope(ctx, binding.ScopeID) {
		return Binding{}, authorization.ErrForbidden
	}
	group, err := s.store.GetGroup(ctx, binding.GroupID)
	if err != nil {
		return Binding{}, err
	}
	if !allowsScope(ctx, group.ScopeID) {
		return Binding{}, authorization.ErrForbidden
	}
	saved, err := s.store.SetBinding(ctx, strings.TrimSpace(actorID), binding)
	if err != nil {
		return Binding{}, err
	}
	return saved, nil
}

func (s *Service) ListBindings(ctx context.Context, scopeID string) ([]Binding, error) {
	scopeID = strings.TrimSpace(scopeID)
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListBindings(ctx, scopeID)
}

func (s *Service) RemoveBinding(ctx context.Context, binding Binding) error {
	binding.ScopeID = strings.TrimSpace(binding.ScopeID)
	binding.SceneTag = strings.TrimSpace(binding.SceneTag)
	binding.GroupID = strings.TrimSpace(binding.GroupID)
	binding.ApplicationID = strings.TrimSpace(binding.ApplicationID)
	if binding.ScopeID == "" || binding.SceneTag == "" || !validSceneTag(binding.SceneTag) {
		return invalid("binding scope_id and scene_tag are required")
	}
	if !allowsExactScope(ctx, binding.ScopeID) {
		return authorization.ErrForbidden
	}
	return s.store.RemoveBinding(ctx, binding)
}

func validSceneTag(tag string) bool {
	for _, item := range SceneTags {
		if item == tag {
			return true
		}
	}
	return false
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}

func allowsExactScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}
