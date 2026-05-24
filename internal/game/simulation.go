package game

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"agentbackend/internal/models"
)

const (
	MaxBotSimulationGames                  = 1000
	MaxBotSimulationMemorandumCount        = 50
	MaxBotSimulationWorkers                = 64
	DefaultBotSimulationMaxWorkers         = 8
	MaxBotSimulationMonteCarloRollouts     = 512
	DefaultBotSimulationMonteCarloRollouts = 32
	maxSimulationSteps                     = 5000
)

type BotSimulationMemorandumType string
type BotSimulationMemorandumVariant string

const (
	BotSimulationMemorandumTypeMixed       BotSimulationMemorandumType = "mixed"
	BotSimulationMemorandumTypeOpportunity BotSimulationMemorandumType = "opportunity"
	BotSimulationMemorandumTypeRisk        BotSimulationMemorandumType = "risk"
)

const (
	BotSimulationMemorandumVariantMixed    BotSimulationMemorandumVariant = "mixed"
	BotSimulationMemorandumVariantStandard BotSimulationMemorandumVariant = "standard"
	BotSimulationMemorandumVariantAdvanced BotSimulationMemorandumVariant = "advanced"
)

type BotSimulationRequest struct {
	Games                int                            `json:"games"`
	Players              int                            `json:"players"`
	Seed                 *int64                         `json:"seed,omitempty"`
	IncludeGames         bool                           `json:"include_games"`
	BotMemorandumCount   int                            `json:"bot_memorandum_count,omitempty"`
	BotMemorandumType    BotSimulationMemorandumType    `json:"bot_memorandum_type,omitempty"`
	BotMemorandumVariant BotSimulationMemorandumVariant `json:"bot_memorandum_variant,omitempty"`
	Variant              BotSimulationMemorandumVariant `json:"variant,omitempty"`
	Workers              int                            `json:"workers,omitempty"`
	MonteCarloRollouts   int                            `json:"monte_carlo_rollouts,omitempty"`
}

type BotSimulationResponse struct {
	Games                           int                            `json:"games"`
	Players                         int                            `json:"players"`
	Seed                            int64                          `json:"seed"`
	BotMemorandumCount              int                            `json:"bot_memorandum_count"`
	BotMemorandumType               BotSimulationMemorandumType    `json:"bot_memorandum_type"`
	BotMemorandumVariant            BotSimulationMemorandumVariant `json:"bot_memorandum_variant"`
	Workers                         int                            `json:"workers"`
	MonteCarloRollouts              int                            `json:"monte_carlo_rollouts"`
	DurationMS                      int64                          `json:"duration_ms"`
	GamesPerSecond                  float64                        `json:"games_per_second"`
	MoleWins                        int                            `json:"mole_wins"`
	PlayersWins                     int                            `json:"players_wins"`
	MoleWinrate                     float64                        `json:"mole_winrate"`
	PlayersWinrate                  float64                        `json:"players_winrate"`
	AverageRounds                   float64                        `json:"average_rounds"`
	AcceptedCleanCount              int                            `json:"accepted_clean_count"`
	AcceptedTargetCount             int                            `json:"accepted_target_count"`
	AcceptedSabotageCount           int                            `json:"accepted_sabotage_count"`
	ComplianceCatchesCount          int                            `json:"compliance_catches_count"`
	PlayersWinsByComplianceCount    int                            `json:"players_wins_by_compliance_count"`
	AverageAcceptedCleanCount       float64                        `json:"average_accepted_clean_count"`
	AverageAcceptedTargetCount      float64                        `json:"average_accepted_target_count"`
	AverageAcceptedSabotageCount    float64                        `json:"average_accepted_sabotage_count"`
	AverageComplianceCatchesPerGame float64                        `json:"average_compliance_catches_per_game"`
	MostCommonScenario              string                         `json:"most_common_scenario,omitempty"`
	ScenarioStats                   BotSimulationScenarioStats     `json:"scenario_stats"`
	Results                         []BotSimulationGameResult      `json:"results,omitempty"`
}

