package game

import (
	"fmt"
	"sort"
	"time"
)

func ProjectStateForViewer(state *GameState, viewerUserID int64) (*PublicGameState, error) {
	publicState := &PublicGameState{
		GameID:                state.GameID,
		Title:                 state.Title,
		CompanyName:           state.CompanyName,
		CompanySituation:      state.CompanySituation,
		Status:                state.Status,
		Phase:                 state.Phase,
		IsFinished:            state.IsFinished,
		Winner:                state.Winner,
		CurrentRound:          state.CurrentRound,
		GovernanceRound:       state.GovernanceRound,
		TreasuryShareBPS:      state.TreasuryShareBPS,
		AcceptedDecisions:     append([]string(nil), state.AcceptedOrder...),
		RejectedDecisions:     append([]string(nil), state.RejectedOrder...),
		MajorVoteOptions:      append([]string(nil), state.MajorVoteOptions...),
		MajorVoteUnlockedAt:   cloneTimePtr(state.MajorVoteUnlockedAt),
		DecisionTypes:         publicDecisionTypes(),
		GovernanceProposals:   publicGovernanceProposals(state),
		GovernanceSubmissions: publicGovernanceSubmissions(state),
		GovernanceReports:     publicGovernanceReports(state.GovernanceReports),
		RoundReports:          publicRoundReports(state.RoundReports),
		ChatMessages:          publicChatMessages(state, viewerUserID),
		AvailableActions:      availableActionsForViewer(state, viewerUserID),
	}
	if state.Status != GameStatusLobby {
		publicState.AvailableDecisions = sortedAvailableDecisions(state.Available)
	}

	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player == nil || player.IsKicked || player.IsLeft {
			continue
		}

		publicPlayer := PublicPlayerState{
			UserID:       player.UserID,
			Name:         player.Name,
			Position:     effectivePlayerPosition(player),
			ShareBPS:     player.ShareBPS,
			AuthorityBPS: effectiveAuthorityBPS(player),
			IsHost:       player.IsHost,
			IsCEO:        player.IsCEO,
		}

		if player.UserID == viewerUserID {
			publicPlayer.Role = player.Role
			publicState.Me = publicPlayer
			if player.Role == "mole" {
				publicState.MoleTargets = append([]string(nil), state.MoleTargets...)
				publicState.MoleSabotage = state.MoleSabotage
				molePoints, playersPoints := victoryPoints(state)
				publicState.MoleVictoryPoints = &molePoints
				publicState.PlayersVictoryPoints = &playersPoints
			} else {
				publicState.MemorandumPreference = state.MemorandumPreferences[player.UserID]
				if memorandum, ok := state.Memorandums[player.UserID]; ok {
					publicState.Memorandum = &PublicMemorandum{
						Type:      memorandum.Type,
						Decisions: append([]string(nil), memorandum.Decisions...),
					}
				}
			}
		}

		publicState.Players = append(publicState.Players, publicPlayer)
		publicState.CurrentVotes = append(publicState.CurrentVotes, publicVoteStateForPlayer(state, player))
	}

	if state.IsFinished {
		publicState.FinalSummary = publicFinalSummary(state)
		publicState.ReplaySteps = publicReplaySteps(state)
	}
	if state.MoleSabotage != "" && decisionAccepted(state, state.MoleSabotage) && publicState.MoleVictoryPoints == nil {
		molePoints, playersPoints := victoryPoints(state)
		publicState.MoleVictoryPoints = &molePoints
		publicState.PlayersVictoryPoints = &playersPoints
	}

	if state.Phase == GamePhaseGovernanceVoting {
		if vote, ok := state.GovernanceVotes[viewerUserID]; ok {
			publicState.MyCurrentVote = &PublicOwnVoteState{
				Abstain: vote.Abstain,
			}
			if vote.ProposalID != nil {
				publicState.MyCurrentVote.ProposalID = *vote.ProposalID
			}
		}
	} else if vote, ok := state.CurrentVotes[viewerUserID]; ok {
		publicState.MyCurrentVote = &PublicOwnVoteState{
			Abstain: vote.Abstain,
		}
		if vote.Decision != nil {
			publicState.MyCurrentVote.Decision = *vote.Decision
		}
	}

	return publicState, nil
}

