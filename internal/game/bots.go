package game

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"agentbackend/internal/models"
)

const maxBotTurnIterations = 96

var botNames = []string{
	"AI Strategy",
	"AI Finance",
	"AI Legal",
	"AI Operations",
	"AI Risk",
	"AI Growth",
	"AI Audit",
	"AI People",
}

func (e *Engine) handleAddBot(state *GameState, actor *models.User, raw []byte) ([]models.Event, error) {
	if state.Status != GameStatusLobby {
		return nil, errors.New("cannot add bots after game started")
	}
	if actor.ID != state.HostUserID {
		return nil, errors.New("only host can add bots")
	}

	var payload AddBotActionPayload
	if err := decodeActionPayload(raw, &payload); err != nil {
		return nil, err
	}
	count := payload.Count
	if count == 0 {
		count = 1
	}
	if count < 0 {
		return nil, errors.New("bot count must be positive")
	}
	activeCount := len(activePlayers(state))
	if activeCount+count > MaxPlayers {
		return nil, fmt.Errorf("game is full: max %d players", MaxPlayers)
	}

	events := make([]models.Event, 0, count)
	nextID := nextBotUserID(state)
	usedNames := activePlayerNames(state)
	for i := 0; i < count; i++ {
		botID := nextID - int64(i)
		name := strings.TrimSpace(payload.Name)
		if count != 1 || name == "" || usedNames[strings.ToLower(name)] {
			name = e.randomBotName(usedNames, botID)
		}
		usedNames[strings.ToLower(name)] = true

		events = append(events, models.Event{
			GameID:    state.GameID,
			ActorName: actor.Name,
			EventType: models.EventPlayerJoined,
			EventValue: mustJSON(PlayerJoinedPayload{
				UserID:   botID,
				Name:     name,
				Position: e.randomGeneratedPosition(),
				IsBot:    true,
			}),
		})
	}
	return events, nil
}

func (e *Engine) botTurnEvents(state *GameState, now time.Time) ([]models.Event, error) {
	if state == nil || state.Status != GameStatusStarted || state.IsFinished {
		return nil, nil
	}

	var events []models.Event
	for i := 0; i < maxBotTurnIterations; i++ {
		nextEvents, err := e.nextBotTurnEvents(state, now)
		if err != nil {
			return nil, err
		}
		if len(nextEvents) == 0 {
			return events, nil
		}
		events = append(events, nextEvents...)
		for _, event := range nextEvents {
			if err := ApplyEvent(state, event); err != nil {
				return nil, err
			}
		}
		if state.IsFinished {
			return events, nil
		}
	}
	return nil, errors.New("bot turn loop did not settle")
}

func (e *Engine) nextBotTurnEvents(state *GameState, now time.Time) ([]models.Event, error) {
	if state.Status != GameStatusStarted || state.IsFinished {
		return nil, nil
	}
	if !botsMayAct(state, now) {
		return nil, nil
	}

	switch state.Phase {
	case GamePhaseMoleObjectiveSelection:
		for _, bot := range activeBots(state) {
			if bot.Role != RoleMole && bot.Role != RoleCompliance && state.MemorandumPreferences[bot.UserID] == "" && len(state.MoleTargets) == 0 && state.MoleSabotage == "" {
				return []models.Event{e.botMemorandumPreferenceEvent(state, bot)}, nil
			}
		}
		if len(state.MoleTargets) == 0 && state.MoleSabotage == "" {
			if mole := activePlayerByID(state, state.MoleUserID); mole != nil && mole.IsBot {
				return e.botMoleObjectiveEvents(state, mole, now), nil
			}
		}
	case GamePhaseMajorVoting:
		for _, bot := range activeBots(state) {
			if bot.Role == RoleCompliance && complianceWatchAvailable(state) && state.ComplianceWatches[state.CurrentRound].TargetUserID == 0 {
				return e.botComplianceWatchEvents(state, bot), nil
			}
		}
		if state.MajorVoteUnlockedAt != nil && now.Before(*state.MajorVoteUnlockedAt) {
			return nil, nil
		}
		for _, bot := range activeBots(state) {
			if _, ok := state.CurrentVotes[bot.UserID]; !ok {
				return e.botMajorVoteEvents(state, bot), nil
			}
		}
	case GamePhaseGovernanceProposal:
		for _, bot := range activeBots(state) {
			if _, ok := state.GovernanceSubmissions[bot.UserID]; !ok {
				return e.botGovernanceSubmissionEvents(state, bot), nil
			}
		}
	case GamePhaseGovernanceVoting:
		for _, bot := range activeBots(state) {
			if _, ok := state.GovernanceVotes[bot.UserID]; !ok {
				return e.botGovernanceVoteEvents(state, bot), nil
			}
		}
	}
	return nil, nil
}

