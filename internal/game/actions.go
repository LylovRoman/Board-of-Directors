package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"agentbackend/internal/models"
)

func (e *Engine) handleJoinGame(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("game already started")
	}
	if player := state.Players[actor.ID]; player != nil {
		if player.IsKicked {
			return nil, errors.New("kicked player cannot rejoin")
		}
		if !player.IsLeft {
			return nil, errors.New("player already joined")
		}
	}
	if len(activePlayers(state)) >= MaxPlayers {
		return nil, fmt.Errorf("game is full: max %d players", MaxPlayers)
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventPlayerJoined,
		EventValue: mustJSON(PlayerJoinedPayload{UserID: actor.ID, Name: actor.Name, Position: actor.Position}),
	}}, nil
}

func (e *Engine) handleLeaveGame(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("cannot leave after game started")
	}
	if activePlayerByID(state, actor.ID) == nil {
		return nil, errors.New("player is not in lobby")
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventPlayerLeft,
		EventValue: mustJSON(PlayerLeftPayload{UserID: actor.ID}),
	}}, nil
}

func (e *Engine) handleKickPlayer(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("cannot kick after game started")
	}
	if actor.ID != state.HostUserID {
		return nil, errors.New("only host can kick players")
	}

	var payload KickPlayerActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if payload.UserID == 0 {
		return nil, errors.New("user_id is required")
	}
	if payload.UserID == actor.ID {
		return nil, errors.New("host cannot kick themselves")
	}

	if activePlayerByID(state, payload.UserID) == nil {
		return nil, errors.New("target player is not in lobby")
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventPlayerLeft,
		EventValue: mustJSON(PlayerLeftPayload{UserID: payload.UserID}),
	}}, nil
}

func (e *Engine) handleBanPlayer(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("cannot ban after game started")
	}
	if actor.ID != state.HostUserID {
		return nil, errors.New("only host can ban players")
	}

	var payload BanPlayerActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if payload.UserID == 0 {
		return nil, errors.New("user_id is required")
	}
	if payload.UserID == actor.ID {
		return nil, errors.New("host cannot ban themselves")
	}

	if activePlayerByID(state, payload.UserID) == nil {
		return nil, errors.New("target player is not in lobby")
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventPlayerKicked,
		EventValue: mustJSON(PlayerKickedPayload{UserID: payload.UserID}),
	}}, nil
}

func (e *Engine) handleSendChatMessage(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can send chat messages")
	}

	var payload SendChatMessageActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return nil, errors.New("message is required")
	}
	if len([]rune(message)) > MaxChatMessageLength {
		return nil, fmt.Errorf("message cannot exceed %d characters", MaxChatMessageLength)
	}
	kind := "user"
	if strings.HasPrefix(message, "/me") {
		text := strings.TrimSpace(strings.TrimPrefix(message, "/me"))
		if text == "" {
			return nil, errors.New("official statement text is required")
		}
		position := effectivePlayerPosition(player)
		if position == "" {
			return nil, errors.New("company position is required for official statements")
		}
		message = fmt.Sprintf("Официальное заявление %s: %s", position, text)
		kind = "official"
		if len([]rune(message)) > MaxChatMessageLength {
			return nil, fmt.Errorf("message cannot exceed %d characters", MaxChatMessageLength)
		}
	}

	return []models.Event{{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventChatMessageSent,
		EventValue: mustJSON(ChatMessageSentPayload{
			UserID:  actor.ID,
			Message: message,
			Kind:    kind,
		}),
	}}, nil
}

func (e *Engine) handleReactChatMessage(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if activePlayerByID(state, actor.ID) == nil {
		return nil, errors.New("only active players can react to chat messages")
	}

	var payload ReactChatMessageActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if payload.MessageID <= 0 {
		return nil, errors.New("message_id is required")
	}
	payload.Emoji = strings.TrimSpace(payload.Emoji)
	if !isAllowedChatReaction(payload.Emoji) {
		return nil, errors.New("unsupported chat reaction")
	}
	if !chatMessageExists(state, payload.MessageID) {
		return nil, errors.New("chat message not found")
	}

	return []models.Event{{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventChatReactionToggled,
		EventValue: mustJSON(ChatReactionToggledPayload{
			MessageID: payload.MessageID,
			UserID:    actor.ID,
			Emoji:     payload.Emoji,
		}),
	}}, nil
}

func (e *Engine) handleStartGame(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("game already started")
	}
	if actor.ID != state.HostUserID {
		return nil, errors.New("only host can start the game")
	}

	players := activePlayers(state)
	if len(players) < MinPlayers || len(players) > MaxPlayers {
		return nil, fmt.Errorf("game requires %d-%d players", MinPlayers, MaxPlayers)
	}

	shares, ok := sharePresets[len(players)]
	if !ok {
		return nil, fmt.Errorf("share preset not found for %d players", len(players))
	}

	shuffledPlayers := append([]*PlayerState(nil), players...)
	e.shufflePlayers(shuffledPlayers)
	mole := shuffledPlayers[0]
	ceo := shuffledPlayers[1%len(shuffledPlayers)]

	events := []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGameStarted,
		EventValue: mustJSON(GameStartedPayload{PlayerCount: len(players)}),
	}, {
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventMoleSelected,
		EventValue: mustJSON(MoleSelectedPayload{UserID: mole.UserID}),
	}}

	for i, player := range shuffledPlayers {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventPlayerReceivedShare,
			EventValue: mustJSON(PlayerReceivedSharePayload{UserID: player.UserID, ShareBPS: shares[i]}),
		})
		if strings.TrimSpace(player.Position) == "" {
			events = append(events, models.Event{
				GameID:     state.GameID,
				UserID:     &actor.ID,
				ActorName:  actor.Name,
				EventType:  models.EventPlayerPositionAssigned,
				EventValue: mustJSON(PlayerPositionAssignedPayload{UserID: player.UserID, Position: e.randomGeneratedPosition()}),
			})
		}
	}

	events = append(events,
		models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventCEOSelected,
			EventValue: mustJSON(CEOSelectedPayload{UserID: ceo.UserID}),
		},
	)

	return events, nil
}

func (e *Engine) handleChooseMemorandum(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase != GamePhaseMoleObjectiveSelection {
		return nil, errors.New("memorandum choice is not active")
	}
	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can choose a memorandum")
	}
	if player.Role == "mole" {
		return nil, errors.New("mole cannot choose a memorandum")
	}
	if len(state.MoleTargets) > 0 || state.MoleSabotage != "" {
		return nil, errors.New("memorandum choice is closed")
	}
	if state.MemorandumPreferences[actor.ID] != "" {
		return nil, errors.New("memorandum already selected")
	}

	var payload ChooseMemorandumActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if !isMemorandumType(payload.Type) {
		return nil, errors.New("unsupported memorandum type")
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventMemorandumPreferenceSelected,
		EventValue: mustJSON(MemorandumPreferenceSelectedPayload{UserID: actor.ID, Type: payload.Type}),
	}}, nil
}

