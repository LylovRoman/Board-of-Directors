package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"agentbackend/internal/models"
)

func newTestPostgres(t *testing.T) *Postgres {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	db, err := NewPostgres(dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = db.db.ExecContext(ctx, `
		DROP TABLE IF EXISTS events;
		DROP TABLE IF EXISTS games;
		DROP TABLE IF EXISTS user_respects;
		DROP TABLE IF EXISTS users;

		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			login VARCHAR(255),
			name VARCHAR(255) NOT NULL,
			password_hash TEXT,
			avatar_url TEXT,
			company_position TEXT,
			last_seen_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE UNIQUE INDEX idx_users_login_lower
			ON users (LOWER(login))
			WHERE login IS NOT NULL;

		CREATE TABLE games (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE TABLE events (
			id BIGSERIAL PRIMARY KEY,
			game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
			user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			actor_name VARCHAR(255),
			event_type VARCHAR(255) NOT NULL,
			event_value TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE TABLE user_respects (
			giver_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			receiver_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			week_start TIMESTAMP NOT NULL DEFAULT date_trunc('week', (NOW() AT TIME ZONE 'UTC')),
			PRIMARY KEY (giver_user_id, receiver_user_id, week_start),
			CONSTRAINT user_respects_no_self CHECK (giver_user_id <> receiver_user_id)
		);
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("prepare schema: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.db.ExecContext(context.Background(), `
			DROP TABLE IF EXISTS events;
			DROP TABLE IF EXISTS games;
			DROP TABLE IF EXISTS user_respects;
			DROP TABLE IF EXISTS users;
		`)
		_ = db.Close()
	})

	return db
}

func TestUTCWeekStartStartsOnMonday(t *testing.T) {
	got := utcWeekStart(time.Date(2026, 5, 20, 15, 30, 0, 0, time.UTC))
	want := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}

	got = utcWeekStart(time.Date(2026, 5, 24, 23, 59, 0, 0, time.UTC))
	if !got.Equal(want) {
		t.Fatalf("expected Sunday to stay in same UTC week %s, got %s", want, got)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	user := &models.User{Name: "Alice"}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if user.ID == 0 {
		t.Fatal("expected non-zero user ID")
	}

	got, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	if got.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", got.Name)
	}
}

func TestUpdateUser(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	user := &models.User{Name: "Alice"}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	user.Name = "Alice Updated"
	if err := store.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	if got.Name != "Alice Updated" {
		t.Fatalf("expected Alice Updated, got %s", got.Name)
	}
}

func TestUserAuthProfileFields(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user := &models.User{
		Login:        "alice",
		Name:         "Alice",
		PasswordHash: "hash",
		AvatarURL:    "https://example.com/a.png",
		Position:     "CFO",
		LastSeenAt:   &now,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := store.GetUserByLogin(ctx, "ALICE")
	if err != nil {
		t.Fatalf("GetUserByLogin: %v", err)
	}
	if got.PasswordHash != "hash" || got.AvatarURL != "https://example.com/a.png" || got.Position != "CFO" || got.LastSeenAt == nil {
		t.Fatalf("expected auth/profile fields, got %+v", got)
	}

	got.Name = "Alice Updated"
	got.AvatarURL = "https://example.com/new.png"
	got.Position = "Chief Strategy Officer"
	if err := store.UpdateUserProfile(ctx, got); err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if err := store.UpdateUserPassword(ctx, got.ID, "new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	if err := store.TouchUserLastSeen(ctx, got.ID, 0); err != nil {
		t.Fatalf("TouchUserLastSeen: %v", err)
	}

	got, err = store.GetUserByID(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Name != "Alice Updated" || got.AvatarURL != "https://example.com/new.png" || got.Position != "Chief Strategy Officer" || got.PasswordHash != "new-hash" {
		t.Fatalf("expected updated fields, got %+v", got)
	}
}

func TestUserRespectPersistence(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	alice := &models.User{Login: "alice", Name: "Alice"}
	bob := &models.User{Login: "bob", Name: "Bob"}
	if err := store.CreateUser(ctx, alice); err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	if err := store.CreateUser(ctx, bob); err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}
	if err := store.GiveUserRespect(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("GiveUserRespect: %v", err)
	}
	if err := store.GiveUserRespect(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("GiveUserRespect repeat: %v", err)
	}
	count, err := store.CountUserRespect(ctx, bob.ID)
	if err != nil {
		t.Fatalf("CountUserRespect: %v", err)
	}
	respected, err := store.HasUserRespect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("HasUserRespect: %v", err)
	}
	if count != 1 || !respected {
		t.Fatalf("expected one respect from alice to bob this week, count=%d respected=%v", count, respected)
	}

	previousWeek := utcWeekStart(time.Now().UTC()).AddDate(0, 0, -7)
	if _, err := store.db.ExecContext(ctx, `
		UPDATE user_respects
		SET week_start = $1, created_at = $1
		WHERE giver_user_id = $2 AND receiver_user_id = $3
	`, previousWeek, alice.ID, bob.ID); err != nil {
		t.Fatalf("move respect to previous week: %v", err)
	}
	respected, err = store.HasUserRespect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("HasUserRespect after week shift: %v", err)
	}
	if respected {
		t.Fatalf("expected previous-week respect to not count as respected_by_me")
	}
	if err := store.GiveUserRespect(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("GiveUserRespect next week: %v", err)
	}
	count, err = store.CountUserRespect(ctx, bob.ID)
	if err != nil {
		t.Fatalf("CountUserRespect after next week: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected respects to accumulate across weeks, count=%d", count)
	}
}

func TestCreateAndListGames(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	game := &models.Game{Title: "Mafia"}
	if err := store.CreateGame(ctx, game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}

	games, err := store.ListGames(ctx)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].Title != "Mafia" {
		t.Fatalf("expected Mafia, got %s", games[0].Title)
	}
}

func TestCreateAndGetEvent(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	user := &models.User{Name: "Alice"}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	game := &models.Game{Title: "Mafia"}
	if err := store.CreateGame(ctx, game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}

	event := &models.Event{
		GameID:     game.ID,
		UserID:     &user.ID,
		ActorName:  "Alice",
		EventType:  models.EventPlayerJoined,
		EventValue: "lobby",
	}
	if err := store.CreateEvent(ctx, event); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	got, err := store.GetEventByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetEventByID: %v", err)
	}

	if got.EventType != models.EventPlayerJoined {
		t.Fatalf("expected %s, got %s", models.EventPlayerJoined, got.EventType)
	}
	if got.ActorName != "Alice" {
		t.Fatalf("expected Alice, got %s", got.ActorName)
	}
}

func TestListEventsByGameID(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	game1 := &models.Game{Title: "Game 1"}
	if err := store.CreateGame(ctx, game1); err != nil {
		t.Fatalf("CreateGame game1: %v", err)
	}

	game2 := &models.Game{Title: "Game 2"}
	if err := store.CreateGame(ctx, game2); err != nil {
		t.Fatalf("CreateGame game2: %v", err)
	}

	event1 := &models.Event{
		GameID:    game1.ID,
		EventType: models.EventPlayerJoined,
	}
	if err := store.CreateEvent(ctx, event1); err != nil {
		t.Fatalf("CreateEvent event1: %v", err)
	}

	event2 := &models.Event{
		GameID:    game1.ID,
		EventType: models.EventDecisionAccepted,
	}
	if err := store.CreateEvent(ctx, event2); err != nil {
		t.Fatalf("CreateEvent event2: %v", err)
	}

	event3 := &models.Event{
		GameID:    game2.ID,
		EventType: models.EventGameFinished,
	}
	if err := store.CreateEvent(ctx, event3); err != nil {
		t.Fatalf("CreateEvent event3: %v", err)
	}

	events, err := store.ListEventsByGameID(ctx, game1.ID)
	if err != nil {
		t.Fatalf("ListEventsByGameID: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for _, e := range events {
		if e.GameID != game1.ID {
			t.Fatalf("expected game_id %d, got %d", game1.ID, e.GameID)
		}
	}
}

func TestDeleteUser_SetsNullInEvents(t *testing.T) {
	store := newTestPostgres(t)
	ctx := context.Background()

	user := &models.User{Name: "Alice"}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	game := &models.Game{Title: "Mafia"}
	if err := store.CreateGame(ctx, game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}

	event := &models.Event{
		GameID:     game.ID,
		UserID:     &user.ID,
		ActorName:  "Alice",
		EventType:  models.EventPlayerJoined,
		EventValue: "lobby",
	}
	if err := store.CreateEvent(ctx, event); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	got, err := store.GetEventByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetEventByID: %v", err)
	}

	if got.UserID != nil {
		t.Fatalf("expected user_id to be nil after user delete, got %v", *got.UserID)
	}
	if got.ActorName != "Alice" {
		t.Fatalf("expected actor_name to remain Alice, got %s", got.ActorName)
	}
}
