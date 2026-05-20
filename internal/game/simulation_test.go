package game

import (
	"math/rand"
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
	})
	if err != nil {
		t.Fatalf("SimulateBotGames: %v", err)
	}
	if response.Games != 1 || response.Players != 3 || response.Seed != seed || response.BotMemorandumCount != 7 || response.BotMemorandumType != BotSimulationMemorandumTypeRisk {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected included game result, got %+v", response.Results)
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
