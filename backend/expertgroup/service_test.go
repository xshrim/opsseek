package expertgroup

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"opsseek/backend/engine"
)

// fakeResolver resolves personas from an in-memory map keyed by persona ID.
type fakeResolver struct {
	personas map[string]engine.Persona
}

func (r fakeResolver) Resolve(_ context.Context, _, personaID string) (engine.Persona, error) {
	profile, ok := r.personas[personaID]
	if !ok {
		return engine.Persona{}, ErrNotFound
	}
	return profile, nil
}

// memStore is an in-memory Store used by the resolution truth table tests.
type memStore struct {
	groups   map[string]Group
	members  map[string][]Member
	bindings []Binding
	nextID   int
}

func newMemStore() *memStore {
	return &memStore{groups: map[string]Group{}, members: map[string][]Member{}}
}

func (s *memStore) CreateGroup(_ context.Context, group Group) (Group, error) {
	s.nextID++
	group.ID = fmt.Sprintf("group-%d", s.nextID)
	group.Members = nil
	s.groups[group.ID] = group
	return group, nil
}

func (s *memStore) GetGroup(_ context.Context, id string) (Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return Group{}, ErrNotFound
	}
	return group, nil
}

func (s *memStore) ListGroups(_ context.Context, scopeID string) ([]Group, error) {
	var groups []Group
	for _, group := range s.groups {
		if group.ScopeID == scopeID {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (s *memStore) UpdateGroup(_ context.Context, id string, patch Group) (Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return Group{}, ErrNotFound
	}
	group.Name, group.Description, group.Status = patch.Name, patch.Description, patch.Status
	s.groups[id] = group
	return group, nil
}

func (s *memStore) ListMembers(_ context.Context, groupID string) ([]Member, error) {
	return append([]Member(nil), s.members[groupID]...), nil
}

func (s *memStore) ReplaceMembers(_ context.Context, groupID string, members []Member) ([]Member, error) {
	s.members[groupID] = append([]Member(nil), members...)
	return members, nil
}

func (s *memStore) ListBindings(_ context.Context, scopeID string) ([]Binding, error) {
	var bindings []Binding
	for _, binding := range s.bindings {
		if binding.ScopeID == scopeID {
			bindings = append(bindings, binding)
		}
	}
	return bindings, nil
}

func (s *memStore) SetBinding(_ context.Context, _ string, binding Binding) (Binding, error) {
	for index, existing := range s.bindings {
		if existing.ScopeID == binding.ScopeID && existing.SceneTag == binding.SceneTag && existing.ApplicationID == binding.ApplicationID {
			s.bindings[index] = binding
			return binding, nil
		}
	}
	s.bindings = append(s.bindings, binding)
	return binding, nil
}

func (s *memStore) RemoveBinding(_ context.Context, binding Binding) error {
	for index, existing := range s.bindings {
		if existing == binding {
			s.bindings = append(s.bindings[:index], s.bindings[index+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (s *memStore) ApplicationBinding(_ context.Context, applicationID, sceneTag string) (Binding, bool, error) {
	for _, binding := range s.bindings {
		if binding.ApplicationID == applicationID && binding.SceneTag == sceneTag {
			return binding, true, nil
		}
	}
	return Binding{}, false, nil
}

func (s *memStore) ScopeChainBinding(_ context.Context, scopeID, sceneTag string) (Binding, bool, error) {
	for _, binding := range s.bindings {
		if binding.ApplicationID == "" && binding.ScopeID == scopeID && binding.SceneTag == sceneTag {
			return binding, true, nil
		}
	}
	return Binding{}, false, nil
}

func seededService(t *testing.T) (*Service, *memStore) {
	t.Helper()
	store := newMemStore()
	personas := fakeResolver{personas: map[string]engine.Persona{
		"manager-1": {PersonaID: "manager-1", ScopeID: "scope", Name: "lead", Role: "manager", WorkerPersonaIDs: []string{"worker-9"}, Version: 3},
		"worker-1":  {PersonaID: "worker-1", ScopeID: "scope", Name: "db", Role: "worker", Domain: "PostgreSQL", Version: 2},
		"worker-2":  {PersonaID: "worker-2", ScopeID: "scope", Name: "cache", Role: "worker", Domain: "Redis", Version: 4},
		"manager-2": {PersonaID: "manager-2", ScopeID: "scope", Name: "ops", Role: "manager", Version: 1},
	}}
	service := NewService(store, personas)
	return service, store
}

func mustGroup(t *testing.T, service *Service, name string) Group {
	t.Helper()
	group, err := service.CreateGroup(context.Background(), GroupInput{ScopeID: "scope", Identifier: name, Name: name})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	return group
}

func TestResolveEffectiveTruthTable(t *testing.T) {
	// The authoritative truth table from R001 §2.2.
	cases := []struct {
		name              string
		appExact          string // group bound to application with scene_tag=diagnosis
		appGeneral        string // group bound to application with scene_tag=general
		scopeExact        string // group bound to scope with scene_tag=diagnosis
		scopeGeneral      string // group bound to scope with scene_tag=general
		orphan            bool   // application binding scope no longer matches
		wantGroup         string
		wantFound         bool
		wantOrphanWarning bool
	}{
		{name: "application exact tag wins", appExact: "g1", wantGroup: "g1", wantFound: true},
		{name: "application general beats scope exact", appGeneral: "g2", scopeExact: "g3", wantGroup: "g2", wantFound: true},
		{name: "scope exact beats scope general", scopeExact: "g3", scopeGeneral: "g4", wantGroup: "g3", wantFound: true},
		{name: "scope general fallback", scopeGeneral: "g4", wantGroup: "g4", wantFound: true},
		{name: "nothing configured", wantFound: false},
		{name: "orphan application binding degrades to scope chain", appExact: "g1", scopeGeneral: "g4", orphan: true, wantGroup: "g4", wantFound: true, wantOrphanWarning: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, store := seededService(t)
			bind := func(groupName, tag, applicationID, bindingScope string) Group {
				group := mustGroup(t, service, groupName)
				if _, err := service.SetBinding(context.Background(), "actor", Binding{ScopeID: bindingScope, SceneTag: tag, GroupID: group.ID, ApplicationID: applicationID}); err != nil {
					t.Fatalf("set binding: %v", err)
				}
				return group
			}
			want := Group{}
			for _, candidate := range []struct{ group, tag, app string }{
				{tc.appExact, "diagnosis", "app"},
				{tc.appGeneral, "general", "app"},
				{tc.scopeExact, "diagnosis", ""},
				{tc.scopeGeneral, "general", ""},
			} {
				if candidate.group == "" {
					continue
				}
				bound := bind(candidate.group, candidate.tag, candidate.app, "scope")
				switch candidate.group {
				case tc.appExact, tc.appGeneral, tc.scopeExact, tc.scopeGeneral:
				}
				if want.ID == "" {
					want = bound
				}
			}
			// The expected winner is the highest configured level; an orphan
			// application binding falls through to the scope chain.
			expected := tc.appExact
			if expected == "" || tc.orphan {
				expected = ""
			}
			if expected == "" {
				expected = tc.appGeneral
			}
			if expected == "" || tc.orphan {
				if expected == tc.appGeneral && tc.orphan {
					expected = ""
				}
			}
			if expected == "" {
				expected = tc.scopeExact
			}
			if expected == "" {
				expected = tc.scopeGeneral
			}
			want = Group{}
			for _, group := range store.groups {
				if group.Identifier == expected {
					want = group
				}
			}
			for _, group := range store.groups {
				group.Members = nil
				if _, err := service.ReplaceMembers(context.Background(), group.ID, []Member{
					{PersonaID: "manager-1", Seat: SeatLead},
					{PersonaID: "worker-1", Seat: SeatAssistant},
				}); err != nil {
					t.Fatalf("seed members for %s: %v", group.ID, err)
				}
			}
			applicationScope := "scope"
			if tc.orphan {
				applicationScope = "other-scope"
			}
			resolved, found, err := service.ResolveEffective(context.Background(), "scope", "app", applicationScope, "diagnosis")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v", found, tc.wantFound)
			}
			if !found {
				return
			}
			if resolved.GroupID != want.ID {
				t.Fatalf("resolved group = %s, want %s", resolved.GroupID, want.ID)
			}
			if resolved.Lead.PersonaID != "manager-1" || resolved.Lead.Role != "manager" || resolved.Lead.Version != 3 {
				t.Fatalf("lead snapshot = %+v", resolved.Lead)
			}
			if len(resolved.Assistants) != 1 || resolved.Assistants[0].Version != 2 {
				t.Fatalf("assistants = %+v", resolved.Assistants)
			}
			hasOrphan := false
			for _, warning := range resolved.Warnings {
				if warning.Kind == "orphan_application_binding" {
					hasOrphan = true
				}
			}
			if hasOrphan != tc.wantOrphanWarning {
				t.Fatalf("orphan warning = %v (warnings %+v), want %v", hasOrphan, resolved.Warnings, tc.wantOrphanWarning)
			}
		})
	}
}

func TestResolveEffectiveDisabledGroupDegrades(t *testing.T) {
	service, _ := seededService(t)
	disabled := mustGroup(t, service, "disabled")
	if _, err := service.ReplaceMembers(context.Background(), disabled.ID, []Member{{PersonaID: "manager-1", Seat: SeatLead}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateGroup(context.Background(), disabled.ID, GroupPatch{Status: strPtr("disabled")}); err != nil {
		t.Fatal(err)
	}
	active := mustGroup(t, service, "active")
	if _, err := service.ReplaceMembers(context.Background(), active.ID, []Member{{PersonaID: "manager-1", Seat: SeatLead}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetBinding(context.Background(), "actor", Binding{ScopeID: "scope", SceneTag: "inspection", GroupID: disabled.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetBinding(context.Background(), "actor", Binding{ScopeID: "scope", SceneTag: "general", GroupID: active.ID}); err != nil {
		t.Fatal(err)
	}
	// The disabled inspection binding is skipped and the general binding wins.
	resolved, found, err := service.ResolveEffective(context.Background(), "scope", "", "", "inspection")
	if err != nil || !found {
		t.Fatalf("resolve = %+v %v %v", resolved, found, err)
	}
	if resolved.GroupID != active.ID {
		t.Fatalf("resolved group = %s, want %s", resolved.GroupID, active.ID)
	}
	sawWarning := false
	for _, warning := range resolved.Warnings {
		if warning.Kind == "group_disabled" {
			sawWarning = true
		}
	}
	if !sawWarning {
		t.Fatalf("missing group_disabled warning: %+v", resolved.Warnings)
	}
}

func TestResolveGroupIgnoresLeadWorkerList(t *testing.T) {
	service, _ := seededService(t)
	group := mustGroup(t, service, "g")
	if _, err := service.ReplaceMembers(context.Background(), group.ID, []Member{
		{PersonaID: "manager-1", Seat: SeatLead},
		{PersonaID: "worker-1", Seat: SeatAssistant},
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveGroup(context.Background(), "scope", group.ID, "diagnosis")
	if err != nil {
		t.Fatalf("resolve group: %v", err)
	}
	// Q2: the lead's embedded worker list (worker-9) is ignored and recorded.
	if len(resolved.IgnoredWorkerPersonaIDs) != 1 || resolved.IgnoredWorkerPersonaIDs[0] != "worker-9" {
		t.Fatalf("ignored workers = %v", resolved.IgnoredWorkerPersonaIDs)
	}
	if len(resolved.Assistants) != 1 || resolved.Assistants[0].PersonaID != "worker-1" {
		t.Fatalf("assistants = %+v", resolved.Assistants)
	}
}

func TestReplaceMembersSeatValidation(t *testing.T) {
	cases := []struct {
		name    string
		members []Member
		wantErr string
	}{
		{name: "valid", members: []Member{{PersonaID: "manager-1", Seat: SeatLead}, {PersonaID: "worker-1", Seat: SeatAssistant}}},
		{name: "empty group rejected", members: nil, wantErr: "at least one member"},
		{name: "no lead rejected", members: []Member{{PersonaID: "worker-1", Seat: SeatAssistant}}, wantErr: "exactly one lead"},
		{name: "two leads rejected", members: []Member{{PersonaID: "manager-1", Seat: SeatLead}, {PersonaID: "worker-2", Seat: SeatLead, Priority: 1}}, wantErr: "exactly one lead"},
		{name: "lead must be manager", members: []Member{{PersonaID: "worker-1", Seat: SeatLead}}, wantErr: "lead seat requires a manager persona"},
		{name: "assistant must be worker", members: []Member{{PersonaID: "manager-1", Seat: SeatLead}, {PersonaID: "manager-2", Seat: SeatAssistant}}, wantErr: "assistant seat requires a worker persona"},
		{name: "oncall rejected", members: []Member{{PersonaID: "manager-1", Seat: SeatLead}, {PersonaID: "worker-1", Seat: SeatOncall}}, wantErr: "oncall seat is reserved"},
		{name: "invalid scene tag", members: []Member{{PersonaID: "manager-1", Seat: SeatLead, SceneTags: []string{"general"}}}, wantErr: "scene tags are invalid"},
		{name: "duplicate member", members: []Member{{PersonaID: "manager-1", Seat: SeatLead}, {PersonaID: "manager-1", Seat: SeatAssistant}}, wantErr: "members must be unique"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := seededService(t)
			group := mustGroup(t, service, "g")
			_, err := service.ReplaceMembers(context.Background(), group.ID, tc.members)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestResolveGroupFailsOnMissingLeadOrDeletedPersona(t *testing.T) {
	service, store := seededService(t)
	group := mustGroup(t, service, "g")
	// Lead member references a persona deleted after save: execution must fail
	// loudly, never partially degrade (R001 §3.4.7).
	if _, err := service.ReplaceMembers(context.Background(), group.ID, []Member{{PersonaID: "manager-1", Seat: SeatLead}}); err != nil {
		t.Fatal(err)
	}
	store.members[group.ID] = []Member{{PersonaID: "gone", Seat: SeatLead}}
	if _, err := service.ResolveGroup(context.Background(), "scope", group.ID, "diagnosis"); err == nil {
		t.Fatal("missing lead persona must fail resolution")
	}
	// Scene-filtered lead is also a hard failure (§2.2).
	store.members[group.ID] = []Member{{PersonaID: "manager-1", Seat: SeatLead, SceneTags: []string{"inspection"}}}
	if _, err := service.ResolveGroup(context.Background(), "scope", group.ID, "diagnosis"); err == nil || !errors.Is(err, err) {
		if err == nil {
			t.Fatal("lead filtered out for scene must fail resolution")
		}
	}
	// A disabled group is not resolvable directly.
	if _, err := service.UpdateGroup(context.Background(), group.ID, GroupPatch{Status: strPtr("disabled")}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveGroup(context.Background(), "scope", group.ID, "diagnosis"); err == nil {
		t.Fatal("disabled group must not resolve")
	}
}

func strPtr(value string) *string { return &value }

func TestResolveExpertGroupImplementsEngineResolver(t *testing.T) {
	service, _ := seededService(t)
	group := mustGroup(t, service, "g")
	if _, err := service.ReplaceMembers(context.Background(), group.ID, []Member{
		{PersonaID: "manager-1", Seat: SeatLead},
		{PersonaID: "worker-2", Seat: SeatAssistant},
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveExpertGroup(context.Background(), "scope", group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Lead.PersonaID != "manager-1" || len(resolved.Assistants) != 1 || resolved.Assistants[0].PersonaID != "worker-2" {
		t.Fatalf("resolution = %+v", resolved)
	}
}
