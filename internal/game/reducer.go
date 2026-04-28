package game

import (
	"encoding/json"
	"fmt"
	"sort"

	"agentbackend/internal/models"
)

func BuildState(gameID int64, title string, events []models.Event) (*GameState, error) {
	state := &GameState{
		GameID:                gameID,
		Title:                 title,
		Status:                GameStatusLobby,
		Players:               map[int64]*PlayerState{},
		CurrentVotes:          map[int64]VoteState{},
		GovernanceProposals:   map[int]*GovernanceProposalState{},
		GovernanceSubmissions: map[int64]GovernanceSubmissionState{},
		GovernanceVotes:       map[int64]GovernanceVoteState{},
		Available:             map[string]bool{},
	}

	for _, decision := range allDecisions {
		state.Available[decision] = true
	}

	for _, event := range events {
		if err := ApplyEvent(state, event); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func ApplyEvent(state *GameState, event models.Event) error {
	switch event.EventType {
	case models.EventGameCreated:
		var payload GameCreatedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.Title = payload.Title
		state.HostUserID = payload.HostUserID
	case models.EventPlayerJoined:
		var payload PlayerJoinedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		player, exists := state.Players[payload.UserID]
		if !exists {
			player = &PlayerState{UserID: payload.UserID}
			state.Players[payload.UserID] = player
			state.PlayerOrder = append(state.PlayerOrder, payload.UserID)
		}
		player.Name = payload.Name
		player.IsKicked = false
		player.IsLeft = false
		if activePlayerByID(state, state.HostUserID) == nil {
			state.HostUserID = payload.UserID
		}
	case models.EventPlayerLeft:
		var payload PlayerLeftPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.IsLeft = true
		}
		if state.HostUserID == payload.UserID {
			for _, candidate := range activePlayers(state) {
				state.HostUserID = candidate.UserID
				break
			}
		}
	case models.EventPlayerKicked:
		var payload PlayerKickedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.IsKicked = true
		}
	case models.EventChatMessageSent:
		var payload ChatMessageSentPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		userName := event.ActorName
		if player := state.Players[payload.UserID]; player != nil && player.Name != "" {
			userName = player.Name
		}
		state.ChatMessages = append(state.ChatMessages, ChatMessageState{
			ID:        event.ID,
			UserID:    payload.UserID,
			UserName:  userName,
			Message:   payload.Message,
			CreatedAt: event.CreatedAt,
		})
	case models.EventGameStarted:
		state.Status = GameStatusStarted
		state.TreasuryShareBPS = InitialTreasurySharesBPS
	case models.EventMoleSelected:
		var payload MoleSelectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.MoleUserID = payload.UserID
	case models.EventMoleTargetsGenerated:
		var payload MoleTargetsGeneratedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.MoleTargets = append([]string(nil), payload.Targets...)
	case models.EventPlayerReceivedShare:
		var payload PlayerReceivedSharePayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.ShareBPS = payload.ShareBPS
		}
	case models.EventCEOSelected:
		var payload CEOSelectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.CEOUserID = payload.UserID
	case models.EventVotingRoundStarted:
		var payload VotingRoundStartedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.Phase = GamePhaseMajorVoting
		state.CurrentRound = payload.Round
		state.CurrentVotes = map[int64]VoteState{}
		state.GovernanceProposals = map[int]*GovernanceProposalState{}
		state.GovernanceProposalOrder = nil
		state.GovernanceSubmissions = map[int64]GovernanceSubmissionState{}
		state.GovernanceVotes = map[int64]GovernanceVoteState{}
	case models.EventVoteSubmitted:
		var payload VoteSubmittedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.CurrentVotes[payload.UserID] = VoteState{
			UserID:   payload.UserID,
			Decision: payload.Decision,
			Abstain:  payload.Abstain,
		}
	case models.EventDecisionAccepted:
		var payload DecisionAcceptedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.RoundReports = append(state.RoundReports, buildRoundReport(state, payload.Round, "accepted", payload.Decision, ""))
		state.AcceptedOrder = append(state.AcceptedOrder, payload.Decision)
		delete(state.Available, payload.Decision)
	case models.EventDecisionRejected:
		var payload DecisionRejectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.RoundReports = append(state.RoundReports, buildRoundReport(state, payload.Round, "rejected", "", payload.Reason))
		label := payload.Reason
		if len(payload.Options) > 0 {
			sort.Strings(payload.Options)
			label = fmt.Sprintf("%s:%v", payload.Reason, payload.Options)
		}
		state.RejectedOrder = append(state.RejectedOrder, label)
	case models.EventGovernanceProposalPhaseStarted:
		var payload GovernanceProposalPhaseStartedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.Phase = GamePhaseGovernanceProposal
		state.GovernanceRound = payload.Round
		state.GovernanceProposals = map[int]*GovernanceProposalState{}
		state.GovernanceProposalOrder = nil
		state.GovernanceSubmissions = map[int64]GovernanceSubmissionState{}
		state.GovernanceVotes = map[int64]GovernanceVoteState{}
	case models.EventGovernanceProposalSubmitted:
		var payload GovernanceProposalSubmittedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if state.GovernanceProposals == nil {
			state.GovernanceProposals = map[int]*GovernanceProposalState{}
		}
		state.GovernanceProposals[payload.ProposalID] = &GovernanceProposalState{
			ID:             payload.ProposalID,
			Round:          payload.Round,
			ProposerUserID: payload.ProposerUserID,
			ProposalType:   payload.ProposalType,
			FromUserID:     payload.FromUserID,
			ToUserID:       payload.ToUserID,
			TargetUserID:   payload.TargetUserID,
			ShareBPS:       payload.ShareBPS,
		}
		state.GovernanceProposalOrder = append(state.GovernanceProposalOrder, payload.ProposalID)
		state.GovernanceSubmissions[payload.ProposerUserID] = GovernanceSubmissionState{
			UserID:     payload.ProposerUserID,
			Status:     "submitted",
			ProposalID: payload.ProposalID,
		}
	case models.EventGovernanceProposalSkipped:
		var payload GovernanceProposalSkippedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.GovernanceSubmissions[payload.UserID] = GovernanceSubmissionState{
			UserID: payload.UserID,
			Status: "skipped",
		}
	case models.EventGovernanceVotingStarted:
		var payload GovernanceVotingStartedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.Phase = GamePhaseGovernanceVoting
		state.GovernanceRound = payload.Round
		state.GovernanceVotes = map[int64]GovernanceVoteState{}
	case models.EventGovernanceVoteSubmitted:
		var payload GovernanceVoteSubmittedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.GovernanceVotes[payload.UserID] = GovernanceVoteState{
			UserID:     payload.UserID,
			ProposalID: payload.ProposalID,
			Abstain:    payload.Abstain,
		}
	case models.EventGovernanceProposalAccepted:
		var payload GovernanceProposalAcceptedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.GovernanceReports = append(state.GovernanceReports, buildGovernanceReport(state, payload.Round, "accepted", payload.ProposalID, ""))
	case models.EventGovernanceProposalRejected:
		var payload GovernanceProposalRejectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.GovernanceReports = append(state.GovernanceReports, buildGovernanceReport(state, payload.Round, "rejected", 0, payload.Reason))
	case models.EventPlayerShareTransferred:
		var payload PlayerShareTransferredPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if from := state.Players[payload.FromUserID]; from != nil {
			from.ShareBPS -= payload.ShareBPS
		}
		if to := state.Players[payload.ToUserID]; to != nil {
			to.ShareBPS += payload.ShareBPS
		}
	case models.EventTreasuryShareGranted:
		var payload TreasuryShareGrantedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.TreasuryShareBPS -= payload.ShareBPS
		if target := state.Players[payload.TargetUserID]; target != nil {
			target.ShareBPS += payload.ShareBPS
		}
	case models.EventTreasuryShareBoughtBack:
		var payload TreasuryShareBoughtBackPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.TreasuryShareBPS += payload.ShareBPS
		if target := state.Players[payload.TargetUserID]; target != nil {
			target.ShareBPS -= payload.ShareBPS
		}
	case models.EventCEOChanged:
		var payload CEOChangedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.CEOUserID = payload.TargetUserID
	case models.EventGameFinished:
		var payload GameFinishedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.Status = GameStatusFinished
		state.IsFinished = true
		state.Winner = payload.Winner
	}

	for _, player := range state.Players {
		player.IsHost = player.UserID == state.HostUserID
		player.IsCEO = player.UserID == state.CEOUserID
		if player.UserID == state.MoleUserID {
			player.Role = "mole"
		} else {
			player.Role = "player"
		}
	}

	return nil
}

