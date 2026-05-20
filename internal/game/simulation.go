package game

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"agentbackend/internal/models"
)

const (
	MaxBotSimulationGames           = 1000
	MaxBotSimulationMemorandumCount = 50
	maxSimulationSteps              = 5000
)

type BotSimulationMemorandumType string

const (
	BotSimulationMemorandumTypeMixed       BotSimulationMemorandumType = "mixed"
	BotSimulationMemorandumTypeOpportunity BotSimulationMemorandumType = "opportunity"
	BotSimulationMemorandumTypeRisk        BotSimulationMemorandumType = "risk"
)

type BotSimulationRequest struct {
	Games              int                         `json:"games"`
	Players            int                         `json:"players"`
	Seed               *int64                      `json:"seed,omitempty"`
	IncludeGames       bool                        `json:"include_games"`
	BotMemorandumCount int                         `json:"bot_memorandum_count,omitempty"`
	BotMemorandumType  BotSimulationMemorandumType `json:"bot_memorandum_type,omitempty"`
}

type BotSimulationResponse struct {
	Games                        int                         `json:"games"`
	Players                      int                         `json:"players"`
	Seed                         int64                       `json:"seed"`
	BotMemorandumCount           int                         `json:"bot_memorandum_count"`
	BotMemorandumType            BotSimulationMemorandumType `json:"bot_memorandum_type"`
	MoleWins                     int                         `json:"mole_wins"`
	PlayersWins                  int                         `json:"players_wins"`
	MoleWinrate                  float64                     `json:"mole_winrate"`
	PlayersWinrate               float64                     `json:"players_winrate"`
	AverageRounds                float64                     `json:"average_rounds"`
	AcceptedCleanCount           int                         `json:"accepted_clean_count"`
	AcceptedTargetCount          int                         `json:"accepted_target_count"`
	AcceptedSabotageCount        int                         `json:"accepted_sabotage_count"`
	AverageAcceptedCleanCount    float64                     `json:"average_accepted_clean_count"`
	AverageAcceptedTargetCount   float64                     `json:"average_accepted_target_count"`
	AverageAcceptedSabotageCount float64                     `json:"average_accepted_sabotage_count"`
	Results                      []BotSimulationGameResult   `json:"results,omitempty"`
}

type BotSimulationGameResult struct {
	Index                 int      `json:"index"`
	Winner                string   `json:"winner"`
	Rounds                int      `json:"rounds"`
	MoleUserID            int64    `json:"mole_user_id"`
	MolePoints            int      `json:"mole_points"`
	PlayersPoints         int      `json:"players_points"`
	AcceptedCleanCount    int      `json:"accepted_clean_count"`
	AcceptedTargetCount   int      `json:"accepted_target_count"`
	AcceptedSabotageCount int      `json:"accepted_sabotage_count"`
	AcceptedDecisions     []string `json:"accepted_decisions,omitempty"`
}

