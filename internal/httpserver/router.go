package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"agentbackend/internal/game"
	"agentbackend/internal/models"
	"agentbackend/internal/storage"
)

type Server struct {
	store  storage.Storage
	engine *game.Engine
}

func NewRouter(store storage.Storage) http.Handler {
	s := &Server{
		store:  store,
		engine: game.NewEngine(store),
	}

	r := chi.NewRouter()
	r.Use(devCORSMiddleware)

	r.Route("/users", func(r chi.Router) {
		r.Post("/", s.handleCreateUser)
		r.Get("/", s.handleListUsers)
		r.Get("/{id}", s.handleGetUser)
		r.Put("/{id}", s.handleUpdateUser)
		r.Delete("/{id}", s.handleDeleteUser)
	})

	r.Route("/games", func(r chi.Router) {
		r.Post("/", s.handleCreateGame)
		r.Get("/", s.handleListGames)
		r.Get("/{id}", s.handleGetGame)
		r.Get("/{id}/state", s.handleGetGameState)
		r.Post("/{id}/actions", s.handleGameAction)
		r.Put("/{id}", s.handleUpdateGame)
		r.Delete("/{id}", s.handleDeleteGame)
	})

	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openAPISpecYAML)
	})
	r.Mount("/", swaggerUI())

	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func parseNullableInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	return v
}

type errorResponse struct {
	Error string `json:"error"`
}

type statusResponse struct {
	Status string `json:"status"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Name string `json:"name"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "name is required"})
		return
	}

	user := &models.User{
		Name: in.Name,
	}

	if err := s.store.CreateUser(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid user id"})
		return
	}

	user, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid user id"})
		return
	}

	type req struct {
		Name string `json:"name"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "name is required"})
		return
	}

	user := &models.User{
		ID:   id,
		Name: in.Name,
	}

	if err := s.store.UpdateUser(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid user id"})
		return
	}

	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "deleted"})
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Title      string `json:"title"`
		HostUserID int64  `json:"host_user_id"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if in.Title == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "title is required"})
		return
	}
	if in.HostUserID <= 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "host_user_id is required"})
		return
	}

	createdGame, state, _, err := s.engine.CreateGame(r.Context(), in.Title, in.HostUserID)
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"game": createdGame, "state": state})
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.store.ListGames(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"games": games})
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}

	game, err := s.store.GetGameByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"game": game})
}

func (s *Server) handleUpdateGame(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}

	type req struct {
		Title string `json:"title"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if in.Title == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "title is required"})
		return
	}

	game := &models.Game{
		ID:    id,
		Title: in.Title,
	}

	if err := s.store.UpdateGame(r.Context(), game); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"game": game})
}

func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}

	if err := s.store.DeleteGame(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "deleted"})
}

func (s *Server) handleGameAction(w http.ResponseWriter, r *http.Request) {
	type req struct {
		UserID  int64           `json:"user_id"`
		Type    game.ActionType `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	gameID, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}
	if in.UserID <= 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "user_id is required"})
		return
	}
	if in.Type == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "type is required"})
		return
	}

	state, events, err := s.engine.HandleAction(r.Context(), gameID, game.Action{
		UserID:  in.UserID,
		Type:    in.Type,
		Payload: in.Payload,
	})
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	deleted := false
	if in.Type == game.ActionLeaveGame && state != nil && len(state.Players) == 0 {
		if err := s.store.DeleteGame(r.Context(), gameID); err != nil {
			writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
			return
		}
		deleted = true
	}

	writeJSON(w, http.StatusOK, map[string]any{"events": events, "state": state, "game_deleted": deleted})
}

func (s *Server) handleGetGameState(w http.ResponseWriter, r *http.Request) {
	gameID, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}
	viewerID, err := strconv.ParseInt(r.URL.Query().Get("viewer_user_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid viewer_user_id"})
		return
	}

	gameModel, err := s.store.GetGameByID(r.Context(), gameID)
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	events, err := s.store.ListEventsByGameID(r.Context(), gameID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	state, err := game.BuildState(gameID, gameModel.Title, events)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	publicState, err := game.ProjectStateForViewer(state, viewerID)
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"state": publicState})
}

func statusFromError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, strconv.ErrSyntax):
		return http.StatusBadRequest
	case containsText(err.Error(), "not found"):
		return http.StatusNotFound
	case containsText(err.Error(), "required"),
		containsText(err.Error(), "invalid"),
		containsText(err.Error(), "must "),
		containsText(err.Error(), "unsupported"),
		containsText(err.Error(), "only "),
		containsText(err.Error(), "cannot "),
		containsText(err.Error(), "already "),
		containsText(err.Error(), "not in lobby"),
		containsText(err.Error(), "viewer is not an active player"),
		containsText(err.Error(), "game requires"),
		containsText(err.Error(), "game is full"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func containsText(s, sub string) bool {
	return strings.Contains(s, sub)
}

func devCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