func buildRoundReport(state *GameState, round int, outcome string, decision string, reason string) RoundReport {
	type bucket struct {
		decision string
		abstain  bool
		shareBPS int
		count    int
	}

	buckets := map[string]*bucket{}
	for _, vote := range state.CurrentVotes {
		player := state.Players[vote.UserID]
		if player == nil || player.IsKicked || player.IsLeft {
			continue
		}

		key := "abstain"
		decisionLabel := "Воздержались"
		if !vote.Abstain && vote.Decision != nil && *vote.Decision != "" {
			key = *vote.Decision
			decisionLabel = *vote.Decision
		}

		if buckets[key] == nil {
			buckets[key] = &bucket{decision: decisionLabel, abstain: vote.Abstain}
		}
		buckets[key].shareBPS += player.ShareBPS
		buckets[key].count++
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "abstain" {
			return false
		}
		if keys[j] == "abstain" {
			return true
		}
		return keys[i] < keys[j]
	})

	report := RoundReport{
		Round:    round,
		Outcome:  outcome,
		Decision: decision,
		Reason:   reason,
		Votes:    make([]DecisionVoteReport, 0, len(keys)),
	}
	for _, key := range keys {
		bucket := buckets[key]
		report.Votes = append(report.Votes, DecisionVoteReport{
			Decision:   bucket.decision,
			Abstain:    bucket.abstain,
			ShareBPS:   bucket.shareBPS,
			VoterCount: bucket.count,
		})
	}
	return report
}

func buildGovernanceReport(state *GameState, round int, outcome string, proposalID int, reason string) GovernanceReport {
	report := GovernanceReport{
		Round:   round,
		Outcome: outcome,
		Reason:  reason,
	}
	if proposal := state.GovernanceProposals[proposalID]; proposal != nil {
		cp := *proposal
		report.Proposal = &cp
	}
	return report
}

func decodeEventValue(value string, dst any) error {
	if value == "" {
		value = "{}"
	}
	if err := json.Unmarshal([]byte(value), dst); err != nil {
		return fmt.Errorf("decode event payload: %w", err)
	}
	return nil
}