func botsMayAct(state *GameState, now time.Time) bool {
	if state.PhaseStartedAt == nil {
		return true
	}
	return !now.Before(state.PhaseStartedAt.Add(BotActionDelay))
}

func (e *Engine) botMemorandumPreferenceEvent(state *GameState, bot *PlayerState) models.Event {
	actor := botActor(bot)
	memorandumType := MemorandumTypeOpportunity
	if e.chance(45) {
		memorandumType = MemorandumTypeRisk
	}
	return models.Event{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventMemorandumPreferenceSelected,
		EventValue: mustJSON(MemorandumPreferenceSelectedPayload{
			UserID: bot.UserID,
			Type:   memorandumType,
		}),
	}
}

func (e *Engine) botMoleObjectiveEvents(state *GameState, bot *PlayerState, now time.Time) []models.Event {
	actor := botActor(bot)
	targets, sabotage := e.chooseBotMoleObjectives()
	showcase := e.majorShowcase(state.Available, targets, sabotage)
	unlockedAt := now.UTC().Add(FirstMajorVoteLock)

	events := []models.Event{{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventMoleObjectivesSelected,
		EventValue: mustJSON(MoleObjectivesSelectedPayload{
			Targets:  targets,
			Sabotage: sabotage,
		}),
	}}
	events = append(events, e.memorandumAssignmentEvents(state, actor, targets, sabotage)...)
	events = append(events, models.Event{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventVotingRoundStarted,
		EventValue: mustJSON(VotingRoundStartedPayload{
			Round:             1,
			ShowcaseDecisions: showcase,
			UnlockedAt:        &unlockedAt,
		}),
	})
	return events
}

func (e *Engine) botComplianceWatchEvents(state *GameState, bot *PlayerState) []models.Event {
	target := e.chooseBotComplianceWatchTarget(state, bot)
	if target == nil {
		return nil
	}
	actor := botActor(bot)
	return []models.Event{{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventComplianceWatchPlaced,
		EventValue: mustJSON(ComplianceWatchPlacedPayload{
			RoundNumber:      state.CurrentRound,
			ComplianceUserID: bot.UserID,
			TargetUserID:     target.UserID,
		}),
	}}
}

