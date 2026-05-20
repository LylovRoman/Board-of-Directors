package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"agentbackend/internal/models"
)

func BuildState(gameID int64, title string, events []models.Event) (*GameState, error) {
	state := &GameState{
		GameID:                gameID,
		Title:                 title,
		CompanyName:           title,
		CompanySituation:      "Совет директоров собрался на внеочередное заседание.",
		Status:                GameStatusLobby,
		Players:               map[int64]*PlayerState{},
		CurrentVotes:          map[int64]VoteState{},
		MemorandumPreferences: map[int64]MemorandumType{},
		Memorandums:           map[int64]MemorandumState{},
		GovernanceProposals:   map[int]*GovernanceProposalState{},
		GovernanceSubmissions: map[int64]GovernanceSubmissionState{},
		GovernanceVotes:       map[int64]GovernanceVoteState{},
		Available:             map[string]bool{},
		ChatReactions:         map[int64]map[string]map[int64]bool{},
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
		state.CompanyName = payload.CompanyName
		if state.CompanyName == "" {
			state.CompanyName = payload.Title
		}
		state.CompanySituation = payload.CompanySituation
		if state.CompanySituation == "" {
			state.CompanySituation = "Совет директоров собрался на внеочередное заседание."
		}
	case models.EventPlayerJoined:
		var payload PlayerJoinedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		player, exists := state.Players[payload.UserID]
		if !exists {
			player = &PlayerState{UserID: payload.UserID, AuthorityBPS: InitialAuthorityBPS}
			state.Players[payload.UserID] = player
			state.PlayerOrder = append(state.PlayerOrder, payload.UserID)
		}
		if player.AuthorityBPS == 0 {
			player.AuthorityBPS = InitialAuthorityBPS
		}
		player.Name = payload.Name
		player.Position = payload.Position
		player.IsBot = payload.IsBot || payload.UserID < 0
		player.IsKicked = false
		player.IsLeft = false
		if !player.IsBot && activeRealPlayerByID(state, state.HostUserID) == nil {
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
			state.HostUserID = 0
			for _, candidate := range activeRealPlayers(state) {
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
		if state.HostUserID == payload.UserID {
			state.HostUserID = 0
			for _, candidate := range activeRealPlayers(state) {
				state.HostUserID = candidate.UserID
				break
			}
		}
	case models.EventPlayerReplacedByBot:
		var payload PlayerReplacedByBotPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.IsLeft = true
		}
		bot := &PlayerState{
			UserID:       payload.BotUserID,
			Name:         payload.Name,
			Position:     payload.Position,
			ShareBPS:     payload.ShareBPS,
			AuthorityBPS: payload.AuthorityBPS,
			IsBot:        true,
		}
		if bot.AuthorityBPS == 0 {
			bot.AuthorityBPS = InitialAuthorityBPS
		}
		state.Players[payload.BotUserID] = bot
		inserted := false
		for index, userID := range state.PlayerOrder {
			if userID == payload.UserID {
				nextOrder := append([]int64{}, state.PlayerOrder[:index+1]...)
				nextOrder = append(nextOrder, payload.BotUserID)
				nextOrder = append(nextOrder, state.PlayerOrder[index+1:]...)
				state.PlayerOrder = nextOrder
				inserted = true
				break
			}
		}
		if !inserted {
			state.PlayerOrder = append(state.PlayerOrder, payload.BotUserID)
		}
		if state.HostUserID == payload.UserID {
			state.HostUserID = 0
			for _, candidate := range activeRealPlayers(state) {
				state.HostUserID = candidate.UserID
				break
			}
		}
		if state.CEOUserID == payload.UserID || payload.IsCEO {
			state.CEOUserID = payload.BotUserID
		}
		if state.MoleUserID == payload.UserID || payload.Role == "mole" {
			state.MoleUserID = payload.BotUserID
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
		userPosition := ""
		if player := state.Players[payload.UserID]; player != nil {
			userPosition = effectivePlayerPosition(player)
		}
		state.ChatMessages = append(state.ChatMessages, ChatMessageState{
			ID:              event.ID,
			UserID:          payload.UserID,
			UserName:        userName,
			UserPosition:    userPosition,
			Message:         payload.Message,
			Kind:            payload.Kind,
			SystemEventType: payload.SystemEventType,
			Title:           payload.Title,
			Summary:         payload.Summary,
			Details:         append([]string(nil), payload.Details...),
			Tone:            payload.Tone,
			Collapsible:     payload.Collapsible,
			CreatedAt:       event.CreatedAt,
		})
	case models.EventGameStarted:
		var payload GameStartedPayload
		if event.EventValue != "" {
			if err := decodeEventValue(event.EventValue, &payload); err != nil {
				return err
			}
		}
		state.Status = GameStatusStarted
		state.Phase = GamePhaseMoleObjectiveSelection
		state.TreasuryShareBPS = InitialTreasurySharesBPS
		state.StartedPlayerCount = payload.PlayerCount
		if state.StartedPlayerCount == 0 {
			state.StartedPlayerCount = len(activePlayers(state))
		}
		setPhaseTiming(state, event)
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
	case models.EventMoleObjectivesSelected:
		var payload MoleObjectivesSelectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		state.MoleTargets = append([]string(nil), payload.Targets...)
		state.MoleSabotage = payload.Sabotage
	case models.EventMemorandumPreferenceSelected:
		var payload MemorandumPreferenceSelectedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if state.MemorandumPreferences == nil {
			state.MemorandumPreferences = map[int64]MemorandumType{}
		}
		state.MemorandumPreferences[payload.UserID] = payload.Type
	case models.EventMemorandumAssigned:
		var payload MemorandumAssignedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if state.Memorandums == nil {
			state.Memorandums = map[int64]MemorandumState{}
		}
		state.Memorandums[payload.UserID] = MemorandumState{
			UserID:    payload.UserID,
			Type:      payload.Type,
			Decisions: append([]string(nil), payload.Decisions...),
		}
	case models.EventPlayerReceivedShare:
		var payload PlayerReceivedSharePayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.ShareBPS = payload.ShareBPS
		}
	case models.EventPlayerAuthorityGranted:
		var payload PlayerAuthorityGrantedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.AuthorityBPS += payload.AuthorityBPS
		}
	case models.EventPlayerPositionAssigned:
		var payload PlayerPositionAssignedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if player := state.Players[payload.UserID]; player != nil {
			player.Position = payload.Position
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
		if len(payload.ShowcaseDecisions) > 0 {
			state.MajorVoteOptions = append([]string(nil), payload.ShowcaseDecisions...)
		} else {
			state.MajorVoteOptions = sortedAvailableDecisions(state.Available)
		}
		if payload.UnlockedAt != nil {
			unlockedAt := payload.UnlockedAt.UTC()
			state.MajorVoteUnlockedAt = &unlockedAt
		} else {
			state.MajorVoteUnlockedAt = nil
		}
		setPhaseTiming(state, event)
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
		state.MajorVoteOptions = nil
		setPhaseTiming(state, event)
	case models.EventGovernanceProposalSubmitted:
		var payload GovernanceProposalSubmittedPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if state.GovernanceProposals == nil {
			state.GovernanceProposals = map[int]*GovernanceProposalState{}
		}
		authors := append([]int64(nil), payload.AuthorUserIDs...)
		if len(authors) == 0 {
			authors = []int64{payload.ProposerUserID}
		}
		if existing := state.GovernanceProposals[payload.ProposalID]; existing != nil {
			existing.AuthorUserIDs = mergeAuthorIDs(existing.AuthorUserIDs, authors...)
			if existing.ShareBPS < payload.ShareBPS {
				existing.ShareBPS = payload.ShareBPS
			}
		} else {
			state.GovernanceProposals[payload.ProposalID] = &GovernanceProposalState{
				ID:             payload.ProposalID,
				Round:          payload.Round,
				ProposerUserID: payload.ProposerUserID,
				AuthorUserIDs:  mergeAuthorIDs(nil, authors...),
				ProposalType:   payload.ProposalType,
				FromUserID:     payload.FromUserID,
				ToUserID:       payload.ToUserID,
				TargetUserID:   payload.TargetUserID,
				ShareBPS:       payload.ShareBPS,
			}
			state.GovernanceProposalOrder = append(state.GovernanceProposalOrder, payload.ProposalID)
		}
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
		if len(payload.ProposalIDs) > 0 {
			filtered := make(map[int]*GovernanceProposalState, len(payload.ProposalIDs))
			for _, proposalID := range payload.ProposalIDs {
				if proposal := state.GovernanceProposals[proposalID]; proposal != nil {
					filtered[proposalID] = proposal
				}
			}
			state.GovernanceProposals = filtered
			state.GovernanceProposalOrder = append([]int(nil), payload.ProposalIDs...)
		}
		state.Phase = GamePhaseGovernanceVoting
		state.GovernanceRound = payload.Round
		state.GovernanceVotes = map[int64]GovernanceVoteState{}
		setPhaseTiming(state, event)
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
		state.PhaseDeadlineAt = nil
	case models.EventChatReactionToggled:
		var payload ChatReactionToggledPayload
		if err := decodeEventValue(event.EventValue, &payload); err != nil {
			return err
		}
		if state.ChatReactions == nil {
			state.ChatReactions = map[int64]map[string]map[int64]bool{}
		}
		if state.ChatReactions[payload.MessageID] == nil {
			state.ChatReactions[payload.MessageID] = map[string]map[int64]bool{}
		}
		if state.ChatReactions[payload.MessageID][payload.Emoji] == nil {
			state.ChatReactions[payload.MessageID][payload.Emoji] = map[int64]bool{}
		}
		if state.ChatReactions[payload.MessageID][payload.Emoji][payload.UserID] {
			delete(state.ChatReactions[payload.MessageID][payload.Emoji], payload.UserID)
		} else {
			state.ChatReactions[payload.MessageID][payload.Emoji][payload.UserID] = true
		}
	}

	for _, player := range state.Players {
		if player.AuthorityBPS == 0 {
			player.AuthorityBPS = InitialAuthorityBPS
		}
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

func setPhaseTiming(state *GameState, event models.Event) {
	startedAt := event.CreatedAt.UTC()
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	deadlineAt := startedAt.Add(PhaseDuration)
	state.PhaseStartedAt = &startedAt
	state.PhaseDeadlineAt = &deadlineAt
}

func buildRoundReport(state *GameState, round int, outcome string, decision string, reason string) RoundReport {
	type bucket struct {
		decision string
		abstain  bool
		shareBPS int
		count    int
		voters   []DecisionVoterReport
	}

	buckets := map[string]*bucket{}
	for _, userID := range state.PlayerOrder {
		vote, ok := state.CurrentVotes[userID]
		if !ok {
			continue
		}
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
		buckets[key].voters = append(buckets[key].voters, DecisionVoterReport{
			UserID:   player.UserID,
			Name:     player.Name,
			ShareBPS: player.ShareBPS,
		})
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
			Voters:     append([]DecisionVoterReport(nil), bucket.voters...),
		})
	}
	return report
}

func buildGovernanceReport(state *GameState, round int, outcome string, proposalID int, reason string) GovernanceReport {
	report := GovernanceReport{
		Round:   round,
		Outcome: outcome,
		Reason:  reason,
		Votes:   buildGovernanceVoteReports(state),
	}
	if proposal := state.GovernanceProposals[proposalID]; proposal != nil {
		cp := *proposal
		cp.AuthorUserIDs = append([]int64(nil), proposal.AuthorUserIDs...)
		report.Proposal = &cp
	}
	return report
}

func buildGovernanceVoteReports(state *GameState) []GovernanceVoteReport {
	type bucket struct {
		proposalID     int
		proposal       *GovernanceProposalState
		proposalTitle  string
		abstain        bool
		shareBPS       int
		authorityBPS   int
		votingPowerBPS int
		count          int
		voters         []GovernanceVoterReport
	}

	buckets := map[string]*bucket{}
	for _, userID := range state.PlayerOrder {
		vote, ok := state.GovernanceVotes[userID]
		if !ok {
			continue
		}
		player := state.Players[vote.UserID]
		if player == nil || player.IsKicked || player.IsLeft {
			continue
		}

		key := "abstain"
		proposalID := 0
		abstain := true
		var proposal *GovernanceProposalState
		proposalTitle := ""
		if !vote.Abstain && vote.ProposalID != nil && state.GovernanceProposals[*vote.ProposalID] != nil {
			proposalID = *vote.ProposalID
			key = fmt.Sprintf("proposal:%d", proposalID)
			abstain = false
			proposal = cloneGovernanceProposal(state.GovernanceProposals[proposalID])
			proposalTitle = describeGovernanceProposalForChat(state, proposal)
		}

		if buckets[key] == nil {
			buckets[key] = &bucket{
				proposalID:    proposalID,
				proposal:      proposal,
				proposalTitle: proposalTitle,
				abstain:       abstain,
			}
		}
		authorityBPS := effectiveAuthorityBPS(player)
		votingPowerBPS := player.ShareBPS + authorityBPS
		buckets[key].shareBPS += player.ShareBPS
		buckets[key].authorityBPS += authorityBPS
		buckets[key].votingPowerBPS += votingPowerBPS
		buckets[key].count++
		buckets[key].voters = append(buckets[key].voters, GovernanceVoterReport{
			UserID:         player.UserID,
			Name:           player.Name,
			ShareBPS:       player.ShareBPS,
			AuthorityBPS:   authorityBPS,
			VotingPowerBPS: votingPowerBPS,
		})
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

	out := make([]GovernanceVoteReport, 0, len(keys))
	for _, key := range keys {
		bucket := buckets[key]
		out = append(out, GovernanceVoteReport{
			ProposalID:     bucket.proposalID,
			Proposal:       cloneGovernanceProposal(bucket.proposal),
			ProposalTitle:  bucket.proposalTitle,
			Abstain:        bucket.abstain,
			ShareBPS:       bucket.shareBPS,
			AuthorityBPS:   bucket.authorityBPS,
			VotingPowerBPS: bucket.votingPowerBPS,
			VoterCount:     bucket.count,
			Voters:         append([]GovernanceVoterReport(nil), bucket.voters...),
		})
	}
	return out
}

func effectiveAuthorityBPS(player *PlayerState) int {
	if player == nil {
		return 0
	}
	authorityBPS := player.AuthorityBPS
	if authorityBPS == 0 {
		authorityBPS = InitialAuthorityBPS
	}
	if player.IsCEO {
		authorityBPS += CEOAuthorityBonusBPS
	}
	return authorityBPS
}

func mergeAuthorIDs(existing []int64, incoming ...int64) []int64 {
	seen := map[int64]bool{}
	for _, userID := range existing {
		if userID != 0 {
			seen[userID] = true
		}
	}
	for _, userID := range incoming {
		if userID != 0 {
			seen[userID] = true
		}
	}
	out := make([]int64, 0, len(seen))
	for userID := range seen {
		out = append(out, userID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneGovernanceProposal(proposal *GovernanceProposalState) *GovernanceProposalState {
	if proposal == nil {
		return nil
	}
	cp := *proposal
	cp.AuthorUserIDs = append([]int64(nil), proposal.AuthorUserIDs...)
	return &cp
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
