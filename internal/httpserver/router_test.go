package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	authpkg "agentbackend/internal/auth"
	"agentbackend/internal/game"
	"agentbackend/internal/models"
)

type mockStorage struct {
	users    []models.User
	games    []models.Game
	events   []models.Event
	respects map[int64]map[int64]bool
}

func (m *mockStorage) CreateUser(ctx context.Context, user *models.User) error {
	if user.Login != "" {
		for _, existing := range m.users {
			if strings.EqualFold(existing.Login, user.Login) {
				return errors.New("duplicate login")
			}
		}
	}
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

func (m *mockStorage) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {
	for _, u := range m.users {
		if strings.EqualFold(u.Login, login) {
			user := u
			return &user, nil
		}
	}
	return nil, errNotFound("user")
}

func (m *mockStorage) UpdateUser(ctx context.Context, user *models.User) error {
	for i := range m.users {
		if m.users[i].ID == user.ID {
			m.users[i].Name = user.Name
			m.users[i].UpdatedAt = timePtr(time.Now())
			*user = m.users[i]
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) UpdateUserProfile(ctx context.Context, user *models.User) error {
	for i := range m.users {
		if m.users[i].ID == user.ID {
			m.users[i].Name = user.Name
			m.users[i].AvatarURL = user.AvatarURL
			m.users[i].Position = user.Position
			m.users[i].UpdatedAt = timePtr(time.Now())
			*user = m.users[i]
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	for i := range m.users {
		if m.users[i].ID == id {
			m.users[i].PasswordHash = passwordHash
			m.users[i].UpdatedAt = timePtr(time.Now())
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) TouchUserLastSeen(ctx context.Context, id int64, minInterval time.Duration) error {
	now := time.Now()
	for i := range m.users {
		if m.users[i].ID == id {
			if minInterval <= 0 || m.users[i].LastSeenAt == nil || m.users[i].LastSeenAt.Before(now.Add(-minInterval)) {
				m.users[i].LastSeenAt = &now
			}
			return nil
		}
	}
	return errNotFound("user")
}

func (m *mockStorage) GiveUserRespect(ctx context.Context, giverID int64, receiverID int64) error {
	if m.respects == nil {
		m.respects = map[int64]map[int64]bool{}
	}
	if m.respects[receiverID] == nil {
		m.respects[receiverID] = map[int64]bool{}
	}
	m.respects[receiverID][giverID] = true
	return nil
}

func (m *mockStorage) CountUserRespect(ctx context.Context, userID int64) (int, error) {
	return len(m.respects[userID]), nil
}

func (m *mockStorage) HasUserRespect(ctx context.Context, giverID int64, receiverID int64) (bool, error) {
	return m.respects[receiverID][giverID], nil
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

func TestAuthRegisterLoginAndDuplicateLogin(t *testing.T) {
	store := &mockStorage{}
	router := NewRouter(store, "test-secret")

	registerBody := []byte(`{
		"login": "Alice",
		"name": "Alice",
		"password": "password123",
		"avatar_url": "https://example.com/a.png"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if len(store.users) != 1 || store.users[0].PasswordHash == "" || store.users[0].PasswordHash == "password123" {
		t.Fatalf("expected registered user with password hash, got %+v", store.users)
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate registration to fail with %d, got %d", http.StatusBadRequest, rec.Code)
	}

	loginBody := []byte(`{"login":"alice","password":"password123"}`)
	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp authResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" || resp.User.Login != "alice" {
		t.Fatalf("expected token and normalized login, got %+v", resp)
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"login":"alice","password":"wrongpass"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid password status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	store := &mockStorage{}
	router := NewRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/games/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestUpdateMyProfileStoresCompanyPosition(t *testing.T) {
	user := models.User{ID: 1, Login: "alice", Name: "Alice"}
	store := &mockStorage{users: []models.User{user}}
	router := NewRouter(store, "test-secret")

	body := []byte(`{"name":"Alice","avatar_url":"https://example.com/a.png","company_position":"Финансовый директор"}`)
	req := httptest.NewRequest(http.MethodPut, "/users/me/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, user)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		User authUserResponse `json:"user"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.User.Position != "Финансовый директор" || store.users[0].Position != "Финансовый директор" {
		t.Fatalf("expected profile position to be saved, resp=%+v store=%+v", resp.User, store.users[0])
	}
}

func TestRespectUserIsIdempotentAndBlocksSelf(t *testing.T) {
	alice := models.User{ID: 1, Login: "alice", Name: "Alice"}
	bob := models.User{ID: 2, Login: "bob", Name: "Bob"}
	store := &mockStorage{users: []models.User{alice, bob}}
	router := NewRouter(store, "test-secret")

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users/2/respect", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		setAuth(req, t, alice)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
		}
	}
	if len(store.respects[2]) != 1 {
		t.Fatalf("expected one respect entry, got %+v", store.respects)
	}

	req := httptest.NewRequest(http.MethodPost, "/users/1/respect", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, alice)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected self respect to fail, got %d", rec.Code)
	}
}

func TestCreateUser(t *testing.T) {
	admin := models.User{ID: 1, Login: "admin", Name: "Admin"}
	store := &mockStorage{users: []models.User{admin}}
	router := NewRouter(store, "test-secret")

	body := []byte(`{"name":"Alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/users/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, admin)

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

	if resp.User.ID != 2 {
		t.Fatalf("expected user id 2, got %d", resp.User.ID)
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
	router := NewRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	setAuth(req, t, models.User{ID: 1, Login: "alice"})
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
	router := NewRouter(store, "test-secret")

	body := []byte(`{"title":"Mafia","host_user_id":2}`)
	req := httptest.NewRequest(http.MethodPost, "/games/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, models.User{ID: 1, Login: "alice"})

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
	if len(store.events) != 3 {
		t.Fatalf("expected 3 bootstrap events, got %d", len(store.events))
	}
	if store.events[0].EventType != models.EventGameCreated {
		t.Fatalf("expected first event %s, got %s", models.EventGameCreated, store.events[0].EventType)
	}
	if !strings.Contains(store.events[0].EventValue, `"host_user_id":1`) {
		t.Fatalf("expected authenticated user to be host, got event payload %s", store.events[0].EventValue)
	}
	if !strings.Contains(store.events[0].EventValue, `"company_name"`) || !strings.Contains(store.events[0].EventValue, `"company_situation"`) {
		t.Fatalf("expected generated company scenario, got event payload %s", store.events[0].EventValue)
	}
	if store.events[1].EventType != models.EventPlayerJoined {
		t.Fatalf("expected second event %s, got %s", models.EventPlayerJoined, store.events[1].EventType)
	}
	if store.events[2].EventType != models.EventChatMessageSent {
		t.Fatalf("expected third event %s, got %s", models.EventChatMessageSent, store.events[2].EventType)
	}
}

func TestListGamesReturnsSummariesWithoutState(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		},
		games: []models.Game{
			{ID: 1, Title: "Mafia"},
			{ID: 2, Title: "Second Room"},
		},
		events: []models.Event{
			{ID: 1, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
			{ID: 2, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
			{ID: 3, GameID: 1, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			{ID: 4, GameID: 2, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventGameCreated, EventValue: `{"host_user_id":2,"title":"Second Room"}`},
			{ID: 5, GameID: 2, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
		},
	}
	router := NewRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/games/", nil)
	setAuth(req, t, models.User{ID: 1, Login: "alice"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var raw map[string][]map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, ok := raw["games"][0]["state"]; ok {
		t.Fatalf("list games response must not include full state: %s", rec.Body.String())
	}

	var resp struct {
		Games []gameListItem `json:"games"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Games) != 2 {
		t.Fatalf("expected 2 games, got %d", len(resp.Games))
	}
	if resp.Games[0].Status != game.GameStatusLobby || resp.Games[0].PlayerCount != 2 {
		t.Fatalf("expected lobby summary with 2 players, got %+v", resp.Games[0])
	}
	if len(resp.Games[0].PlayerUserIDs) != 2 || resp.Games[0].PlayerUserIDs[0] != 1 || resp.Games[0].PlayerUserIDs[1] != 2 {
		t.Fatalf("expected player user ids [1 2], got %+v", resp.Games[0].PlayerUserIDs)
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
	router := NewRouter(store, "test-secret")

	body := []byte(`{
		"user_id": 1,
		"type": "join_game"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, models.User{ID: 2, Login: "bob"})

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
	if !strings.Contains(resp.Events[0].EventValue, `"user_id":2`) {
		t.Fatalf("expected authenticated user to join, got event payload %s", resp.Events[0].EventValue)
	}
	if len(resp.State.Players) != 2 {
		t.Fatalf("expected 2 players in state, got %d", len(resp.State.Players))
	}
	if len(store.events) != 3 {
		t.Fatalf("expected 3 stored events, got %d", len(store.events))
	}
}

func TestGameActionStartGameHidesPrivateEvents(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
			{ID: 3, Name: "Carol"},
		},
		games: []models.Game{
			{ID: 1, Title: "Mafia"},
		},
		events: []models.Event{
			{ID: 1, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
			{ID: 2, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
			{ID: 3, GameID: 1, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			{ID: 4, GameID: 1, UserID: int64Ptr(3), ActorName: "Carol", EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
		},
	}
	router := NewRouter(store, "test-secret")

	body := []byte(`{
		"user_id": 1,
		"type": "start_game"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, models.User{ID: 1, Login: "alice"})

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

	if !hasEventType(store.events, models.EventMoleSelected) {
		t.Fatalf("expected stored event %s", models.EventMoleSelected)
	}
	if hasEventType(store.events, models.EventMoleTargetsGenerated) {
		t.Fatalf("did not expect stored legacy event %s", models.EventMoleTargetsGenerated)
	}
	if hasEventType(resp.Events, models.EventMoleSelected) {
		t.Fatalf("response must not expose %s: %+v", models.EventMoleSelected, resp.Events)
	}
	if !hasEventType(resp.Events, models.EventGameStarted) {
		t.Fatalf("expected public response event %s", models.EventGameStarted)
	}
	if hasEventType(resp.Events, models.EventVotingRoundStarted) {
		t.Fatalf("did not expect voting to start before mole objectives are selected")
	}
	if resp.State.Phase != game.GamePhaseMoleObjectiveSelection {
		t.Fatalf("expected mole objective selection phase, got %s", resp.State.Phase)
	}
	if resp.State.Me.Role == "mole" {
		if len(resp.State.MoleTargets) != 0 || resp.State.MoleSabotage != "" {
			t.Fatalf("expected mole objectives to be empty before selection, got %v / %q", resp.State.MoleTargets, resp.State.MoleSabotage)
		}
		if !hasAction(resp.State.AvailableActions, game.ActionSelectMoleObjectives) {
			t.Fatalf("expected mole host to be able to select objectives, got %v", resp.State.AvailableActions)
		}
	} else {
		if resp.State.Me.Role != "player" {
			t.Fatalf("expected non-mole host role player, got %q", resp.State.Me.Role)
		}
		if len(resp.State.MoleTargets) != 0 {
			t.Fatalf("expected non-mole host to not see mole targets, got %v", resp.State.MoleTargets)
		}
	}
}

func hasAction(actions []game.ActionType, action game.ActionType) bool {
	for _, item := range actions {
		if item == action {
			return true
		}
	}
	return false
}

func TestPublicActionEventsHidesMoleObjectivesSelected(t *testing.T) {
	events := []models.Event{
		{EventType: models.EventMoleObjectivesSelected, EventValue: `{"targets":["A","C","F"],"sabotage":"H"}`},
		{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
	}
	publicEvents := publicActionEvents(events)
	if hasEventType(publicEvents, models.EventMoleObjectivesSelected) {
		t.Fatalf("response must not expose %s: %+v", models.EventMoleObjectivesSelected, publicEvents)
	}
	if !hasEventType(publicEvents, models.EventVotingRoundStarted) {
		t.Fatalf("expected public response event %s", models.EventVotingRoundStarted)
	}
}

func TestGameActionVoteHidesSubmittedVoteEvent(t *testing.T) {
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
			{ID: 3, Name: "Carol"},
		},
		games: []models.Game{
			{ID: 1, Title: "Mafia"},
		},
		events: []models.Event{
			{ID: 1, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
			{ID: 2, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
			{ID: 3, GameID: 1, UserID: int64Ptr(2), ActorName: "Bob", EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			{ID: 4, GameID: 1, UserID: int64Ptr(3), ActorName: "Carol", EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
			{ID: 5, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameStarted, EventValue: `{}`},
			{ID: 6, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventMoleSelected, EventValue: `{"user_id":3}`},
			{ID: 7, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
			{ID: 8, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":3500}`},
			{ID: 9, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2500}`},
			{ID: 10, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
			{ID: 11, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
			{ID: 12, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
		},
	}
	router := NewRouter(store, "test-secret")

	body := []byte(`{
		"user_id": 2,
		"type": "vote",
		"payload": {
			"decision": "B"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, models.User{ID: 2, Login: "bob"})

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

	if !hasEventType(store.events, models.EventVoteSubmitted) {
		t.Fatalf("expected stored event %s", models.EventVoteSubmitted)
	}
	if len(resp.Events) != 0 {
		t.Fatalf("expected no public response events for a single current vote, got %+v", resp.Events)
	}

	var bobVote *game.PublicVoteState
	for i := range resp.State.CurrentVotes {
		if resp.State.CurrentVotes[i].UserID == 2 {
			bobVote = &resp.State.CurrentVotes[i]
			break
		}
	}
	if bobVote == nil || !bobVote.HasVoted {
		t.Fatalf("expected Bob has_voted=true, got %+v", resp.State.CurrentVotes)
	}
	if resp.State.MyCurrentVote == nil || resp.State.MyCurrentVote.Decision != "B" {
		t.Fatalf("expected own vote decision B, got %+v", resp.State.MyCurrentVote)
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
	router := NewRouter(store, "test-secret")

	body := []byte(`{
		"user_id": 1,
		"type": "leave_game"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/games/1/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, t, models.User{ID: 1, Login: "alice"})

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
	router := NewRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/games/1/state?viewer_user_id=1", nil)
	setAuth(req, t, models.User{ID: 1, Login: "alice"})

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

	req = httptest.NewRequest(http.MethodGet, "/games/1/state?viewer_user_id=1", nil)
	setAuth(req, t, models.User{ID: 2, Login: "bob"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	resp = struct {
		State game.PublicGameState `json:"state"`
	}{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode mole response: %v", err)
	}
	if len(resp.State.MoleTargets) != 3 {
		t.Fatalf("expected mole player to see 3 mole targets, got %v", resp.State.MoleTargets)
	}
	if resp.State.Me.Role != "mole" {
		t.Fatalf("expected viewer 2 role mole, got %q", resp.State.Me.Role)
	}
}

func TestGameWebSocketAcceptsQueryToken(t *testing.T) {
	secret := "test-secret"
	store := &mockStorage{
		users: []models.User{{ID: 1, Login: "alice", Name: "Alice"}},
		games: []models.Game{{ID: 1, Title: "Mafia"}},
		events: []models.Event{
			{ID: 1, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`, CreatedAt: time.Now()},
			{ID: 2, GameID: 1, UserID: int64Ptr(1), ActorName: "Alice", EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`, CreatedAt: time.Now()},
		},
	}
	server := httptest.NewServer(NewRouter(store, secret))
	defer server.Close()

	token, err := authpkg.IssueToken(secret, 1, "alice", time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/games/1/ws?token=" + url.QueryEscape(token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var message liveMessage
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("read live state: %v", err)
	}
	if message.Type != "state" || message.State == nil || message.State.GameID != 1 {
		t.Fatalf("unexpected live message: %+v", message)
	}
}

func TestGamesWebSocketPushesLobbyListUpdates(t *testing.T) {
	secret := "test-secret"
	store := &mockStorage{
		users: []models.User{{ID: 1, Login: "alice", Name: "Alice"}},
	}
	router := NewRouter(store, secret)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := authpkg.IssueToken(secret, 1, "alice", time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/games/ws?token=" + url.QueryEscape(token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial games websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var initial liveMessage
	if err := conn.ReadJSON(&initial); err != nil {
		t.Fatalf("read initial lobby list: %v", err)
	}
	if initial.Type != "games" || len(initial.Games) != 0 {
		t.Fatalf("unexpected initial lobby message: %+v", initial)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/games/", strings.NewReader(`{"title":"Mafia"}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	setAuth(req, t, models.User{ID: 1, Login: "alice"})
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var pushed liveMessage
	if err := conn.ReadJSON(&pushed); err != nil {
		t.Fatalf("read pushed lobby list: %v", err)
	}
	if pushed.Type != "games" || len(pushed.Games) != 1 || pushed.Games[0].Title != "Mafia" {
		t.Fatalf("unexpected pushed lobby message: %+v", pushed)
	}
}

func TestSwaggerRoutesServeUIAndSpec(t *testing.T) {
	router := NewRouter(&mockStorage{}, "test-secret")

	specReq := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	specRec := httptest.NewRecorder()
	router.ServeHTTP(specRec, specReq)
	if specRec.Code != http.StatusOK || !strings.Contains(specRec.Body.String(), "openapi: 3.0.3") {
		t.Fatalf("expected openapi yaml, got status=%d body=%s", specRec.Code, specRec.Body.String())
	}

	uiReq := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	uiRec := httptest.NewRecorder()
	router.ServeHTTP(uiRec, uiReq)
	if uiRec.Code != http.StatusOK || !strings.Contains(uiRec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("expected swagger ui html, got status=%d body=%s", uiRec.Code, uiRec.Body.String())
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/swagger/swagger-ui.css", nil)
	assetRec := httptest.NewRecorder()
	router.ServeHTTP(assetRec, assetReq)
	if assetRec.Code != http.StatusOK {
		t.Fatalf("expected swagger asset, got status=%d", assetRec.Code)
	}
}

func TestProfileStatsAndLeaderboardFromFinishedGames(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStorage{
		users: []models.User{
			{ID: 1, Login: "alice", Name: "Alice"},
			{ID: 2, Login: "bob", Name: "Bob"},
			{ID: 3, Login: "carol", Name: "Carol"},
			{ID: 4, Login: "dave", Name: "Dave"},
		},
		games: []models.Game{
			{ID: 1, Title: "Game 1"},
			{ID: 2, Title: "Game 2"},
			{ID: 3, Title: "Game 3"},
			{ID: 4, Title: "Game 4"},
			{ID: 5, Title: "Game 5"},
		},
	}
	store.events = append(store.events, finishedGameEvents(1, 1, "mole", now.Add(-24*time.Hour))...)
	store.events = append(store.events, finishedGameEvents(2, 2, "players", now.Add(-23*time.Hour))...)
	store.events = append(store.events, finishedGameEvents(3, 2, "mole", now.Add(-22*time.Hour))...)
	store.events = append(store.events, finishedGameEvents(4, 4, "mole", now.Add(-21*time.Hour))...)
	store.events = append(store.events, finishedGameEvents(5, 4, "mole", now.Add(-20*time.Hour))...)

	router := NewRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/users/me/profile", nil)
	setAuth(req, t, models.User{ID: 1, Login: "alice"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var profileResp struct {
		Profile profileResponse `json:"profile"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&profileResp); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profileResp.Profile.Stats.Total.Games != 3 || profileResp.Profile.Stats.Total.Wins != 2 || profileResp.Profile.Stats.Total.Losses != 1 {
		t.Fatalf("unexpected total stats: %+v", profileResp.Profile.Stats.Total)
	}
	if profileResp.Profile.Stats.Mole.Games != 1 || profileResp.Profile.Stats.Mole.Wins != 1 {
		t.Fatalf("unexpected mole stats: %+v", profileResp.Profile.Stats.Mole)
	}

	req = httptest.NewRequest(http.MethodGet, "/leaderboard?period=week", nil)
	setAuth(req, t, models.User{ID: 1, Login: "alice"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var leaderboard leaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&leaderboard); err != nil {
		t.Fatalf("decode leaderboard: %v", err)
	}
	for _, entry := range leaderboard.Entries {
		if entry.User.ID == 4 {
			t.Fatalf("expected Dave to be excluded by 3 game minimum, got %+v", leaderboard.Entries)
		}
	}
	if len(leaderboard.Entries) == 0 || leaderboard.Entries[0].Games < 3 {
		t.Fatalf("expected leaderboard entries with at least 3 games, got %+v", leaderboard.Entries)
	}
}

func TestDevCORSPreflight(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")

	store := &mockStorage{}
	router := NewRouter(store, "test-secret")

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

func timePtr(v time.Time) *time.Time {
	return &v
}

func authHeader(t *testing.T, userID int64, login string) string {
	t.Helper()
	token, err := authpkg.IssueToken("test-secret", userID, login, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	return "Bearer " + token
}

func setAuth(req *http.Request, t *testing.T, user models.User) {
	t.Helper()
	req.Header.Set("Authorization", authHeader(t, user.ID, user.Login))
}

func hasEventType(events []models.Event, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func finishedGameEvents(gameID int64, moleUserID int64, winner string, finishedAt time.Time) []models.Event {
	players := []int64{1, 2, 3}
	names := map[int64]string{1: "Alice", 2: "Bob", 3: "Carol", 4: "Dave"}
	if moleUserID == 4 {
		players = []int64{4, 2, 3}
	}

	events := []models.Event{
		{ID: 1, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventGameCreated, EventValue: `{"host_user_id":` + strconv.FormatInt(players[0], 10) + `,"title":"Game"}`, CreatedAt: finishedAt.Add(-time.Minute)},
	}
	for i, userID := range players {
		events = append(events, models.Event{
			ID:         int64(2 + i),
			GameID:     gameID,
			UserID:     int64Ptr(userID),
			ActorName:  names[userID],
			EventType:  models.EventPlayerJoined,
			EventValue: `{"user_id":` + strconv.FormatInt(userID, 10) + `,"name":"` + names[userID] + `"}`,
			CreatedAt:  finishedAt.Add(-time.Minute),
		})
	}
	events = append(events,
		models.Event{ID: 10, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventGameStarted, EventValue: `{}`, CreatedAt: finishedAt.Add(-time.Minute)},
		models.Event{ID: 11, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventMoleSelected, EventValue: `{"user_id":` + strconv.FormatInt(moleUserID, 10) + `}`, CreatedAt: finishedAt.Add(-time.Minute)},
		models.Event{ID: 12, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","B","C"]}`, CreatedAt: finishedAt.Add(-time.Minute)},
	)
	for i, userID := range players {
		events = append(events, models.Event{
			ID:         int64(13 + i),
			GameID:     gameID,
			UserID:     int64Ptr(players[0]),
			ActorName:  names[players[0]],
			EventType:  models.EventPlayerReceivedShare,
			EventValue: `{"user_id":` + strconv.FormatInt(userID, 10) + `,"share_bps":` + []string{"3500", "2500", "2000"}[i] + `}`,
			CreatedAt:  finishedAt.Add(-time.Minute),
		})
	}
	events = append(events,
		models.Event{ID: 20, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventCEOSelected, EventValue: `{"user_id":` + strconv.FormatInt(players[0], 10) + `}`, CreatedAt: finishedAt.Add(-time.Minute)},
		models.Event{ID: 21, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`, CreatedAt: finishedAt.Add(-time.Minute)},
		models.Event{ID: 22, GameID: gameID, UserID: int64Ptr(players[0]), ActorName: names[players[0]], EventType: models.EventGameFinished, EventValue: `{"winner":"` + winner + `","reason":"test"}`, CreatedAt: finishedAt},
	)
	return events
}