func (e *Engine) chooseBotComplianceWatchTarget(state *GameState, bot *PlayerState) *PlayerState {
	if state == nil || bot == nil {
		return nil
	}
	profile := e.botSuspicionProfile(state, bot)
	if suspect := mostSuspiciousPlayer(state, bot, profile, 1, func(player *PlayerState) bool {
		return player.UserID != bot.UserID
	}); suspect != nil {
		return suspect
	}
	candidates := []*PlayerState{}
	for _, player := range activePlayers(state) {
		if player.UserID != bot.UserID {
			candidates = append(candidates, player)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[complianceWatchFallbackIndex(state, bot, len(candidates))]
}

func complianceWatchFallbackIndex(state *GameState, bot *PlayerState, count int) int {
	if count <= 0 || state == nil || bot == nil {
		return 0
	}
	x := uint64(state.GameID)*0x9e3779b97f4a7c15 ^
		uint64(state.CurrentRound)*0xbf58476d1ce4e5b9 ^
		uint64(bot.UserID)*0x94d049bb133111eb
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int(x % uint64(count))
}

func (e *Engine) botMajorVoteEvents(state *GameState, bot *PlayerState) []models.Event {
	decision := e.chooseBotMajorDecision(state, bot)
	if decision == "" {
		return nil
	}
	actor := botActor(bot)
	event := models.Event{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventVoteSubmitted,
		EventValue: mustJSON(VoteSubmittedPayload{
			Round:    state.CurrentRound,
			UserID:   bot.UserID,
			Decision: &decision,
			Abstain:  false,
		}),
	}

	events := []models.Event{event}
	projected := cloneState(state)
	projected.CurrentVotes[bot.UserID] = VoteState{UserID: bot.UserID, Decision: &decision}
	if len(projected.CurrentVotes) == len(activePlayers(projected)) {
		events = append(events, e.resolveRound(projected, actor)...)
	}
	return events
}

func (e *Engine) botGovernanceSubmissionEvents(state *GameState, bot *PlayerState) []models.Event {
	actor := botActor(bot)
	payload, ok := e.chooseBotGovernanceProposal(state, bot)
	if ok {
		payload.ShareBPS = effectiveAuthorityBPS(bot)
		clampGovernanceProposalShareBPS(state, &payload)
		if err := validateGovernanceProposal(state, bot.UserID, payload); err != nil {
			ok = false
		}
	}

	var event models.Event
	if ok {
		proposalID := nextGovernanceProposalID(state)
		if existing := findMatchingGovernanceProposal(state, payload); existing != nil {
			proposalID = existing.ID
		}
		event = models.Event{
			GameID:    state.GameID,
			ActorName: actor.Name,
			EventType: models.EventGovernanceProposalSubmitted,
			EventValue: mustJSON(GovernanceProposalSubmittedPayload{
				Round:          state.GovernanceRound,
				ProposalID:     proposalID,
				ProposerUserID: bot.UserID,
				AuthorUserIDs:  []int64{bot.UserID},
				ProposalType:   payload.ProposalType,
				FromUserID:     payload.FromUserID,
				ToUserID:       payload.ToUserID,
				TargetUserID:   payload.TargetUserID,
				ShareBPS:       payload.ShareBPS,
			}),
		}
	} else {
		event = models.Event{
			GameID:    state.GameID,
			ActorName: actor.Name,
			EventType: models.EventGovernanceProposalSkipped,
			EventValue: mustJSON(GovernanceProposalSkippedPayload{
				Round:  state.GovernanceRound,
				UserID: bot.UserID,
			}),
		}
	}

	events := []models.Event{event}
	projected := cloneState(state)
	if err := ApplyEvent(projected, event); err != nil {
		return events
	}
	events = append(events, e.governanceEventsAfterSubmission(projected, actor)...)
	return events
}

func (e *Engine) botGovernanceVoteEvents(state *GameState, bot *PlayerState) []models.Event {
	proposalID, abstain := e.chooseBotGovernanceVote(state, bot)
	if bot.IsCEO {
		abstain = false
	}
	if proposalID == 0 && !abstain {
		return nil
	}

	actor := botActor(bot)
	var proposalIDPtr *int
	if !abstain {
		proposalIDPtr = &proposalID
	}
	event := models.Event{
		GameID:    state.GameID,
		ActorName: actor.Name,
		EventType: models.EventGovernanceVoteSubmitted,
		EventValue: mustJSON(GovernanceVoteSubmittedPayload{
			Round:      state.GovernanceRound,
			UserID:     bot.UserID,
			ProposalID: proposalIDPtr,
			Abstain:    abstain,
		}),
	}

	events := []models.Event{event}
	projected := cloneState(state)
	projected.GovernanceVotes[bot.UserID] = GovernanceVoteState{
		UserID:     bot.UserID,
		ProposalID: proposalIDPtr,
		Abstain:    abstain,
	}
	if len(projected.GovernanceVotes) == len(activePlayers(projected)) {
		events = append(events, e.resolveGovernance(projected, actor)...)
	}
	return events
}

func (e *Engine) chooseBotMoleObjectives() ([]string, string) {
	targets := e.randomTargets()
	targetSet := stringSet(targets)
	candidates := make([]string, 0, len(allDecisions)-len(targets))
	for _, decision := range allDecisions {
		if !targetSet[decision] {
			candidates = append(candidates, decision)
		}
	}
	if len(candidates) == 0 {
		candidates = append([]string(nil), allDecisions...)
	}
	sabotage := randomDecisionSubset(e, candidates, 1)[0]
	return targets, sabotage
}

func (e *Engine) chooseBotMajorDecision(state *GameState, bot *PlayerState) string {
	if decision, ok := e.chooseBotMajorDecisionByPolicy(state, bot); ok {
		return decision
	}
	return e.chooseBotMajorDecisionFast(state, bot)
}

func (e *Engine) chooseBotMajorDecisionFast(state *GameState, bot *PlayerState) string {
	options := currentMajorOptions(state)
	if len(options) == 0 {
		return ""
	}

	bestDecision := options[0]
	bestScore := -1 << 30
	for _, decision := range options {
		score := e.scoreBotMajorDecision(state, bot, decision) + e.randInt(21)
		if e.chance(8) {
			score += e.randInt(45)
		}
		if score > bestScore {
			bestScore = score
			bestDecision = decision
		}
	}
	return bestDecision
}

func (e *Engine) scoreBotMajorDecision(state *GameState, bot *PlayerState, decision string) int {
	if bot.Role == "mole" {
		if decision == state.MoleSabotage {
			return 110
		}
		if stringSet(state.MoleTargets)[decision] {
			return 85
		}
		return 20
	}

	score := 50
	memorandums := e.botKnownMemorandums(state, bot)
	if inference, ok := e.botObjectiveInference(state, bot); ok {
		score += inference.directorDecisionScore(decision)
	} else if len(memorandums) > 0 {
		memorandum := memorandums[0]
		inMemo := stringSet(memorandum.Decisions)[decision]
		if memorandum.Type == MemorandumTypeRisk {
			if inMemo {
				score -= 28
			} else {
				score += 18
			}
		} else {
			if inMemo {
				score += 28
			} else {
				score -= 8
			}
		}
	}
	if decisionTypes[decision] == DecisionTypeEmpowerment {
		if effectiveAuthorityBPS(bot) < 500 {
			score += 10
		}
	} else if bot.ShareBPS < 1500 || state.TreasuryShareBPS > 0 {
		score += 8
	}
	score -= e.currentVoteSuspicionPressure(state, bot, decision) / 4
	return score
}

type botObjectiveInference struct {
	Total          int
	TargetCounts   map[string]int
	SabotageCounts map[string]int
	CleanCounts    map[string]int
}

func (e *Engine) botObjectiveInference(state *GameState, bot *PlayerState) (botObjectiveInference, bool) {
	memorandums := e.botKnownMemorandums(state, bot)
	if len(memorandums) <= 1 {
		return botObjectiveInference{}, false
	}

	inference := inferBotObjectives(state, memorandums, true)
	if inference.Total == 0 {
		inference = inferBotObjectives(state, memorandums, false)
	}
	return inference, inference.Total > 0
}

func (e *Engine) botKnownMemorandums(state *GameState, bot *PlayerState) []MemorandumState {
	if bot == nil {
		return nil
	}
	if len(e.botSimulationMemorandums[bot.UserID]) > 0 {
		out := e.botSimulationMemorandums[bot.UserID]
		return append([]MemorandumState(nil), out...)
	}
	if memorandum, ok := state.Memorandums[bot.UserID]; ok {
		return []MemorandumState{memorandum}
	}
	return nil
}

func inferBotObjectives(state *GameState, memorandums []MemorandumState, useShowcaseConstraint bool) botObjectiveInference {
	inference := botObjectiveInference{
		TargetCounts:   map[string]int{},
		SabotageCounts: map[string]int{},
		CleanCounts:    map[string]int{},
	}
	options := currentMajorOptions(state)
	constrainShowcase := useShowcaseConstraint && len(options) == 4
	accepted := stringSet(state.AcceptedOrder)
	knownSabotage := ""
	if state.MoleSabotage != "" && accepted[state.MoleSabotage] {
		knownSabotage = state.MoleSabotage
	}

	for _, sabotage := range allDecisions {
		if knownSabotage != "" && sabotage != knownSabotage {
			continue
		}
		if knownSabotage == "" && accepted[sabotage] {
			continue
		}
		candidates := make([]string, 0, len(allDecisions)-1)
		for _, decision := range allDecisions {
			if decision != sabotage {
				candidates = append(candidates, decision)
			}
		}
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				for k := j + 1; k < len(candidates); k++ {
					targets := map[string]bool{
						candidates[i]: true,
						candidates[j]: true,
						candidates[k]: true,
					}
					objectives := map[string]bool{sabotage: true}
					for target := range targets {
						objectives[target] = true
					}
					if constrainShowcase && countDecisionsInSet(options, objectives) != 2 {
						continue
					}
					if !memorandumsMatchObjectives(memorandums, objectives) {
						continue
					}
					if state.Status == GameStatusStarted && !state.IsFinished && hypothesisHasAlreadyWon(state.AcceptedOrder, targets, sabotage) {
						continue
					}

					inference.Total++
					for _, decision := range allDecisions {
						if decision == sabotage {
							inference.SabotageCounts[decision]++
						} else if targets[decision] {
							inference.TargetCounts[decision]++
						} else {
							inference.CleanCounts[decision]++
						}
					}
				}
			}
		}
	}
	return inference
}