func (e *Engine) handleSelectMoleObjectives(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase != GamePhaseMoleObjectiveSelection {
		return nil, errors.New("mole objective selection is not active")
	}
	if actor.ID != state.MoleUserID {
		return nil, errors.New("only mole can select objectives")
	}
	if len(state.MoleTargets) > 0 || state.MoleSabotage != "" {
		return nil, errors.New("mole objectives already selected")
	}

	var payload SelectMoleObjectivesActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	targets, sabotage, err := normalizeMoleObjectives(payload)
	if err != nil {
		return nil, err
	}

	showcase := e.majorShowcase(state.Available, targets, sabotage)
	unlockedAt := time.Now().UTC().Add(FirstMajorVoteLock)
	events := []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventMoleObjectivesSelected,
		EventValue: mustJSON(MoleObjectivesSelectedPayload{Targets: targets, Sabotage: sabotage}),
	}}
	events = append(events, e.memorandumAssignmentEvents(state, actor, targets, sabotage)...)
	events = append(events, models.Event{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventVotingRoundStarted,
		EventValue: mustJSON(VotingRoundStartedPayload{Round: 1, ShowcaseDecisions: showcase, UnlockedAt: &unlockedAt}),
	})
	return events, nil
}

func (e *Engine) handleVote(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase == GamePhaseGovernanceVoting {
		return e.handleGovernanceVote(state, actor, raw)
	}
	if state.Phase != GamePhaseMajorVoting {
		return nil, errors.New("voting is not active")
	}

	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can vote")
	}
	if state.MajorVoteUnlockedAt != nil && time.Now().UTC().Before(*state.MajorVoteUnlockedAt) {
		return nil, errors.New("major voting is locked while directors review memorandums")
	}
	var payload VoteActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}

	if payload.Abstain {
		return nil, errors.New("major voting does not allow abstain")
	}
	if payload.Decision == nil || *payload.Decision == "" {
		return nil, errors.New("decision is required")
	}
	if !state.Available[*payload.Decision] {
		return nil, errors.New("decision is not available")
	}
	if !isDecisionInCurrentShowcase(state, *payload.Decision) {
		return nil, errors.New("decision is not in the current showcase")
	}

	events := []models.Event{{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventVoteSubmitted,
		EventValue: mustJSON(VoteSubmittedPayload{
			Round:    state.CurrentRound,
			UserID:   actor.ID,
			Decision: payload.Decision,
			Abstain:  payload.Abstain,
		}),
	}}

	projected := cloneState(state)
	projected.CurrentVotes[actor.ID] = VoteState{UserID: actor.ID, Decision: payload.Decision, Abstain: payload.Abstain}
	if len(projected.CurrentVotes) != len(activePlayers(projected)) {
		return events, nil
	}

	events = append(events, e.resolveRound(projected, actor)...)
	return events, nil
}

func (e *Engine) handleSubmitGovernanceProposal(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase != GamePhaseGovernanceProposal {
		return nil, errors.New("governance proposal phase is not active")
	}
	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can submit proposals")
	}
	if _, ok := state.GovernanceSubmissions[actor.ID]; ok {
		return nil, errors.New("governance proposal already submitted")
	}

	var payload SubmitGovernanceProposalActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	payload.ShareBPS = effectiveAuthorityBPS(player)
	clampGovernanceProposalShareBPS(state, &payload)
	if err := validateGovernanceProposal(state, actor.ID, payload); err != nil {
		return nil, err
	}

	proposalID := nextGovernanceProposalID(state)
	if existing := findMatchingGovernanceProposal(state, payload); existing != nil {
		proposalID = existing.ID
	}
	event := models.Event{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventGovernanceProposalSubmitted,
		EventValue: mustJSON(GovernanceProposalSubmittedPayload{
			Round:          state.GovernanceRound,
			ProposalID:     proposalID,
			ProposerUserID: actor.ID,
			AuthorUserIDs:  []int64{actor.ID},
			ProposalType:   payload.ProposalType,
			FromUserID:     payload.FromUserID,
			ToUserID:       payload.ToUserID,
			TargetUserID:   payload.TargetUserID,
			ShareBPS:       payload.ShareBPS,
		}),
	}

	events := []models.Event{event}
	projected := cloneState(state)
	if err := ApplyEvent(projected, event); err != nil {
		return nil, err
	}
	events = append(events, e.governanceEventsAfterSubmission(projected, actor)...)
	return events, nil
}

func (e *Engine) handleSkipGovernanceProposal(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase != GamePhaseGovernanceProposal {
		return nil, errors.New("governance proposal phase is not active")
	}
	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can skip proposals")
	}
	if _, ok := state.GovernanceSubmissions[actor.ID]; ok {
		return nil, errors.New("governance proposal already submitted")
	}

	event := models.Event{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGovernanceProposalSkipped,
		EventValue: mustJSON(GovernanceProposalSkippedPayload{Round: state.GovernanceRound, UserID: actor.ID}),
	}

	events := []models.Event{event}
	projected := cloneState(state)
	if err := ApplyEvent(projected, event); err != nil {
		return nil, err
	}
	events = append(events, e.governanceEventsAfterSubmission(projected, actor)...)
	return events, nil
}

func (e *Engine) handleGovernanceVote(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	player := activePlayerByID(state, actor.ID)
	if player == nil {
		return nil, errors.New("only active players can vote")
	}

	var payload VoteActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Abstain {
		if player.IsCEO {
			return nil, errors.New("ceo cannot abstain")
		}
	} else {
		if payload.ProposalID == nil || *payload.ProposalID == 0 {
			return nil, errors.New("proposal_id is required when abstain is false")
		}
		if state.GovernanceProposals[*payload.ProposalID] == nil {
			return nil, errors.New("proposal is not available")
		}
	}

	events := []models.Event{{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventGovernanceVoteSubmitted,
		EventValue: mustJSON(GovernanceVoteSubmittedPayload{
			Round:      state.GovernanceRound,
			UserID:     actor.ID,
			ProposalID: payload.ProposalID,
			Abstain:    payload.Abstain,
		}),
	}}

	projected := cloneState(state)
	projected.GovernanceVotes[actor.ID] = GovernanceVoteState{
		UserID:     actor.ID,
		ProposalID: payload.ProposalID,
		Abstain:    payload.Abstain,
	}
	if len(projected.GovernanceVotes) != len(activePlayers(projected)) {
		return events, nil
	}

	events = append(events, e.resolveGovernance(projected, actor)...)
	return events, nil
}

