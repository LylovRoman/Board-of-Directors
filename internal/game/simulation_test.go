package game

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"agentbackend/internal/models"
)

func TestSimulateBotGamesAcceptsBotMemorandumCount(t *testing.T) {
	seed := int64(3033)
	response, err := SimulateBotGames(BotSimulationRequest{
		Games:              1,
		Players:            3,
		Seed:               &seed,
		IncludeGames:       true,
		BotMemorandumCount: 7,
		BotMemorandumType:  BotSimulationMemorandumTypeRisk,
		Workers:            1,
		MonteCarloRollouts: 5,
	})
	if err != nil {
		t.Fatalf("SimulateBotGames: %v", err)
	}
	if response.Games != 1 || response.Players != 3 || response.Seed != seed || response.BotMemorandumCount != 7 || response.BotMemorandumType != BotSimulationMemorandumTypeRisk || response.BotMemorandumVariant != BotSimulationMemorandumVariantAdvanced || response.Workers != 1 || response.MonteCarloRollouts != 5 {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
	if response.DurationMS < 0 || response.GamesPerSecond <= 0 {
		t.Fatalf("expected timing metadata, got duration=%d gps=%f", response.DurationMS, response.GamesPerSecond)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected included game result, got %+v", response.Results)
	}
	if response.Results[0].ComplianceUserID == 0 {
		t.Fatalf("expected included game to reveal compliance user, got %+v", response.Results[0])
	}
}

func TestSimulateBotGamesAcceptsSingleForcedAdvancedMemorandumType(t *testing.T) {
	seed := int64(12350)
	response, err := SimulateBotGames(BotSimulationRequest{
		Games:              1,
		Players:            6,
		Seed:               &seed,
		IncludeGames:       false,
		BotMemorandumCount: 1,
		BotMemorandumType:  BotSimulationMemorandumTypeRisk,
		Workers:            1,
		MonteCarloRollouts: 4,
	})
	if err != nil {
		t.Fatalf("SimulateBotGames: %v", err)
	}
	if response.Games != 1 || response.Players != 6 || response.Seed != seed || response.BotMemorandumCount != 1 || response.BotMemorandumType != BotSimulationMemorandumTypeRisk || response.BotMemorandumVariant != BotSimulationMemorandumVariantAdvanced {
		t.Fatalf("unexpected simulation response: %+v", response)
	}
}

func TestSimulateBotGamesAcceptsVariantAlias(t *testing.T) {
	seed := int64(12351)
	response, err := SimulateBotGames(BotSimulationRequest{
		Games:              1,
		Players:            6,
		Seed:               &seed,
		BotMemorandumCount: 1,
		BotMemorandumType:  BotSimulationMemorandumTypeRisk,
		Variant:            BotSimulationMemorandumVariantStandard,
		Workers:            1,
		MonteCarloRollouts: 4,
	})
	if err != nil {
		t.Fatalf("SimulateBotGames: %v", err)
	}
	if response.BotMemorandumVariant != BotSimulationMemorandumVariantStandard {
		t.Fatalf("expected variant alias to select standard memorandums, got %+v", response)
	}
}

func TestSimulateBotGamesDeterministicAcrossWorkerCounts(t *testing.T) {
	seed := int64(9091)
	left, err := SimulateBotGames(BotSimulationRequest{
		Games:              8,
		Players:            6,
		Seed:               &seed,
		IncludeGames:       true,
		Workers:            1,
		MonteCarloRollouts: 4,
	})
	if err != nil {
		t.Fatalf("SimulateBotGames workers=1: %v", err)
	}
	right, err := SimulateBotGames(BotSimulationRequest{
		Games:              8,
		Players:            6,
		Seed:               &seed,
		IncludeGames:       true,
		Workers:            4,
		MonteCarloRollouts: 4,
	})
	if err != nil {
		t.Fatalf("SimulateBotGames workers=4: %v", err)
	}

	if left.MoleWins != right.MoleWins ||
		left.PlayersWins != right.PlayersWins ||
		left.AverageRounds != right.AverageRounds ||
		left.AcceptedCleanCount != right.AcceptedCleanCount ||
		left.AcceptedTargetCount != right.AcceptedTargetCount ||
		left.AcceptedSabotageCount != right.AcceptedSabotageCount ||
		left.ComplianceCatchesCount != right.ComplianceCatchesCount ||
		left.PlayersWinsByComplianceCount != right.PlayersWinsByComplianceCount ||
		left.AverageComplianceCatchesPerGame != right.AverageComplianceCatchesPerGame {
		t.Fatalf("expected deterministic aggregate across workers, left=%+v right=%+v", left, right)
	}
	if len(left.Results) != len(right.Results) {
		t.Fatalf("expected same result count, left=%d right=%d", len(left.Results), len(right.Results))
	}
	for i := range left.Results {
		if left.Results[i].Winner != right.Results[i].Winner ||
			left.Results[i].Rounds != right.Results[i].Rounds ||
			left.Results[i].MoleUserID != right.Results[i].MoleUserID ||
			left.Results[i].ComplianceUserID != right.Results[i].ComplianceUserID ||
			left.Results[i].WinnerReason != right.Results[i].WinnerReason ||
			left.Results[i].ComplianceCaught != right.Results[i].ComplianceCaught ||
			left.Results[i].MolePoints != right.Results[i].MolePoints ||
			left.Results[i].PlayersPoints != right.Results[i].PlayersPoints ||
			strings.Join(left.Results[i].AcceptedDecisions, ",") != strings.Join(right.Results[i].AcceptedDecisions, ",") ||
			complianceWatchSignature(left.Results[i].ComplianceWatches) != complianceWatchSignature(right.Results[i].ComplianceWatches) {
			t.Fatalf("expected deterministic game %d, left=%+v right=%+v", i, left.Results[i], right.Results[i])
		}
	}
}

func TestSimulateBotGamesDeterministicAcrossRepeatedRuns(t *testing.T) {
	tests := []struct {
		name   string
		index  int
		config botSimulationConfig
	}{
		{
			name:  "forced advanced risk",
			index: 15,
			config: botSimulationConfig{
				Games:                100,
				Players:              6,
				Seed:                 22222,
				BotMemorandumCount:   1,
				BotMemorandumType:    BotSimulationMemorandumTypeRisk,
				BotMemorandumVariant: BotSimulationMemorandumVariantAdvanced,
				Workers:              1,
				MonteCarloRollouts:   32,
			},
		},
		{
			name:  "mixed default",
			index: 7,
			config: botSimulationConfig{
				Games:                8,
				Players:              6,
				Seed:                 9091,
				BotMemorandumCount:   1,
				BotMemorandumType:    BotSimulationMemorandumTypeMixed,
				BotMemorandumVariant: BotSimulationMemorandumVariantMixed,
				Workers:              1,
				MonteCarloRollouts:   4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, err := newBotSimulationEngine(tt.config, tt.index).simulateBotGame(tt.index, tt.config.Players)
			if err != nil {
				t.Fatalf("simulate first game: %v", err)
			}
			right, err := newBotSimulationEngine(tt.config, tt.index).simulateBotGame(tt.index, tt.config.Players)
			if err != nil {
				t.Fatalf("simulate second game: %v", err)
			}
			if left.Winner != right.Winner ||
				left.Rounds != right.Rounds ||
				left.WinnerReason != right.WinnerReason ||
				left.ComplianceCaught != right.ComplianceCaught ||
				left.MolePoints != right.MolePoints ||
				left.PlayersPoints != right.PlayersPoints ||
				strings.Join(left.AcceptedDecisions, ",") != strings.Join(right.AcceptedDecisions, ",") ||
				complianceWatchSignature(left.ComplianceWatches) != complianceWatchSignature(right.ComplianceWatches) {
				t.Fatalf("expected deterministic game, left=%+v right=%+v", left, right)
			}
		})
	}
}

func TestSimulateBotGamesValidatesBotMemorandumCount(t *testing.T) {
	_, err := SimulateBotGames(BotSimulationRequest{
		Games:              1,
		Players:            3,
		BotMemorandumCount: MaxBotSimulationMemorandumCount + 1,
	})
	if err == nil {
		t.Fatalf("expected oversized bot_memorandum_count to fail")
	}
}

func TestSimulateBotGamesValidatesBotMemorandumType(t *testing.T) {
	_, err := SimulateBotGames(BotSimulationRequest{
		Games:             1,
		Players:           3,
		BotMemorandumType: "unknown",
	})
	if err == nil {
		t.Fatalf("expected unsupported bot_memorandum_type to fail")
	}
}

func TestSimulateBotGamesValidatesBotMemorandumVariant(t *testing.T) {
	_, err := SimulateBotGames(BotSimulationRequest{
		Games:                1,
		Players:              3,
		BotMemorandumVariant: "unknown",
	})
	if err == nil {
		t.Fatalf("expected unsupported bot_memorandum_variant to fail")
	}
}

func TestSimulateBotGamesValidatesWorkersAndRollouts(t *testing.T) {
	if _, err := SimulateBotGames(BotSimulationRequest{Games: 1, Players: 3, Workers: MaxBotSimulationWorkers + 1}); err == nil {
		t.Fatalf("expected oversized workers to fail")
	}
	if _, err := SimulateBotGames(BotSimulationRequest{Games: 1, Players: 3, Workers: -1}); err == nil {
		t.Fatalf("expected negative workers to fail")
	}
	if _, err := SimulateBotGames(BotSimulationRequest{Games: 1, Players: 3, MonteCarloRollouts: MaxBotSimulationMonteCarloRollouts + 1}); err == nil {
		t.Fatalf("expected oversized monte_carlo_rollouts to fail")
	}
	if _, err := SimulateBotGames(BotSimulationRequest{Games: 1, Players: 3, MonteCarloRollouts: -1}); err == nil {
		t.Fatalf("expected negative monte_carlo_rollouts to fail")
	}
}

func TestRememberBotSimulationMemorandumsUsesConfiguredType(t *testing.T) {
	engine := &Engine{
		rng:                          rand.New(rand.NewSource(1)),
		botSimulationMemorandumCount: 3,
		botSimulationMemorandumType:  BotSimulationMemorandumTypeRisk,
		botSimulationMemorandums:     map[int64][]MemorandumState{},
	}
	state := &GameState{
		Status:        GameStatusStarted,
		MoleTargets:   []string{"A", "C", "F"},
		MoleSabotage:  "H",
		Players:       map[int64]*PlayerState{},
		PlayerOrder:   []int64{-1},
		Memorandums:   map[int64]MemorandumState{},
		CurrentVotes:  map[int64]VoteState{},
		Available:     map[string]bool{},
		AcceptedOrder: nil,
	}
	for _, decision := range allDecisions {
		state.Available[decision] = true
	}
	state.Players[-1] = &PlayerState{UserID: -1, IsBot: true, Role: "player"}

	err := engine.rememberBotSimulationMemorandums(state, []models.Event{{
		EventType:  models.EventMemorandumAssigned,
		EventValue: mustJSON(MemorandumAssignedPayload{UserID: -1, Type: MemorandumTypeOpportunity, Decisions: []string{"B", "D", "E"}}),
	}})
	if err != nil {
		t.Fatalf("remember memorandums: %v", err)
	}
	memorandums := engine.botSimulationMemorandums[-1]
	if len(memorandums) != 3 {
		t.Fatalf("expected 3 memorandums, got %+v", memorandums)
	}
	for _, memorandum := range memorandums {
		if memorandum.Type != MemorandumTypeRisk {
			t.Fatalf("expected forced risk memorandums, got %+v", memorandums)
		}
		if memorandum.Variant != MemorandumVariantAdvanced || len(memorandum.Decisions) != 2 {
			t.Fatalf("expected forced simulation memorandums to be advanced pairs, got %+v", memorandums)
		}
		if !memorandumMatches(memorandum.Decisions, moleObjectiveSet(state.MoleTargets, state.MoleSabotage), MemorandumTypeRisk) {
			t.Fatalf("expected forced advanced risk pair to include a Mole target, got %+v", memorandum)
		}
	}
}

func TestRememberBotSimulationMemorandumsUsesAdvancedPairForSingleForcedType(t *testing.T) {
	engine := &Engine{
		rng:                          rand.New(rand.NewSource(2)),
		botSimulationMemorandumCount: 1,
		botSimulationMemorandumType:  BotSimulationMemorandumTypeRisk,
		botSimulationMemorandums:     map[int64][]MemorandumState{},
	}
	state := &GameState{
		Status:        GameStatusStarted,
		MoleTargets:   []string{"A", "C", "F"},
		MoleSabotage:  "H",
		Players:       map[int64]*PlayerState{},
		PlayerOrder:   []int64{-1},
		Memorandums:   map[int64]MemorandumState{},
		CurrentVotes:  map[int64]VoteState{},
		Available:     map[string]bool{},
		AcceptedOrder: nil,
	}
	for _, decision := range allDecisions {
		state.Available[decision] = true
	}
	state.Players[-1] = &PlayerState{UserID: -1, IsBot: true, Role: "player"}

	err := engine.rememberBotSimulationMemorandums(state, []models.Event{{
		EventType:  models.EventMemorandumAssigned,
		EventValue: mustJSON(MemorandumAssignedPayload{UserID: -1, Type: MemorandumTypeOpportunity, Variant: MemorandumVariantStandard, Decisions: []string{"B", "D", "E"}}),
	}})
	if err != nil {
		t.Fatalf("remember memorandums: %v", err)
	}
	memorandums := engine.botSimulationMemorandums[-1]
	if len(memorandums) != 1 {
		t.Fatalf("expected 1 memorandum, got %+v", memorandums)
	}
	memorandum := memorandums[0]
	if memorandum.Type != MemorandumTypeRisk || memorandum.Variant != MemorandumVariantAdvanced || len(memorandum.Decisions) != 2 {
		t.Fatalf("expected single forced risk simulation memorandum to be an advanced pair, got %+v", memorandum)
	}
}

func TestRememberBotSimulationMemorandumsUsesConfiguredStandardVariant(t *testing.T) {
	engine := &Engine{
		rng:                            rand.New(rand.NewSource(3)),
		botSimulationMemorandumCount:   1,
		botSimulationMemorandumType:    BotSimulationMemorandumTypeRisk,
		botSimulationMemorandumVariant: BotSimulationMemorandumVariantStandard,
		botSimulationMemorandums:       map[int64][]MemorandumState{},
	}
	state := &GameState{
		Status:        GameStatusStarted,
		MoleTargets:   []string{"A", "C", "F"},
		MoleSabotage:  "H",
		Players:       map[int64]*PlayerState{},
		PlayerOrder:   []int64{-1},
		Memorandums:   map[int64]MemorandumState{},
		CurrentVotes:  map[int64]VoteState{},
		Available:     map[string]bool{},
		AcceptedOrder: nil,
	}
	for _, decision := range allDecisions {
		state.Available[decision] = true
	}
	state.Players[-1] = &PlayerState{UserID: -1, IsBot: true, Role: "player"}

	err := engine.rememberBotSimulationMemorandums(state, []models.Event{{
		EventType:  models.EventMemorandumAssigned,
		EventValue: mustJSON(MemorandumAssignedPayload{UserID: -1, Type: MemorandumTypeOpportunity, Variant: MemorandumVariantStandard, Decisions: []string{"B", "D", "E"}}),
	}})
	if err != nil {
		t.Fatalf("remember memorandums: %v", err)
	}
	memorandums := engine.botSimulationMemorandums[-1]
	if len(memorandums) != 1 {
		t.Fatalf("expected 1 memorandum, got %+v", memorandums)
	}
	memorandum := memorandums[0]
	if memorandum.Type != MemorandumTypeRisk || memorandum.Variant != MemorandumVariantStandard || len(memorandum.Decisions) != 3 {
		t.Fatalf("expected forced standard risk simulation memorandum to be a trio, got %+v", memorandum)
	}
}

func TestMultiMemorandumInferenceScoresCleanIntersection(t *testing.T) {
	state := &GameState{
		Status:           GameStatusStarted,
		Phase:            GamePhaseMajorVoting,
		Players:          map[int64]*PlayerState{},
		PlayerOrder:      []int64{-1},
		Available:        map[string]bool{},
		MajorVoteOptions: []string{"A", "B", "C", "D"},
		Memorandums:      map[int64]MemorandumState{},
	}
	for _, decision := range allDecisions {
		state.Available[decision] = true
	}
	bot := &PlayerState{
		UserID:       -1,
		Name:         "AI Strategy",
		ShareBPS:     3500,
		AuthorityBPS: InitialAuthorityBPS,
		IsBot:        true,
		Role:         "player",
	}
	state.Players[bot.UserID] = bot

	engine := &Engine{
		rng: rand.New(rand.NewSource(1)),
		botSimulationMemorandums: map[int64][]MemorandumState{
			bot.UserID: {
				{UserID: bot.UserID, Type: MemorandumTypeRisk, Decisions: []string{"A", "C", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeRisk, Decisions: []string{"A", "D", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeRisk, Decisions: []string{"C", "D", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeOpportunity, Decisions: []string{"A", "B", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeOpportunity, Decisions: []string{"B", "C", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeOpportunity, Decisions: []string{"B", "D", "E"}},
				{UserID: bot.UserID, Type: MemorandumTypeRisk, Decisions: []string{"A", "B", "E"}},
			},
		},
	}

	inference, ok := engine.botObjectiveInference(state, bot)
	if !ok {
		t.Fatalf("expected multi-memorandum inference")
	}
	if inference.CleanCounts["B"] != inference.Total {
		t.Fatalf("expected B to be inferred clean in all hypotheses, inference=%+v", inference)
	}
	if scoreB, scoreA := engine.scoreBotMajorDecision(state, bot, "B"), engine.scoreBotMajorDecision(state, bot, "A"); scoreB <= scoreA {
		t.Fatalf("expected inferred-clean B to outscore A, scoreB=%d scoreA=%d", scoreB, scoreA)
	}
}

func complianceWatchSignature(watches []BotSimulationComplianceWatch) string {
	parts := make([]string, 0, len(watches))
	for _, watch := range watches {
		parts = append(parts, strconv.Itoa(watch.RoundNumber)+":"+strconv.FormatInt(watch.ComplianceUserID, 10)+">"+strconv.FormatInt(watch.TargetUserID, 10))
	}
	return strings.Join(parts, ",")
}

func BenchmarkSimulateBotGames100Workers1(b *testing.B) {
	seed := int64(1001)
	for i := 0; i < b.N; i++ {
		if _, err := SimulateBotGames(BotSimulationRequest{
			Games:              100,
			Players:            6,
			Seed:               &seed,
			Workers:            1,
			MonteCarloRollouts: 8,
		}); err != nil {
			b.Fatalf("SimulateBotGames: %v", err)
		}
	}
}

func BenchmarkSimulateBotGames1000DefaultWorkers(b *testing.B) {
	seed := int64(1002)
	for i := 0; i < b.N; i++ {
		if _, err := SimulateBotGames(BotSimulationRequest{
			Games:              1000,
			Players:            6,
			Seed:               &seed,
			MonteCarloRollouts: 8,
		}); err != nil {
			b.Fatalf("SimulateBotGames: %v", err)
		}
	}
}
