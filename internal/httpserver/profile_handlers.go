package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"agentbackend/internal/game"
	"agentbackend/internal/models"
)

type roleStatsResponse struct {
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"winrate"`
}

type userStatsResponse struct {
	Total    roleStatsResponse `json:"total"`
	Mole     roleStatsResponse `json:"mole"`
	Director roleStatsResponse `json:"director"`
}

type profileResponse struct {
	ID         int64             `json:"id"`
	Login      string            `json:"login,omitempty"`
	Name       string            `json:"name"`
	AvatarURL  string            `json:"avatar_url,omitempty"`
	LastSeenAt *time.Time        `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  *time.Time        `json:"updated_at,omitempty"`
	Stats      userStatsResponse `json:"stats"`
}

type leaderboardEntryResponse struct {
	Rank         int             `json:"rank"`
	User         profileResponse `json:"user"`
	Games        int             `json:"games"`
	Wins         int             `json:"wins"`
	Losses       int             `json:"losses"`
	WinRate      float64         `json:"winrate"`
	RatingPoints int             `json:"rating_points"`
}

type leaderboardResponse struct {
	Period  string                     `json:"period"`
	Entries []leaderboardEntryResponse `json:"entries"`
}

type playerResult struct {
	UserID     int64
	Role       string
	Won        bool
	FinishedAt time.Time
}

func (s *Server) handleMyProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.profileForUser(r.Context(), currentUser(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": profile})
}

func (s *Server) handleUserProfile(w http.ResponseWriter, r *http.Request) {
	id, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid user id"})
		return
	}

	user, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}

	profile, err := s.profileForUser(r.Context(), user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": profile})
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	since, err := leaderboardSince(period)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	stats, err := s.statsForAllUsers(r.Context(), since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	usersByID := map[int64]models.User{}
	for _, user := range users {
		usersByID[user.ID] = user
	}

	entries := make([]leaderboardEntryResponse, 0, len(stats))
	for userID, stat := range stats {
		if stat.Total.Games < 3 {
			continue
		}
		user, ok := usersByID[userID]
		if !ok {
			continue
		}
		entries = append(entries, leaderboardEntryResponse{
			User:         profileFromUser(&user, stat),
			Games:        stat.Total.Games,
			Wins:         stat.Total.Wins,
			Losses:       stat.Total.Losses,
			WinRate:      stat.Total.WinRate,
			RatingPoints: stat.Total.Wins * 3,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RatingPoints != entries[j].RatingPoints {
			return entries[i].RatingPoints > entries[j].RatingPoints
		}
		if entries[i].WinRate != entries[j].WinRate {
			return entries[i].WinRate > entries[j].WinRate
		}
		if entries[i].Games != entries[j].Games {
			return entries[i].Games > entries[j].Games
		}
		return entries[i].User.Name < entries[j].User.Name
	})

	for i := range entries {
		entries[i].Rank = i + 1
	}

	writeJSON(w, http.StatusOK, leaderboardResponse{Period: period, Entries: entries})
}

func (s *Server) profileForUser(ctx context.Context, user *models.User) (profileResponse, error) {
	statsByUser, err := s.statsForAllUsers(ctx, nil)
	if err != nil {
		return profileResponse{}, err
	}
	return profileFromUser(user, statsByUser[user.ID]), nil
}

func profileFromUser(user *models.User, stats userStatsResponse) profileResponse {
	if user == nil {
		return profileResponse{Stats: stats}
	}
	return profileResponse{
		ID:         user.ID,
		Login:      user.Login,
		Name:       user.Name,
		AvatarURL:  user.AvatarURL,
		LastSeenAt: user.LastSeenAt,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
		Stats:      stats,
	}
}

func (s *Server) statsForAllUsers(ctx context.Context, since *time.Time) (map[int64]userStatsResponse, error) {
	results, err := s.completedPlayerResults(ctx)
	if err != nil {
		return nil, err
	}

	statsByUser := map[int64]userStatsResponse{}
	for _, result := range results {
		if since != nil && result.FinishedAt.Before(*since) {
			continue
		}
		stat := statsByUser[result.UserID]
		addResult(&stat.Total, result.Won)
		if result.Role == "mole" {
			addResult(&stat.Mole, result.Won)
		} else {
			addResult(&stat.Director, result.Won)
		}
		statsByUser[result.UserID] = stat
	}

	return statsByUser, nil
}

func (s *Server) completedPlayerResults(ctx context.Context) ([]playerResult, error) {
	games, err := s.store.ListGames(ctx)
	if err != nil {
		return nil, err
	}

	var results []playerResult
	for _, gameModel := range games {
		events, err := s.store.ListEventsByGameID(ctx, gameModel.ID)
		if err != nil {
			return nil, err
		}
		finishedAt, ok := gameFinishedAt(events)
		if !ok {
			continue
		}
		state, err := game.BuildState(gameModel.ID, gameModel.Title, events)
		if err != nil {
			return nil, err
		}
		if !state.IsFinished || state.Winner == "" {
			continue
		}
		for _, userID := range state.PlayerOrder {
			player := state.Players[userID]
			if player == nil || player.IsKicked || player.IsLeft {
				continue
			}
			role := player.Role
			won := (state.Winner == "mole" && role == "mole") ||
				(state.Winner == "players" && role != "mole")
			results = append(results, playerResult{
				UserID:     userID,
				Role:       role,
				Won:        won,
				FinishedAt: finishedAt,
			})
		}
	}

	return results, nil
}

func gameFinishedAt(events []models.Event) (time.Time, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType == models.EventGameFinished {
			return events[i].CreatedAt, true
		}
	}
	return time.Time{}, false
}

func addResult(stats *roleStatsResponse, won bool) {
	stats.Games++
	if won {
		stats.Wins++
	} else {
		stats.Losses++
	}
	if stats.Games > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.Games)
	}
}

func leaderboardSince(period string) (*time.Time, error) {
	now := time.Now().UTC()
	switch period {
	case "week":
		since := now.AddDate(0, 0, -7)
		return &since, nil
	case "month":
		since := now.AddDate(0, 0, -30)
		return &since, nil
	case "all":
		return nil, nil
	default:
		return nil, strconv.ErrSyntax
	}
}

func (s *Server) decoratePublicState(ctx context.Context, state *game.PublicGameState) {
	if state == nil {
		return
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return
	}
	avatars := map[int64]string{}
	for _, user := range users {
		avatars[user.ID] = user.AvatarURL
	}
	for i := range state.Players {
		state.Players[i].AvatarURL = avatars[state.Players[i].UserID]
		if state.Me.UserID == state.Players[i].UserID {
			state.Me.AvatarURL = state.Players[i].AvatarURL
		}
	}
	for i := range state.ChatMessages {
		state.ChatMessages[i].AvatarURL = avatars[state.ChatMessages[i].UserID]
	}
}

func decodeGameFinishedPayload(value string) (game.GameFinishedPayload, bool) {
	var payload game.GameFinishedPayload
	if value == "" {
		return payload, false
	}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return payload, false
	}
	return payload, payload.Winner != ""
}
