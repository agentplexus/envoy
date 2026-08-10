package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/agents"
	"github.com/plexusone/omniagent/team/ent"
)

// AgentsHTTP serves the virtual-agents management + discovery surface
// (INIT-OMNIAGENT-005 Phase 5): the owner/maintainer configuration area
// (RMI-311: create/update/delete, enabled skills, maintainers, visibility),
// the discovery catalog and agent-bound chat starts (RMI-312, chat starts live
// on TeamChatHTTP), and superadmin featured curation (RMI-313).
//
// It is a thin HTTP adapter over agents.Service — every route is scoped to the
// authenticated principal (set on the context by TeamHTTP.RequireAuth, which
// must wrap this handler) and every authorization decision is the service's
// (Can / requireEditor / requireOwner); PostgreSQL row-level security is the
// defense-in-depth backstop. Secret management is intentionally out of scope
// here (an INIT-004 concern), so no secret values cross this surface.
type AgentsHTTP struct {
	agents *agents.Service
	logger *slog.Logger
	mux    *http.ServeMux
}

// AgentsHTTPConfig configures the agents handler.
type AgentsHTTPConfig struct {
	Agents *agents.Service
	Logger *slog.Logger
}

// NewAgentsHTTP builds the agents handler set.
func NewAgentsHTTP(cfg AgentsHTTPConfig) *AgentsHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	h := &AgentsHTTP{agents: cfg.Agents, logger: cfg.Logger}
	h.routes()
	return h
}

// Handler returns the routed handler. Mount it at "/api/agents",
// "/api/agents/" and "/api/catalog", wrapped in TeamHTTP.RequireAuth so every
// route has an authenticated principal on its context.
func (h *AgentsHTTP) Handler() http.Handler { return h.mux }

func (h *AgentsHTTP) routes() {
	h.mux = http.NewServeMux()
	// Discovery (RMI-312): the catalog the caller may see.
	h.mux.HandleFunc("GET /api/catalog", h.handleCatalog)
	// Owner/maintainer area (RMI-311).
	h.mux.HandleFunc("GET /api/agents", h.handleList)
	h.mux.HandleFunc("POST /api/agents", h.handleCreate)
	h.mux.HandleFunc("GET /api/agents/{id}", h.handleGet)
	h.mux.HandleFunc("PATCH /api/agents/{id}", h.handleUpdate)
	h.mux.HandleFunc("DELETE /api/agents/{id}", h.handleDelete)
	h.mux.HandleFunc("PUT /api/agents/{id}/skills", h.handleSetSkills)
	h.mux.HandleFunc("GET /api/agents/{id}/roles", h.handleRoles)
	h.mux.HandleFunc("POST /api/agents/{id}/maintainers", h.handleAddMaintainer)
	h.mux.HandleFunc("DELETE /api/agents/{id}/maintainers/{userId}", h.handleRemoveMaintainer)
	h.mux.HandleFunc("POST /api/agents/{id}/leave", h.handleLeave)
	h.mux.HandleFunc("PUT /api/agents/{id}/visibility", h.handleSetVisibility)
	// Superadmin curation (RMI-313).
	h.mux.HandleFunc("PUT /api/agents/{id}/featured", h.handleSetFeatured)
}

// ---- Views ---------------------------------------------------------------