type BotSimulationGameResult struct {
	Index                 int                            `json:"index"`
	Winner                string                         `json:"winner"`
	WinnerReason          string                         `json:"winner_reason,omitempty"`
	Rounds                int                            `json:"rounds"`
	MoleUserID            int64                          `json:"mole_user_id"`
	ComplianceUserID      int64                          `json:"compliance_user_id,omitempty"`
	ComplianceCaught      bool                           `json:"compliance_caught"`
	ComplianceWatches     []BotSimulationComplianceWatch `json:"compliance_watches,omitempty"`
	MolePoints            int                            `json:"mole_points"`
	PlayersPoints         int                            `json:"players_points"`
	AcceptedCleanCount    int                            `json:"accepted_clean_count"`
	AcceptedTargetCount   int                            `json:"accepted_target_count"`
	AcceptedSabotageCount int                            `json:"accepted_sabotage_count"`
	AcceptedDecisions     []string                       `json:"accepted_decisions,omitempty"`
	Scenario              string                         `json:"scenario"`
}

type BotSimulationScenarioStats struct {
	FirstPodkopNoSabotageCount  int     `json:"first_podkop_no_sabotage_count"`
	FirstPodkopThenSabotage     int     `json:"first_podkop_then_sabotage_count"`
	FirstDecisionSabotage       int     `json:"first_decision_sabotage_count"`
	PlayersWinCount             int     `json:"players_win_count"`
	MoleWinUnclassifiedCount    int     `json:"mole_win_unclassified_count"`
	FirstPodkopNoSabotageRate   float64 `json:"first_podkop_no_sabotage_rate"`
	FirstPodkopThenSabotageRate float64 `json:"first_podkop_then_sabotage_rate"`
	FirstDecisionSabotageRate   float64 `json:"first_decision_sabotage_rate"`
	PlayersWinRate              float64 `json:"players_win_rate"`
	MoleWinUnclassifiedRate     float64 `json:"mole_win_unclassified_rate"`
}

type BotSimulationComplianceWatch struct {
	RoundNumber      int   `json:"round_number"`
	ComplianceUserID int64 `json:"compliance_user_id"`
	TargetUserID     int64 `json:"target_user_id"`
}

type botSimulationConfig struct {
	Games                int
	Players              int
	Seed                 int64
	IncludeGames         bool
	BotMemorandumCount   int
	BotMemorandumType    BotSimulationMemorandumType
	BotMemorandumVariant BotSimulationMemorandumVariant
	Workers              int
	MonteCarloRollouts   int
}

