package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"opsseek/backend/authorization"
	"opsseek/backend/expertgroup"
)

type expertGroupsStub struct {
	groups    []expertgroup.Group
	members   []expertgroup.Member
	bindings  []expertgroup.Binding
	resolved  expertgroup.ResolvedGroup
	found     bool
	createErr error
	memberErr error
}

func (s *expertGroupsStub) ListGroups(context.Context, string) ([]expertgroup.Group, error) {
	return s.groups, nil
}
func (s *expertGroupsStub) GetGroup(_ context.Context, id string) (expertgroup.Group, error) {
	for _, group := range s.groups {
		if group.ID == id {
			return group, nil
		}
	}
	return expertgroup.Group{}, expertgroup.ErrNotFound
}
func (s *expertGroupsStub) CreateGroup(_ context.Context, input expertgroup.GroupInput) (expertgroup.Group, error) {
	if s.createErr != nil {
		return expertgroup.Group{}, s.createErr
	}
	return expertgroup.Group{ID: "group-1", ScopeID: input.ScopeID, Identifier: input.Identifier, Name: input.Name, Status: "active"}, nil
}
func (s *expertGroupsStub) UpdateGroup(_ context.Context, id string, patch expertgroup.GroupPatch) (expertgroup.Group, error) {
	return expertgroup.Group{ID: id, Name: *patch.Name, Status: *patch.Status}, nil
}
func (s *expertGroupsStub) ReplaceMembers(_ context.Context, _ string, members []expertgroup.Member) ([]expertgroup.Member, error) {
	if s.memberErr != nil {
		return nil, s.memberErr
	}
	if len(members) == 0 {
		return nil, &expertgroup.ValidationError{Message: "expert group requires at least one member"}
	}
	return members, nil
}
func (s *expertGroupsStub) ListBindings(context.Context, string) ([]expertgroup.Binding, error) {
	return s.bindings, nil
}
func (s *expertGroupsStub) SetBinding(_ context.Context, _ string, binding expertgroup.Binding) (expertgroup.Binding, error) {
	switch binding.SceneTag {
	case "general", "diagnosis", "inspection", "workflow":
	default:
		return expertgroup.Binding{}, &expertgroup.ValidationError{Message: "binding scope_id, scene_tag and group_id are required"}
	}
	return binding, nil
}
func (s *expertGroupsStub) RemoveBinding(context.Context, expertgroup.Binding) error { return nil }
func (s *expertGroupsStub) ResolveEffective(context.Context, string, string, string, string) (expertgroup.ResolvedGroup, bool, error) {
	return s.resolved, s.found, nil
}

func expertGroupRouter(service expertGroupService) http.Handler {
	router := chi.NewRouter()
	registerExpertGroupRoutes(router, service, nil, func(authorization.Permission) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	})
	return router
}

func TestExpertGroupRoutes(t *testing.T) {
	stub := &expertGroupsStub{
		groups:   []expertgroup.Group{{ID: "group-1", ScopeID: "scope-1", Identifier: "db-team", Name: "db-team", Status: "active", Members: []expertgroup.Member{{PersonaID: "p1", Seat: "lead"}}}},
		bindings: []expertgroup.Binding{{ScopeID: "scope-1", SceneTag: "diagnosis", GroupID: "group-1"}},
		resolved: expertgroup.ResolvedGroup{Snapshot: expertgroup.Snapshot{GroupID: "group-1", Scene: "diagnosis", Lead: expertgroup.MemberSnapshot{PersonaID: "manager-1", Role: "manager", Version: 2}}},
		found:    true,
	}
	router := expertGroupRouter(stub)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{name: "list groups", method: http.MethodGet, path: "/expert-groups?scope_id=scope-1", wantStatus: http.StatusOK, wantBody: "db-team"},
		{name: "get group", method: http.MethodGet, path: "/expert-groups/group-1", wantStatus: http.StatusOK, wantBody: "db-team"},
		{name: "missing group", method: http.MethodGet, path: "/expert-groups/missing", wantStatus: http.StatusNotFound},
		{name: "create group", method: http.MethodPost, path: "/expert-groups", body: `{"scope_id":"scope-1","identifier":"cache-team","name":"cache-team"}`, wantStatus: http.StatusCreated, wantBody: "group-1"},
		{name: "update group", method: http.MethodPatch, path: "/expert-groups/group-1", body: `{"name":"renamed","status":"disabled"}`, wantStatus: http.StatusOK, wantBody: "renamed"},
		{name: "list members", method: http.MethodGet, path: "/expert-groups/group-1/members", wantStatus: http.StatusOK, wantBody: "lead"},
		{name: "replace members", method: http.MethodPut, path: "/expert-groups/group-1/members", body: `{"members":[{"persona_id":"p1","seat":"lead"}]}`, wantStatus: http.StatusOK, wantBody: "p1"},
		{name: "empty members rejected", method: http.MethodPut, path: "/expert-groups/group-1/members", body: `{"members":[]}`, wantStatus: http.StatusBadRequest},
		{name: "list bindings", method: http.MethodGet, path: "/scopes/scope-1/expert-group-bindings", wantStatus: http.StatusOK, wantBody: "diagnosis"},
		{name: "set binding", method: http.MethodPut, path: "/scopes/scope-1/expert-group-bindings", body: `{"scene_tag":"inspection","group_id":"group-1"}`, wantStatus: http.StatusOK, wantBody: "group-1"},
		{name: "invalid binding tag", method: http.MethodPut, path: "/scopes/scope-1/expert-group-bindings", body: `{"scene_tag":"later","group_id":"group-1"}`, wantStatus: http.StatusBadRequest},
		{name: "remove binding", method: http.MethodDelete, path: "/scopes/scope-1/expert-group-bindings?scene_tag=diagnosis&group_id=group-1", wantStatus: http.StatusNoContent},
		{name: "effective group", method: http.MethodGet, path: "/applications/app-1/effective-expert-group?scope_id=scope-1&scene_tag=diagnosis", wantStatus: http.StatusOK, wantBody: "manager-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reader *strings.Reader
			if tc.body == "" {
				reader = strings.NewReader("")
			} else {
				reader = strings.NewReader(tc.body)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, reader))
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tc.wantStatus, response.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(response.Body.String(), tc.wantBody) {
				t.Fatalf("body = %s, want to contain %q", response.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestExpertGroupEffectiveNotFoundShape(t *testing.T) {
	stub := &expertGroupsStub{found: false}
	router := expertGroupRouter(stub)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/applications/app-1/effective-expert-group?scope_id=scope-1&scene_tag=diagnosis", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Found {
		t.Fatalf("found = true, want false")
	}
}

func TestExpertGroupSeatValidationSurfaces(t *testing.T) {
	stub := &expertGroupsStub{memberErr: &expertgroup.ValidationError{Message: "lead seat requires a manager persona"}}
	router := expertGroupRouter(stub)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/expert-groups/group-1/members", strings.NewReader(`{"members":[{"persona_id":"p1","seat":"lead"}]}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestExpertGroupConflictSurfaces(t *testing.T) {
	stub := &expertGroupsStub{createErr: expertgroup.ErrConflict}
	router := expertGroupRouter(stub)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/expert-groups", strings.NewReader(`{"scope_id":"scope-1","identifier":"db-team","name":"db-team"}`)))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d", response.Code)
	}
	_ = errors.New
	_ = authorization.ExpertGroupRead
}
