package game

import (
	"context"
	"sync"
	"testing"

	"agentbackend/internal/models"
)

type stubStore struct {
	mu     sync.Mutex
	users  map[int64]models.User
	games  map[int64]models.Game
	events map[int64][]models.Event
}

func (s *stubStore) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return nil, errStub("user not found")
	}
	return &user, nil
}

func (s *stubStore) GetGameByID(ctx context.Context, id int64) (*models.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	if !ok {
		return nil, errStub("game not found")
	}
	return &game, nil
}

func (s *stubStore) ListEventsByGameID(ctx context.Context, gameID int64) ([]models.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.Event(nil), s.events[gameID]...), nil
}

func (s *stubStore) CreateGameWithEvents(ctx context.Context, game *models.Game, events []models.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	game.ID = int64(len(s.games) + 1)
	s.games[game.ID] = *game
	for i := range events {
		events[i].GameID = game.ID
		events[i].ID = int64(len(s.events[game.ID]) + 1)
		s.events[game.ID] = append(s.events[game.ID], events[i])
	}
	return nil
}

func (s *stubStore) AppendEvents(ctx context.Context, gameID int64, events []models.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range events {
		events[i].GameID = gameID
		events[i].ID = int64(len(s.events[gameID]) + 1)
		s.events[gameID] = append(s.events[gameID], events[i])
	}
	return nil
}

type stubError string

func (e stubError) Error() string { return string(e) }

func errStub(s string) error { return stubError(s) }

