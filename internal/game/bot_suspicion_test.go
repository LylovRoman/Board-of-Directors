package game

import "testing"

func TestSingleMemorandumBotScoringStillUsesOriginalHint(t *testing.T) {
	bot := &PlayerState{UserID: -1, IsBot: true, Role: "player", ShareBPS: 2000, AuthorityBPS: InitialAuthorityBPS}
	state := baseBotSuspicionTestState(bot)
	state.Memorandums[bot.UserID] = MemorandumState{
		UserID:    bot.UserID,
		Type:      MemorandumTypeRisk,
		Decisions: []string{"A", "B", "C"},
	}

	engine := &Engine{}
	riskyScore := engine.scoreBotMajorDecision(state, bot, "A")
	outsideRiskScore := engine.scoreBotMajorDecision(state, bot, "D")
	if outsideRiskScore <= riskyScore {
		t.Fatalf("expected one-memorandum risk scoring to prefer decision outside risk memo, risky=%d outside=%d", riskyScore, outsideRiskScore)
	}
}

func TestBotSuspicionTracksSabotageVoters(t *testing.T) {
	bot, suspect, trusted := botSuspicionPlayers()
	state := baseBotSuspicionTestState(bot, suspect, trusted)
	addSabotageAcceptedReport(state, suspect, trusted)

	profile := (&Engine{}).botSuspicionProfile(state, bot)
	if profile.Scores[suspect.UserID] < 80 {
		t.Fatalf("expected sabotage voter to become highly suspicious, profile=%+v", profile.Scores)
	}
	if profile.Scores[trusted.UserID] >= 0 {
		t.Fatalf("expected non-sabotage voter to become slightly trusted, profile=%+v", profile.Scores)
	}
}

func TestSuspiciousPlayerIsTargetedByGovernance(t *testing.T) {
	bot, suspect, trusted := botSuspicionPlayers()
	state := baseBotSuspicionTestState(bot, suspect, trusted)
	state.Phase = GamePhaseGovernanceProposal
	state.TreasuryShareBPS = 2000
	addSabotageAcceptedReport(state, suspect, trusted)

	payload, ok := (&Engine{}).chooseBotGovernanceProposal(state, bot)
	if !ok {
		t.Fatalf("expected bot to submit a governance proposal")
	}
	if payload.ProposalType != GovernanceProposalTreasuryBuyback || payload.TargetUserID != suspect.UserID {
		t.Fatalf("expected buyback against suspicious player, got %+v", payload)
	}
}

func TestSuspicionInfluencesCurrentMajorVoteAndGovernanceScoring(t *testing.T) {
	bot, suspect, trusted := botSuspicionPlayers()
	state := baseBotSuspicionTestState(bot, suspect, trusted)
	addSabotageAcceptedReport(state, suspect, trusted)
	b := "B"
	d := "D"
	state.CurrentVotes = map[int64]VoteState{
		suspect.UserID: {UserID: suspect.UserID, Decision: &b},
		trusted.UserID: {UserID: trusted.UserID, Decision: &d},
	}

	engine := &Engine{}
	if scoreD, scoreB := engine.scoreBotMajorDecision(state, bot, "D"), engine.scoreBotMajorDecision(state, bot, "B"); scoreD <= scoreB {
		t.Fatalf("expected bot to avoid following suspicious voter, scoreD=%d scoreB=%d", scoreD, scoreB)
	}

	buybackSuspect := &GovernanceProposalState{ID: 1, ProposerUserID: trusted.UserID, ProposalType: GovernanceProposalTreasuryBuyback, TargetUserID: suspect.UserID}
	grantSuspect := &GovernanceProposalState{ID: 2, ProposerUserID: suspect.UserID, ProposalType: GovernanceProposalTreasuryGrant, TargetUserID: suspect.UserID}
	if buybackScore, grantScore := engine.scoreBotGovernanceProposal(state, bot, buybackSuspect), engine.scoreBotGovernanceProposal(state, bot, grantSuspect); buybackScore <= grantScore {
		t.Fatalf("expected anti-suspect governance proposal to outscore grant to suspect, buyback=%d grant=%d", buybackScore, grantScore)
	}
}

func botSuspicionPlayers() (*PlayerState, *PlayerState, *PlayerState) {
	bot := &PlayerState{UserID: -1, Name: "Bot", IsBot: true, Role: "player", ShareBPS: 2500, AuthorityBPS: InitialAuthorityBPS}
	suspect := &PlayerState{UserID: -2, Name: "Suspect", IsBot: true, Role: "player", ShareBPS: 3500, AuthorityBPS: InitialAuthorityBPS}
	trusted := &PlayerState{UserID: -3, Name: "Trusted", IsBot: true, Role: "player", ShareBPS: 2000, AuthorityBPS: InitialAuthorityBPS}
	return bot, suspect, trusted
}

func baseBotSuspicionTestState(players ...*PlayerState) *GameState {
	state := &GameState{
		Status:                GameStatusStarted,
		Phase:                 GamePhaseMajorVoting,
		Players:               map[int64]*PlayerState{},
		PlayerOrder:           []int64{},
		CurrentVotes:          map[int64]VoteState{},
		Memorandums:           map[int64]MemorandumState{},
		Available:             map[string]bool{},
		GovernanceProposals:   map[int]*GovernanceProposalState{},
		GovernanceVotes:       map[int64]GovernanceVoteState{},
		GovernanceSubmissions: map[int64]GovernanceSubmissionState{},
		MajorVoteOptions:      []string{"B", "D", "E", "F"},
		TreasuryShareBPS:      0,
	}
	for _, decision := range allDecisions {
		state.Available[decision] = true
	}
	for _, player := range players {
		state.Players[player.UserID] = player
		state.PlayerOrder = append(state.PlayerOrder, player.UserID)
	}
	return state
}

func addSabotageAcceptedReport(state *GameState, suspect *PlayerState, trusted *PlayerState) {
	state.MoleSabotage = "A"
	state.AcceptedOrder = []string{"A"}
	state.RoundReports = append(state.RoundReports, RoundReport{
		Round:    1,
		Outcome:  "accepted",
		Decision: "A",
		Votes: []DecisionVoteReport{
			{
				Decision:   "A",
				ShareBPS:   suspect.ShareBPS,
				VoterCount: 1,
				Voters: []DecisionVoterReport{{
					UserID:   suspect.UserID,
					Name:     suspect.Name,
					ShareBPS: suspect.ShareBPS,
				}},
			},
			{
				Decision:   "D",
				ShareBPS:   trusted.ShareBPS,
				VoterCount: 1,
				Voters: []DecisionVoterReport{{
					UserID:   trusted.UserID,
					Name:     trusted.Name,
					ShareBPS: trusted.ShareBPS,
				}},
			},
		},
	})
}
