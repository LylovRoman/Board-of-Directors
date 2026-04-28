package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentbackend/internal/game"
	"agentbackend/internal/models"
)

type mockStorage struct {
	users  []models.User
	games  []models.Game
	events []models.Event
}

func (m *mockStorage) CreateUser(ctx context.Context, user *models.User) error {
	user.ID = int64(len(m.users) + 1)
	user.CreatedAt = time.Now()
	m.users = append(m.users, *user)
	return nil
}

func (m *mockStorage) ListUsers(ctx context.Context) ([]models.User, error) {
	if m.users == nil {
		return []models.User{}, nil
	}
	return m.users, nil
}

func (m *mockStorage) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			user := u
			return &user, nil
		}
	}
	return nil, errNotFound("user")
}

func (m *mockStorage) UpdateUser(ctx context.Context, user *models.User) error {
	for i := range m.users {
		if m.users[i].ID == user.ID {
			user.CreatedAt = m.users[i].CreatedAt
			m.users[i] = *user
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) DeleteUser(ctx context.Context, id int64) error {
	for i := range m.users {
		if m.users[i].ID == id {
			m.users = append(m.users[:i], m.users[i+1:]...)
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) CreateGame(ctx context.Context, game *models.Game) error {
	game.ID = int64(len(m.games) + 1)
	game.CreatedAt = time.Now()
	m.games = append(m.games, *game)
	return nil
}

func (m *mockStorage) CreateGameWithEvents(ctx context.Context, game *models.Game, events []models.Event) error {
	if err := m.CreateGame(ctx, game); err != nil {
		return err
	}
	for i := range events {
		events[i].GameID = game.ID
		if err := m.CreateEvent(ctx, &events[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockStorage) ListGames(ctx context.Context) ([]models.Game, error) {
	if m.games == nil {
		return []models.Game{}, nil
	}
	return m.games, nil
}

func (m *mockStorage) GetGameByID(ctx context.Context, id int64) (*models.Game, error) {
	for _, g := range m.games {
		if g.ID == id {
			game := g
			return &game, nil
		}
	}
	return nil, errNotFound("game")
}

func (m *mockStorage) UpdateGame(ctx context.Context, game *models.Game) error {
	for i := range m.games {
		if m.games[i].ID == game.ID {
			game.CreatedAt = m.games[i].CreatedAt
			m.games[i] = *game
			return nil
		}
	}
	return errNotFound("game")
}

func (m *mockStorage) DeleteGame(ctx context.Context, id int64) error {
	for i := range m.games {
		if m.games[i].ID == id {
			m.games = append(m.games[:i], m.games[i+1:]...)
			return nil
		}
	}
	return errNotFound("game")
}

func (m *mockStorage) CreateEvent(ctx context.Context, event *models.Event) error {
	event.ID = int64(len(m.events) + 1)
	event.CreatedAt = time.Now()
	m.events = append(m.events, *event)
	return nil
}

func (m *mockStorage) AppendEvents(ctx context.Context, gameID int64, events []models.Event) error {
	for i := range events {
		events[i].GameID = gameID
		if err := m.CreateEvent(ctx, &events[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockStorage) ListEvents(ctx context.Context) ([]models.Event, error) {
	if m.events == nil {
		return []models.Event{}, nil
	}
	return m.events, nil
}

func (m *mockStorage) GetEventByID(ctx context.Context, id int64) (*models.Event, error) {
	for _, e := range m.events {
		if e.ID == id {
			event := e
			return &event, nil
		}
	}
	return nil, errNotFound("event")
}

func (m *mockStorage) ListEventsByGameID(ctx context.Context, gameID int64) ([]models.Event, error) {
	var out []models.Event
	for _, e := range m.events {
		if e.GameID == gameID {
			out = append(out, e)
		}
	}
	if out == nil {
		out = []models.Event{}
	}
	return out, nil
}

func (m *mockStorage) Close() error {
	return nil
}

type notFoundError struct {
	entity string
}

func (e notFoundError) Error() string {
	return e.entity + " not found"
}

func errNotFound(entity string) error {
	return notFoundError{entity: entity}
}

func TestCreateUser(t *testing.T) {
	store := &mockStorage{}
	router := NewRouter(store)

	body := []byte(`{"name":"Alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/users/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp struct {
		User models.User `json:"user"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.User.ID != 1 {
		t.Fatalf("expected user id 1, got %d", resp.User.ID)
	}
	if resp.User.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", resp.User.Name)
	}
}

func TestListUsers(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		},
	}
	router := NewRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Users []models.User `json:"users"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Users))
	}
}

func TestCreateGame(t *testing.T) {
	store := &mockStorage{
		users: []models.User{{ID: 1, Name: "Alice"}},
	}
	router := NewRouter(store)

	body := []byte(`{"title":"Mafia","host_user_id":1}`)
	req := httptest.NewRequest(http.MethodPost, "/games/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp struct {
		Game models.Game `json:"game"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Game.ID != 1 {
		t.Fatalf("expected game id 1, got %d", resp.Game.ID)
	}
	if resp.Game.Title != "Mafia" {
		t.Fatalf("expected title Mafia, got %s", resp.Game.Title)
	}
	if len(store.events) != 2 {
		t.Fatalf("expected 2 bootstrap events, got %d", len(store.events))
	}
	if store.events[0].EventType != models.EventGameCreated {
		t.Fatalf("expected first event %s, got %s", models.EventGameCreated, store.events[0].EventType)
	}
	if store.events[1].EventType != models.EventPlayerJoined {
		t.Fatalf("expected second event %s, got %s", models.EventPlayerJoined, store.events[1].EventType)
	}
}

func TestGameActionJoinGame(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		},
		games: []models.Game{
			{ID: 1, Title: "Mafia"},
		},
		events: []models.Event{
			{
				ID:         1,
				GameID:     1,
				UserID:     int64Ptr(1),
				ActorName:  "Alice",
				EventType:  models.EventGameCreated,
				EventValue: `{"host_user_id":1,"title":"Mafia"}`,
			},
			{
				ID:         2,
				GameID:     1,
				UserID:     int64Ptr(1),
				ActorName:  "Alice",
				EventType:  models.EventPlayerJoined,
				EventValue: `{"user_id":1,"name":"Alice"}`,
			},
		},
	}
	router := NewRouter(store)

	body := []byte(`{
		"user_id": 2,
		"type": "join_game"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		Events []models.Event       `json:"events"`
		State  game.PublicGameState `json:"state"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Events[0].EventType != models.EventPlayerJoined {
		t.Fatalf("expected event_type %s, got %s", models.EventPlayerJoined, resp.Events[0].EventType)
	}
	if len(resp.State.Players) != 2 {
		t.Fatalf("expected 2 players in state, got %d", len(resp.State.Players))
	}
	if len(store.events) != 3 {
		t.Fatalf("expected 3 stored events, got %d", len(store.events))
	}
}

func TestLeaveGameDeletesEmptyLobby(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
		},
		games: []models.Game{
			{ID: 1, Title: "Mafia"},
		},
		events: []models.Event{
			{
				ID:         1,
				GameID:     1,
				UserID:     int64Ptr(1),
				ActorName:  "Alice",
				EventType:  models.EventGameCreated,
				EventValue: `{"host_user_id":1,"title":"Mafia"}`,
			},
			{
				ID:         2,
				GameID:     1,
				UserID:     int64Ptr(1),
				ActorName:  "Alice",
				EventType:  models.EventPlayerJoined,
				EventValue: `{"user_id":1,"name":"Alice"}`,
			},
		},
	}
	router := NewRouter(store)

	body := []byte(`{
		"user_id": 1,
		"type": "leave_game"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		Events      []models.Event       `json:"events"`
		State       game.PublicGameState `json:"state"`
		GameDeleted bool                 `json:"game_deleted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Events) != 1 || resp.Events[0].EventType != models.EventPlayerLeft {
		t.Fatalf("expected player_left event, got %+v", resp.Events)
	}
	if !resp.GameDeleted {
		t.Fatalf("expected game_deleted=true")
	}
	if len(store.games) != 0 {
		t.Fatalf("expected game to be deleted, got %+v", store.games)
	}
}

func TestGetGameState_HidesMoleTargetsForRegularPlayer(t *testing.T) {
	store := &mockStorage{
		games: []models.Game{{ID: 1, Title: "Mafia"}},
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
			{ID: 3, Name: "Carol"},
		},
		events: []models.Event{
			{ID: 1, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
			{ID: 2, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
			{ID: 3, GameID: 1, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			{ID: 4, GameID: 1, UserID: int64Ptr(3), ActorName: "Carol", EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
			{ID: 5, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameStarted, EventValue: `{}`},
			{ID: 6, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventMoleSelected, EventValue: `{"user_id":2}`},
			{ID: 7, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","C","F"]}`},
			{ID: 8, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":4000}`},
			{ID: 9, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":3000}`},
			{ID: 10, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2500}`},
			{ID: 11, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
			{ID: 12, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
		},
	}
	router := NewRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/games/1/state?viewer_user_id=1", nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		State game.PublicGameState `json:"state"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.State.MoleTargets) != 0 {
		t.Fatalf("expected regular player not to see mole targets, got %v", resp.State.MoleTargets)
	}
}

func TestDevCORSPreflight(t *testing.T) {
	store := &mockStorage{}
	router := NewRouter(store)

	req := httptest.NewRequest(http.MethodOptions, "/users/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected allow origin header, got %q", got)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
