package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"agentbackend/internal/models"
)

type Store interface {
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	GetGameByID(ctx context.Context, id int64) (*models.Game, error)
	ListEventsByGameID(ctx context.Context, gameID int64) ([]models.Event, error)
	CreateGameWithEvents(ctx context.Context, game *models.Game, events []models.Event) error
	AppendEvents(ctx context.Context, gameID int64, events []models.Event) error
}

type Engine struct {
	store Store
	rng   *rand.Rand
	mu    sync.Mutex
	rngMu sync.Mutex
}

func NewEngine(store Store) *Engine {
	return &Engine{
		store: store,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (e *Engine) CreateGame(ctx context.Context, title string, hostUserID int64) (*models.Game, *PublicGameState, []models.Event, error) {
	if title == "" {
		return nil, nil, nil, errors.New("title is required")
	}
	if hostUserID <= 0 {
		return nil, nil, nil, errors.New("host_user_id is required")
	}

	host, err := e.store.GetUserByID(ctx, hostUserID)
	if err != nil {
		return nil, nil, nil, err
	}

	gameModel := &models.Game{Title: title}
	gameCreatedPayload := mustJSON(GameCreatedPayload{
		HostUserID: hostUserID,
		Title:      title,
	})
	playerJoinedPayload := mustJSON(PlayerJoinedPayload{
		UserID: host.ID,
		Name:   host.Name,
	})

	events := []models.Event{
		{
			UserID:     &host.ID,
			ActorName:  host.Name,
			EventType:  models.EventGameCreated,
			EventValue: gameCreatedPayload,
		},
		{
			UserID:     &host.ID,
			ActorName:  host.Name,
			EventType:  models.EventPlayerJoined,
			EventValue: playerJoinedPayload,
		},
	}

	if err := e.store.CreateGameWithEvents(ctx, gameModel, events); err != nil {
		return nil, nil, nil, err
	}

	state, err := BuildState(gameModel.ID, title, events)
	if err != nil {
		return nil, nil, nil, err
	}

	publicState, err := ProjectStateForViewer(state, hostUserID)
	if err != nil {
		return nil, nil, nil, err
	}

	return gameModel, publicState, events, nil
}

func (e *Engine) HandleAction(ctx context.Context, gameID int64, action Action) (*PublicGameState, []models.Event, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := e.store.GetGameByID(ctx, gameID); err != nil {
		return nil, nil, err
	}

	actor, err := e.store.GetUserByID(ctx, action.UserID)
	if err != nil {
		return nil, nil, err
	}

	events, err := e.store.ListEventsByGameID(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}

	gameModel, err := e.store.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}

	state, err := BuildState(gameID, gameModel.Title, events)
	if err != nil {
		return nil, nil, err
	}

	newEvents, err := e.decideEvents(state, actor, action)
	if err != nil {
		return nil, nil, err
	}

	if len(newEvents) == 0 {
		publicState, projErr := ProjectStateForViewer(state, action.UserID)
		return publicState, nil, projErr
	}

	if err := e.store.AppendEvents(ctx, gameID, newEvents); err != nil {
		return nil, nil, err
	}

	allEvents := append(append([]models.Event{}, events...), newEvents...)
	newState, err := BuildState(gameID, gameModel.Title, allEvents)
	if err != nil {
		return nil, nil, err
	}

	publicState, err := ProjectStateForViewer(newState, action.UserID)
	if err != nil {
		return nil, nil, err
	}

	return publicState, newEvents, nil
}

func (e *Engine) decideEvents(state *GameState, actor *models.User, action Action) ([]models.Event, error) {
	switch action.Type {
	case ActionJoinGame:
		return e.handleJoinGame(state, actor)
	case ActionKickPlayer:
		return e.handleKickPlayer(state, actor, action.Payload)
	case ActionStartGame:
		return e.handleStartGame(state, actor)
	case ActionVote:
		return e.handleVote(state, actor, action.Payload)
	case ActionSubmitGovernanceProposal:
		return e.handleSubmitGovernanceProposal(state, actor, action.Payload)
	case ActionSkipGovernanceProposal:
		return e.handleSkipGovernanceProposal(state, actor)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (e *Engine) shuffleWithRNG(n int, swap func(i, j int)) {
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	e.rng.Shuffle(n, swap)
}