type agentView struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Persona     string    `json:"persona"`
	Model       string    `json:"model"`
	Provider    string    `json:"provider"`
	Visibility  string    `json:"visibility"`
	Featured    bool      `json:"featured"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toAgentView(a *ent.Agent) agentView {
	return agentView{
		ID:          a.ID,
		Slug:        a.Slug,
		Name:        a.Name,
		Description: a.Description,
		Persona:     a.Persona,
		Model:       a.Model,
		Provider:    a.Provider,
		Visibility:  a.Visibility.String(),
		Featured:    a.Featured,
		CreatedAt:   a.CreatedAt,
	}
}

// capsView is the caller's per-agent capability set, so the SPA can show only
// the controls the actor may use (the service re-checks every mutation).
type capsView struct {
	Configure         bool `json:"configure"`
	ManageMaintainers bool `json:"manageMaintainers"`
	ManageRegistry    bool `json:"manageRegistry"`
	Administer        bool `json:"administer"`
}

// agentDetail is a single agent plus what the edit view needs: its enabled
// skills, the deployment's available-skills catalog to choose from, and the
// caller's capabilities on it.
type agentDetail struct {
	agentView
	EnabledSkills   []string `json:"enabledSkills"`
	AvailableSkills []string `json:"availableSkills"`
	Caps            capsView `json:"caps"`
}

// ---- Discovery (RMI-312) -------------------------------------------------

func (h *AgentsHTTP) handleCatalog(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	cat, err := h.agents.Catalog(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// ---- Owner/maintainer area (RMI-311) -------------------------------------

func (h *AgentsHTTP) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	as, err := h.agents.ListMyAgents(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	out := make([]agentView, len(as))
	for i, a := range as {
		out[i] = toAgentView(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": out})
}

type createAgentRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Persona     string `json:"persona"`
	Model       string `json:"model"`
	Provider    string `json:"provider"`
}

func (h *AgentsHTTP) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	var req createAgentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, err := h.agents.CreateAgent(r.Context(), actor, agents.CreateSpec{
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Persona:     req.Persona,
		Model:       req.Model,
		Provider:    req.Provider,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAgentView(a))
}

func (h *AgentsHTTP) handleGet(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	a, err := h.agents.GetAgent(ctx, actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	skills, err := h.agents.AgentSkills(ctx, actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	caps, err := h.caps(ctx, actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agentDetail{
		agentView:       toAgentView(a),
		EnabledSkills:   skills,
		AvailableSkills: h.agents.AvailableSkills(),
		Caps:            caps,
	})
}

type updateAgentRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Persona     *string `json:"persona"`
	Model       *string `json:"model"`
	Provider    *string `json:"provider"`
}

func (h *AgentsHTTP) handleUpdate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	var req updateAgentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, err := h.agents.UpdateAgent(r.Context(), actor, id, agents.UpdateSpec{
		Name:        req.Name,
		Description: req.Description,
		Persona:     req.Persona,
		Model:       req.Model,
		Provider:    req.Provider,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAgentView(a))
}

func (h *AgentsHTTP) handleDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	if err := h.agents.DeleteAgent(r.Context(), actor, id); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type setSkillsRequest struct {
	Skills []string `json:"skills"`
}

func (h *AgentsHTTP) handleSetSkills(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	var req setSkillsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := h.agents.SetAgentSkills(r.Context(), actor, id, req.Skills); err != nil {
		h.writeServiceError(w, err)
		return
	}
	// Echo the persisted (canonical, sorted) set back.
	skills, err := h.agents.AgentSkills(r.Context(), actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (h *AgentsHTTP) handleRoles(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	roles, err := h.agents.Roles(r.Context(), actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

type addMaintainerRequest struct {
	Username string `json:"username"`
}

func (h *AgentsHTTP) handleAddMaintainer(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	var req addMaintainerRequest
	if !decodeBody(w, r, &req) {
		return
	}
	role, err := h.agents.AddMaintainer(r.Context(), actor, id, req.Username)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "userId": role.UserID, "role": role.Role.String()})
}

func (h *AgentsHTTP) handleRemoveMaintainer(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.agents.RemoveMaintainer(r.Context(), actor, id, userID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *AgentsHTTP) handleLeave(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	if err := h.agents.LeaveAgent(r.Context(), actor, id); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type setVisibilityRequest struct {
	Visibility string `json:"visibility"`
}

func (h *AgentsHTTP) handleSetVisibility(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	var req setVisibilityRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, err := h.agents.SetVisibility(r.Context(), actor, id, req.Visibility)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAgentView(a))
}

// ---- Superadmin curation (RMI-313) ---------------------------------------

type setFeaturedRequest struct {
	Featured bool `json:"featured"`
}

func (h *AgentsHTTP) handleSetFeatured(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, ok := h.agentID(w, r)
	if !ok {
		return
	}
	var req setFeaturedRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, err := h.agents.SetFeatured(r.Context(), actor, id, req.Featured)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAgentView(a))
}

// ---- Internals -----------------------------------------------------------

// caps resolves the caller's per-agent capabilities via the service matrix, so
// the SPA renders only the controls the actor may use.
func (h *AgentsHTTP) caps(ctx context.Context, actor agents.Actor, id uuid.UUID) (capsView, error) {
	var out capsView
	var err error
	if out.Configure, err = h.agents.Can(ctx, actor, id, agents.CapConfigure); err != nil {
		return out, err
	}
	if out.ManageMaintainers, err = h.agents.Can(ctx, actor, id, agents.CapManageMaintainers); err != nil {
		return out, err
	}
	if out.ManageRegistry, err = h.agents.Can(ctx, actor, id, agents.CapManageRegistry); err != nil {
		return out, err
	}
	if out.Administer, err = h.agents.Can(ctx, actor, id, agents.CapAdminister); err != nil {
		return out, err
	}
	return out, nil
}

// actor resolves the authenticated principal (set by RequireAuth) to an agents
// Actor; responds 401 if absent.
func (h *AgentsHTTP) actor(w http.ResponseWriter, r *http.Request) (agents.Actor, bool) {
	p := principalFrom(r.Context())
	if p == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return agents.Actor{}, false
	}
	return agents.Actor{UserID: p.UserID, Superadmin: p.Superadmin}, true
}

// actorCSRF is actor plus the custom-header CSRF check required on mutations.
func (h *AgentsHTTP) actorCSRF(w http.ResponseWriter, r *http.Request) (agents.Actor, bool) {
	if !hasCSRF(r) {
		writeError(w, http.StatusForbidden, "missing CSRF header")
		return agents.Actor{}, false
	}
	return h.actor(w, r)
}

func (h *AgentsHTTP) agentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *AgentsHTTP) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, agents.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, agents.ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, agents.ErrEmptyName):
		writeError(w, http.StatusBadRequest, "agent name is required")
	case errors.Is(err, agents.ErrSlugTaken):
		writeError(w, http.StatusConflict, "agent slug is already taken")
	case errors.Is(err, agents.ErrUnknownSkill):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, agents.ErrBlockedSkill):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, agents.ErrInvalidVisibility):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, agents.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, agents.ErrLastOwner):
		writeError(w, http.StatusConflict, "you are the agent's only owner")
	case errors.Is(err, agents.ErrCannotRemoveOwner):
		writeError(w, http.StatusBadRequest, "cannot remove an owner")
	default:
		h.logger.Error("agents request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// decodeBody decodes a JSON request body (bounded to hold the largest free-text
// field, the persona), writing a 400 and returning false on failure.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, agents.MaxPersonaBytes+4<<10)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
