package game

import "sort"

func ProjectStateForViewer(state *GameState, viewerUserID int64) (*PublicGameState, error) {
	publicState := &PublicGameState{
		GameID:                state.GameID,
		Title:                 state.Title,
		Status:                state.Status,
		Phase:                 state.Phase,
		IsFinished:            state.IsFinished,
		Winner:                state.Winner,
		CurrentRound:          state.CurrentRound,
		GovernanceRound:       state.GovernanceRound,
		TreasuryShareBPS:      state.TreasuryShareBPS,
		AcceptedDecisions:     append([]string(nil), state.AcceptedOrder...),
		RejectedDecisions:     append([]string(nil), state.RejectedOrder...),
		GovernanceProposals:   publicGovernanceProposals(state),
		GovernanceSubmissions: publicGovernanceSubmissions(state),
		GovernanceReports:     publicGovernanceReports(state.GovernanceReports),
		RoundReports:          publicRoundReports(state.RoundReports),
		AvailableActions:      availableActionsForViewer(state, viewerUserID),
	}
	if state.Status != GameStatusLobby {
		publicState.AvailableDecisions = sortedAvailableDecisions(state.Available)
	}

	for _, userID := range state.PlayerOrder {
		player := state.Players[userID]
		if player == nil || player.IsKicked {
			continue
		}

		publicPlayer := PublicPlayerState{
			UserID:   player.UserID,
			Name:     player.Name,
			ShareBPS: player.ShareBPS,
			IsHost:   player.IsHost,
			IsCEO:    player.IsCEO,
		}

		if player.UserID == viewerUserID {
			publicPlayer.Role = player.Role
			publicState.Me = publicPlayer
			if player.Role == "mole" {
				publicState.MoleTargets = append([]string(nil), state.MoleTargets...)
			}
		}

		publicState.Players = append(publicState.Players, publicPlayer)
		publicState.CurrentVotes = append(publicState.CurrentVotes, PublicVoteState{
			UserID:   player.UserID,
			HasVoted: hasPlayerVotedForPhase(state, player.UserID),
		})
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
		if player == nil || player.IsKicked {
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
		out = append(out, publicReport)
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
			})
		}
		out = append(out, publicReport)
	}
	return out
}

func availableActionsForViewer(state *GameState, viewerUserID int64) []ActionType {
	player := state.Players[viewerUserID]
	if state.IsFinished {
		return []ActionType{}
	}

	actions := []ActionType{}
	switch state.Status {
	case GameStatusLobby:
		if player == nil {
			actions = append(actions, ActionJoinGame)
		}
		if player != nil && player.IsHost && !player.IsKicked {
			actions = append(actions, ActionKickPlayer, ActionStartGame)
		}
	case GameStatusStarted:
		if player == nil || player.IsKicked {
			return actions
		}
		switch state.Phase {
		case GamePhaseMajorVoting:
			if !hasPlayerVoted(state, viewerUserID) {
				actions = append(actions, ActionVote)
			}
		case GamePhaseGovernanceProposal:
			if _, ok := state.GovernanceSubmissions[viewerUserID]; !ok {
				actions = append(actions, ActionSubmitGovernanceProposal, ActionSkipGovernanceProposal)
			}
		case GamePhaseGovernanceVoting:
			if _, ok := state.GovernanceVotes[viewerUserID]; !ok {
				actions = append(actions, ActionVote)
			}
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