func (e *Engine) resolveRound(state *GameState, actor *models.User) []models.Event {
	events := []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventVotingResolved,
		EventValue: mustJSON(map[string]int{"round": state.CurrentRound}),
	}}

	decision, tied, resolved := resolveDecision(state)
	if resolved {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventDecisionAccepted,
			EventValue: mustJSON(DecisionAcceptedPayload{Round: state.CurrentRound, Decision: decision}),
		})
		events = append(events, majorDecisionRewardEvents(state, actor, decision)...)
		events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, formatMajorVoteSystemMessage(state, "accepted", decision, ""))...)
		if decision == state.MoleSabotage {
			events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, sabotageAcceptedSystemMessage(state, decision))...)
		}

		nextState := cloneState(state)
		nextState.AcceptedOrder = append(nextState.AcceptedOrder, decision)
		delete(nextState.Available, decision)
		if winner, reason := detectWinner(nextState); winner != "" {
			events = append(events, models.Event{
				GameID:     state.GameID,
				UserID:     &actor.ID,
				ActorName:  actor.Name,
				EventType:  models.EventGameFinished,
				EventValue: mustJSON(GameFinishedPayload{Winner: winner, Reason: reason}),
			})
			events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, moleRevealSystemMessage(nextState))...)
			return events
		}
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventGovernanceProposalPhaseStarted,
			EventValue: mustJSON(GovernanceProposalPhaseStartedPayload{Round: state.GovernanceRound + 1}),
		})
		return events
	} else {
		events = append(events, models.Event{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventDecisionRejected,
			EventValue: mustJSON(DecisionRejectedPayload{
				Round:   state.CurrentRound,
				Options: tied,
				Reason:  "tie_not_resolved",
			}),
		})
		events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, formatMajorVoteSystemMessage(state, "rejected", "", "tie_not_resolved"))...)
	}

	events = append(events, e.votingRoundStartedEvent(state, actor, state.CurrentRound+1))

	return events
}

func majorDecisionRewardEvents(state *GameState, actor *models.User, decision string) []models.Event {
	reward := majorDecisionReward(state, decision)
	if reward.BPS <= 0 || len(reward.Voters) == 0 {
		return nil
	}

	if reward.Kind == "authority" {
		events := make([]models.Event, 0, len(reward.Voters))
		for _, voter := range reward.Voters {
			events = append(events, models.Event{
				GameID:    state.GameID,
				UserID:    &actor.ID,
				ActorName: actor.Name,
				EventType: models.EventPlayerAuthorityGranted,
				EventValue: mustJSON(PlayerAuthorityGrantedPayload{
					UserID:       voter.UserID,
					AuthorityBPS: reward.BPS,
				}),
			})
		}
		return events
	}

	events := make([]models.Event, 0, len(reward.Voters))
	for _, voter := range reward.Voters {
		events = append(events, models.Event{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventTreasuryShareGranted,
			EventValue: mustJSON(TreasuryShareGrantedPayload{
				TargetUserID: voter.UserID,
				ShareBPS:     reward.BPS,
			}),
		})
	}
	return events
}

type majorReward struct {
	Kind   string
	BPS    int
	Voters []*PlayerState
}

func majorDecisionReward(state *GameState, decision string) majorReward {
	voters := make([]*PlayerState, 0, len(state.CurrentVotes))
	for _, userID := range state.PlayerOrder {
		vote, ok := state.CurrentVotes[userID]
		if !ok || vote.Abstain || vote.Decision == nil || *vote.Decision != decision {
			continue
		}
		player := activePlayerByID(state, userID)
		if player != nil {
			voters = append(voters, player)
		}
	}
	if len(voters) == 0 {
		return majorReward{}
	}

	if decisionTypes[decision] == DecisionTypeEmpowerment {
		return majorReward{Kind: "authority", BPS: MajorAuthorityRewardBPS, Voters: voters}
	}

	if state.TreasuryShareBPS <= 0 {
		return majorReward{}
	}

	rewardBPS := MajorDecisionRewardBPS
	if totalReward := rewardBPS * len(voters); totalReward > state.TreasuryShareBPS {
		rewardBPS = state.TreasuryShareBPS / len(voters)
	}
	if rewardBPS <= 0 {
		return majorReward{}
	}

	return majorReward{Kind: "share", BPS: rewardBPS, Voters: voters}
}

func (e *Engine) governanceEventsAfterSubmission(state *GameState, actor *models.User) []models.Event {
	if len(state.GovernanceSubmissions) != len(activePlayers(state)) {
		return nil
	}

	if len(state.GovernanceProposalOrder) == 0 {
		return []models.Event{e.votingRoundStartedEvent(state, actor, state.CurrentRound+1)}
	}

	proposalIDs := topGovernanceProposalIDs(state)
	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGovernanceVotingStarted,
		EventValue: mustJSON(GovernanceVotingStartedPayload{Round: state.GovernanceRound, ProposalIDs: proposalIDs}),
	}}
}

func topGovernanceProposalIDs(state *GameState) []int {
	ids := append([]int(nil), state.GovernanceProposalOrder...)
	sort.Slice(ids, func(i, j int) bool {
		left := state.GovernanceProposals[ids[i]]
		right := state.GovernanceProposals[ids[j]]
		leftAuthority := 0
		rightAuthority := 0
		if left != nil {
			leftAuthority = left.ShareBPS
		}
		if right != nil {
			rightAuthority = right.ShareBPS
		}
		if leftAuthority != rightAuthority {
			return leftAuthority > rightAuthority
		}
		leftShare := governanceProposalAuthorShareBPS(state, left)
		rightShare := governanceProposalAuthorShareBPS(state, right)
		if leftShare != rightShare {
			return leftShare > rightShare
		}
		return ids[i] < ids[j]
	})
	if len(ids) > MaxGovernanceProposals {
		ids = ids[:MaxGovernanceProposals]
	}
	sort.Ints(ids)
	return ids
}

func governanceProposalAuthorShareBPS(state *GameState, proposal *GovernanceProposalState) int {
	if proposal == nil {
		return 0
	}
	authorIDs := proposal.AuthorUserIDs
	if len(authorIDs) == 0 {
		authorIDs = []int64{proposal.ProposerUserID}
	}
	maxShare := 0
	for _, userID := range authorIDs {
		if player := activePlayerByID(state, userID); player != nil && player.ShareBPS > maxShare {
			maxShare = player.ShareBPS
		}
	}
	return maxShare
}

func (e *Engine) resolveGovernance(state *GameState, actor *models.User) []models.Event {
	events := []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGovernanceResolved,
		EventValue: mustJSON(GovernanceResolvedPayload{Round: state.GovernanceRound}),
	}}

	proposalID, resolved := resolveGovernanceProposal(state)
	if resolved {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventGovernanceProposalAccepted,
			EventValue: mustJSON(GovernanceProposalAcceptedPayload{Round: state.GovernanceRound, ProposalID: proposalID}),
		})
		events = append(events, governanceEffectEvents(state, actor, state.GovernanceProposals[proposalID])...)
		events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, formatGovernanceSystemMessage(state, "accepted", proposalID, ""))...)
	} else {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventGovernanceProposalRejected,
			EventValue: mustJSON(GovernanceProposalRejectedPayload{Round: state.GovernanceRound, Reason: "tie_or_no_votes"}),
		})
		events = append(events, systemChatPayloadEvents(state.GameID, actor.ID, formatGovernanceSystemMessage(state, "rejected", 0, "tie_or_no_votes"))...)
	}

	events = append(events, e.votingRoundStartedEvent(state, actor, state.CurrentRound+1))
	return events
}