func TestBuildStateProjectsRolesAndDecisions(t *testing.T) {
	events := []models.Event{
		{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
		{EventType: models.EventGameStarted, EventValue: `{}`},
		{EventType: models.EventMoleSelected, EventValue: `{"user_id":2}`},
		{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
		{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":4000}`},
		{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2200}`},
		{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":1800}`},
		{EventType: models.EventCEOSelected, EventValue: `{"user_id":2}`},
		{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
		{EventType: models.EventDecisionAccepted, EventValue: `{"round":1,"decision":"B"}`},
	}

	state, err := BuildState(1, "Mafia", events)
	if err != nil {
		t.Fatalf("BuildState: %v", err)
	}

	if state.Status != GameStatusStarted {
		t.Fatalf("expected started status, got %s", state.Status)
	}
	if state.Players[2].Role != "mole" {
		t.Fatalf("expected player 2 to be mole")
	}
	if state.Players[2].IsCEO != true {
		t.Fatalf("expected player 2 to be CEO")
	}
	if state.Available["B"] {
		t.Fatalf("expected accepted decision B to be removed from available")
	}
	if state.TreasuryShareBPS != InitialTreasurySharesBPS {
		t.Fatalf("expected treasury %d, got %d", InitialTreasurySharesBPS, state.TreasuryShareBPS)
	}
}

func TestHandleVoteRejectsTieWithoutCEOResolution(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
			4: {ID: 4, Name: "Dave"},
			5: {ID: 5, Name: "Eve"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":4,"name":"Dave"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":5,"name":"Eve"}`},
				{EventType: models.EventGameStarted, EventValue: `{}`},
				{EventType: models.EventMoleSelected, EventValue: `{"user_id":5}`},
				{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":2500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2000}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":4,"share_bps":1500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":5,"share_bps":1500}`},
				{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
				{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
				{EventType: models.EventVoteSubmitted, EventValue: `{"round":1,"user_id":1,"abstain":true}`},
				{EventType: models.EventVoteSubmitted, EventValue: `{"round":1,"user_id":2,"decision":"B","abstain":false}`},
				{EventType: models.EventVoteSubmitted, EventValue: `{"round":1,"user_id":3,"decision":"C","abstain":false}`},
				{EventType: models.EventVoteSubmitted, EventValue: `{"round":1,"user_id":4,"decision":"B","abstain":false}`},
			},
		},
	}

	engine := NewEngine(store)
	state, events, err := engine.HandleAction(context.Background(), 1, Action{
		UserID:  5,
		Type:    ActionVote,
		Payload: []byte(`{"decision":"C"}`),
	})
	if err != nil {
		t.Fatalf("HandleAction: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 emitted events, got %d", len(events))
	}
	if events[2].EventType != models.EventDecisionRejected {
		t.Fatalf("expected %s, got %s", models.EventDecisionRejected, events[2].EventType)
	}
	if state.CurrentRound != 2 {
		t.Fatalf("expected next round 2, got %d", state.CurrentRound)
	}
}

func TestProjectStateShowsMoleTargetsOnlyToMole(t *testing.T) {
	state, err := BuildState(1, "Mafia", []models.Event{
		{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
		{EventType: models.EventGameStarted, EventValue: `{}`},
		{EventType: models.EventMoleSelected, EventValue: `{"user_id":2}`},
		{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
		{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
		{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
	})
	if err != nil {
		t.Fatalf("BuildState: %v", err)
	}

	regular, err := ProjectStateForViewer(state, 1)
	if err != nil {
		t.Fatalf("ProjectStateForViewer regular: %v", err)
	}
	if len(regular.MoleTargets) != 0 {
		t.Fatalf("expected regular player to not see mole targets")
	}

	mole, err := ProjectStateForViewer(state, 2)
	if err != nil {
		t.Fatalf("ProjectStateForViewer mole: %v", err)
	}
	if len(mole.MoleTargets) != 3 {
		t.Fatalf("expected mole to see 3 targets, got %d", len(mole.MoleTargets))
	}
}

func TestProjectStateAllowsLobbyViewForNonParticipant(t *testing.T) {
	state, err := BuildState(1, "Mafia", []models.Event{
		{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
	})
	if err != nil {
		t.Fatalf("BuildState: %v", err)
	}

	publicState, err := ProjectStateForViewer(state, 999)
	if err != nil {
		t.Fatalf("ProjectStateForViewer: %v", err)
	}
	if len(publicState.AvailableActions) != 1 || publicState.AvailableActions[0] != ActionJoinGame {
		t.Fatalf("expected outsider to be able to join, got %v", publicState.AvailableActions)
	}
	if len(publicState.AvailableDecisions) != 0 {
		t.Fatalf("expected no available decisions in lobby, got %v", publicState.AvailableDecisions)
	}
}

func TestProjectStateJoinedNonHostDoesNotSeeJoinGame(t *testing.T) {
	state, err := BuildState(1, "Mafia", []models.Event{
		{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
		{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
	})
	if err != nil {
		t.Fatalf("BuildState: %v", err)
	}

	publicState, err := ProjectStateForViewer(state, 2)
	if err != nil {
		t.Fatalf("ProjectStateForViewer: %v", err)
	}
	for _, action := range publicState.AvailableActions {
		if action == ActionJoinGame {
			t.Fatalf("joined non-host must not see join_game")
		}
	}
}

func TestLeaveLobbyAllowsRejoinAndTransfersHost(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			},
		},
	}
	engine := NewEngine(store)

	_, events, err := engine.HandleAction(context.Background(), 1, Action{UserID: 1, Type: ActionLeaveGame})
	if err != nil {
		t.Fatalf("leave lobby: %v", err)
	}
	if len(events) != 1 || events[0].EventType != models.EventPlayerLeft {
		t.Fatalf("expected player_left event, got %+v", events)
	}
	allEvents, err := store.ListEventsByGameID(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListEventsByGameID: %v", err)
	}
	state, err := BuildState(1, "Mafia", allEvents)
	if err != nil {
		t.Fatalf("BuildState: %v", err)
	}
	bobState, err := ProjectStateForViewer(state, 2)
	if err != nil {
		t.Fatalf("ProjectStateForViewer for Bob: %v", err)
	}
	if len(bobState.Players) != 1 || bobState.Players[0].UserID != 2 || !bobState.Players[0].IsHost {
		t.Fatalf("expected Bob to become host, got %+v", bobState.Players)
	}

	leftState, err := ProjectStateForViewer(state, 1)
	if err != nil {
		t.Fatalf("ProjectStateForViewer for left player: %v", err)
	}
	if len(leftState.AvailableActions) != 1 || leftState.AvailableActions[0] != ActionJoinGame {
		t.Fatalf("expected left player to be able to rejoin, got %v", leftState.AvailableActions)
	}

	rejoinedState, events, err := engine.HandleAction(context.Background(), 1, Action{UserID: 1, Type: ActionJoinGame})
	if err != nil {
		t.Fatalf("rejoin lobby: %v", err)
	}
	if len(events) != 1 || events[0].EventType != models.EventPlayerJoined {
		t.Fatalf("expected player_joined event, got %+v", events)
	}
	if len(rejoinedState.Players) != 2 {
		t.Fatalf("expected two active players after rejoin, got %+v", rejoinedState.Players)
	}
}

func TestSendChatMessageAddsPublicMessage(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
			},
		},
	}
	engine := NewEngine(store)

	state, events, err := engine.HandleAction(context.Background(), 1, Action{
		UserID:  2,
		Type:    ActionSendChatMessage,
		Payload: []byte(`{"message":"  Ready for the board meeting  "}`),
	})
	if err != nil {
		t.Fatalf("send chat message: %v", err)
	}
	if len(events) != 1 || events[0].EventType != models.EventChatMessageSent {
		t.Fatalf("expected chat event, got %+v", events)
	}
	if len(state.ChatMessages) != 1 {
		t.Fatalf("expected one public chat message, got %d", len(state.ChatMessages))
	}
	message := state.ChatMessages[0]
	if message.UserID != 2 || message.UserName != "Bob" || message.Message != "Ready for the board meeting" {
		t.Fatalf("unexpected chat message: %+v", message)
	}
	hasChatAction := false
	for _, action := range state.AvailableActions {
		if action == ActionSendChatMessage {
			hasChatAction = true
		}
	}
	if !hasChatAction {
		t.Fatalf("expected chat action, got %v", state.AvailableActions)
	}
}

func TestKickedPlayerCannotRejoin(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerKicked, EventValue: `{"user_id":2}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
			},
		},
	}

	engine := NewEngine(store)
	_, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 2, Type: ActionJoinGame})
	if err == nil || err.Error() != "kicked player cannot rejoin" {
		t.Fatalf("expected kicked player cannot rejoin error, got %v", err)
	}
}

func TestConcurrentStartGameOnlyOneSucceeds(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
			},
		},
	}
	engine := NewEngine(store)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 1, Type: ActionStartGame})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected one success and one failure, got successes=%d failures=%d", successes, failures)
	}
}

func TestConcurrentDoubleVoteOnlyOneSucceeds(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
				{EventType: models.EventGameStarted, EventValue: `{}`},
				{EventType: models.EventMoleSelected, EventValue: `{"user_id":3}`},
				{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":3500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
				{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
				{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
			},
		},
	}
	engine := NewEngine(store)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, payload := range []string{`{"decision":"A"}`, `{"decision":"B"}`} {
		wg.Add(1)
		go func(payload string) {
			defer wg.Done()
			_, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 2, Type: ActionVote, Payload: []byte(payload)})
			results <- err
		}(payload)
	}
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected one success and one failure, got successes=%d failures=%d", successes, failures)
	}
}

func TestCEOCannotAbstain(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
				{EventType: models.EventGameStarted, EventValue: `{}`},
				{EventType: models.EventMoleSelected, EventValue: `{"user_id":3}`},
				{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":3500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
				{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
				{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
			},
		},
	}

	engine := NewEngine(store)
	_, _, err := engine.HandleAction(context.Background(), 1, Action{
		UserID:  1,
		Type:    ActionVote,
		Payload: []byte(`{"abstain":true}`),
	})
	if err == nil || err.Error() != "ceo cannot abstain" {
		t.Fatalf("expected ceo cannot abstain error, got %v", err)
	}
}

func TestAcceptedMajorDecisionStartsGovernanceAndAppliesProposal(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
				{EventType: models.EventGameStarted, EventValue: `{}`},
				{EventType: models.EventMoleSelected, EventValue: `{"user_id":3}`},
				{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":3500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
				{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
				{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
			},
		},
	}
	engine := NewEngine(store)

	for _, userID := range []int64{1, 2} {
		if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: userID, Type: ActionVote, Payload: []byte(`{"decision":"B"}`)}); err != nil {
			t.Fatalf("major vote by %d: %v", userID, err)
		}
	}
	state, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 3, Type: ActionVote, Payload: []byte(`{"decision":"B"}`)})
	if err != nil {
		t.Fatalf("major final vote: %v", err)
	}
	if state.Phase != GamePhaseGovernanceProposal {
		t.Fatalf("expected governance proposal phase, got %s", state.Phase)
	}
	if state.TreasuryShareBPS != 1700 {
		t.Fatalf("expected treasury 1700 after major rewards, got %d", state.TreasuryShareBPS)
	}

	state, _, err = engine.HandleAction(context.Background(), 1, Action{
		UserID: 1,
		Type:   ActionSubmitGovernanceProposal,
		Payload: []byte(`{
			"proposal_type":"treasury_grant",
			"target_user_id":2,
			"share_bps":500
		}`),
	})
	if err != nil {
		t.Fatalf("submit governance proposal: %v", err)
	}
	if len(state.GovernanceProposals) != 1 {
		t.Fatalf("expected public proposal, got %d", len(state.GovernanceProposals))
	}
	if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 2, Type: ActionSkipGovernanceProposal}); err != nil {
		t.Fatalf("skip governance proposal by user 2: %v", err)
	}
	state, _, err = engine.HandleAction(context.Background(), 1, Action{UserID: 3, Type: ActionSkipGovernanceProposal})
	if err != nil {
		t.Fatalf("skip governance proposal by user 3: %v", err)
	}
	if state.Phase != GamePhaseGovernanceVoting {
		t.Fatalf("expected governance voting phase, got %s", state.Phase)
	}

	for _, userID := range []int64{1, 2} {
		if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: userID, Type: ActionVote, Payload: []byte(`{"proposal_id":1}`)}); err != nil {
			t.Fatalf("governance vote by %d: %v", userID, err)
		}
	}
	state, _, err = engine.HandleAction(context.Background(), 1, Action{UserID: 3, Type: ActionVote, Payload: []byte(`{"proposal_id":1}`)})
	if err != nil {
		t.Fatalf("governance final vote: %v", err)
	}
	if state.Phase != GamePhaseMajorVoting || state.CurrentRound != 2 {
		t.Fatalf("expected next major round 2, got phase=%s round=%d", state.Phase, state.CurrentRound)
	}
	if state.TreasuryShareBPS != 1200 {
		t.Fatalf("expected treasury 1200, got %d", state.TreasuryShareBPS)
	}
	var bobShare int
	for _, player := range state.Players {
		if player.UserID == 2 {
			bobShare = player.ShareBPS
		}
	}
	if bobShare != 3100 {
		t.Fatalf("expected Bob share 3100, got %d", bobShare)
	}
	if len(state.GovernanceReports) != 1 || state.GovernanceReports[0].Outcome != "accepted" {
		t.Fatalf("expected accepted governance report, got %+v", state.GovernanceReports)
	}
}

func TestGovernanceBuybackMovesShareToTreasury(t *testing.T) {
	store := &stubStore{
		users: map[int64]models.User{
			1: {ID: 1, Name: "Alice"},
			2: {ID: 2, Name: "Bob"},
			3: {ID: 3, Name: "Carol"},
		},
		games: map[int64]models.Game{
			1: {ID: 1, Title: "Mafia"},
		},
		events: map[int64][]models.Event{
			1: {
				{EventType: models.EventGameCreated, EventValue: `{"host_user_id":1,"title":"Mafia"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":1,"name":"Alice"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":2,"name":"Bob"}`},
				{EventType: models.EventPlayerJoined, EventValue: `{"user_id":3,"name":"Carol"}`},
				{EventType: models.EventGameStarted, EventValue: `{}`},
				{EventType: models.EventMoleSelected, EventValue: `{"user_id":3}`},
				{EventType: models.EventMoleTargetsGenerated, EventValue: `{"targets":["A","D","F"]}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":1,"share_bps":3500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":2,"share_bps":2500}`},
				{EventType: models.EventPlayerReceivedShare, EventValue: `{"user_id":3,"share_bps":2000}`},
				{EventType: models.EventCEOSelected, EventValue: `{"user_id":1}`},
				{EventType: models.EventVotingRoundStarted, EventValue: `{"round":1}`},
				{EventType: models.EventDecisionAccepted, EventValue: `{"round":1,"decision":"B"}`},
				{EventType: models.EventGovernanceProposalPhaseStarted, EventValue: `{"round":1}`},
			},
		},
	}
	engine := NewEngine(store)

	state, _, err := engine.HandleAction(context.Background(), 1, Action{
		UserID: 1,
		Type:   ActionSubmitGovernanceProposal,
		Payload: []byte(`{
			"proposal_type":"treasury_buyback",
			"target_user_id":2,
			"share_bps":500
		}`),
	})
	if err != nil {
		t.Fatalf("submit governance buyback: %v", err)
	}
	if state.GovernanceProposals[0].ProposalType != GovernanceProposalTreasuryBuyback {
		t.Fatalf("expected buyback proposal, got %+v", state.GovernanceProposals)
	}
	if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 2, Type: ActionSkipGovernanceProposal}); err != nil {
		t.Fatalf("skip governance proposal by user 2: %v", err)
	}
	if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: 3, Type: ActionSkipGovernanceProposal}); err != nil {
		t.Fatalf("skip governance proposal by user 3: %v", err)
	}
	for _, userID := range []int64{1, 2} {
		if _, _, err := engine.HandleAction(context.Background(), 1, Action{UserID: userID, Type: ActionVote, Payload: []byte(`{"proposal_id":1}`)}); err != nil {
			t.Fatalf("governance vote by %d: %v", userID, err)
		}
	}
	state, _, err = engine.HandleAction(context.Background(), 1, Action{UserID: 3, Type: ActionVote, Payload: []byte(`{"proposal_id":1}`)})
	if err != nil {
		t.Fatalf("governance final vote: %v", err)
	}
	if state.TreasuryShareBPS != 2500 {
		t.Fatalf("expected treasury 2500, got %d", state.TreasuryShareBPS)
	}
	var bobShare int
	for _, player := range state.Players {
		if player.UserID == 2 {
			bobShare = player.ShareBPS
		}
	}
	if bobShare != 2000 {
		t.Fatalf("expected Bob share 2000, got %d", bobShare)
	}
}
