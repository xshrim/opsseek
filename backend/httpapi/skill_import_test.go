package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"opsseek/backend/skill"
)

type importSkillsStub struct {
	calls   []skill.ImportInput
	results []skill.ImportItemResult
	err     error
}

func (s *importSkillsStub) ListSkills(context.Context, string) ([]skill.Skill, error) {
	return nil, skill.ErrNotFound
}
func (s *importSkillsStub) CreateVersion(context.Context, string, skill.CreateVersionInput) (skill.Version, error) {
	return skill.Version{}, nil
}
func (s *importSkillsStub) ListVersions(context.Context, string) ([]skill.Version, error) {
	return nil, nil
}
func (s *importSkillsStub) Publish(context.Context, string, string) (skill.Version, error) {
	return skill.Version{}, nil
}
func (s *importSkillsStub) Disable(context.Context, string, string) (skill.Version, error) {
	return skill.Version{}, nil
}
func (s *importSkillsStub) SetDefault(context.Context, string, string, string, string) (skill.Default, error) {
	return skill.Default{}, nil
}
func (s *importSkillsStub) Resolve(context.Context, string, string, string) (skill.Version, error) {
	return skill.Version{}, nil
}
func (s *importSkillsStub) Import(_ context.Context, input skill.ImportInput) ([]skill.ImportItemResult, error) {
	s.calls = append(s.calls, input)
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}

func TestImportSkillsHandlerDecodesBase64AndReturnsItems(t *testing.T) {
	stub := &importSkillsStub{results: []skill.ImportItemResult{{Status: "imported", Identifier: "pg-wal-check"}}}
	handler := aiHandler{skills: stub}
	document := "---\napi_version: opsseek.skill/v1\nkind: Skill\n---\nbody\n"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/skills/import", strings.NewReader(`{"scope_id":"2817fc61-c924-4760-84a7-06a0d3f8995f","format":"markdown","content_base64":"`+base64.StdEncoding.EncodeToString([]byte(document))+`"}`))
	response := httptest.NewRecorder()

	handler.importSkills(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"identifier":"pg-wal-check"`) {
		t.Fatalf("body = %s", response.Body.String())
	}
	if len(stub.calls) != 1 || stub.calls[0].Format != "markdown" {
		t.Fatalf("service calls = %+v", stub.calls)
	}
}

func TestImportSkillsHandlerRejectsInvalidBase64(t *testing.T) {
	handler := aiHandler{skills: &importSkillsStub{}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/skills/import", strings.NewReader(`{"scope_id":"s","format":"markdown","content_base64":"@@@"}`))
	response := httptest.NewRecorder()

	handler.importSkills(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestImportSkillsHandlerMapsValidationAndPackageErrors(t *testing.T) {
	handler := aiHandler{skills: &importSkillsStub{err: errors.New("boom")}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/skills/import", strings.NewReader(`{"scope_id":"s","format":"zip","content_base64":"UEsDBA=="}`))
	response := httptest.NewRecorder()

	handler.importSkills(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}

	validation := &skill.ValidationError{Message: "zip package exceeds 200 entries"}
	handler = aiHandler{skills: &importSkillsStub{err: validation}}
	response = httptest.NewRecorder()
	handler.importSkills(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d", response.Code)
	}
}