func resolveDecision(state *GameState) (string, []string, bool) {
	scores := map[string]int{}
	for _, vote := range state.CurrentVotes {
		if vote.Abstain || vote.Decision == nil {
			continue
		}
		player := activePlayerByID(state, vote.UserID)
		if player == nil {
			continue
		}
		scores[*vote.Decision] += player.ShareBPS
	}

	maxScore := 0
	leaders := []string{}
	for decision, score := range scores {
		if score > maxScore {
			maxScore = score
			leaders = []string{decision}
		} else if score == maxScore {
			leaders = append(leaders, decision)
		}
	}
	sort.Strings(leaders)
	if len(leaders) == 0 {
		return "", nil, false
	}
	if len(leaders) == 1 {
		return leaders[0], nil, true
	}

	ceoVote, ok := state.CurrentVotes[state.CEOUserID]
	if !ok || ceoVote.Abstain || ceoVote.Decision == nil {
		return "", leaders, false
	}
	for _, leader := range leaders {
		if leader == *ceoVote.Decision {
			return leader, leaders, true
		}
	}

	return "", leaders, false
}

func resolveGovernanceProposal(state *GameState) (int, bool) {
	scores := map[int]int{}
	for _, vote := range state.GovernanceVotes {
		if vote.Abstain || vote.ProposalID == nil {
			continue
		}
		player := activePlayerByID(state, vote.UserID)
		if player == nil || state.GovernanceProposals[*vote.ProposalID] == nil {
			continue
		}
		scores[*vote.ProposalID] += player.ShareBPS + effectiveAuthorityBPS(player)
	}

	maxScore := 0
	leaders := []int{}
	for proposalID, score := range scores {
		if score > maxScore {
			maxScore = score
			leaders = []int{proposalID}
		} else if score == maxScore {
			leaders = append(leaders, proposalID)
		}
	}
	sort.Ints(leaders)
	if len(leaders) == 0 {
		return 0, false
	}
	if len(leaders) == 1 {
		return leaders[0], true
	}

	ceoVote, ok := state.GovernanceVotes[state.CEOUserID]
	if !ok || ceoVote.Abstain || ceoVote.ProposalID == nil {
		return 0, false
	}
	for _, leader := range leaders {
		if leader == *ceoVote.ProposalID {
			return leader, true
		}
	}
	return 0, false
}

func governanceEffectEvents(state *GameState, actor *models.User, proposal *GovernanceProposalState) []models.Event {
	if proposal == nil {
		return nil
	}

	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		return []models.Event{{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventPlayerShareTransferred,
			EventValue: mustJSON(PlayerShareTransferredPayload{
				FromUserID: proposal.FromUserID,
				ToUserID:   proposal.ToUserID,
				ShareBPS:   proposal.ShareBPS,
			}),
		}}
	case GovernanceProposalTreasuryGrant:
		return []models.Event{{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventTreasuryShareGranted,
			EventValue: mustJSON(TreasuryShareGrantedPayload{
				TargetUserID: proposal.TargetUserID,
				ShareBPS:     proposal.ShareBPS,
			}),
		}}
	case GovernanceProposalTreasuryBuyback:
		return []models.Event{{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventTreasuryShareBoughtBack,
			EventValue: mustJSON(TreasuryShareBoughtBackPayload{
				TargetUserID: proposal.TargetUserID,
				ShareBPS:     proposal.ShareBPS,
			}),
		}}
	case GovernanceProposalAppointCEO:
		return []models.Event{{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventCEOChanged,
			EventValue: mustJSON(CEOChangedPayload{TargetUserID: proposal.TargetUserID}),
		}}
	default:
		return nil
	}
}

func validateGovernanceProposal(state *GameState, proposerUserID int64, payload SubmitGovernanceProposalActionPayload) error {
	switch payload.ProposalType {
	case GovernanceProposalShareTransfer:
		if payload.FromUserID == 0 || payload.ToUserID == 0 {
			return errors.New("from_user_id and to_user_id are required")
		}
		if payload.FromUserID == payload.ToUserID {
			return errors.New("cannot transfer shares to self")
		}
		if payload.ShareBPS <= 0 {
			return errors.New("share_bps must be positive")
		}
		from := activePlayerByID(state, payload.FromUserID)
		to := activePlayerByID(state, payload.ToUserID)
		if from == nil || to == nil {
			return errors.New("proposal target must be an active player")
		}
	case GovernanceProposalTreasuryGrant:
		if payload.TargetUserID == 0 {
			return errors.New("target_user_id is required")
		}
		if payload.ShareBPS <= 0 {
			return errors.New("share_bps must be positive")
		}
		if activePlayerByID(state, payload.TargetUserID) == nil {
			return errors.New("proposal target must be an active player")
		}
	case GovernanceProposalTreasuryBuyback:
		if payload.TargetUserID == 0 {
			return errors.New("target_user_id is required")
		}
		if payload.ShareBPS <= 0 {
			return errors.New("share_bps must be positive")
		}
		target := activePlayerByID(state, payload.TargetUserID)
		if target == nil {
			return errors.New("proposal target must be an active player")
		}
	case GovernanceProposalAppointCEO:
		return errors.New("appoint_ceo proposals are no longer supported")
	default:
		return errors.New("unsupported governance proposal type")
	}

	if activePlayerByID(state, proposerUserID) == nil {
		return errors.New("only active players can submit proposals")
	}
	return nil
}

func clampGovernanceProposalShareBPS(state *GameState, payload *SubmitGovernanceProposalActionPayload) {
	if state == nil || payload == nil || payload.ShareBPS <= 0 {
		return
	}
	switch payload.ProposalType {
	case GovernanceProposalShareTransfer:
		from := activePlayerByID(state, payload.FromUserID)
		if from == nil {
			return
		}
		payload.ShareBPS = minInt(payload.ShareBPS, maxInt(0, from.ShareBPS-MinPlayerShareBPS))
	case GovernanceProposalTreasuryGrant:
		payload.ShareBPS = minInt(payload.ShareBPS, maxInt(0, state.TreasuryShareBPS))
	case GovernanceProposalTreasuryBuyback:
		target := activePlayerByID(state, payload.TargetUserID)
		if target == nil {
			return
		}
		payload.ShareBPS = minInt(payload.ShareBPS, maxInt(0, target.ShareBPS-MinPlayerShareBPS))
	}
}

func validateShareChange(shareBPS int) error {
	if shareBPS <= 0 {
		return errors.New("share_bps must be positive")
	}
	if shareBPS > MaxShareChangeBPS {
		return fmt.Errorf("share_bps cannot exceed %d", MaxShareChangeBPS)
	}
	return nil
}

func activePlayerByID(state *GameState, userID int64) *PlayerState {
	player := state.Players[userID]
	if player == nil || player.IsKicked || player.IsLeft {
		return nil
	}
	return player
}

func activeRealPlayerByID(state *GameState, userID int64) *PlayerState {
	player := activePlayerByID(state, userID)
	if player == nil || player.IsBot || player.UserID <= 0 {
		return nil
	}
	return player
}

func nextGovernanceProposalID(state *GameState) int {
	maxID := 0
	for proposalID := range state.GovernanceProposals {
		if proposalID > maxID {
			maxID = proposalID
		}
	}
	return maxID + 1
}

