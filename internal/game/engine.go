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

	botSimulationMemorandumCount int
	botSimulationMemorandumType  BotSimulationMemorandumType
	botSimulationMemorandums     map[int64][]MemorandumState
	botSimulationRollouts        int
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
	company := e.randomCompanyScenario()
	briefingMessage := fmt.Sprintf(
		"Совет директоров компании %s собирается на внеочередное заседание.\nНа кону — будущее корпорации.\nОдин из присутствующих работает против компании.\n\n%s",
		company.Name,
		company.Situation,
	)
	gameCreatedPayload := mustJSON(GameCreatedPayload{
		HostUserID:       hostUserID,
		Title:            title,
		CompanyName:      company.Name,
		CompanySituation: company.Situation,
	})
	playerJoinedPayload := mustJSON(PlayerJoinedPayload{
		UserID:   host.ID,
		Name:     host.Name,
		Position: host.Position,
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
		{
			ActorName: "Система",
			EventType: models.EventChatMessageSent,
			EventValue: mustJSON(ChatMessageSentPayload{
				UserID:          0,
				Kind:            "system",
				SystemEventType: "company_briefing",
				Title:           "Брифинг компании",
				Summary:         briefingMessage,
				Message:         briefingMessage,
				Details: []string{
					fmt.Sprintf("Компания: %s", company.Name),
					fmt.Sprintf("Ситуация: %s", company.Situation),
				},
				Tone:        "warning",
				Collapsible: true,
			}),
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

type companyScenario struct {
	Name      string
	Situation string
}

var companyScenarios = []companyScenario{
	{Name: "AsterPay", Situation: "Финтех-сервис пережил неудачный релиз платежного ядра, и совет решает, сохранять ли темп экспансии."},
	{Name: "NordClinic", Situation: "Сеть клиник готовится к слиянию, пока регулятор проверяет качество внутренних процессов."},
	{Name: "SkyForge Robotics", Situation: "Производитель автономных дронов получил крупный контракт, но поставщики задерживают критические компоненты."},
	{Name: "PixelHarbor", Situation: "Игровая студия стоит за месяц до глобального запуска, а бюджет маркетинга уже трещит по швам."},
	{Name: "VectorRail", Situation: "Логистическая компания восстанавливает маршруты после сбоя цепочек поставок и давления ключевых клиентов."},
	{Name: "HelioFoods", Situation: "Производитель растительного питания выходит в федеральные сети, но маржинальность новых SKU под вопросом."},
	{Name: "DeepSignal Labs", Situation: "AI-компания получила внимание инвесторов, пока команда спорит о рисках публичной демонстрации продукта."},
	{Name: "TerraVolt Energy", Situation: "Энергетический оператор запускает пилот накопителей, а финансовый департамент требует сократить капитальные расходы."},
}

func (e *Engine) randomCompanyScenario() companyScenario {
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	return companyScenarios[e.rng.Intn(len(companyScenarios))]
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

	now := time.Now().UTC()
	dueEvents, err := e.timeoutEvents(state, now)
	if err != nil {
		return nil, nil, err
	}
	if len(dueEvents) > 0 {
		scrubSyntheticEventUserIDs(dueEvents)
		if err := e.store.AppendEvents(ctx, gameID, dueEvents); err != nil {
			return nil, nil, err
		}
		allEvents := append(append([]models.Event{}, events...), dueEvents...)
		newState, err := BuildState(gameID, gameModel.Title, allEvents)
		if err != nil {
			return nil, nil, err
		}
		publicState, err := ProjectStateForViewer(newState, action.UserID)
		if err != nil {
			return nil, nil, err
		}
		return publicState, dueEvents, nil
	}

	newEvents, err := e.decideEvents(state, actor, action)
	if err != nil {
		return nil, nil, err
	}

	if len(newEvents) > 0 {
		scrubSyntheticEventUserIDs(newEvents)
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

	autoEvents, err := e.automaticEvents(newState, now)
	if err != nil {
		return nil, nil, err
	}
	if len(autoEvents) > 0 {
		scrubSyntheticEventUserIDs(autoEvents)
		if err := e.store.AppendEvents(ctx, gameID, autoEvents); err != nil {
			return nil, nil, err
		}
		allEvents = append(allEvents, autoEvents...)
		newState, err = BuildState(gameID, gameModel.Title, allEvents)
		if err != nil {
			return nil, nil, err
		}
		newEvents = append(newEvents, autoEvents...)
	}

	publicState, err := ProjectStateForViewer(newState, action.UserID)
	if err != nil {
		return nil, nil, err
	}

	return publicState, newEvents, nil
}

func (e *Engine) AdvanceGame(ctx context.Context, gameID int64, now time.Time) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	gameModel, err := e.store.GetGameByID(ctx, gameID)
	if err != nil {
		return false, err
	}
	events, err := e.store.ListEventsByGameID(ctx, gameID)
	if err != nil {
		return false, err
	}
	state, err := BuildState(gameID, gameModel.Title, events)
	if err != nil {
		return false, err
	}
	newEvents, err := e.automaticEvents(state, now.UTC())
	if err != nil {
		return false, err
	}
	if len(newEvents) == 0 {
		return false, nil
	}
	scrubSyntheticEventUserIDs(newEvents)
	if err := e.store.AppendEvents(ctx, gameID, newEvents); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Engine) decideEvents(state *GameState, actor *models.User, action Action) ([]models.Event, error) {
	switch action.Type {
	case ActionJoinGame:
		return e.handleJoinGame(state, actor)
	case ActionLeaveGame:
		return e.handleLeaveGame(state, actor)
	case ActionKickPlayer:
		return e.handleKickPlayer(state, actor, action.Payload)
	case ActionBanPlayer:
		return e.handleBanPlayer(state, actor, action.Payload)
	case ActionAddBot:
		return e.handleAddBot(state, actor, action.Payload)
	case ActionSendChatMessage:
		return e.handleSendChatMessage(state, actor, action.Payload)
	case ActionReactChatMessage:
		return e.handleReactChatMessage(state, actor, action.Payload)
	case ActionStartGame:
		return e.handleStartGame(state, actor)
	case ActionChooseMemorandum:
		return e.handleChooseMemorandum(state, actor, action.Payload)
	case ActionSelectMoleObjectives:
		return e.handleSelectMoleObjectives(state, actor, action.Payload)
	case ActionPlaceComplianceWatch:
		return e.handlePlaceComplianceWatch(state, actor, action.Payload)
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