func SimulateBotGames(request BotSimulationRequest) (BotSimulationResponse, error) {
	games := request.Games
	if games == 0 {
		games = 1
	}
	if games < 0 || games > MaxBotSimulationGames {
		return BotSimulationResponse{}, fmt.Errorf("games must be between 1 and %d", MaxBotSimulationGames)
	}
	players := request.Players
	if players == 0 {
		players = 6
	}
	if players < MinPlayers || players > MaxPlayers {
		return BotSimulationResponse{}, fmt.Errorf("players must be between %d and %d", MinPlayers, MaxPlayers)
	}
	botMemorandumCount := request.BotMemorandumCount
	if botMemorandumCount == 0 {
		botMemorandumCount = 1
	}
	if botMemorandumCount < 0 || botMemorandumCount > MaxBotSimulationMemorandumCount {
		return BotSimulationResponse{}, fmt.Errorf("bot_memorandum_count must be between 1 and %d", MaxBotSimulationMemorandumCount)
	}
	botMemorandumType := request.BotMemorandumType
	if botMemorandumType == "" {
		botMemorandumType = BotSimulationMemorandumTypeMixed
	}
	if !isBotSimulationMemorandumType(botMemorandumType) {
		return BotSimulationResponse{}, errors.New("bot_memorandum_type must be one of mixed, opportunity, risk")
	}

	seed := time.Now().UTC().UnixNano()
	if request.Seed != nil {
		seed = *request.Seed
	}
	engine := &Engine{
		rng:                          rand.New(rand.NewSource(seed)),
		botSimulationMemorandumCount: botMemorandumCount,
		botSimulationMemorandumType:  botMemorandumType,
		botSimulationMemorandums:     map[int64][]MemorandumState{},
	}
	response := BotSimulationResponse{
		Games:              games,
		Players:            players,
		Seed:               seed,
		BotMemorandumCount: botMemorandumCount,
		BotMemorandumType:  botMemorandumType,
	}
	if request.IncludeGames {
		response.Results = make([]BotSimulationGameResult, 0, games)
	}

	for i := 1; i <= games; i++ {
		result, err := engine.simulateBotGame(i, players)
		if err != nil {
			return BotSimulationResponse{}, err
		}
		if result.Winner == "mole" {
			response.MoleWins++
		} else if result.Winner == "players" {
			response.PlayersWins++
		}
		response.AverageRounds += float64(result.Rounds)
		response.AcceptedCleanCount += result.AcceptedCleanCount
		response.AcceptedTargetCount += result.AcceptedTargetCount
		response.AcceptedSabotageCount += result.AcceptedSabotageCount
		if request.IncludeGames {
			response.Results = append(response.Results, result)
		}
	}

	if games > 0 {
		response.MoleWinrate = float64(response.MoleWins) / float64(games)
		response.PlayersWinrate = float64(response.PlayersWins) / float64(games)
		response.AverageRounds /= float64(games)
		response.AverageAcceptedCleanCount = float64(response.AcceptedCleanCount) / float64(games)
		response.AverageAcceptedTargetCount = float64(response.AcceptedTargetCount) / float64(games)
		response.AverageAcceptedSabotageCount = float64(response.AcceptedSabotageCount) / float64(games)
	}

	return response, nil
}

func (e *Engine) simulateBotGame(index int, players int) (BotSimulationGameResult, error) {
	now := time.Unix(1700000000, 0).UTC().Add(time.Duration(index) * time.Hour)
	state, err := e.newBotSimulationState(index, players, now)
	if err != nil {
		return BotSimulationGameResult{}, err
	}

	host := botActor(state.Players[-1])
	startEvents, err := e.handleStartGame(state, host)
	if err != nil {
		return BotSimulationGameResult{}, err
	}
	now = now.Add(time.Second)
	if err := e.applySimulationEvents(state, startEvents, now); err != nil {
		return BotSimulationGameResult{}, err
	}

	for step := 0; step < maxSimulationSteps && !state.IsFinished; step++ {
		now = nextSimulationActionTime(state, now)
		events, err := e.nextBotTurnEvents(state, now)
		if err != nil {
			return BotSimulationGameResult{}, err
		}
		if len(events) == 0 {
			return BotSimulationGameResult{}, errors.New("bot simulation stalled")
		}
		if err := e.applySimulationEvents(state, events, now); err != nil {
			return BotSimulationGameResult{}, err
		}
		now = now.Add(time.Millisecond)
	}
	if !state.IsFinished {
		return BotSimulationGameResult{}, errors.New("bot simulation exceeded step limit")
	}

	clean, target, sabotage := acceptedDecisionCounts(state)
	molePoints, playersPoints := victoryPoints(state)
	return BotSimulationGameResult{
		Index:                 index,
		Winner:                state.Winner,
		Rounds:                state.CurrentRound,
		MoleUserID:            state.MoleUserID,
		MolePoints:            molePoints,
		PlayersPoints:         playersPoints,
		AcceptedCleanCount:    clean,
		AcceptedTargetCount:   target,
		AcceptedSabotageCount: sabotage,
		AcceptedDecisions:     append([]string(nil), state.AcceptedOrder...),
	}, nil
}