func (inference botObjectiveInference) directorDecisionScore(decision string) int {
	if inference.Total <= 0 {
		return 0
	}
	cleanBPS := inference.CleanCounts[decision] * 10000 / inference.Total
	targetBPS := inference.TargetCounts[decision] * 10000 / inference.Total
	sabotageBPS := inference.SabotageCounts[decision] * 10000 / inference.Total
	riskBPS := targetBPS + 2*sabotageBPS
	return (cleanBPS-5000)/70 - riskBPS/250
}

func memorandumsMatchObjectives(memorandums []MemorandumState, objectives map[string]bool) bool {
	for _, memorandum := range memorandums {
		if !memorandumMatches(memorandum.Decisions, objectives, memorandum.Type) {
			return false
		}
	}
	return true
}

func countDecisionsInSet(decisions []string, set map[string]bool) int {
	count := 0
	for _, decision := range decisions {
		if set[decision] {
			count++
		}
	}
	return count
}

func hypothesisHasAlreadyWon(acceptedOrder []string, targets map[string]bool, sabotage string) bool {
	molePoints := 0
	playersPoints := 0
	seen := map[string]bool{}
	for _, decision := range acceptedOrder {
		if seen[decision] {
			continue
		}
		seen[decision] = true
		switch {
		case decision == sabotage:
			molePoints += 2
		case targets[decision]:
			molePoints++
		default:
			playersPoints++
		}
		if molePoints >= 3 || playersPoints >= 3 {
			return true
		}
	}
	return false
}

