package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	authpkg "agentbackend/internal/auth"
	"agentbackend/internal/game"
	"agentbackend/internal/models"
	"agentbackend/internal/storage"
)

type Server struct {
	store     storage.Storage
	engine    *game.Engine
	jwtSecret string
	tokenTTL  time.Duration
}

func NewRouter(store storage.Storage, jwtSecret ...string) http.Handler {
	secret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) > 0 && jwtSecret[0] != "" {
		secret = jwtSecret[0]
	}
	if secret == "" {
		secret = "dev-test-secret"
	}

	s := &Server{
		store:     store,
		engine:    game.NewEngine(store),
		jwtSecret: secret,
		tokenTTL:  7 * 24 * time.Hour,
	}

	r := chi.NewRouter()
	r.Use(devCORSMiddleware)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/me", s.handleAuthMe)
			r.Put("/password", s.handleChangePassword)
		})
	})

	r.Get("/leaderboard", s.authMiddleware(http.HandlerFunc(s.handleLeaderboard)).ServeHTTP)

	r.Route("/users", func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/me/profile", s.handleMyProfile)
		r.Put("/me/profile", s.handleUpdateMyProfile)
		r.Get("/{id}/profile", s.handleUserProfile)
		r.Post("/", s.handleCreateUser)
		r.Get("/", s.handleListUsers)
		r.Get("/{id}", s.handleGetUser)
		r.Put("/{id}", s.handleUpdateUser)
		r.Delete("/{id}", s.handleDeleteUser)
	})

	r.Route("/games", func(r chi.Router) {
		r.Use(s.authMiddleware)
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

type contextKey string

const currentUserContextKey contextKey = "current_user"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := authpkg.BearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		claims, err := authpkg.ParseToken(s.jwtSecret, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
			return
		}

		user, err := s.store.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
			return
		}

		_ = s.store.TouchUserLastSeen(r.Context(), user.ID, 5*time.Minute)

		ctx := context.WithValue(r.Context(), currentUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) *models.User {
	user, _ := r.Context().Value(currentUserContextKey).(*models.User)
	return user
}

func currentUserID(r *http.Request) int64 {
	if user := currentUser(r); user != nil {
		return user.ID
	}
	return 0
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

	createdGame, state, _, err := s.engine.CreateGame(r.Context(), in.Title, currentUserID(r))
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}
	s.decoratePublicState(r.Context(), state)

	writeJSON(w, http.StatusCreated, map[string]any{"game": createdGame, "state": state})
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.store.ListGames(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	items := make([]gameListItem, 0, len(games))
	for _, gameModel := range games {
		item := gameListItem{
			ID:        gameModel.ID,
			Title:     gameModel.Title,
			CreatedAt: gameModel.CreatedAt,
		}

		events, err := s.store.ListEventsByGameID(r.Context(), gameModel.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}

		state, err := game.BuildState(gameModel.ID, gameModel.Title, events)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}

		item.Title = state.Title
		item.Status = state.Status
		item.CurrentRound = state.CurrentRound
		for _, userID := range state.PlayerOrder {
			if player := state.Players[userID]; player != nil && !player.IsLeft && !player.IsKicked {
				item.PlayerUserIDs = append(item.PlayerUserIDs, userID)
			}
		}
		item.PlayerCount = len(item.PlayerUserIDs)
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"games": items})
}

type gameListItem struct {
	ID            int64           `json:"id"`
	Title         string          `json:"title"`
	CreatedAt     time.Time       `json:"created_at"`
	Status        game.GameStatus `json:"status"`
	CurrentRound  int             `json:"current_round"`
	PlayerCount   int             `json:"player_count"`
	PlayerUserIDs []int64         `json:"player_user_ids"`
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
	if in.Type == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "type is required"})
		return
	}

	state, events, err := s.engine.HandleAction(r.Context(), gameID, game.Action{
		UserID:  currentUserID(r),
		Type:    in.Type,
		Payload: in.Payload,
	})
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}
	s.decoratePublicState(r.Context(), state)

	deleted := false
	if in.Type == game.ActionLeaveGame && state != nil && len(state.Players) == 0 {
		if err := s.store.DeleteGame(r.Context(), gameID); err != nil {
			writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
			return
		}
		deleted = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events":       publicActionEvents(events),
		"state":        state,
		"game_deleted": deleted,
	})
}

func publicActionEvents(events []models.Event) []models.Event {
	publicEvents := make([]models.Event, 0, len(events))
	for _, event := range events {
		if isPrivateActionEvent(event.EventType) {
			continue
		}
		publicEvents = append(publicEvents, event)
	}
	return publicEvents
}

func isPrivateActionEvent(eventType string) bool {
	switch eventType {
	case models.EventMoleSelected,
		models.EventMoleTargetsGenerated,
		models.EventVoteSubmitted,
		models.EventGovernanceVoteSubmitted:
		return true
	default:
		return false
	}
}

func (s *Server) handleGetGameState(w http.ResponseWriter, r *http.Request) {
	gameID, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}
	viewerID := currentUserID(r)

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
	s.decoratePublicState(r.Context(), publicState)

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
		containsText(err.Error(), "game is full"),
		containsText(err.Error(), "duplicate"),
		containsText(err.Error(), "unique constraint"),
		containsText(err.Error(), "idx_users_login_lower"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func containsText(s, sub string) bool {
	return strings.Contains(s, sub)
}

func devCORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{}

	for _, origin := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