func publicDecisionTypes() map[string]DecisionType {
	out := make(map[string]DecisionType, len(decisionTypes))
	for decision, decisionType := range decisionTypes {
		out[decision] = decisionType
	}
	return out
}

func publicVoteStateForPlayer(state *GameState, player *PlayerState) PublicVoteState {
	out := PublicVoteState{
		UserID:   player.UserID,
		HasVoted: hasPlayerVotedForPhase(state, player.UserID),
	}
	if state.Phase != GamePhaseGovernanceVoting || !out.HasVoted {
		return out
	}
	vote := state.GovernanceVotes[player.UserID]
	out.Abstain = vote.Abstain
	if vote.ProposalID != nil {
		out.ProposalID = *vote.ProposalID
	}
	out.ShareBPS = player.ShareBPS
	out.AuthorityBPS = effectiveAuthorityBPS(player)
	out.VotingPowerBPS = out.ShareBPS + out.AuthorityBPS
	return out
}

func publicGovernanceProposals(state *GameState) []PublicGovernanceProposal {
	out := make([]PublicGovernanceProposal, 0, len(state.GovernanceProposalOrder))
	for _, proposalID := range state.GovernanceProposalOrder {
		proposal := state.GovernanceProposals[proposalID]
		if proposal == nil {
			continue
		}
		out = append(out, publicGovernanceProposal(proposal))
	}
	return out
}

func publicGovernanceProposal(proposal *GovernanceProposalState) PublicGovernanceProposal {
	return PublicGovernanceProposal{
		ID:             proposal.ID,
		Round:          proposal.Round,
		ProposerUserID: proposal.ProposerUserID,
		AuthorUserIDs:  append([]int64(nil), proposal.AuthorUserIDs...),
		ProposalType:   proposal.ProposalType,
		FromUserID:     proposal.FromUserID,
		ToUserID:       proposal.ToUserID,
		TargetUserID:   proposal.TargetUserID,
		ShareBPS:       proposal.ShareBPS,
	}
}

func publicGovernanceSubmissions(state *GameState) []PublicGovernanceSubmission {
	out := make([]PublicGovernanceSubmission, 0, len(state.PlayerOrder))
	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player == nil || player.IsKicked || player.IsLeft {
			continue
		}
		submission := state.GovernanceSubmissions[userID]
		out = append(out, PublicGovernanceSubmission{
			UserID:     userID,
			Status:     submission.Status,
			ProposalID: submission.ProposalID,
		})
	}
	return out
}

func publicGovernanceReports(reports []GovernanceReport) []PublicGovernanceReport {
	out := make([]PublicGovernanceReport, 0, len(reports))
	for _, report := range reports {
		publicReport := PublicGovernanceReport{
			Round:   report.Round,
			Outcome: report.Outcome,
			Reason:  report.Reason,
		}
		if report.Proposal != nil {
			proposal := publicGovernanceProposal(report.Proposal)
			publicReport.Proposal = &proposal
		}
		publicReport.Votes = publicGovernanceVoteReports(report.Votes)
		out = append(out, publicReport)
	}
	return out
}

func publicGovernanceVoteReports(reports []GovernanceVoteReport) []PublicGovernanceVoteReport {
	out := make([]PublicGovernanceVoteReport, 0, len(reports))
	for _, report := range reports {
		publicReport := PublicGovernanceVoteReport{
			ProposalID:     report.ProposalID,
			ProposalTitle:  report.ProposalTitle,
			Abstain:        report.Abstain,
			ShareBPS:       report.ShareBPS,
			AuthorityBPS:   report.AuthorityBPS,
			VotingPowerBPS: report.VotingPowerBPS,
			VoterCount:     report.VoterCount,
			Voters:         publicGovernanceVoters(report.Voters),
		}
		if report.Proposal != nil {
			proposal := publicGovernanceProposal(report.Proposal)
			publicReport.Proposal = &proposal
		}
		out = append(out, publicReport)
	}
	return out
}

func publicGovernanceVoters(voters []GovernanceVoterReport) []PublicGovernanceVoterReport {
	out := make([]PublicGovernanceVoterReport, 0, len(voters))
	for _, voter := range voters {
		out = append(out, PublicGovernanceVoterReport{
			UserID:         voter.UserID,
			Name:           voter.Name,
			ShareBPS:       voter.ShareBPS,
			AuthorityBPS:   voter.AuthorityBPS,
			VotingPowerBPS: voter.VotingPowerBPS,
		})
	}
	return out
}

