package diagnosis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	appdomain "opskeeper/backend/application"
	"opskeeper/backend/authorization"
)

type scopeToolApplications struct{}

func (scopeToolApplications) Get(context.Context, string) (appdomain.Application, error) {
	return appdomain.Application{}, nil
}
func (scopeToolApplications) ContextResourceIDs(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (scopeToolApplications) DiagnosticContext(context.Context, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{}, nil
}
func (scopeToolApplications) DiagnosticScopeTeams(context.Context, string) ([]appdomain.DiagnosticScopeTeam, error) {
	return []appdomain.DiagnosticScopeTeam{{Name: "Payments", Status: "active", ProjectCount: 2}}, nil
}
func (scopeToolApplications) DiagnosticScopeContext(context.Context, string) (appdomain.DiagnosticScopeContext, error) {
	return appdomain.DiagnosticScopeContext{ScopeType: "project", TeamName: "Commerce Team", ProjectName: "Commerce", ProjectCode: "commerce"}, nil
}
func (scopeToolApplications) DiagnosticScopeProjects(context.Context, string, string) ([]appdomain.DiagnosticScopeProject, error) {
	return []appdomain.DiagnosticScopeProject{{Name: "Checkout", Code: "checkout", Status: "active", ApplicationCount: 1}}, nil
}
func (scopeToolApplications) DiagnosticScopeApplications(context.Context, string, string, string, string, int, int) (appdomain.DiagnosticScopeApplicationPage, error) {
	return appdomain.DiagnosticScopeApplicationPage{Items: []appdomain.DiagnosticScopeApplication{{Name: "Orders", Code: "orders", RuntimeKind: appdomain.RuntimeDocker}}, Total: 1, Limit: 100}, nil
}
func (scopeToolApplications) DiagnosticApplicationContextByLocator(context.Context, string, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{Application: appdomain.DiagnosticApplication{ID: "internal-app-id", Name: "Orders"}, Resources: []appdomain.DiagnosticResource{{ID: "internal-resource-id", Name: "Postgres", Kind: "postgresql"}}}, nil
}

type missingScopeToolApplications struct{ scopeToolApplications }

func (missingScopeToolApplications) DiagnosticApplicationContextByLocator(context.Context, string, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{}, appdomain.ErrNotFound
}

type recordingScopeApplications struct {
	scopeToolApplications
	teamName    string
	projectName string
}

func (s *recordingScopeApplications) DiagnosticScopeApplications(_ context.Context, _, teamName, projectName, _ string, _, _ int) (appdomain.DiagnosticScopeApplicationPage, error) {
	s.teamName = teamName
	s.projectName = projectName
	return appdomain.DiagnosticScopeApplicationPage{Limit: 100}, nil
}

func TestDiagnosticScopeToolsAreBoundedAndStripInternalIDs(t *testing.T) {
	service := &Service{applications: scopeToolApplications{}}
	tools := service.diagnosticScopeTools(context.Background(), "scope-1")
	if len(tools) != 4 {
		t.Fatalf("tool count = %d", len(tools))
	}
	applicationsSchema := string(tools[2].Definition().InputSchema)
	if !strings.Contains(applicationsSchema, `"project_name"`) || !strings.Contains(applicationsSchema, `绝不能填写项目名`) {
		t.Fatalf("application scope tool does not distinguish project and team names: %s", applicationsSchema)
	}
	applicationsDescription := tools[2].Definition().Description
	if !strings.Contains(applicationsDescription, "当前团队") || !strings.Contains(applicationsDescription, "diagnosis_application_context") || !strings.Contains(tools[3].Definition().Description, "绝不能用于列出团队/项目") {
		t.Fatalf("application-list and application-detail tools are not clearly distinguished: %q / %q", applicationsDescription, tools[3].Definition().Description)
	}
	for _, item := range tools {
		definition := item.Definition()
		if definition.Source != scopeToolSource || !definition.ReadOnly {
			t.Fatalf("definition = %#v", definition)
		}
	}
	result, err := tools[3].Invoke(context.Background(), map[string]any{"application_code": "orders"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "internal-app-id") || strings.Contains(string(encoded), "internal-resource-id") {
		t.Fatalf("internal IDs leaked from scope tool: %v", result.Output)
	}
}

func TestDiagnosticScopeApplicationToolPassesTeamAndProjectNamesSeparately(t *testing.T) {
	applications := &recordingScopeApplications{}
	tools := (&Service{applications: applications}).diagnosticScopeTools(context.Background(), "scope-1")
	tool := tools[2]
	if _, err := tool.Invoke(context.Background(), map[string]any{"team_name": "Commerce Team"}); err != nil {
		t.Fatal(err)
	}
	if applications.teamName != "Commerce Team" || applications.projectName != "" {
		t.Fatalf("team query filters = team %q, project %q", applications.teamName, applications.projectName)
	}
	if _, err := tool.Invoke(context.Background(), map[string]any{"project_name": "Commerce"}); err != nil {
		t.Fatal(err)
	}
	if applications.teamName != "" || applications.projectName != "Commerce" {
		t.Fatalf("project query filters = team %q, project %q", applications.teamName, applications.projectName)
	}
}

func TestDiagnosticApplicationContextMissDoesNotTerminateObjectResolution(t *testing.T) {
	service := &Service{applications: missingScopeToolApplications{}}
	tools := service.diagnosticScopeTools(context.Background(), "scope-1")
	definition := tools[3].Definition()
	if !strings.Contains(definition.Description, "selected_resources") || !strings.Contains(string(definition.InputSchema), "application_name") {
		t.Fatalf("application lookup definition is ambiguous: %#v", definition)
	}
	result, err := tools[3].Invoke(context.Background(), map[string]any{"application_name": "git"})
	if err != nil {
		t.Fatalf("not-found application should be a recoverable result: %v", err)
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"found":false`) || !strings.Contains(string(encoded), "resource-specific tools") {
		t.Fatalf("application miss did not direct the model to continue object resolution: %s", encoded)
	}
}

func TestDiagnosticScopeApplicationPageSerializationStaysBounded(t *testing.T) {
	page := appdomain.DiagnosticScopeApplicationPage{Items: make([]appdomain.DiagnosticScopeApplication, 0, 100), Total: 50000, Limit: 100}
	for index := 0; index < 100; index++ {
		page.Items = append(page.Items, appdomain.DiagnosticScopeApplication{
			TeamName: fmt.Sprintf("team-%d", index%50), ProjectName: fmt.Sprintf("project-%d", index), ProjectCode: fmt.Sprintf("project-%d", index),
			Name: fmt.Sprintf("application-%d", index), Code: fmt.Sprintf("app-%d", index), RuntimeKind: appdomain.RuntimeDocker, Status: "active", InstanceCount: 2,
		})
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 50000 {
		t.Fatalf("one bounded page is too large: %d bytes", len(encoded))
	}
}

func TestWithAuthorizationContextPreservesServerFilters(t *testing.T) {
	source := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}})
	source = authorization.WithResourceFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-1"}})
	source = authorization.WithResourceUseFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-2"}})
	source = authorization.WithResourceReadFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-3"}})
	got := withAuthorizationContext(context.Background(), source)
	if filter, ok := authorization.ScopeFilterFromContext(got); !ok || !filter.Allows("scope-1") {
		t.Fatalf("scope filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceFilterFromContext(got); !ok || !filter.Allows("", "resource-1") {
		t.Fatalf("resource filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceUseFilterFromContext(got); !ok || !filter.Allows("", "resource-2") {
		t.Fatalf("resource-use filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceReadFilterFromContext(got); !ok || !filter.Allows("", "resource-3") {
		t.Fatalf("resource-read filter was not preserved: %#v, %v", filter, ok)
	}
}
