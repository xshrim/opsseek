package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"opsseek/backend/application"
	"opsseek/backend/authorization"
	"opsseek/backend/expertgroup"
)

type expertGroupService interface {
	ListGroups(context.Context, string) ([]expertgroup.Group, error)
	GetGroup(context.Context, string) (expertgroup.Group, error)
	CreateGroup(context.Context, expertgroup.GroupInput) (expertgroup.Group, error)
	UpdateGroup(context.Context, string, expertgroup.GroupPatch) (expertgroup.Group, error)
	ReplaceMembers(context.Context, string, []expertgroup.Member) ([]expertgroup.Member, error)
	ListBindings(context.Context, string) ([]expertgroup.Binding, error)
	SetBinding(context.Context, string, expertgroup.Binding) (expertgroup.Binding, error)
	RemoveBinding(context.Context, expertgroup.Binding) error
	ResolveEffective(context.Context, string, string, string, string) (expertgroup.ResolvedGroup, bool, error)
}

// expertGroupApplicationReader reads an application's owning project scope for
// the effective-expert-group preview. application.Service satisfies it.
type expertGroupApplicationReader interface {
	Get(context.Context, string) (application.Application, error)
}

type createExpertGroupRequest struct {
	ScopeID     string `json:"scope_id"`
	Identifier  string `json:"identifier"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type updateExpertGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type setExpertGroupBindingRequest struct {
	ScopeID       string `json:"scope_id"`
	SceneTag      string `json:"scene_tag"`
	GroupID       string `json:"group_id"`
	ApplicationID string `json:"application_id"`
}

type expertGroupHandler struct {
	service       expertGroupService
	applications  expertGroupApplicationReader
	authorization authorizationService
}

func registerExpertGroupRoutes(router chi.Router, service expertGroupService, applications any, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := expertGroupHandler{service: service}
	if reader, ok := applications.(expertGroupApplicationReader); ok {
		h.applications = reader
	}
	router.With(guard(authorization.ExpertGroupRead)).Get("/expert-groups", h.list)
	router.With(guard(authorization.ExpertGroupManage)).Post("/expert-groups", h.create)
	router.With(guard(authorization.ExpertGroupRead)).Get("/expert-groups/{groupID}", h.get)
	router.With(guard(authorization.ExpertGroupManage)).Patch("/expert-groups/{groupID}", h.update)
	router.With(guard(authorization.ExpertGroupRead)).Get("/expert-groups/{groupID}/members", h.listMembers)
	router.With(guard(authorization.ExpertGroupManage)).Put("/expert-groups/{groupID}/members", h.replaceMembers)
	router.With(guard(authorization.ExpertGroupRead)).Get("/scopes/{scopeID}/expert-group-bindings", h.listBindings)
	router.With(guard(authorization.ExpertGroupManage)).Put("/scopes/{scopeID}/expert-group-bindings", h.setBinding)
	router.With(guard(authorization.ExpertGroupManage)).Delete("/scopes/{scopeID}/expert-group-bindings", h.removeBinding)
	router.With(guard(authorization.ExpertGroupRead)).Get("/applications/{applicationID}/effective-expert-group", h.effective)
}

func (h expertGroupHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListGroups(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h expertGroupHandler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetGroup(r.Context(), chi.URLParam(r, "groupID"))
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h expertGroupHandler) create(w http.ResponseWriter, r *http.Request) {
	var body createExpertGroupRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.CreateGroup(r.Context(), expertgroup.GroupInput{
		ScopeID: body.ScopeID, Identifier: body.Identifier, Name: body.Name,
		Description: body.Description, Status: body.Status,
	})
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h expertGroupHandler) update(w http.ResponseWriter, r *http.Request) {
	var body updateExpertGroupRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.UpdateGroup(r.Context(), chi.URLParam(r, "groupID"), expertgroup.GroupPatch{
		Name: body.Name, Description: body.Description, Status: body.Status,
	})
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h expertGroupHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetGroup(r.Context(), chi.URLParam(r, "groupID"))
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	if item.Members == nil {
		item.Members = []expertgroup.Member{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": item.Members})
}

func (h expertGroupHandler) replaceMembers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Members []expertgroup.Member `json:"members"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Members == nil {
		body.Members = []expertgroup.Member{}
	}
	items, err := h.service.ReplaceMembers(r.Context(), chi.URLParam(r, "groupID"), body.Members)
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h expertGroupHandler) listBindings(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListBindings(r.Context(), chi.URLParam(r, "scopeID"))
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	if items == nil {
		items = []expertgroup.Binding{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h expertGroupHandler) setBinding(w http.ResponseWriter, r *http.Request) {
	var body setExpertGroupBindingRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	scopeID := chi.URLParam(r, "scopeID")
	if body.ScopeID == "" {
		body.ScopeID = scopeID
	}
	item, err := h.service.SetBinding(r.Context(), currentUser(r).ID, expertgroup.Binding{
		ScopeID: body.ScopeID, SceneTag: body.SceneTag, GroupID: body.GroupID, ApplicationID: body.ApplicationID,
	})
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h expertGroupHandler) removeBinding(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	err := h.service.RemoveBinding(r.Context(), expertgroup.Binding{
		ScopeID: chi.URLParam(r, "scopeID"), SceneTag: query.Get("scene_tag"),
		GroupID: query.Get("group_id"), ApplicationID: query.Get("application_id"),
	})
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h expertGroupHandler) effective(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationID")
	applicationScopeID := ""
	if applicationID != "" && h.applications != nil {
		app, err := h.applications.Get(r.Context(), applicationID)
		if err != nil {
			writeExpertGroupError(w, r, err)
			return
		}
		applicationScopeID = app.ProjectScopeID
	}
	resolved, found, err := h.service.ResolveEffective(r.Context(), r.URL.Query().Get("scope_id"), applicationID, applicationScopeID, r.URL.Query().Get("scene_tag"))
	if err != nil {
		writeExpertGroupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"found": found, "group": resolved})
}

func writeExpertGroupError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *expertgroup.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, r, http.StatusBadRequest, "invalid_request", validation.Message)
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, expertgroup.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Expert group was not found")
	case errors.Is(err, expertgroup.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", "Expert group conflicts with existing data")
	default:
		writeError(w, r, http.StatusBadRequest, "expert_group_error", err.Error())
	}
}