func (e *Engine) newBotSimulationState(index int, players int, now time.Time) (*GameState, error) {
	events := []models.Event{{
		ID:        1,
		CreatedAt: now,
		EventType: models.EventGameCreated,
		EventValue: mustJSON(GameCreatedPayload{
			HostUserID:       -1,
			Title:            fmt.Sprintf("Bot Simulation %d", index),
			CompanyName:      "Bot Simulation",
			CompanySituation: "Automated in-memory balance simulation.",
		}),
	}}
	usedNames := map[string]bool{}
	for i := 0; i < players; i++ {
		userID := -int64(i + 1)
		name := e.randomBotName(usedNames, userID)
		usedNames[strings.ToLower(name)] = true
		events = append(events, models.Event{
			ID:        int64(len(events) + 1),
			CreatedAt: now,
			EventType: models.EventPlayerJoined,
			EventValue: mustJSON(PlayerJoinedPayload{
				UserID:   userID,
				Name:     name,
				Position: e.randomGeneratedPosition(),
				IsBot:    true,
			}),
		})
	}
	return BuildState(int64(index), fmt.Sprintf("Bot Simulation %d", index), events)
}

func (e *Engine) applySimulationEvents(state *GameState, events []models.Event, now time.Time) error {
	for i := range events {
		if events[i].CreatedAt.IsZero() {
			events[i].CreatedAt = now
		}
		if err := ApplyEvent(state, events[i]); err != nil {
			return err
		}
	}
	return e.rememberBotSimulationMemorandums(state, events)
}

func (e *Engine) rememberBotSimulationMemorandums(state *GameState, events []models.Event) error {
	if e.botSimulationMemorandumCount <= 0 {
		return nil
	}
	if e.botSimulationMemorandumCount <= 1 && (e.botSimulationMemorandumType == "" || e.botSimulationMemorandumType == BotSimulationMemorandumTypeMixed) {
		return nil
	}
	if e.botSimulationMemorandums == nil {
		e.botSimulationMemorandums = map[int64][]MemorandumState{}
	}

	for _, event := range events {
		if event.EventType != models.EventMemorandumAssigned {
			continue
		}
		var payload MemorandumAssignedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		player := activePlayerByID(state, payload.UserID)
		if player == nil || !player.IsBot || player.Role == "mole" {
			continue
		}

		memorandums := make([]MemorandumState, 0, e.botSimulationMemorandumCount)
		for len(memorandums) < e.botSimulationMemorandumCount {
			index := len(memorandums)
			memorandumType := e.botSimulationMemorandumTypeAt(payload.Type, index)
			decisions := e.randomMemorandumDecisions(memorandumType, state.MoleTargets, state.MoleSabotage)
			if index == 0 && memorandumType == payload.Type {
				decisions = append([]string(nil), payload.Decisions...)
			}
			memorandums = append(memorandums, MemorandumState{
				UserID:    payload.UserID,
				Type:      memorandumType,
				Decisions: decisions,
			})
		}
		e.botSimulationMemorandums[payload.UserID] = memorandums
	}
	return nil
}

func isBotSimulationMemorandumType(value BotSimulationMemorandumType) bool {
	return value == BotSimulationMemorandumTypeMixed ||
		value == BotSimulationMemorandumTypeOpportunity ||
		value == BotSimulationMemorandumTypeRisk
}

func (e *Engine) botSimulationMemorandumTypeAt(defaultType MemorandumType, index int) MemorandumType {
	switch e.botSimulationMemorandumType {
	case BotSimulationMemorandumTypeOpportunity:
		return MemorandumTypeOpportunity
	case BotSimulationMemorandumTypeRisk:
		return MemorandumTypeRisk
	default:
		return alternatingMemorandumType(defaultType, index)
	}
}

func alternatingMemorandumType(first MemorandumType, index int) MemorandumType {
	if index%2 == 0 {
		return first
	}
	if first == MemorandumTypeRisk {
		return MemorandumTypeOpportunity
	}
	return MemorandumTypeRisk
}

func nextSimulationActionTime(state *GameState, now time.Time) time.Time {
	if state.MajorVoteUnlockedAt != nil && now.Before(*state.MajorVoteUnlockedAt) {
		now = state.MajorVoteUnlockedAt.Add(time.Millisecond)
	}
	if state.PhaseStartedAt != nil {
		readyAt := state.PhaseStartedAt.Add(BotActionDelay)
		if now.Before(readyAt) {
			now = readyAt
		}
	}
	return now
}

func acceptedDecisionCounts(state *GameState) (clean int, target int, sabotage int) {
	targets := stringSet(state.MoleTargets)
	for _, decision := range state.AcceptedOrder {
		switch {
		case decision == state.MoleSabotage:
			sabotage++
		case targets[decision]:
			target++
		default:
			clean++
		}
	}
	return clean, target, sabotage
}