func publicRoundReports(reports []RoundReport) []PublicRoundReport {
	out := make([]PublicRoundReport, 0, len(reports))
	for _, report := range reports {
		publicReport := PublicRoundReport{
			Round:    report.Round,
			Outcome:  report.Outcome,
			Decision: report.Decision,
			Reason:   report.Reason,
			Votes:    make([]PublicDecisionVoteReport, 0, len(report.Votes)),
		}
		for _, vote := range report.Votes {
			publicReport.Votes = append(publicReport.Votes, PublicDecisionVoteReport{
				Decision:   vote.Decision,
				Abstain:    vote.Abstain,
				ShareBPS:   vote.ShareBPS,
				VoterCount: vote.VoterCount,
				Voters:     publicDecisionVoters(vote.Voters),
			})
		}
		out = append(out, publicReport)
	}
	return out
}

func publicDecisionVoters(voters []DecisionVoterReport) []PublicDecisionVoterReport {
	out := make([]PublicDecisionVoterReport, 0, len(voters))
	for _, voter := range voters {
		out = append(out, PublicDecisionVoterReport{
			UserID:   voter.UserID,
			Name:     voter.Name,
			ShareBPS: voter.ShareBPS,
		})
	}
	return out
}

func publicChatMessages(state *GameState, viewerUserID int64) []PublicChatMessage {
	messages := state.ChatMessages
	start := 0
	if len(messages) > MaxPublicChatMessages {
		start = len(messages) - MaxPublicChatMessages
	}
	out := make([]PublicChatMessage, 0, len(messages)-start)
	for _, message := range messages[start:] {
		out = append(out, PublicChatMessage{
			ID:              message.ID,
			UserID:          message.UserID,
			UserName:        message.UserName,
			UserPosition:    message.UserPosition,
			Message:         message.Message,
			Kind:            message.Kind,
			SystemEventType: message.SystemEventType,
			Title:           message.Title,
			Summary:         message.Summary,
			Details:         append([]string(nil), message.Details...),
			Tone:            message.Tone,
			Collapsible:     message.Collapsible,
			Reactions:       publicChatReactions(state, message.ID, viewerUserID),
			CreatedAt:       message.CreatedAt,
		})
	}
	return out
}

func publicChatReactions(state *GameState, messageID int64, viewerUserID int64) []PublicChatReaction {
	if state == nil || state.ChatReactions == nil {
		return nil
	}
	reactions := state.ChatReactions[messageID]
	if len(reactions) == 0 {
		return nil
	}
	emojis := make([]string, 0, len(reactions))
	for emoji, users := range reactions {
		if len(users) > 0 {
			emojis = append(emojis, emoji)
		}
	}
	sort.Strings(emojis)
	out := make([]PublicChatReaction, 0, len(emojis))
	for _, emoji := range emojis {
		users := reactions[emoji]
		out = append(out, PublicChatReaction{
			Emoji:       emoji,
			Count:       len(users),
			ReactedByMe: users[viewerUserID],
		})
	}
	return out
}

func availableActionsForViewer(state *GameState, viewerUserID int64) []ActionType {
	player := state.Players[viewerUserID]
	actions := []ActionType{}
	if player != nil && !player.IsKicked && !player.IsLeft {
		actions = append(actions, ActionSendChatMessage)
		actions = append(actions, ActionReactChatMessage)
	}
	if state.IsFinished {
		return actions
	}

	switch state.Status {
	case GameStatusLobby:
		if player == nil || player.IsLeft {
			actions = append(actions, ActionJoinGame)
		} else if !player.IsKicked {
			actions = append(actions, ActionLeaveGame)
		}
		if player != nil && player.IsHost && !player.IsKicked && !player.IsLeft {
			actions = append(actions, ActionKickPlayer, ActionBanPlayer, ActionStartGame)
		}
	case GameStatusStarted:
		if player == nil || player.IsKicked || player.IsLeft {
			return actions
		}
		switch state.Phase {
		case GamePhaseMoleObjectiveSelection:
			if player.Role == "mole" && len(state.MoleTargets) == 0 && state.MoleSabotage == "" {
				actions = append(actions, ActionSelectMoleObjectives)
			}
			if player.Role != "mole" && state.MemorandumPreferences[viewerUserID] == "" && len(state.MoleTargets) == 0 && state.MoleSabotage == "" {
				actions = append(actions, ActionChooseMemorandum)
			}
		case GamePhaseMajorVoting:
			actions = append(actions, ActionVote)
		case GamePhaseGovernanceProposal:
			if _, ok := state.GovernanceSubmissions[viewerUserID]; !ok {
				actions = append(actions, ActionSubmitGovernanceProposal, ActionSkipGovernanceProposal)
			}
		case GamePhaseGovernanceVoting:
			actions = append(actions, ActionVote)
		}
	}

	return actions
}

