package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	authpkg "agentbackend/internal/auth"
	"agentbackend/internal/models"
)

type authUserResponse struct {
	ID         int64      `json:"id"`
	Login      string     `json:"login"`
	Name       string     `json:"name"`
	AvatarURL  string     `json:"avatar_url,omitempty"`
	Position   string     `json:"company_position,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type authResponse struct {
	User  authUserResponse `json:"user"`
	Token string           `json:"token"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Login     string `json:"login"`
		Password  string `json:"password"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url,omitempty"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	login := authpkg.NormalizeLogin(in.Login)
	name := strings.TrimSpace(in.Name)
	avatarURL := strings.TrimSpace(in.AvatarURL)

	if err := authpkg.ValidateLogin(login); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err := validateDisplayName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err := validateAvatarURL(avatarURL); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	passwordHash, err := authpkg.HashPassword(in.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	now := time.Now().UTC()
	user := &models.User{
		Login:        login,
		Name:         name,
		PasswordHash: passwordHash,
		AvatarURL:    avatarURL,
		LastSeenAt:   &now,
	}

	if err := s.store.CreateUser(r.Context(), user); err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	token, err := s.issueToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{User: publicAuthUser(user), Token: token})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	login := authpkg.NormalizeLogin(in.Login)
	user, err := s.store.GetUserByLogin(r.Context(), login)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid login or password"})
		return
	}
	if err := authpkg.CheckPassword(user.PasswordHash, in.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid login or password"})
		return
	}

	if err := s.store.TouchUserLastSeen(r.Context(), user.ID, 0); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	user, _ = s.store.GetUserByID(r.Context(), user.ID)

	token, err := s.issueToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, authResponse{User: publicAuthUser(user), Token: token})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": publicAuthUser(currentUser(r))})
}

func (s *Server) handleUpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Position  string `json:"company_position"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	user := currentUser(r)
	user.Name = strings.TrimSpace(in.Name)
	user.AvatarURL = strings.TrimSpace(in.AvatarURL)
	user.Position = strings.TrimSpace(in.Position)

	if err := validateDisplayName(user.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err := validateAvatarURL(user.AvatarURL); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err := validateCompanyPosition(user.Position); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := s.store.UpdateUserProfile(r.Context(), user); err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": publicAuthUser(user)})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	type req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	user := currentUser(r)
	if err := authpkg.CheckPassword(user.PasswordHash, in.CurrentPassword); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid current password"})
		return
	}
	passwordHash, err := authpkg.HashPassword(in.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := s.store.UpdateUserPassword(r.Context(), user.ID, passwordHash); err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "password_updated"})
}

func (s *Server) issueToken(user *models.User) (string, error) {
	if user == nil {
		return "", errors.New("user is required")
	}
	return authpkg.IssueToken(s.jwtSecret, user.ID, user.Login, s.tokenTTL)
}

func publicAuthUser(user *models.User) authUserResponse {
	if user == nil {
		return authUserResponse{}
	}
	return authUserResponse{
		ID:         user.ID,
		Login:      user.Login,
		Name:       user.Name,
		AvatarURL:  user.AvatarURL,
		Position:   user.Position,
		LastSeenAt: user.LastSeenAt,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func validateDisplayName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len([]rune(name)) > 64 {
		return errors.New("name cannot exceed 64 characters")
	}
	return nil
}

func validateCompanyPosition(position string) error {
	if len([]rune(position)) > 64 {
		return errors.New("company_position cannot exceed 64 characters")
	}
	return nil
}

func validateAvatarURL(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 2048 {
		return errors.New("avatar_url cannot exceed 2048 characters")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("avatar_url must be a valid http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("avatar_url must be a valid http or https URL")
	}
	return nil
}