func findMatchingGovernanceProposal(state *GameState, payload SubmitGovernanceProposalActionPayload) *GovernanceProposalState {
	for _, proposalID := range state.GovernanceProposalOrder {
		proposal := state.GovernanceProposals[proposalID]
		if proposal == nil || proposal.ProposalType != payload.ProposalType {
			continue
		}
		switch payload.ProposalType {
		case GovernanceProposalShareTransfer:
			if proposal.FromUserID == payload.FromUserID && proposal.ToUserID == payload.ToUserID {
				return proposal
			}
		case GovernanceProposalTreasuryGrant, GovernanceProposalTreasuryBuyback:
			if proposal.TargetUserID == payload.TargetUserID {
				return proposal
			}
		}
	}
	return nil
}

func isDecisionInCurrentShowcase(state *GameState, decision string) bool {
	options := state.MajorVoteOptions
	if len(options) == 0 {
		options = sortedAvailableDecisions(state.Available)
	}
	for _, option := range options {
		if option == decision {
			return true
		}
	}
	return false
}

func (e *Engine) votingRoundStartedEvent(state *GameState, actor *models.User, round int) models.Event {
	return models.Event{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventVotingRoundStarted,
		EventValue: mustJSON(VotingRoundStartedPayload{
			Round:             round,
			ShowcaseDecisions: e.majorShowcase(state.Available, state.MoleTargets, state.MoleSabotage),
		}),
	}
}

func (e *Engine) majorShowcase(available map[string]bool, targets []string, sabotage string) []string {
	targetSet := map[string]bool{}
	for _, target := range targets {
		targetSet[target] = true
	}
	if sabotage != "" {
		targetSet[sabotage] = true
	}

	moleOptions := []string{}
	cleanOptions := []string{}
	for _, decision := range allDecisions {
		if !available[decision] {
			continue
		}
		if targetSet[decision] {
			moleOptions = append(moleOptions, decision)
		} else {
			cleanOptions = append(cleanOptions, decision)
		}
	}

	if len(moleOptions) < 2 || len(cleanOptions) < 2 {
		return sortedAvailableDecisions(available)
	}

	showcase := append(randomDecisionSubset(e, moleOptions, 2), randomDecisionSubset(e, cleanOptions, 2)...)
	sort.Strings(showcase)
	return showcase
}

func (e *Engine) memorandumAssignmentEvents(state *GameState, actor *models.User, targets []string, sabotage string) []models.Event {
	events := []models.Event{}
	for _, player := range activePlayers(state) {
		if player.UserID == state.MoleUserID {
			continue
		}
		memorandumType := state.MemorandumPreferences[player.UserID]
		if memorandumType == "" {
			memorandumType = MemorandumTypeOpportunity
		}
		events = append(events, models.Event{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventMemorandumAssigned,
			EventValue: mustJSON(MemorandumAssignedPayload{
				UserID:    player.UserID,
				Type:      memorandumType,
				Decisions: e.randomMemorandumDecisions(memorandumType, targets, sabotage),
			}),
		})
	}
	return events
}