type botSuspicionProfile struct {
	Scores map[int64]int
}

func (e *Engine) botSuspicionProfile(state *GameState, bot *PlayerState) botSuspicionProfile {
	profile := botSuspicionProfile{Scores: map[int64]int{}}
	if state == nil || bot == nil || bot.Role == "mole" {
		return profile
	}
	for _, player := range activePlayers(state) {
		if player.UserID != bot.UserID {
			profile.Scores[player.UserID] = 0
		}
	}

	for _, report := range state.RoundReports {
		acceptedSabotage := report.Outcome == "accepted" &&
			report.Decision != "" &&
			state.MoleSabotage != "" &&
			report.Decision == state.MoleSabotage &&
			decisionAccepted(state, report.Decision)

		for _, vote := range report.Votes {
			voteDecision := vote.Decision
			if vote.Abstain || voteDecision == "" {
				continue
			}
			for _, voter := range vote.Voters {
				if voter.UserID == bot.UserID {
					continue
				}
				if activePlayerByID(state, voter.UserID) == nil {
					continue
				}
				switch {
				case acceptedSabotage && voteDecision == report.Decision:
					profile.Scores[voter.UserID] += 95
				case acceptedSabotage:
					profile.Scores[voter.UserID] -= 14
				case report.Outcome == "accepted" && voteDecision == report.Decision:
					profile.Scores[voter.UserID] += e.botDecisionSuspicionScore(state, bot, voteDecision)
				case report.Outcome == "rejected":
					profile.Scores[voter.UserID] += e.botDecisionSuspicionScore(state, bot, voteDecision) / 2
				default:
					profile.Scores[voter.UserID] += e.botDecisionSuspicionScore(state, bot, voteDecision) / 3
				}
			}
		}
	}

	return profile
}