func hasPlayerVotedForPhase(state *GameState, userID int64) bool {
	if state.Phase == GamePhaseGovernanceVoting {
		_, ok := state.GovernanceVotes[userID]
		return ok
	}
	if state.Phase == GamePhaseMajorVoting {
		return hasPlayerVoted(state, userID)
	}
	return false
}

func hasPlayerVoted(state *GameState, userID int64) bool {
	_, ok := state.CurrentVotes[userID]
	return ok
}

func sortedAvailableDecisions(available map[string]bool) []string {
	out := make([]string, 0, len(available))
	for decision, ok := range available {
		if ok {
			out = append(out, decision)
		}
	}
	sort.Strings(out)
	return out
}

func publicFinalSummary(state *GameState) *PublicFinalSummary {
	if state == nil || !state.IsFinished {
		return nil
	}
	molePoints, playersPoints := victoryPoints(state)
	summary := &PublicFinalSummary{
		Winner:        state.Winner,
		MoleUserID:    state.MoleUserID,
		MoleTargets:   append([]string(nil), state.MoleTargets...),
		MoleSabotage:  state.MoleSabotage,
		MolePoints:    molePoints,
		PlayersPoints: playersPoints,
		PlayerStats:   make([]PublicFinalPlayerStats, 0, len(state.PlayerOrder)),
		WinnerUserIDs: []int64{},
	}

	minMistakes := -1
	targetSet := moleObjectiveSet(state.MoleTargets, state.MoleSabotage)
	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player == nil || player.IsKicked || player.IsLeft {
			continue
		}

		won := (state.Winner == "mole" && player.Role == "mole") ||
			(state.Winner == "players" && player.Role != "mole")
		if won {
			summary.WinnerUserIDs = append(summary.WinnerUserIDs, player.UserID)
		}

		stat := PublicFinalPlayerStats{
			UserID: player.UserID,
			Name:   player.Name,
			Role:   player.Role,
			Won:    won,
		}
		for _, report := range state.RoundReports {
			for _, vote := range report.Votes {
				if vote.Abstain || vote.Decision == "" {
					continue
				}
				for _, voter := range vote.Voters {
					if voter.UserID != player.UserID {
						continue
					}
					stat.MajorVotes++
					isMoleObjective := targetSet[vote.Decision]
					if (player.Role == "mole" && isMoleObjective) || (player.Role != "mole" && !isMoleObjective) {
						stat.AlignedVotes++
					}
				}
			}
		}
		stat.Mistakes = stat.MajorVotes - stat.AlignedVotes
		if stat.MajorVotes > 0 {
			stat.AccuracyBPS = stat.AlignedVotes * TotalSharesBPS / stat.MajorVotes
		}

		if minMistakes == -1 || stat.Mistakes < minMistakes {
			minMistakes = stat.Mistakes
			summary.LeastMistakeUserIDs = []int64{stat.UserID}
		} else if stat.Mistakes == minMistakes {
			summary.LeastMistakeUserIDs = append(summary.LeastMistakeUserIDs, stat.UserID)
		}
		summary.PlayerStats = append(summary.PlayerStats, stat)
	}

	return summary
}

