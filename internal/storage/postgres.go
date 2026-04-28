package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"agentbackend/internal/models"
)

type Storage interface {
	CreateUser(ctx context.Context, user *models.User) error
	ListUsers(ctx context.Context) ([]models.User, error)
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, id int64) error

	CreateGame(ctx context.Context, game *models.Game) error
	CreateGameWithEvents(ctx context.Context, game *models.Game, events []models.Event) error
	ListGames(ctx context.Context) ([]models.Game, error)
	GetGameByID(ctx context.Context, id int64) (*models.Game, error)
	UpdateGame(ctx context.Context, game *models.Game) error
	DeleteGame(ctx context.Context, id int64) error

	CreateEvent(ctx context.Context, event *models.Event) error
	AppendEvents(ctx context.Context, gameID int64, events []models.Event) error
	ListEvents(ctx context.Context) ([]models.Event, error)
	GetEventByID(ctx context.Context, id int64) (*models.Event, error)
	ListEventsByGameID(ctx context.Context, gameID int64) ([]models.Event, error)

	Close() error
}

type Postgres struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &Postgres{db: db}, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}

func (p *Postgres) CreateUser(ctx context.Context, user *models.User) error {
	if user.Name == "" {
		return errors.New("name is required")
	}

	query := `
		INSERT INTO users (name)
		VALUES ($1)
		RETURNING id, created_at
	`

	return p.db.QueryRowContext(ctx, query, user.Name).Scan(&user.ID, &user.CreatedAt)
}

func (p *Postgres) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name, created_at
		FROM users
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if users == nil {
		users = []models.User{}
	}

	return users, rows.Err()
}

func (p *Postgres) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User

	query := `
		SELECT id, name, created_at
		FROM users
		WHERE id = $1
	`

	err := p.db.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Name, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (p *Postgres) UpdateUser(ctx context.Context, user *models.User) error {
	if user.ID <= 0 {
		return errors.New("id is required")
	}
	if user.Name == "" {
		return errors.New("name is required")
	}

	query := `
		UPDATE users
		SET name = $1
		WHERE id = $2
		RETURNING created_at
	`

	err := p.db.QueryRowContext(ctx, query, user.Name, user.ID).Scan(&user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}

	return nil
}

func (p *Postgres) DeleteUser(ctx context.Context, id int64) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (p *Postgres) CreateGame(ctx context.Context, game *models.Game) error {
	if game.Title == "" {
		return errors.New("title is required")
	}

	query := `
		INSERT INTO games (title)
		VALUES ($1)
		RETURNING id, created_at
	`

	return p.db.QueryRowContext(ctx, query, game.Title).Scan(&game.ID, &game.CreatedAt)
}

func (p *Postgres) CreateGameWithEvents(ctx context.Context, game *models.Game, events []models.Event) error {
	if game.Title == "" {
		return errors.New("title is required")
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = tx.QueryRowContext(ctx, `
		INSERT INTO games (title)
		VALUES ($1)
		RETURNING id, created_at
	`, game.Title).Scan(&game.ID, &game.CreatedAt); err != nil {
		return err
	}

	for i := range events {
		events[i].GameID = game.ID
		if err = tx.QueryRowContext(ctx, `
			INSERT INTO events (game_id, user_id, actor_name, event_type, event_value)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at
		`, events[i].GameID, events[i].UserID, events[i].ActorName, events[i].EventType, events[i].EventValue).
			Scan(&events[i].ID, &events[i].CreatedAt); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func (p *Postgres) ListGames(ctx context.Context) ([]models.Game, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, title, created_at
		FROM games
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var game models.Game
		if err := rows.Scan(&game.ID, &game.Title, &game.CreatedAt); err != nil {
			return nil, err
		}
		games = append(games, game)
	}

	if games == nil {
		games = []models.Game{}
	}

	return games, rows.Err()
}

func (p *Postgres) GetGameByID(ctx context.Context, id int64) (*models.Game, error) {
	var game models.Game

	query := `
		SELECT id, title, created_at
		FROM games
		WHERE id = $1
	`

	err := p.db.QueryRowContext(ctx, query, id).Scan(&game.ID, &game.Title, &game.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("game not found")
		}
		return nil, err
	}

	return &game, nil
}

func (p *Postgres) UpdateGame(ctx context.Context, game *models.Game) error {
	if game.ID <= 0 {
		return errors.New("id is required")
	}
	if game.Title == "" {
		return errors.New("title is required")
	}

	query := `
		UPDATE games
		SET title = $1
		WHERE id = $2
		RETURNING created_at
	`

	err := p.db.QueryRowContext(ctx, query, game.Title, game.ID).Scan(&game.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("game not found")
		}
		return err
	}

	return nil
}

