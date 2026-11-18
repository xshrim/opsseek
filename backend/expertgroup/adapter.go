package expertgroup

import (
	"context"

	"opsseek/backend/engine"
)

// ResolveExpertGroup implements engine.ExpertGroupResolver: group ID plus the
// caller's scope map to the frozen member seats. The scene-specific filtering
// happened in the caller's resolution; this entry point uses the general scene.
func (s *Service) ResolveExpertGroup(ctx context.Context, scopeID, groupID string) (engine.ExpertGroupResolution, error) {
	resolved, err := s.ResolveGroup(ctx, scopeID, groupID, "general")
	if err != nil {
		return engine.ExpertGroupResolution{}, err
	}
	return resolved.toEngineResolution(), nil
}

func (r ResolvedGroup) toEngineResolution() engine.ExpertGroupResolution {
	result := engine.ExpertGroupResolution{
		GroupID: r.GroupID, Name: r.Name, ScopeID: r.ScopeID, Scene: r.Scene,
		Lead:                    r.Lead.toEngineSeat(),
		IgnoredWorkerPersonaIDs: r.IgnoredWorkerPersonaIDs,
	}
	for _, member := range r.Assistants {
		result.Assistants = append(result.Assistants, member.toEngineSeat())
	}
	return result
}

func (m MemberSnapshot) toEngineSeat() engine.ExpertGroupSeat {
	return engine.ExpertGroupSeat{
		PersonaID: m.PersonaID, Seat: m.Seat, SceneTags: m.SceneTags,
		Priority: m.Priority, Name: m.Name, Role: m.Role, Domain: m.Domain, Version: m.Version,
	}
}

var _ engine.ExpertGroupResolver = (*Service)(nil)
