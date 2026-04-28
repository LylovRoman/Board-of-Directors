package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"agentbackend/internal/models"
)

func (e *Engine) handleJoinGame(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("game already started")
	}
	if len(activePlayers(state)) >= MaxPlayers {
		return nil, fmt.Errorf("game is full: max %d players", MaxPlayers)
	}
	if player := state.Players[actor.ID]; player != nil {
		if player.IsKicked {
			return nil, errors.New("kicked player cannot rejoin")
		}
		return nil, errors.New("player already joined")
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventPlayerJoined,
		EventValue: mustJSON(PlayerJoinedPayload{UserID: actor.ID, Name: actor.Name}),
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

	player := state.Players[payload.UserID]
	if player == nil || player.IsKicked {
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
	targets := e.randomTargets()

	events := []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGameStarted,
		EventValue: "{}",
	}, {
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventMoleSelected,
		EventValue: mustJSON(MoleSelectedPayload{UserID: mole.UserID}),
	}, {
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventMoleTargetsGenerated,
		EventValue: mustJSON(MoleTargetsGeneratedPayload{Targets: targets}),
	}}

	for i, player := range shuffledPlayers {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventPlayerReceivedShare,
			EventValue: mustJSON(PlayerReceivedSharePayload{UserID: player.UserID, ShareBPS: shares[i]}),
		})
	}

	events = append(events,
		models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventCEOSelected,
			EventValue: mustJSON(CEOSelectedPayload{UserID: ceo.UserID}),
		},
		models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventVotingRoundStarted,
			EventValue: mustJSON(VotingRoundStartedPayload{Round: 1}),
		},
	)

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

	player := state.Players[actor.ID]
	if player == nil || player.IsKicked {
		return nil, errors.New("only active players can vote")
	}
	if hasPlayerVoted(state, actor.ID) {
		return nil, errors.New("player already voted this round")
	}

	var payload VoteActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}

	if !payload.Abstain {
		if payload.Decision == nil || *payload.Decision == "" {
			return nil, errors.New("decision is required when abstain is false")
		}
		if !state.Available[*payload.Decision] {
			return nil, errors.New("decision is not available")
		}
	} else if player.IsCEO {
		return nil, errors.New("ceo cannot abstain")
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
	player := state.Players[actor.ID]
	if player == nil || player.IsKicked {
		return nil, errors.New("only active players can submit proposals")
	}
	if _, ok := state.GovernanceSubmissions[actor.ID]; ok {
		return nil, errors.New("governance proposal already submitted")
	}

	var payload SubmitGovernanceProposalActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	if err := validateGovernanceProposal(state, actor.ID, payload); err != nil {
		return nil, err
	}

	proposalID := nextGovernanceProposalID(state)
	event := models.Event{
		GameID:    state.GameID,
		UserID:    &actor.ID,
		ActorName: actor.Name,
		EventType: models.EventGovernanceProposalSubmitted,
		EventValue: mustJSON(GovernanceProposalSubmittedPayload{
			Round:          state.GovernanceRound,
			ProposalID:     proposalID,
			ProposerUserID: actor.ID,
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
	events = append(events, governanceEventsAfterSubmission(projected, actor)...)
	return events, nil
}

func (e *Engine) handleSkipGovernanceProposal(state *GameState, actor *models.User) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, errors.New("game is not active")
	}
	if state.Phase != GamePhaseGovernanceProposal {
		return nil, errors.New("governance proposal phase is not active")
	}
	player := state.Players[actor.ID]
	if player == nil || player.IsKicked {
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
	events = append(events, governanceEventsAfterSubmission(projected, actor)...)
	return events, nil
}

func (e *Engine) handleGovernanceVote(state *GameState, actor *models.User, raw json.RawMessage) ([]models.Event, error) {
	player := state.Players[actor.ID]
	if player == nil || player.IsKicked {
		return nil, errors.New("only active players can vote")
	}
	if _, ok := state.GovernanceVotes[actor.ID]; ok {
		return nil, errors.New("player already voted this round")
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

	events = append(events, resolveGovernance(projected, actor)...)
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
	}

	events = append(events, models.Event{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventVotingRoundStarted,
		EventValue: mustJSON(VotingRoundStartedPayload{Round: state.CurrentRound + 1}),
	})

	return events
}

func majorDecisionRewardEvents(state *GameState, actor *models.User, decision string) []models.Event {
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
	if len(voters) == 0 || state.TreasuryShareBPS <= 0 {
		return nil
	}

	rewardBPS := MajorDecisionRewardBPS
	if totalReward := rewardBPS * len(voters); totalReward > state.TreasuryShareBPS {
		rewardBPS = state.TreasuryShareBPS / len(voters)
	}
	if rewardBPS <= 0 {
		return nil
	}

	events := make([]models.Event, 0, len(voters))
	for _, voter := range voters {
		events = append(events, models.Event{
			GameID:    state.GameID,
			UserID:    &actor.ID,
			ActorName: actor.Name,
			EventType: models.EventTreasuryShareGranted,
			EventValue: mustJSON(TreasuryShareGrantedPayload{
				TargetUserID: voter.UserID,
				ShareBPS:     rewardBPS,
			}),
		})
	}
	return events
}

func governanceEventsAfterSubmission(state *GameState, actor *models.User) []models.Event {
	if len(state.GovernanceSubmissions) != len(activePlayers(state)) {
		return nil
	}

	if len(state.GovernanceProposalOrder) == 0 {
		return []models.Event{{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventVotingRoundStarted,
			EventValue: mustJSON(VotingRoundStartedPayload{Round: state.CurrentRound + 1}),
		}}
	}

	return []models.Event{{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventGovernanceVotingStarted,
		EventValue: mustJSON(GovernanceVotingStartedPayload{Round: state.GovernanceRound}),
	}}
}

func resolveGovernance(state *GameState, actor *models.User) []models.Event {
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
	} else {
		events = append(events, models.Event{
			GameID:     state.GameID,
			UserID:     &actor.ID,
			ActorName:  actor.Name,
			EventType:  models.EventGovernanceProposalRejected,
			EventValue: mustJSON(GovernanceProposalRejectedPayload{Round: state.GovernanceRound, Reason: "tie_or_no_votes"}),
		})
	}

	events = append(events, models.Event{
		GameID:     state.GameID,
		UserID:     &actor.ID,
		ActorName:  actor.Name,
		EventType:  models.EventVotingRoundStarted,
		EventValue: mustJSON(VotingRoundStartedPayload{Round: state.CurrentRound + 1}),
	})
	return events
}

func resolveDecision(state *GameState) (string, []string, bool) {
	scores := map[string]int{}
	for _, vote := range state.CurrentVotes {
		if vote.Abstain || vote.Decision == nil {
			continue
		}
		player := state.Players[vote.UserID]
		if player == nil || player.IsKicked {
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
		player := state.Players[vote.UserID]
		if player == nil || player.IsKicked || state.GovernanceProposals[*vote.ProposalID] == nil {
			continue
		}
		scores[*vote.ProposalID] += player.ShareBPS
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
		if err := validateShareChange(payload.ShareBPS); err != nil {
			return err
		}
		from := activePlayerByID(state, payload.FromUserID)
		to := activePlayerByID(state, payload.ToUserID)
		if from == nil || to == nil {
			return errors.New("proposal target must be an active player")
		}
		if from.ShareBPS-payload.ShareBPS < MinPlayerShareBPS {
			return errors.New("player share cannot go below minimum")
		}
	case GovernanceProposalTreasuryGrant:
		if payload.TargetUserID == 0 {
			return errors.New("target_user_id is required")
		}
		if err := validateShareChange(payload.ShareBPS); err != nil {
			return err
		}
		if activePlayerByID(state, payload.TargetUserID) == nil {
			return errors.New("proposal target must be an active player")
		}
		if state.TreasuryShareBPS-payload.ShareBPS < 0 {
			return errors.New("treasury share cannot go below 0")
		}
	case GovernanceProposalTreasuryBuyback:
		if payload.TargetUserID == 0 {
			return errors.New("target_user_id is required")
		}
		if err := validateShareChange(payload.ShareBPS); err != nil {
			return err
		}
		target := activePlayerByID(state, payload.TargetUserID)
		if target == nil {
			return errors.New("proposal target must be an active player")
		}
		if target.ShareBPS-payload.ShareBPS < MinPlayerShareBPS {
			return errors.New("player share cannot go below minimum")
		}
	case GovernanceProposalAppointCEO:
		if payload.TargetUserID == 0 {
			return errors.New("target_user_id is required")
		}
		if activePlayerByID(state, payload.TargetUserID) == nil {
			return errors.New("proposal target must be an active player")
		}
		if payload.TargetUserID == state.CEOUserID {
			return errors.New("target player is already CEO")
		}
	default:
		return errors.New("unsupported governance proposal type")
	}

	if activePlayerByID(state, proposerUserID) == nil {
		return errors.New("only active players can submit proposals")
	}
	return nil
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
	if player == nil || player.IsKicked {
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

func detectWinner(state *GameState) (string, string) {
	accepted := map[string]bool{}
	cleanCount := 0
	targets := map[string]bool{}
	for _, target := range state.MoleTargets {
		targets[target] = true
	}
	for _, decision := range state.AcceptedOrder {
		if accepted[decision] {
			continue
		}
		accepted[decision] = true
		if !targets[decision] {
			cleanCount++
		}
	}

	allTargetsAccepted := true
	for _, target := range state.MoleTargets {
		if !accepted[target] {
			allTargetsAccepted = false
			break
		}
	}
	if allTargetsAccepted {
		return "mole", "mole_targets_collected"
	}
	if cleanCount >= 3 {
		return "players", "three_clean_decisions_collected"
	}
	return "", ""
}

func activePlayers(state *GameState) []*PlayerState {
	out := make([]*PlayerState, 0, len(state.Players))
	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player != nil && !player.IsKicked {
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
	cloned.GovernanceProposals = make(map[int]*GovernanceProposalState, len(state.GovernanceProposals))
	for k, v := range state.GovernanceProposals {
		if v == nil {
			continue
		}
		cp := *v
		cloned.GovernanceProposals[k] = &cp
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
	cloned.AcceptedOrder = append([]string(nil), state.AcceptedOrder...)
	cloned.MoleTargets = append([]string(nil), state.MoleTargets...)
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