func (p *Postgres) DeleteGame(ctx context.Context, id int64) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM games WHERE id = $1`, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("game not found")
	}

	return nil
}

func (p *Postgres) CreateEvent(ctx context.Context, event *models.Event) error {
	if event.GameID <= 0 {
		return errors.New("game_id is required")
	}
	if event.EventType == "" {
		return errors.New("event_type is required")
	}

	query := `
		INSERT INTO events (game_id, user_id, actor_name, event_type, event_value)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	return p.db.QueryRowContext(
		ctx,
		query,
		event.GameID,
		event.UserID,
		event.ActorName,
		event.EventType,
		event.EventValue,
	).Scan(&event.ID, &event.CreatedAt)
}

func (p *Postgres) AppendEvents(ctx context.Context, gameID int64, events []models.Event) error {
	if gameID <= 0 {
		return errors.New("game_id is required")
	}
	if len(events) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var lockedGameID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM games WHERE id = $1 FOR UPDATE`, gameID).Scan(&lockedGameID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("game not found")
		}
		return err
	}

	for i := range events {
		events[i].GameID = gameID
		if err = tx.QueryRowContext(ctx, `
			INSERT INTO events (game_id, user_id, actor_name, event_type, event_value)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at
		`, events[i].GameID, events[i].UserID, events[i].ActorName, events[i].EventType, events[i].EventValue).
			Scan(&events[i].ID, &events[i].CreatedAt); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func (p *Postgres) ListEvents(ctx context.Context) ([]models.Event, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, game_id, user_id, actor_name, event_type, event_value, created_at
		FROM events
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(
			&event.ID,
			&event.GameID,
			&event.UserID,
			&event.ActorName,
			&event.EventType,
			&event.EventValue,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if events == nil {
		events = []models.Event{}
	}

	return events, rows.Err()
}

func (p *Postgres) GetEventByID(ctx context.Context, id int64) (*models.Event, error) {
	var event models.Event

	query := `
		SELECT id, game_id, user_id, actor_name, event_type, event_value, created_at
		FROM events
		WHERE id = $1
	`

	err := p.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.GameID,
		&event.UserID,
		&event.ActorName,
		&event.EventType,
		&event.EventValue,
		&event.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, err
	}

	return &event, nil
}

func (p *Postgres) ListEventsByGameID(ctx context.Context, gameID int64) ([]models.Event, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, game_id, user_id, actor_name, event_type, event_value, created_at
		FROM events
		WHERE game_id = $1
		ORDER BY created_at, id
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(
			&event.ID,
			&event.GameID,
			&event.UserID,
			&event.ActorName,
			&event.EventType,
			&event.EventValue,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if events == nil {
		events = []models.Event{}
	}

	return events, rows.Err()
}

// RunMigrations выполняет все .sql файлы из указанной директории в одной транзакции.
func RunMigrations(ctx context.Context, p *Postgres, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		if _, execErr := tx.ExecContext(ctx, string(content)); execErr != nil {
			return fmt.Errorf("exec %s: %w", path, execErr)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}