func (e *Engine) randomMemorandumDecisions(memorandumType MemorandumType, targets []string, sabotage string) []string {
	targetSet := moleObjectiveSet(targets, sabotage)
	candidates := [][]string{}
	for i := 0; i < len(allDecisions); i++ {
		for j := i + 1; j < len(allDecisions); j++ {
			for k := j + 1; k < len(allDecisions); k++ {
				trio := []string{allDecisions[i], allDecisions[j], allDecisions[k]}
				if memorandumMatches(trio, targetSet, memorandumType) {
					candidates = append(candidates, trio)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return randomDecisionSubset(e, allDecisions, 3)
	}
	e.rngMu.Lock()
	index := e.rng.Intn(len(candidates))
	e.rngMu.Unlock()
	out := append([]string(nil), candidates[index]...)
	sort.Strings(out)
	return out
}

func memorandumMatches(decisions []string, targetSet map[string]bool, memorandumType MemorandumType) bool {
	hasMoleTarget := false
	hasCleanDecision := false
	for _, decision := range decisions {
		if targetSet[decision] {
			hasMoleTarget = true
		} else {
			hasCleanDecision = true
		}
	}
	switch memorandumType {
	case MemorandumTypeRisk:
		return hasMoleTarget
	default:
		return hasCleanDecision
	}
}

func moleObjectiveSet(targets []string, sabotage string) map[string]bool {
	out := map[string]bool{}
	for _, target := range targets {
		out[target] = true
	}
	if sabotage != "" {
		out[sabotage] = true
	}
	return out
}

func randomDecisionSubset(e *Engine, decisions []string, count int) []string {
	shuffled := append([]string(nil), decisions...)
	e.shuffleWithRNG(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	out := append([]string(nil), shuffled[:count]...)
	sort.Strings(out)
	return out
}

func systemChatPayloadEvents(gameID int64, eventUserID int64, payload ChatMessageSentPayload) []models.Event {
	payload.UserID = 0
	payload.Kind = "system"
	payload.Collapsible = payload.Collapsible || len(payload.Details) > 0
	if payload.Message == "" {
		payload.Message = payload.Summary
		if payload.Message == "" {
			payload.Message = payload.Title
		}
	}
	return []models.Event{{
		GameID:     gameID,
		UserID:     &eventUserID,
		ActorName:  "Система",
		EventType:  models.EventChatMessageSent,
		EventValue: mustJSON(payload),
	}}
}

func systemChatEvents(gameID int64, eventUserID int64, message string) []models.Event {
	chunks := splitRunes(message, MaxChatMessageLength)
	events := make([]models.Event, 0, len(chunks))
	for _, chunk := range chunks {
		events = append(events, models.Event{
			GameID:     gameID,
			UserID:     &eventUserID,
			ActorName:  "Система",
			EventType:  models.EventChatMessageSent,
			EventValue: mustJSON(ChatMessageSentPayload{UserID: 0, Message: chunk, Kind: "system"}),
		})
	}
	return events
}

func splitRunes(value string, limit int) []string {
	if limit <= 0 || len([]rune(value)) <= limit {
		return []string{value}
	}
	runes := []rune(value)
	out := []string{}
	for len(runes) > 0 {
		take := limit
		if len(runes) < take {
			take = len(runes)
		}
		out = append(out, string(runes[:take]))
		runes = runes[take:]
	}
	return out
}

func formatMajorVoteSystemMessage(state *GameState, outcome string, decision string, reason string) ChatMessageSentPayload {
	report := buildRoundReport(state, state.CurrentRound, outcome, decision, reason)
	details := []string{}
	for _, vote := range report.Votes {
		label := decisionLabelForChat(vote.Decision)
		if vote.Abstain {
			label = "Воздержались"
		}
		names := make([]string, 0, len(vote.Voters))
		for _, voter := range vote.Voters {
			names = append(names, playerNameOrFallbackForChat(state, voter.UserID, voter.Name))
		}
		details = append(details, fmt.Sprintf("%s: %s (%s)", label, formatBPS(vote.ShareBPS), strings.Join(names, ", ")))
	}

	summary := fmt.Sprintf("Раунд %d: решение не принято (%s). Совет уходит на повторное обсуждение.", state.CurrentRound, reason)
	systemEventType := "major_vote_rejected"
	tone := "warning"
	titleValue := "не принято"
	if outcome == "accepted" {
		summary = majorVoteNarrative(state, decision)
		systemEventType = "major_vote_accepted"
		tone = "success"
		titleValue = decision
		if rewardDetail := majorDecisionRewardDetail(state, decision); rewardDetail != "" {
			details = append(details, rewardDetail)
		}
	}
	return ChatMessageSentPayload{
		Title:           fmt.Sprintf("Итоги major vote: %s", titleValue),
		Summary:         summary,
		Message:         summary,
		Details:         details,
		SystemEventType: systemEventType,
		Tone:            tone,
		Collapsible:     true,
	}
}

func majorDecisionRewardDetail(state *GameState, decision string) string {
	reward := majorDecisionReward(state, decision)
	if reward.BPS <= 0 || len(reward.Voters) == 0 {
		return ""
	}

	names := make([]string, 0, len(reward.Voters))
	for _, voter := range reward.Voters {
		names = append(names, playerNameOrFallbackForChat(state, voter.UserID, voter.Name))
	}

	rewardName := "доле"
	if reward.Kind == "authority" {
		rewardName = "полномочиям"
	}
	return fmt.Sprintf("Бонус +%s к %s за принятое решение получили: %s", formatBPS(reward.BPS), rewardName, strings.Join(names, ", "))
}

func formatGovernanceSystemMessage(state *GameState, outcome string, proposalID int, reason string) ChatMessageSentPayload {
	report := buildGovernanceReport(state, state.GovernanceRound, outcome, proposalID, reason)
	details := []string{}
	for _, vote := range report.Votes {
		label := vote.ProposalTitle
		if label == "" && vote.Proposal != nil {
			label = describeGovernanceProposalForChat(state, vote.Proposal)
		}
		if label == "" {
			label = "Предложение"
		}
		if vote.Abstain {
			label = "Воздержались"
		}
		voters := make([]string, 0, len(vote.Voters))
		for _, voter := range vote.Voters {
			voters = append(voters, playerNameOrFallbackForChat(state, voter.UserID, voter.Name))
		}
		details = append(details, fmt.Sprintf("%s: %s (%s)", label, formatBPS(vote.VotingPowerBPS), strings.Join(voters, ", ")))
	}

	summary := fmt.Sprintf("Раунд %d: корпоративный маневр не принят (%s). Баланс влияния остается прежним.", state.GovernanceRound, reason)
	systemEventType := "governance_rejected"
	tone := "warning"
	if outcome == "accepted" {
		summary = governanceVoteNarrative(state, proposalID)
		systemEventType = "governance_accepted"
		tone = "success"
	}
	return ChatMessageSentPayload{
		Title:           "Итоги governance",
		Summary:         summary,
		Message:         summary,
		Details:         details,
		SystemEventType: systemEventType,
		Tone:            tone,
		Collapsible:     true,
	}
}

func sabotageAcceptedSystemMessage(state *GameState, decision string) ChatMessageSentPayload {
	companyName := companyNameForChat(state)
	summary := fmt.Sprintf("В отчетах %s появились строки, которые никто не хочет подписывать. Компания явно идет не туда.", companyName)
	return ChatMessageSentPayload{
		Title:           "Тревожный сигнал",
		Summary:         summary,
		Message:         summary,
		Details:         []string{fmt.Sprintf("Принята диверсия: %s.", decisionLabelForChat(decision))},
		SystemEventType: "sabotage_accepted",
		Tone:            "danger",
		Collapsible:     true,
	}
}

func moleRevealSystemMessage(state *GameState) ChatMessageSentPayload {
	companyName := companyNameForChat(state)
	moleName := playerNameForChat(state, state.MoleUserID)
	targets := make([]string, 0, len(state.MoleTargets))
	for _, target := range state.MoleTargets {
		targets = append(targets, decisionLabelForChat(target))
	}
	details := []string{fmt.Sprintf("Крот: %s.", moleName)}
	if len(targets) > 0 {
		details = append(details, fmt.Sprintf("Подкопы: %s.", strings.Join(targets, "; ")))
	}
	if state.MoleSabotage != "" {
		details = append(details, fmt.Sprintf("Диверсия: %s.", decisionLabelForChat(state.MoleSabotage)))
	}
	summary := fmt.Sprintf("%s: %s был кротом. Все цели раскрыты.", companyName, moleName)
	return ChatMessageSentPayload{
		Title:           "Крот раскрыт",
		Summary:         summary,
		Message:         summary,
		Details:         details,
		SystemEventType: "mole_revealed",
		Tone:            "danger",
		Collapsible:     true,
	}
}

func formatMajorVoteSummary(state *GameState, outcome string, decision string, reason string) string {
	var builder strings.Builder
	if outcome == "accepted" {
		builder.WriteString(fmt.Sprintf("Итоги major vote в %s: принято %s.", companyNameForChat(state), decisionLabelForChat(decision)))
	} else {
		builder.WriteString(fmt.Sprintf("Итоги major vote в %s: решение не принято (%s).", companyNameForChat(state), reason))
	}

	report := buildRoundReport(state, state.CurrentRound, outcome, decision, reason)
	if len(report.Votes) > 0 {
		builder.WriteString(" Голоса: ")
	}
	for i, vote := range report.Votes {
		if i > 0 {
			builder.WriteString("; ")
		}
		label := decisionLabelForChat(vote.Decision)
		if vote.Abstain {
			label = "Воздержались"
		}
		names := make([]string, 0, len(vote.Voters))
		for _, voter := range vote.Voters {
			names = append(names, playerNameOrFallbackForChat(state, voter.UserID, voter.Name))
		}
		builder.WriteString(fmt.Sprintf("%s — %s (%s)", label, formatBPS(vote.ShareBPS), strings.Join(names, ", ")))
	}
	return builder.String()
}

func formatGovernanceSummary(state *GameState, outcome string, proposalID int, reason string) string {
	var builder strings.Builder
	if outcome == "accepted" {
		builder.WriteString(fmt.Sprintf("Итоги governance в %s: принято %s.", companyNameForChat(state), describeGovernanceProposalForChat(state, state.GovernanceProposals[proposalID])))
	} else {
		builder.WriteString(fmt.Sprintf("Итоги governance в %s: предложение не принято (%s).", companyNameForChat(state), reason))
	}

	report := buildGovernanceReport(state, state.GovernanceRound, outcome, proposalID, reason)
	if len(report.Votes) > 0 {
		builder.WriteString(" Голоса: ")
	}
	for i, vote := range report.Votes {
		if i > 0 {
			builder.WriteString("; ")
		}
		label := vote.ProposalTitle
		if label == "" && vote.Proposal != nil {
			label = describeGovernanceProposalForChat(state, vote.Proposal)
		}
		if label == "" {
			label = "Предложение"
		}
		if vote.Abstain {
			label = "Воздержались"
		}
		voters := make([]string, 0, len(vote.Voters))
		for _, voter := range vote.Voters {
			voters = append(voters, fmt.Sprintf(
				"%s %s + %s = %s",
				playerNameOrFallbackForChat(state, voter.UserID, voter.Name),
				formatBPS(voter.ShareBPS),
				formatBPS(voter.AuthorityBPS),
				formatBPS(voter.VotingPowerBPS),
			))
		}
		builder.WriteString(fmt.Sprintf("%s — %s (%s)", label, formatBPS(vote.VotingPowerBPS), strings.Join(voters, ", ")))
	}
	return builder.String()
}

func decisionLabelForChat(decision string) string {
	if title := decisionTitles[decision]; title != "" {
		return fmt.Sprintf("%s — %s", decision, title)
	}
	return decision
}

func describeGovernanceProposalForChat(state *GameState, proposal *GovernanceProposalState) string {
	if proposal == nil {
		return "неизвестное предложение"
	}
	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		return fmt.Sprintf("%s передает %s игроку %s", playerNameForChat(state, proposal.FromUserID), formatBPS(proposal.ShareBPS), playerNameForChat(state, proposal.ToUserID))
	case GovernanceProposalTreasuryGrant:
		return fmt.Sprintf("выдать %s из резерва игроку %s", formatBPS(proposal.ShareBPS), playerNameForChat(state, proposal.TargetUserID))
	case GovernanceProposalTreasuryBuyback:
		return fmt.Sprintf("оштрафовать %s на %s в пользу резерва", playerNameForChat(state, proposal.TargetUserID), formatBPS(proposal.ShareBPS))
	case GovernanceProposalAppointCEO:
		return fmt.Sprintf("назначить CEO: %s", playerNameForChat(state, proposal.TargetUserID))
	default:
		return "корпоративный маневр"
	}
}

func playerNameForChat(state *GameState, userID int64) string {
	if state != nil {
		if player := state.Players[userID]; player != nil && player.Name != "" {
			return player.Name
		}
	}
	return fmt.Sprintf("Игрок #%d", userID)
}

func playerNameOrFallbackForChat(state *GameState, userID int64, fallbackName string) string {
	name := strings.TrimSpace(fallbackName)
	if name == "" {
		name = playerNameForChat(state, userID)
	}
	return name
}

func effectivePlayerPosition(player *PlayerState) string {
	if player == nil {
		return ""
	}
	if player.IsCEO {
		return "CEO"
	}
	return strings.TrimSpace(player.Position)
}

func (e *Engine) randomGeneratedPosition() string {
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	return generatedPositions[e.rng.Intn(len(generatedPositions))]
}

func isAllowedChatReaction(emoji string) bool {
	switch emoji {
	case "👍", "🤝", "💼", "📈", "⚠️", "🕵️", "✅", "🔥":
		return true
	default:
		return false
	}
}

func chatMessageExists(state *GameState, messageID int64) bool {
	if state == nil {
		return false
	}
	for _, message := range state.ChatMessages {
		if message.ID == messageID {
			return true
		}
	}
	return false
}

func majorVoteNarrative(state *GameState, decision string) string {
	if state != nil && decision == state.MoleSabotage {
		if text := majorVoteSabotageNarratives[decision]; text != "" {
			return text
		}
	}
	if text := majorVoteCleanNarratives[decision]; text != "" {
		return text
	}
	return fmt.Sprintf("Совет принял решение %s. Влияние сторонников инициативы растет.", decisionLabelForChat(decision))
}

var majorVoteCleanNarratives = map[string]string{
	"A": "Совет одобрил выпуск облигаций. Капитал приходит вовремя, а сторонники инициативы укрепляют позиции.",
	"B": "Совет одобрил агрессивный выход в новый регион. Рынок в восторге. Регуляторы пока молчат. Те, кто поддержал инициативу, усиливают своё влияние в совете.",
	"C": "Совет утвердил выплату дивидендов. Акционеры довольны, а сторонники решения получают больше веса в кулуарах.",
	"D": "Совет дал зеленый свет экспериментальному продукту. Команда получает шанс на рывок, а инициаторы выглядят смелыми визионерами.",
	"E": "Совет поддержал сделку слияния. Переговорная позиция компании крепнет, а голоса за сделку звучат убедительнее.",
	"F": "Совет одобрил оптимизацию персонала. Расходы взяты под контроль, а сторонники жесткой дисциплины набирают влияние.",
	"G": "Совет принял агрессивную налоговую стратегию. Финансовая модель выглядит легче, а ее защитники получают дополнительные полномочия.",
	"H": "Совет согласовал обратный выкуп акций. Доля капитала концентрируется, а сторонники решения становятся заметнее.",
}

var majorVoteSabotageNarratives = map[string]string{
	"A": "Совет протолкнул выпуск облигаций на токсичных условиях. Долг уже давит на компанию, а доверие инвесторов трещит.",
	"B": "Совет сорвался в чужой регион без подготовки. Рынок встречает компанию штрафами, утечками бюджета и холодной прессой.",
	"C": "Совет вытащил деньги в дивиденды в самый плохой момент. Резерв проседает, а операционные риски остаются без прикрытия.",
	"D": "Совет запустил сырой экспериментальный продукт. Первые пользователи видят сбои, команда тушит пожар вместо развития.",
	"E": "Совет одобрил слияние с проблемным активом. На баланс заходят чужие долги, конфликты и токсичные обязательства.",
	"F": "Совет провел оптимизацию вслепую. Из компании уходят критичные люди, процессы рушатся, а экономия выглядит фикцией.",
	"G": "Совет принял налоговую схему на грани. Регуляторы уже задают вопросы, а юридический риск становится публичным.",
	"H": "Совет запустил обратный выкуп в неподходящий момент. Компания сжигает ликвидность, пока реальные угрозы остаются без финансирования.",
}

func governanceVoteNarrative(state *GameState, proposalID int) string {
	proposal := state.GovernanceProposals[proposalID]
	if proposal == nil {
		return fmt.Sprintf("Раунд %d: корпоративный маневр принят. Расстановка сил меняется.", state.GovernanceRound)
	}
	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		return fmt.Sprintf("Раунд %d: совет утвердил передачу доли. Влияние переходит от %s к %s.", state.GovernanceRound, playerNameForChat(state, proposal.FromUserID), playerNameForChat(state, proposal.ToUserID))
	case GovernanceProposalTreasuryGrant:
		return fmt.Sprintf("Раунд %d: совет выделил долю из резерва. %s получает дополнительный вес в капитале.", state.GovernanceRound, playerNameForChat(state, proposal.TargetUserID))
	case GovernanceProposalTreasuryBuyback:
		return fmt.Sprintf("Раунд %d: совет провел выкуп в резерв. Доля %s сокращена в пользу компании.", state.GovernanceRound, playerNameForChat(state, proposal.TargetUserID))
	default:
		return fmt.Sprintf("Раунд %d: корпоративный маневр принят. Расстановка сил меняется.", state.GovernanceRound)
	}
}

func companyNameForChat(state *GameState) string {
	if state != nil && state.CompanyName != "" {
		return state.CompanyName
	}
	if state != nil && state.Title != "" {
		return state.Title
	}
	return "Компания"
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func formatBPS(bps int) string {
	if bps%100 == 0 {
		return fmt.Sprintf("%d%%", bps/100)
	}
	return fmt.Sprintf("%.1f%%", float64(bps)/100)
}

func normalizeMoleObjectives(payload SelectMoleObjectivesActionPayload) ([]string, string, error) {
	if len(payload.Targets) != 3 {
		return nil, "", errors.New("exactly 3 mole targets are required")
	}
	sabotage := strings.TrimSpace(payload.Sabotage)
	if sabotage == "" {
		return nil, "", errors.New("sabotage is required")
	}
	if !isDecisionID(sabotage) {
		return nil, "", errors.New("sabotage is not a valid decision")
	}

	seen := map[string]bool{}
	targets := make([]string, 0, len(payload.Targets))
	for _, rawTarget := range payload.Targets {
		target := strings.TrimSpace(rawTarget)
		if !isDecisionID(target) {
			return nil, "", errors.New("target is not a valid decision")
		}
		if seen[target] {
			return nil, "", errors.New("mole targets must be unique")
		}
		if target == sabotage {
			return nil, "", errors.New("sabotage cannot also be a mole target")
		}
		seen[target] = true
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets, sabotage, nil
}

func isDecisionID(decision string) bool {
	for _, available := range allDecisions {
		if decision == available {
			return true
		}
	}
	return false
}

func isMemorandumType(value MemorandumType) bool {
	return value == MemorandumTypeOpportunity || value == MemorandumTypeRisk
}

func detectWinner(state *GameState) (string, string) {
	molePoints, playersPoints := victoryPoints(state)
	if molePoints >= 3 {
		return "mole", "mole_targets_collected"
	}
	if playersPoints >= 3 {
		return "players", "three_clean_decisions_collected"
	}
	return "", ""
}

func victoryPoints(state *GameState) (int, int) {
	accepted := map[string]bool{}
	targets := map[string]bool{}
	for _, target := range state.MoleTargets {
		targets[target] = true
	}

	molePoints := 0
	playersPoints := 0
	for _, decision := range state.AcceptedOrder {
		if accepted[decision] {
			continue
		}
		accepted[decision] = true
		switch {
		case state.MoleSabotage != "" && decision == state.MoleSabotage:
			molePoints += 2
		case targets[decision]:
			molePoints++
		default:
			playersPoints++
		}
	}
	return molePoints, playersPoints
}

func activePlayers(state *GameState) []*PlayerState {
	out := make([]*PlayerState, 0, len(state.Players))
	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player != nil && !player.IsKicked && !player.IsLeft {
			out = append(out, player)
		}
	}
	return out
}

func activeRealPlayers(state *GameState) []*PlayerState {
	players := activePlayers(state)
	out := make([]*PlayerState, 0, len(players))
	for _, player := range players {
		if player != nil && !player.IsBot && player.UserID > 0 {
			out = append(out, player)
		}
	}
	return out
}

func (e *Engine) shufflePlayers(players []*PlayerState) {
	e.shuffleWithRNG(len(players), func(i, j int) {
		players[i], players[j] = players[j], players[i]
	})
}

func (e *Engine) randomTargets() []string {
	targets := append([]string(nil), allDecisions...)
	e.shuffleWithRNG(len(targets), func(i, j int) {
		targets[i], targets[j] = targets[j], targets[i]
	})
	sort.Strings(targets[:3])
	return append([]string(nil), targets[:3]...)
}

func cloneState(state *GameState) *GameState {
	cloned := *state
	cloned.CurrentVotes = make(map[int64]VoteState, len(state.CurrentVotes))
	for k, v := range state.CurrentVotes {
		cloned.CurrentVotes[k] = v
	}
	cloned.MemorandumPreferences = make(map[int64]MemorandumType, len(state.MemorandumPreferences))
	for k, v := range state.MemorandumPreferences {
		cloned.MemorandumPreferences[k] = v
	}
	cloned.Memorandums = make(map[int64]MemorandumState, len(state.Memorandums))
	for k, v := range state.Memorandums {
		v.Decisions = append([]string(nil), v.Decisions...)
		cloned.Memorandums[k] = v
	}
	cloned.GovernanceProposals = make(map[int]*GovernanceProposalState, len(state.GovernanceProposals))
	for k, v := range state.GovernanceProposals {
		cloned.GovernanceProposals[k] = cloneGovernanceProposal(v)
	}
	cloned.GovernanceProposalOrder = append([]int(nil), state.GovernanceProposalOrder...)
	cloned.GovernanceSubmissions = make(map[int64]GovernanceSubmissionState, len(state.GovernanceSubmissions))
	for k, v := range state.GovernanceSubmissions {
		cloned.GovernanceSubmissions[k] = v
	}
	cloned.GovernanceVotes = make(map[int64]GovernanceVoteState, len(state.GovernanceVotes))
	for k, v := range state.GovernanceVotes {
		cloned.GovernanceVotes[k] = v
	}
	cloned.Available = make(map[string]bool, len(state.Available))
	for k, v := range state.Available {
		cloned.Available[k] = v
	}
	cloned.MajorVoteOptions = append([]string(nil), state.MajorVoteOptions...)
	if state.MajorVoteUnlockedAt != nil {
		unlockedAt := *state.MajorVoteUnlockedAt
		cloned.MajorVoteUnlockedAt = &unlockedAt
	}
	if state.PhaseStartedAt != nil {
		phaseStartedAt := *state.PhaseStartedAt
		cloned.PhaseStartedAt = &phaseStartedAt
	}
	if state.PhaseDeadlineAt != nil {
		phaseDeadlineAt := *state.PhaseDeadlineAt
		cloned.PhaseDeadlineAt = &phaseDeadlineAt
	}
	cloned.ChatReactions = make(map[int64]map[string]map[int64]bool, len(state.ChatReactions))
	for messageID, reactions := range state.ChatReactions {
		cloned.ChatReactions[messageID] = make(map[string]map[int64]bool, len(reactions))
		for emoji, users := range reactions {
			cloned.ChatReactions[messageID][emoji] = make(map[int64]bool, len(users))
			for userID, reacted := range users {
				cloned.ChatReactions[messageID][emoji][userID] = reacted
			}
		}
	}
	cloned.AcceptedOrder = append([]string(nil), state.AcceptedOrder...)
	cloned.MoleTargets = append([]string(nil), state.MoleTargets...)
	cloned.MoleSabotage = state.MoleSabotage
	cloned.RejectedOrder = append([]string(nil), state.RejectedOrder...)
	cloned.RoundReports = append([]RoundReport(nil), state.RoundReports...)
	cloned.GovernanceReports = append([]GovernanceReport(nil), state.GovernanceReports...)
	cloned.PlayerOrder = append([]int64(nil), state.PlayerOrder...)
	cloned.Players = make(map[int64]*PlayerState, len(state.Players))
	for id, player := range state.Players {
		cp := *player
		cloned.Players[id] = &cp
	}
	return &cloned
}

func decodeActionPayload(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode action payload: %w", err)
	}
	return nil
}