func SimulateBotGames(request BotSimulationRequest) (BotSimulationResponse, error) {
	start := time.Now()
	config, err := normalizeBotSimulationRequest(request)
	if err != nil {
		return BotSimulationResponse{}, err
	}

	results := make([]BotSimulationGameResult, config.Games)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for worker := 0; worker < config.Workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				engine := newBotSimulationEngine(config, index)
				result, err := engine.simulateBotGame(index, config.Players)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				results[index-1] = result
			}
		}()
	}

	for i := 1; i <= config.Games; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	errMu.Lock()
	err = firstErr
	errMu.Unlock()
	if err != nil {
		return BotSimulationResponse{}, err
	}

	response := BotSimulationResponse{
		Games:                config.Games,
		Players:              config.Players,
		Seed:                 config.Seed,
		BotMemorandumCount:   config.BotMemorandumCount,
		BotMemorandumType:    config.BotMemorandumType,
		BotMemorandumVariant: config.BotMemorandumVariant,
		Workers:              config.Workers,
		MonteCarloRollouts:   config.MonteCarloRollouts,
	}
	if config.IncludeGames {
		response.Results = make([]BotSimulationGameResult, 0, config.Games)
	}

	for _, result := range results {
		if result.Winner == "mole" {
			response.MoleWins++
		} else if result.Winner == "players" {
			response.PlayersWins++
		}
		response.AverageRounds += float64(result.Rounds)
		response.AcceptedCleanCount += result.AcceptedCleanCount
		response.AcceptedTargetCount += result.AcceptedTargetCount
		response.AcceptedSabotageCount += result.AcceptedSabotageCount
		if result.ComplianceCaught {
			response.ComplianceCatchesCount++
		}
		if result.WinnerReason == WinnerReasonMoleCaughtByCompliance {
			response.PlayersWinsByComplianceCount++
		}
		if config.IncludeGames {
			response.Results = append(response.Results, result)
		}
		switch result.Scenario {
		case "first_podkop_no_sabotage":
			response.ScenarioStats.FirstPodkopNoSabotageCount++
		case "first_podkop_then_sabotage":
			response.ScenarioStats.FirstPodkopThenSabotage++
		case "first_decision_sabotage":
			response.ScenarioStats.FirstDecisionSabotage++
		case "players_win":
			response.ScenarioStats.PlayersWinCount++
		case "mole_win_unclassified":
			response.ScenarioStats.MoleWinUnclassifiedCount++
		}
	}

	if config.Games > 0 {
		response.MoleWinrate = float64(response.MoleWins) / float64(config.Games)
		response.PlayersWinrate = float64(response.PlayersWins) / float64(config.Games)
		response.AverageRounds /= float64(config.Games)
		response.AverageAcceptedCleanCount = float64(response.AcceptedCleanCount) / float64(config.Games)
		response.AverageAcceptedTargetCount = float64(response.AcceptedTargetCount) / float64(config.Games)
		response.AverageAcceptedSabotageCount = float64(response.AcceptedSabotageCount) / float64(config.Games)
		response.AverageComplianceCatchesPerGame = float64(response.ComplianceCatchesCount) / float64(config.Games)
		response.ScenarioStats.FirstPodkopNoSabotageRate = float64(response.ScenarioStats.FirstPodkopNoSabotageCount) / float64(config.Games)
		response.ScenarioStats.FirstPodkopThenSabotageRate = float64(response.ScenarioStats.FirstPodkopThenSabotage) / float64(config.Games)
		response.ScenarioStats.FirstDecisionSabotageRate = float64(response.ScenarioStats.FirstDecisionSabotage) / float64(config.Games)
		response.ScenarioStats.PlayersWinRate = float64(response.ScenarioStats.PlayersWinCount) / float64(config.Games)
		response.ScenarioStats.MoleWinUnclassifiedRate = float64(response.ScenarioStats.MoleWinUnclassifiedCount) / float64(config.Games)
		response.MostCommonScenario = mostCommonSimulationScenario(response.ScenarioStats)
	}
	response.DurationMS = time.Since(start).Milliseconds()
	if elapsed := time.Since(start).Seconds(); elapsed > 0 {
		response.GamesPerSecond = float64(config.Games) / elapsed
	}

	return response, nil
}

func normalizeBotSimulationRequest(request BotSimulationRequest) (botSimulationConfig, error) {
	games := request.Games
	if games == 0 {
		games = 1
	}
	if games < 0 || games > MaxBotSimulationGames {
		return botSimulationConfig{}, fmt.Errorf("games must be between 1 and %d", MaxBotSimulationGames)
	}
	players := request.Players
	if players == 0 {
		players = 6
	}
	if players < MinPlayers || players > MaxPlayers {
		return botSimulationConfig{}, fmt.Errorf("players must be between %d and %d", MinPlayers, MaxPlayers)
	}
	botMemorandumCount := request.BotMemorandumCount
	if botMemorandumCount == 0 {
		botMemorandumCount = 1
	}
	if botMemorandumCount < 0 || botMemorandumCount > MaxBotSimulationMemorandumCount {
		return botSimulationConfig{}, fmt.Errorf("bot_memorandum_count must be between 1 and %d", MaxBotSimulationMemorandumCount)
	}
	botMemorandumType := request.BotMemorandumType
	if botMemorandumType == "" {
		botMemorandumType = BotSimulationMemorandumTypeMixed
	}
	if !isBotSimulationMemorandumType(botMemorandumType) {
		return botSimulationConfig{}, errors.New("bot_memorandum_type must be one of mixed, opportunity, risk")
	}
	botMemorandumVariant := request.BotMemorandumVariant
	if botMemorandumVariant == "" {
		botMemorandumVariant = request.Variant
	}
	if botMemorandumVariant == "" {
		if botMemorandumType == BotSimulationMemorandumTypeMixed {
			botMemorandumVariant = BotSimulationMemorandumVariantMixed
		} else {
			botMemorandumVariant = BotSimulationMemorandumVariantAdvanced
		}
	}
	if !isBotSimulationMemorandumVariant(botMemorandumVariant) {
		return botSimulationConfig{}, errors.New("bot_memorandum_variant/variant must be one of mixed, standard, advanced")
	}
	workers := request.Workers
	if workers == 0 {
		workers = minInt(minInt(runtime.GOMAXPROCS(0), DefaultBotSimulationMaxWorkers), games)
	}
	if workers < 0 || workers > MaxBotSimulationWorkers {
		return botSimulationConfig{}, fmt.Errorf("workers must be between 1 and %d", MaxBotSimulationWorkers)
	}
	if workers == 0 {
		workers = 1
	}
	if workers > games {
		workers = games
	}
	monteCarloRollouts := request.MonteCarloRollouts
	if monteCarloRollouts == 0 {
		monteCarloRollouts = DefaultBotSimulationMonteCarloRollouts
	}
	if monteCarloRollouts < 0 || monteCarloRollouts > MaxBotSimulationMonteCarloRollouts {
		return botSimulationConfig{}, fmt.Errorf("monte_carlo_rollouts must be between 1 and %d", MaxBotSimulationMonteCarloRollouts)
	}

	seed := time.Now().UTC().UnixNano()
	if request.Seed != nil {
		seed = *request.Seed
	}
	return botSimulationConfig{
		Games:                games,
		Players:              players,
		Seed:                 seed,
		IncludeGames:         request.IncludeGames,
		BotMemorandumCount:   botMemorandumCount,
		BotMemorandumType:    botMemorandumType,
		BotMemorandumVariant: botMemorandumVariant,
		Workers:              workers,
		MonteCarloRollouts:   monteCarloRollouts,
	}, nil
}

