package expertgroup

import (
	"context"
	"fmt"
	"strings"

	"opsseek/backend/authorization"
)

// ResolveGroup resolves one group into its frozen execution snapshot for a
// scene. It performs the execution-time seat re-validation (B2 double check),
// resolves every member's latest published persona version (Q1: no pinning)
// and records the lead persona's embedded worker list as ignored (Q2).
func (s *Service) ResolveGroup(ctx context.Context, scopeID, groupID, scene string) (ResolvedGroup, error) {
	if s == nil || s.store == nil {
		return ResolvedGroup{}, invalid("expert group service is unavailable")
	}
	scopeID, groupID = strings.TrimSpace(scopeID), strings.TrimSpace(groupID)
	if scopeID == "" || groupID == "" {
		return ResolvedGroup{}, invalid("scope_id and expert_group_id are required")
	}
	if scene == "" {
		scene = "general"
	}
	group, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return ResolvedGroup{}, err
	}
	if group.Status != "active" {
		return ResolvedGroup{}, invalid("expert group is disabled")
	}
	if !allowsScope(ctx, group.ScopeID) {
		return ResolvedGroup{}, authorization.ErrForbidden
	}
	members, err := s.store.ListMembers(ctx, group.ID)
	if err != nil {
		return ResolvedGroup{}, err
	}
	if len(members) == 0 {
		return ResolvedGroup{}, invalid("expert group has no members")
	}
	if s.personas == nil {
		return ResolvedGroup{}, invalid("persona resolver is unavailable")
	}
	resolution := ResolvedGroup{Snapshot: Snapshot{GroupID: group.ID, Name: group.Name, ScopeID: group.ScopeID, Scene: scene, Assistants: []MemberSnapshot{}}}
	leadFound := false
	for _, member := range members {
		if !memberParticipates(member, scene) {
			continue
		}
		profile, resolveErr := s.personas.Resolve(ctx, group.ScopeID, member.PersonaID)
		if resolveErr != nil {
			return ResolvedGroup{}, fmt.Errorf("resolve expert group member %s: %w", member.PersonaID, resolveErr)
		}
		if err := validateSeatRole(member.Seat, profile.Role); err != nil {
			return ResolvedGroup{}, err
		}
		if member.Seat == SeatOncall {
			return ResolvedGroup{}, invalid("oncall seat is not executable yet")
		}
		snapshotMember := MemberSnapshot{
			PersonaID: member.PersonaID, Seat: member.Seat, SceneTags: member.SceneTags,
			Priority: member.Priority, Name: profile.Name, Role: profile.Role,
			Domain: profile.Domain, Version: profile.Version,
		}
		if member.Seat == SeatLead {
			leadFound = true
			resolution.Lead = snapshotMember
			if len(profile.WorkerPersonaIDs) > 0 {
				resolution.IgnoredWorkerPersonaIDs = append([]string(nil), profile.WorkerPersonaIDs...)
			}
			continue
		}
		resolution.Assistants = append(resolution.Assistants, snapshotMember)
	}
	if !leadFound {
		return ResolvedGroup{}, invalid("expert group lead is unavailable for this scene")
	}
	return resolution, nil
}

func memberParticipates(member Member, scene string) bool {
	if len(member.SceneTags) == 0 {
		return true
	}
	for _, tag := range member.SceneTags {
		if tag == scene {
			return true
		}
	}
	return false
}

// ResolveEffective implements the full four-level binding resolution
// (R001 §2.2): application exact tag -> application general -> scope chain
// exact tag -> scope chain general; tags beat hierarchy at every level and
// general is the global fallback. A disabled group is equivalent to no
// binding and resolution keeps descending with a recorded warning (M5-8).
// applicationScopeID is the application's current project scope; a mismatch
// with an application binding's stored scope marks an orphan binding which is
// skipped with a warning (m4).
func (s *Service) ResolveEffective(ctx context.Context, scopeID, applicationID, applicationScopeID, scene string) (ResolvedGroup, bool, error) {
	if s == nil || s.store == nil {
		return ResolvedGroup{}, false, invalid("expert group service is unavailable")
	}
	scopeID = strings.TrimSpace(scopeID)
	applicationID = strings.TrimSpace(applicationID)
	if scopeID == "" {
		return ResolvedGroup{}, false, invalid("scope_id is required")
	}
	if !allowsScope(ctx, scopeID) {
		return ResolvedGroup{}, false, authorization.ErrForbidden
	}
	if scene == "" {
		scene = "general"
	}
	var warnings []ResolutionWarning
	tryCandidate := func(binding Binding) (ResolvedGroup, bool, error) {
		group, err := s.store.GetGroup(ctx, binding.GroupID)
		if err != nil {
			if err == ErrNotFound {
				warnings = append(warnings, ResolutionWarning{Kind: "binding_group_missing", GroupID: binding.GroupID})
				return ResolvedGroup{}, false, nil
			}
			return ResolvedGroup{}, false, err
		}
		if group.Status != "active" {
			warnings = append(warnings, ResolutionWarning{Kind: "group_disabled", GroupID: group.ID, Detail: fmt.Sprintf("scene=%s binding skipped", binding.SceneTag)})
			return ResolvedGroup{}, false, nil
		}
		resolved, err := s.ResolveGroup(ctx, scopeID, group.ID, scene)
		if err != nil {
			return ResolvedGroup{}, false, err
		}
		resolved.Warnings = warnings
		return resolved, true, nil
	}
	if applicationID != "" {
		for _, tag := range []string{scene, "general"} {
			binding, found, err := s.store.ApplicationBinding(ctx, applicationID, tag)
			if err != nil {
				return ResolvedGroup{}, false, err
			}
			if !found {
				continue
			}
			if applicationScopeID != "" && binding.ScopeID != applicationScopeID {
				// Orphan application binding (application moved projects): skip
				// and fall through to the scope chain (m4).
				warnings = append(warnings, ResolutionWarning{Kind: "orphan_application_binding", GroupID: binding.GroupID, Detail: fmt.Sprintf("binding scope %s no longer matches application scope %s", binding.ScopeID, applicationScopeID)})
				continue
			}
			if resolved, ok, resolveErr := tryCandidate(binding); ok || resolveErr != nil {
				return resolved, ok, resolveErr
			}
		}
	}
	for _, tag := range []string{scene, "general"} {
		binding, found, err := s.store.ScopeChainBinding(ctx, scopeID, tag)
		if err != nil {
			return ResolvedGroup{}, false, err
		}
		if !found {
			continue
		}
		if resolved, ok, resolveErr := tryCandidate(binding); ok || resolveErr != nil {
			return resolved, ok, resolveErr
		}
	}
	return ResolvedGroup{Snapshot: Snapshot{Scene: scene}, Warnings: warnings}, false, nil
}