func (e *Engine) botDecisionSuspicionScore(state *GameState, bot *PlayerState, decision string) int {
	if decision == "" {
		return 0
	}
	if inference, ok := e.botObjectiveInference(state, bot); ok {
		riskBPS := (inference.TargetCounts[decision] + 2*inference.SabotageCounts[decision]) * 10000 / inference.Total
		cleanBPS := inference.CleanCounts[decision] * 10000 / inference.Total
		return riskBPS/250 - cleanBPS/320
	}
	memorandums := e.botKnownMemorandums(state, bot)
	if len(memorandums) == 0 {
		return 0
	}
	memorandum := memorandums[0]
	inMemo := stringSet(memorandum.Decisions)[decision]
	if memorandum.Type == MemorandumTypeRisk {
		if inMemo {
			return 22
		}
		return -8
	}
	if inMemo {
		return -12
	}
	return 4
}

func (e *Engine) currentVoteSuspicionPressure(state *GameState, bot *PlayerState, decision string) int {
	if state == nil || bot == nil || bot.Role == "mole" || decision == "" {
		return 0
	}
	profile := e.botSuspicionProfile(state, bot)
	pressure := 0
	for userID, vote := range state.CurrentVotes {
		if userID == bot.UserID || vote.Abstain || vote.Decision == nil || *vote.Decision != decision {
			continue
		}
		pressure += profile.Scores[userID]
	}
	return maxInt(-120, minInt(160, pressure))
}

func (e *Engine) chooseBotGovernanceProposal(state *GameState, bot *PlayerState) (SubmitGovernanceProposalActionPayload, bool) {
	if payload, ok, handled := e.chooseBotGovernanceProposalByPolicy(state, bot); handled {
		return payload, ok
	}
	return e.chooseBotGovernanceProposalFast(state, bot)
}

func (e *Engine) chooseBotGovernanceProposalFast(state *GameState, bot *PlayerState) (SubmitGovernanceProposalActionPayload, bool) {
	if bot.Role == "mole" {
		if state.TreasuryShareBPS > 0 {
			return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryGrant, TargetUserID: bot.UserID}, true
		}
		if donor := richestPlayer(state, func(player *PlayerState) bool {
			return player.UserID != bot.UserID && player.ShareBPS > MinPlayerShareBPS
		}); donor != nil {
			return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalShareTransfer, FromUserID: donor.UserID, ToUserID: bot.UserID}, true
		}
		if target := richestPlayer(state, func(player *PlayerState) bool {
			return player.UserID != bot.UserID && player.ShareBPS > MinPlayerShareBPS
		}); target != nil {
			return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryBuyback, TargetUserID: target.UserID}, true
		}
		return SubmitGovernanceProposalActionPayload{}, false
	}

	profile := e.botSuspicionProfile(state, bot)
	if suspect := mostSuspiciousPlayer(state, bot, profile, 45, func(player *PlayerState) bool {
		return player.ShareBPS > MinPlayerShareBPS
	}); suspect != nil {
		return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryBuyback, TargetUserID: suspect.UserID}, true
	}

	averageShare := averageActiveShareBPS(state)
	if bot.ShareBPS <= averageShare && state.TreasuryShareBPS > 0 {
		return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryGrant, TargetUserID: bot.UserID}, true
	}
	if leader := richestPlayer(state, func(player *PlayerState) bool {
		return player.UserID != bot.UserID && player.ShareBPS > MinPlayerShareBPS
	}); leader != nil && leader.ShareBPS > bot.ShareBPS+200 {
		return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryBuyback, TargetUserID: leader.UserID}, true
	}
	if state.TreasuryShareBPS > 0 {
		target := leastSuspiciousPlayer(state, bot, profile, func(player *PlayerState) bool {
			return profile.Scores[player.UserID] < 35
		})
		if target == nil {
			target = bot
		}
		return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryGrant, TargetUserID: target.UserID}, true
	}
	donor := mostSuspiciousPlayer(state, bot, profile, 20, func(player *PlayerState) bool {
		return player.ShareBPS > MinPlayerShareBPS
	})
	if donor == nil {
		donor = richestPlayer(state, func(player *PlayerState) bool {
			return player.UserID != bot.UserID && player.ShareBPS > MinPlayerShareBPS
		})
	}
	recipient := leastSuspiciousPlayer(state, bot, profile, func(player *PlayerState) bool {
		return donor != nil && player.UserID != donor.UserID
	})
	if donor != nil && recipient != nil {
		return SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalShareTransfer, FromUserID: donor.UserID, ToUserID: recipient.UserID}, true
	}
	return SubmitGovernanceProposalActionPayload{}, false
}