func newBotSimulationEngine(config botSimulationConfig, index int) *Engine {
	return &Engine{
		rng:                            rand.New(rand.NewSource(botSimulationGameSeed(config.Seed, index))),
		botSimulationMemorandumCount:   config.BotMemorandumCount,
		botSimulationMemorandumType:    config.BotMemorandumType,
		botSimulationMemorandumVariant: config.BotMemorandumVariant,
		botSimulationMemorandums:       map[int64][]MemorandumState{},
		botSimulationRollouts:          config.MonteCarloRollouts,
	}
}

func botSimulationGameSeed(baseSeed int64, index int) int64 {
	x := uint64(baseSeed) + uint64(index)*0x9e3779b97f4a7c15
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int64(x)
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
		WinnerReason:          state.WinnerReason,
		Rounds:                state.CurrentRound,
		MoleUserID:            state.MoleUserID,
		ComplianceUserID:      state.ComplianceUserID,
		ComplianceCaught:      state.ComplianceCatch != nil,
		ComplianceWatches:     botSimulationComplianceWatches(state),
		MolePoints:            molePoints,
		PlayersPoints:         playersPoints,
		AcceptedCleanCount:    clean,
		AcceptedTargetCount:   target,
		AcceptedSabotageCount: sabotage,
		AcceptedDecisions:     append([]string(nil), state.AcceptedOrder...),
		Scenario:              classifySimulationScenario(state),
	}, nil
}

func classifySimulationScenario(state *GameState) string {
	if state == nil {
		return "mole_win_unclassified"
	}
	if state.Winner == "players" {
		return "players_win"
	}
	targets := stringSet(state.MoleTargets)
	firstObjective := ""
	for _, decision := range state.AcceptedOrder {
		if decision == state.MoleSabotage || targets[decision] {
			firstObjective = decision
			break
		}
	}
	if firstObjective == "" {
		return "mole_win_unclassified"
	}
	if firstObjective == state.MoleSabotage {
		return "first_decision_sabotage"
	}
	for _, decision := range state.AcceptedOrder {
		if decision == state.MoleSabotage {
			return "first_podkop_then_sabotage"
		}
	}
	return "first_podkop_no_sabotage"
}