func publicReplaySteps(state *GameState) []PublicReplayStep {
	if state == nil || !state.IsFinished {
		return nil
	}
	steps := []PublicReplayStep{{
		ID:      "setup",
		Kind:    "setup",
		Title:   "Старт заседания",
		Summary: fmt.Sprintf("В партии %d директоров. Один из них играет за Крота.", len(activePlayers(state))),
	}}

	governanceIndex := 0
	for _, report := range state.RoundReports {
		steps = append(steps, replayStepForRoundReport(report))
		if report.Outcome == "accepted" && governanceIndex < len(state.GovernanceReports) {
			steps = append(steps, replayStepForGovernanceReport(state, state.GovernanceReports[governanceIndex]))
			governanceIndex++
		}
	}
	for governanceIndex < len(state.GovernanceReports) {
		steps = append(steps, replayStepForGovernanceReport(state, state.GovernanceReports[governanceIndex]))
		governanceIndex++
	}

	molePoints, playersPoints := victoryPoints(state)
	steps = append(steps, PublicReplayStep{
		ID:      "final",
		Kind:    "final",
		Title:   "Финальное раскрытие",
		Summary: fmt.Sprintf("Победитель: %s. Счет: Крот %d/3, Совет %d/3.", state.Winner, molePoints, playersPoints),
		Winner:  state.Winner,
	})
	return steps
}

func replayStepForRoundReport(report RoundReport) PublicReplayStep {
	title := fmt.Sprintf("Major vote, раунд %d", report.Round)
	summary := "Решение не принято."
	if report.Outcome == "accepted" {
		summary = fmt.Sprintf("Принято решение %s.", decisionLabelForChat(report.Decision))
	}
	return PublicReplayStep{
		ID:       fmt.Sprintf("major-%d", report.Round),
		Kind:     "major_vote",
		Title:    title,
		Summary:  summary,
		Round:    report.Round,
		Outcome:  report.Outcome,
		Decision: report.Decision,
		Votes:    replayVotesForRoundReport(report),
	}
}

func replayVotesForRoundReport(report RoundReport) []PublicReplayVote {
	out := make([]PublicReplayVote, 0, len(report.Votes))
	for _, vote := range report.Votes {
		label := decisionLabelForChat(vote.Decision)
		if vote.Abstain {
			label = "Воздержались"
		}
		out = append(out, PublicReplayVote{
			Label:    label,
			ShareBPS: vote.ShareBPS,
			Voters:   decisionVoterNames(vote.Voters),
		})
	}
	return out
}

func replayStepForGovernanceReport(state *GameState, report GovernanceReport) PublicReplayStep {
	title := fmt.Sprintf("Governance, раунд %d", report.Round)
	summary := "Маневр не принят."
	if report.Outcome == "accepted" && report.Proposal != nil {
		summary = fmt.Sprintf("Принят маневр: %s.", describeGovernanceProposalForChat(state, report.Proposal))
	}
	var proposal *PublicGovernanceProposal
	if report.Proposal != nil {
		publicProposal := publicGovernanceProposal(report.Proposal)
		proposal = &publicProposal
	}
	return PublicReplayStep{
		ID:       fmt.Sprintf("governance-%d", report.Round),
		Kind:     "governance",
		Title:    title,
		Summary:  summary,
		Round:    report.Round,
		Outcome:  report.Outcome,
		Proposal: proposal,
		Votes:    replayVotesForGovernanceReport(report),
	}
}

func replayVotesForGovernanceReport(report GovernanceReport) []PublicReplayVote {
	out := make([]PublicReplayVote, 0, len(report.Votes))
	for _, vote := range report.Votes {
		label := vote.ProposalTitle
		if label == "" && vote.Proposal != nil {
			label = describeGovernanceProposalForChat(nil, vote.Proposal)
		}
		if label == "" {
			label = "Предложение"
		}
		if vote.Abstain {
			label = "Воздержались"
		}
		out = append(out, PublicReplayVote{
			Label:          label,
			ShareBPS:       vote.ShareBPS,
			VotingPowerBPS: vote.VotingPowerBPS,
			Voters:         governanceVoterNames(vote.Voters),
		})
	}
	return out
}

func decisionVoterNames(voters []DecisionVoterReport) []string {
	out := make([]string, 0, len(voters))
	for _, voter := range voters {
		out = append(out, voter.Name)
	}
	return out
}

func governanceVoterNames(voters []GovernanceVoterReport) []string {
	out := make([]string, 0, len(voters))
	for _, voter := range voters {
		out = append(out, voter.Name)
	}
	return out
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}

func decisionAccepted(state *GameState, decision string) bool {
	for _, accepted := range state.AcceptedOrder {
		if accepted == decision {
			return true
		}
	}
	return false
}