func (e *Engine) chooseBotGovernanceVote(state *GameState, bot *PlayerState) (int, bool) {
	if proposalID, abstain, ok := e.chooseBotGovernanceVoteByPolicy(state, bot); ok {
		return proposalID, abstain
	}
	return e.chooseBotGovernanceVoteFast(state, bot)
}

func (e *Engine) chooseBotGovernanceVoteFast(state *GameState, bot *PlayerState) (int, bool) {
	bestProposalID := 0
	bestScore := -1 << 30
	for _, proposalID := range state.GovernanceProposalOrder {
		proposal := state.GovernanceProposals[proposalID]
		if proposal == nil {
			continue
		}
		score := e.scoreBotGovernanceProposal(state, bot, proposal) + e.randInt(17)
		if score > bestScore {
			bestScore = score
			bestProposalID = proposalID
		}
	}
	if bestProposalID == 0 {
		return 0, true
	}
	if !bot.IsCEO && bestScore < -10 && e.chance(65) {
		return 0, true
	}
	return bestProposalID, false
}

func (e *Engine) scoreBotGovernanceProposal(state *GameState, bot *PlayerState, proposal *GovernanceProposalState) int {
	score := 0
	if proposalAuthoredBy(proposal, bot.UserID) {
		score += 40
	}
	profile := botSuspicionProfile{Scores: map[int64]int{}}
	if bot.Role != "mole" {
		profile = e.botSuspicionProfile(state, bot)
		score -= proposalAuthorSuspicion(profile, proposal) / 5
	}
	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		if proposal.ToUserID == bot.UserID {
			score += 80
		}
		if proposal.FromUserID == bot.UserID {
			score -= 110
		}
		if bot.Role == "mole" {
			if proposal.ToUserID == state.MoleUserID {
				score += 70
			}
			if proposal.FromUserID == state.MoleUserID {
				score -= 120
			}
		} else {
			from := activePlayerByID(state, proposal.FromUserID)
			to := activePlayerByID(state, proposal.ToUserID)
			if from != nil && to != nil && from.ShareBPS > to.ShareBPS {
				score += 20
			}
			score += profile.Scores[proposal.FromUserID] / 2
			score -= profile.Scores[proposal.ToUserID] / 2
		}
	case GovernanceProposalTreasuryGrant:
		if proposal.TargetUserID == bot.UserID {
			score += 75
		}
		if bot.Role == "mole" && proposal.TargetUserID == state.MoleUserID {
			score += 85
		}
		if bot.Role != "mole" {
			target := activePlayerByID(state, proposal.TargetUserID)
			if target != nil && target.ShareBPS <= averageActiveShareBPS(state) {
				score += 18
			}
			score -= profile.Scores[proposal.TargetUserID] / 2
		}
	case GovernanceProposalTreasuryBuyback:
		if proposal.TargetUserID == bot.UserID {
			score -= 110
		}
		if bot.Role == "mole" && proposal.TargetUserID != state.MoleUserID {
			score += 45
		}
		if bot.Role != "mole" {
			target := activePlayerByID(state, proposal.TargetUserID)
			if target != nil && target.ShareBPS > averageActiveShareBPS(state) {
				score += 24
			}
			score += profile.Scores[proposal.TargetUserID] / 2
		}
	}
	return score
}

func mostSuspiciousPlayer(state *GameState, bot *PlayerState, profile botSuspicionProfile, minimumScore int, include func(*PlayerState) bool) *PlayerState {
	var best *PlayerState
	bestScore := minimumScore - 1
	for _, player := range activePlayers(state) {
		if player.UserID == bot.UserID {
			continue
		}
		if include != nil && !include(player) {
			continue
		}
		score := profile.Scores[player.UserID]
		if score > bestScore || (score == bestScore && best != nil && player.ShareBPS > best.ShareBPS) {
			best = player
			bestScore = score
		}
	}
	return best
}