func mostCommonSimulationScenario(stats BotSimulationScenarioStats) string {
	best := "first_podkop_no_sabotage"
	bestCount := stats.FirstPodkopNoSabotageCount
	if stats.FirstPodkopThenSabotage > bestCount {
		best = "first_podkop_then_sabotage"
		bestCount = stats.FirstPodkopThenSabotage
	}
	if stats.FirstDecisionSabotage > bestCount {
		best = "first_decision_sabotage"
		bestCount = stats.FirstDecisionSabotage
	}
	if stats.PlayersWinCount > bestCount {
		best = "players_win"
		bestCount = stats.PlayersWinCount
	}
	if stats.MoleWinUnclassifiedCount > bestCount {
		best = "mole_win_unclassified"
	}
	return best
}

func botSimulationComplianceWatches(state *GameState) []BotSimulationComplianceWatch {
	if state == nil || len(state.ComplianceWatches) == 0 {
		return nil
	}
	rounds := make([]int, 0, len(state.ComplianceWatches))
	for round := range state.ComplianceWatches {
		rounds = append(rounds, round)
	}
	sort.Ints(rounds)
	out := make([]BotSimulationComplianceWatch, 0, len(rounds))
	for _, round := range rounds {
		watch := state.ComplianceWatches[round]
		out = append(out, BotSimulationComplianceWatch{
			RoundNumber:      watch.RoundNumber,
			ComplianceUserID: watch.ComplianceUserID,
			TargetUserID:     watch.TargetUserID,
		})
	}
	return out
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
	if e.botSimulationMemorandumCount <= 1 &&
		(e.botSimulationMemorandumType == "" || e.botSimulationMemorandumType == BotSimulationMemorandumTypeMixed) &&
		e.effectiveBotSimulationMemorandumVariant() == BotSimulationMemorandumVariantMixed {
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
		if player == nil || !player.IsBot || player.Role == RoleMole {
			continue
		}

		memorandums := make([]MemorandumState, 0, e.botSimulationMemorandumCount)
		for len(memorandums) < e.botSimulationMemorandumCount {
			index := len(memorandums)
			memorandumType, variant := e.botSimulationMemorandumAt(payload.Type, payload.Variant, index)
			decisions := e.randomMemorandumDecisionsForVariant(memorandumType, variant, state.MoleTargets, state.MoleSabotage)
			if index == 0 && memorandumType == payload.Type && variant == normalizeMemorandumVariant(payload.Variant) {
				decisions = append([]string(nil), payload.Decisions...)
			}
			memorandums = append(memorandums, MemorandumState{
				UserID:    payload.UserID,
				Type:      memorandumType,
				Variant:   variant,
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

func isBotSimulationMemorandumVariant(value BotSimulationMemorandumVariant) bool {
	return value == BotSimulationMemorandumVariantMixed ||
		value == BotSimulationMemorandumVariantStandard ||
		value == BotSimulationMemorandumVariantAdvanced
}

func (e *Engine) botSimulationMemorandumAt(defaultType MemorandumType, defaultVariant MemorandumVariant, index int) (MemorandumType, MemorandumVariant) {
	memorandumType := defaultType
	switch e.botSimulationMemorandumType {
	case BotSimulationMemorandumTypeOpportunity:
		memorandumType = MemorandumTypeOpportunity
	case BotSimulationMemorandumTypeRisk:
		memorandumType = MemorandumTypeRisk
	case BotSimulationMemorandumTypeMixed, "":
		memorandumType = alternatingMemorandumType(defaultType, index)
	}

	switch e.effectiveBotSimulationMemorandumVariant() {
	case BotSimulationMemorandumVariantStandard:
		return memorandumType, MemorandumVariantStandard
	case BotSimulationMemorandumVariantAdvanced:
		return memorandumType, MemorandumVariantAdvanced
	default:
		if index == 0 {
			return memorandumType, normalizeMemorandumVariant(defaultVariant)
		}
		return memorandumType, MemorandumVariantAdvanced
	}
}

func (e *Engine) effectiveBotSimulationMemorandumVariant() BotSimulationMemorandumVariant {
	if e.botSimulationMemorandumVariant != "" {
		return e.botSimulationMemorandumVariant
	}
	if e.botSimulationMemorandumType == BotSimulationMemorandumTypeOpportunity ||
		e.botSimulationMemorandumType == BotSimulationMemorandumTypeRisk {
		return BotSimulationMemorandumVariantAdvanced
	}
	return BotSimulationMemorandumVariantMixed
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