func leastSuspiciousPlayer(state *GameState, bot *PlayerState, profile botSuspicionProfile, include func(*PlayerState) bool) *PlayerState {
	var best *PlayerState
	bestScore := 1 << 30
	for _, player := range activePlayers(state) {
		if include != nil && !include(player) {
			continue
		}
		score := profile.Scores[player.UserID]
		if player.UserID == bot.UserID {
			score -= 6
		}
		if score < bestScore || (score == bestScore && best != nil && player.ShareBPS < best.ShareBPS) {
			best = player
			bestScore = score
		}
	}
	return best
}

func proposalAuthorSuspicion(profile botSuspicionProfile, proposal *GovernanceProposalState) int {
	if proposal == nil {
		return 0
	}
	score := profile.Scores[proposal.ProposerUserID]
	for _, authorID := range proposal.AuthorUserIDs {
		score = maxInt(score, profile.Scores[authorID])
	}
	return score
}

func nextBotUserID(state *GameState) int64 {
	next := int64(-1)
	for userID := range state.Players {
		if userID <= next {
			next = userID - 1
		}
	}
	return next
}

func activeBots(state *GameState) []*PlayerState {
	players := activePlayers(state)
	out := make([]*PlayerState, 0, len(players))
	for _, player := range players {
		if player.IsBot {
			out = append(out, player)
		}
	}
	return out
}

func activePlayerNames(state *GameState) map[string]bool {
	out := map[string]bool{}
	for _, player := range activePlayers(state) {
		out[strings.ToLower(player.Name)] = true
	}
	return out
}

func (e *Engine) randomBotName(used map[string]bool, botID int64) string {
	indexes := make([]int, len(botNames))
	for i := range indexes {
		indexes[i] = i
	}
	e.shuffleWithRNG(len(indexes), func(i, j int) {
		indexes[i], indexes[j] = indexes[j], indexes[i]
	})
	for _, index := range indexes {
		name := botNames[index]
		if !used[strings.ToLower(name)] {
			return name
		}
	}
	return fmt.Sprintf("AI Director %d", -botID)
}

func currentMajorOptions(state *GameState) []string {
	options := append([]string(nil), state.MajorVoteOptions...)
	if len(options) == 0 {
		options = sortedAvailableDecisions(state.Available)
	}
	filtered := options[:0]
	for _, option := range options {
		if state.Available[option] {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

func botActor(bot *PlayerState) *models.User {
	return &models.User{
		ID:       bot.UserID,
		Name:     bot.Name,
		Position: bot.Position,
	}
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func richestPlayer(state *GameState, include func(*PlayerState) bool) *PlayerState {
	var best *PlayerState
	for _, player := range activePlayers(state) {
		if include != nil && !include(player) {
			continue
		}
		if best == nil || player.ShareBPS > best.ShareBPS || (player.ShareBPS == best.ShareBPS && player.UserID < best.UserID) {
			best = player
		}
	}
	return best
}

func poorestPlayer(state *GameState, include func(*PlayerState) bool) *PlayerState {
	var best *PlayerState
	for _, player := range activePlayers(state) {
		if include != nil && !include(player) {
			continue
		}
		if best == nil || player.ShareBPS < best.ShareBPS || (player.ShareBPS == best.ShareBPS && player.UserID < best.UserID) {
			best = player
		}
	}
	return best
}

func averageActiveShareBPS(state *GameState) int {
	players := activePlayers(state)
	if len(players) == 0 {
		return 0
	}
	total := 0
	for _, player := range players {
		total += player.ShareBPS
	}
	return total / len(players)
}

func proposalAuthoredBy(proposal *GovernanceProposalState, userID int64) bool {
	if proposal == nil {
		return false
	}
	if proposal.ProposerUserID == userID {
		return true
	}
	for _, authorID := range proposal.AuthorUserIDs {
		if authorID == userID {
			return true
		}
	}
	return false
}

func scrubSyntheticEventUserIDs(events []models.Event) {
	for i := range events {
		if events[i].UserID != nil && *events[i].UserID <= 0 {
			events[i].UserID = nil
		}
	}
}

func (e *Engine) randInt(n int) int {
	if n <= 0 {
		return 0
	}
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	return e.rng.Intn(n)
}

func (e *Engine) chance(percent int) bool {
	percent = maxInt(0, minInt(100, percent))
	return e.randInt(100) < percent
}
