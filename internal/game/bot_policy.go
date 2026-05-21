package game

import (
	"hash/fnv"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

const (
	liveBotMonteCarloRollouts     = 4
	maxGovernancePolicyCandidates = 8
)

type BotBelief struct {
	Worlds []BotObjectiveWorld
}

type BotObjectiveWorld struct {
	Targets  []string
	Sabotage string
	Weight   int
}

type TrustProfile struct {
	Trust     map[int64]int
	Suspicion map[int64]int
}

type CoalitionProfile struct {
	Links map[int64]map[int64]int
}

type botPolicyContext struct {
	Belief    BotBelief
	Trust     TrustProfile
	Coalition CoalitionProfile
}

type botGovernanceCandidate struct {
	Payload SubmitGovernanceProposalActionPayload
	Submit  bool
	Score   int
}

func (e *Engine) chooseBotMajorDecisionByPolicy(state *GameState, bot *PlayerState) (string, bool) {
	options := currentMajorOptions(state)
	rollouts := e.botPolicyRollouts()
	if state == nil || bot == nil || len(options) == 0 || rollouts <= 0 {
		return "", false
	}

	ctx := e.botPolicyContext(state, bot)
	bestDecision := options[0]
	bestScore := -1 << 30
	for _, decision := range options {
		score := e.scoreBotMajorDecision(state, bot, decision)*10 +
			e.monteCarloMajorDecisionScore(state, bot, decision, ctx, rollouts)
		if score > bestScore {
			bestScore = score
			bestDecision = decision
		}
	}
	return bestDecision, true
}

func (e *Engine) chooseBotGovernanceProposalByPolicy(state *GameState, bot *PlayerState) (SubmitGovernanceProposalActionPayload, bool, bool) {
	rollouts := e.botPolicyRollouts()
	if state == nil || bot == nil || state.Phase != GamePhaseGovernanceProposal || rollouts <= 0 {
		return SubmitGovernanceProposalActionPayload{}, false, false
	}

	ctx := e.botPolicyContext(state, bot)
	candidates := e.botGovernanceProposalCandidates(state, bot, ctx)
	if len(candidates) == 0 {
		return SubmitGovernanceProposalActionPayload{}, false, true
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > maxGovernancePolicyCandidates {
		candidates = candidates[:maxGovernancePolicyCandidates]
	}

	best := candidates[0]
	bestScore := -1 << 30
	for _, candidate := range candidates {
		score := candidate.Score*10 + e.monteCarloGovernanceProposalScore(state, bot, candidate, ctx, rollouts)
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	if !best.Submit {
		return SubmitGovernanceProposalActionPayload{}, false, true
	}
	return best.Payload, true, true
}

func (e *Engine) chooseBotGovernanceVoteByPolicy(state *GameState, bot *PlayerState) (int, bool, bool) {
	rollouts := e.botPolicyRollouts()
	if state == nil || bot == nil || state.Phase != GamePhaseGovernanceVoting || rollouts <= 0 {
		return 0, false, false
	}

	ctx := e.botPolicyContext(state, bot)
	type voteCandidate struct {
		proposalID int
		abstain    bool
		score      int
	}
	candidates := []voteCandidate{}
	if !bot.IsCEO {
		candidates = append(candidates, voteCandidate{abstain: true})
	}
	for _, proposalID := range state.GovernanceProposalOrder {
		proposal := state.GovernanceProposals[proposalID]
		if proposal == nil {
			continue
		}
		candidates = append(candidates, voteCandidate{
			proposalID: proposalID,
			score:      e.scoreBotGovernanceProposal(state, bot, proposal) + e.policyGovernanceTrustScore(bot, proposal, ctx),
		})
	}
	if len(candidates) == 0 {
		return 0, true, true
	}

	best := candidates[0]
	bestScore := -1 << 30
	for _, candidate := range candidates {
		score := candidate.score*10 + e.monteCarloGovernanceVoteScore(state, bot, candidate.proposalID, candidate.abstain, ctx, rollouts)
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best.proposalID, best.abstain, true
}

func (e *Engine) botPolicyRollouts() int {
	if e.usesDeterministicBotJitter() {
		// Bot-only balance runs use deterministic fast scoring so the same
		// seed produces the same games across worker counts and repeated runs.
		return 0
	}
	if e.botSimulationRollouts > 0 {
		return e.botSimulationRollouts
	}
	return liveBotMonteCarloRollouts
}

func (e *Engine) botPolicyContext(state *GameState, bot *PlayerState) botPolicyContext {
	return botPolicyContext{
		Belief:    e.botBelief(state, bot),
		Trust:     e.botTrustProfile(state, bot),
		Coalition: botCoalitionProfile(state),
	}
}

func (e *Engine) botBelief(state *GameState, bot *PlayerState) BotBelief {
	if state == nil || bot == nil {
		return BotBelief{}
	}
	if bot.Role == "mole" && len(state.MoleTargets) > 0 && state.MoleSabotage != "" {
		return BotBelief{Worlds: []BotObjectiveWorld{{
			Targets:  append([]string(nil), state.MoleTargets...),
			Sabotage: state.MoleSabotage,
			Weight:   1,
		}}}
	}

	memorandums := e.botKnownMemorandums(state, bot)
	cacheKey := botKnowledgeCacheKey(state, bot, memorandums, "belief", false)
	if cacheKey != "" && e.botBeliefCache != nil {
		if cached, ok := e.botBeliefCache[cacheKey]; ok {
			return cached
		}
	}
	worlds := enumerateBotBeliefWorlds(state, memorandums, true)
	if len(worlds) == 0 {
		worlds = enumerateBotBeliefWorlds(state, memorandums, false)
	}
	sortBotObjectiveWorlds(worlds)
	belief := BotBelief{Worlds: worlds}
	if cacheKey != "" {
		if e.botBeliefCache == nil {
			e.botBeliefCache = map[string]BotBelief{}
		}
		e.botBeliefCache[cacheKey] = belief
	}
	return belief
}

func enumerateBotBeliefWorlds(state *GameState, memorandums []MemorandumState, useShowcaseConstraint bool) []BotObjectiveWorld {
	if state == nil {
		return nil
	}
	options := currentMajorOptions(state)
	constrainShowcase := useShowcaseConstraint && len(options) == 4
	accepted := stringSet(state.AcceptedOrder)
	knownSabotage := ""
	if state.MoleSabotage != "" && accepted[state.MoleSabotage] {
		knownSabotage = state.MoleSabotage
	}

	worlds := []BotObjectiveWorld{}
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
					if len(memorandums) > 0 && !memorandumsMatchObjectives(memorandums, objectives) {
						continue
					}
					if state.Status == GameStatusStarted && !state.IsFinished && hypothesisHasAlreadyWon(state.AcceptedOrder, targets, sabotage) {
						continue
					}
					worlds = append(worlds, BotObjectiveWorld{
						Targets:  []string{candidates[i], candidates[j], candidates[k]},
						Sabotage: sabotage,
						Weight:   1,
					})
				}
			}
		}
	}
	return worlds
}

func sortBotObjectiveWorlds(worlds []BotObjectiveWorld) {
	sort.Slice(worlds, func(i, j int) bool {
		leftTargets := strings.Join(worlds[i].Targets, "")
		rightTargets := strings.Join(worlds[j].Targets, "")
		if leftTargets != rightTargets {
			return leftTargets < rightTargets
		}
		return worlds[i].Sabotage < worlds[j].Sabotage
	})
}

func (belief BotBelief) Sample(rng *rand.Rand) BotObjectiveWorld {
	if len(belief.Worlds) == 0 {
		return BotObjectiveWorld{}
	}
	total := 0
	for _, world := range belief.Worlds {
		total += maxInt(1, world.Weight)
	}
	pick := rng.Intn(total)
	for _, world := range belief.Worlds {
		pick -= maxInt(1, world.Weight)
		if pick < 0 {
			world.Targets = append([]string(nil), world.Targets...)
			return world
		}
	}
	world := belief.Worlds[len(belief.Worlds)-1]
	world.Targets = append([]string(nil), world.Targets...)
	return world
}

func (belief BotBelief) DeterministicSample(seed int64) BotObjectiveWorld {
	if len(belief.Worlds) == 0 {
		return BotObjectiveWorld{}
	}
	index := int(uint64(seed) % uint64(len(belief.Worlds)))
	world := belief.Worlds[index]
	world.Targets = append([]string(nil), world.Targets...)
	return world
}

func (e *Engine) botTrustProfile(state *GameState, bot *PlayerState) TrustProfile {
	profile := TrustProfile{Trust: map[int64]int{}, Suspicion: map[int64]int{}}
	if state == nil || bot == nil || bot.Role == "mole" {
		return profile
	}
	suspicion := e.botSuspicionProfile(state, bot)
	for _, player := range activePlayers(state) {
		if player.UserID == bot.UserID {
			continue
		}
		score := suspicion.Scores[player.UserID]
		profile.Suspicion[player.UserID] = score
		profile.Trust[player.UserID] = -score
	}
	for _, report := range state.GovernanceReports {
		if report.Proposal == nil {
			continue
		}
		authorSuspicion := proposalAuthorSuspicion(botSuspicionProfile{Scores: profile.Suspicion}, report.Proposal)
		for _, vote := range report.Votes {
			for _, voter := range vote.Voters {
				if voter.UserID == bot.UserID {
					continue
				}
				if vote.ProposalID == report.Proposal.ID && report.Outcome == "accepted" {
					profile.Suspicion[voter.UserID] += authorSuspicion / 8
					profile.Trust[voter.UserID] -= authorSuspicion / 8
				}
			}
		}
	}
	return profile
}

func botCoalitionProfile(state *GameState) CoalitionProfile {
	profile := CoalitionProfile{Links: map[int64]map[int64]int{}}
	if state == nil {
		return profile
	}
	addLink := func(a, b int64, score int) {
		if a == b {
			return
		}
		if profile.Links[a] == nil {
			profile.Links[a] = map[int64]int{}
		}
		if profile.Links[b] == nil {
			profile.Links[b] = map[int64]int{}
		}
		profile.Links[a][b] += score
		profile.Links[b][a] += score
	}
	for _, report := range state.RoundReports {
		for _, vote := range report.Votes {
			for i := 0; i < len(vote.Voters); i++ {
				for j := i + 1; j < len(vote.Voters); j++ {
					addLink(vote.Voters[i].UserID, vote.Voters[j].UserID, 8)
				}
			}
		}
	}
	for _, report := range state.GovernanceReports {
		if report.Proposal != nil {
			authors := report.Proposal.AuthorUserIDs
			if len(authors) == 0 {
				authors = []int64{report.Proposal.ProposerUserID}
			}
			for i := 0; i < len(authors); i++ {
				for j := i + 1; j < len(authors); j++ {
					addLink(authors[i], authors[j], 14)
				}
			}
		}
		for _, vote := range report.Votes {
			for i := 0; i < len(vote.Voters); i++ {
				for j := i + 1; j < len(vote.Voters); j++ {
					addLink(vote.Voters[i].UserID, vote.Voters[j].UserID, 6)
				}
			}
		}
	}
	return profile
}

func (profile CoalitionProfile) Link(a, b int64) int {
	if profile.Links == nil || profile.Links[a] == nil {
		return 0
	}
	return profile.Links[a][b]
}

func (e *Engine) monteCarloMajorDecisionScore(state *GameState, bot *PlayerState, decision string, ctx botPolicyContext, rollouts int) int {
	total := 0
	for i := 0; i < rollouts; i++ {
		rng := rand.New(rand.NewSource(botPolicySeed(state, bot.UserID, "major:"+decision, i)))
		world := ctx.Belief.Sample(rng)
		if e.usesDeterministicBotJitter() {
			world = ctx.Belief.DeterministicSample(botPolicySeed(state, bot.UserID, "major:"+decision, i))
		}
		projected := cloneState(state)
		applyBeliefWorld(projected, world)
		projected.CurrentVotes[bot.UserID] = VoteState{UserID: bot.UserID, Decision: stringPtr(decision)}
		e.fillRolloutMajorVotes(projected, bot, ctx, rng)
		accepted, _, resolved := resolveDecision(projected)
		total += majorDecisionUtility(projected, bot, accepted, resolved)
	}
	return total / maxInt(1, rollouts)
}

func (e *Engine) monteCarloGovernanceProposalScore(state *GameState, bot *PlayerState, candidate botGovernanceCandidate, ctx botPolicyContext, rollouts int) int {
	return e.governanceCandidateUtility(state, bot, candidate, ctx)
}

func (e *Engine) monteCarloGovernanceVoteScore(state *GameState, bot *PlayerState, proposalID int, abstain bool, ctx botPolicyContext, rollouts int) int {
	total := 0
	for i := 0; i < rollouts; i++ {
		rng := rand.New(rand.NewSource(botPolicySeed(state, bot.UserID, "govvote", i) + int64(proposalID)))
		world := ctx.Belief.Sample(rng)
		if e.usesDeterministicBotJitter() {
			world = ctx.Belief.DeterministicSample(botPolicySeed(state, bot.UserID, "govvote:"+strconv.Itoa(proposalID), i))
		}
		projected := cloneState(state)
		applyBeliefWorld(projected, world)
		var proposalIDPtr *int
		if !abstain {
			proposalIDCopy := proposalID
			proposalIDPtr = &proposalIDCopy
		}
		projected.GovernanceVotes[bot.UserID] = GovernanceVoteState{UserID: bot.UserID, ProposalID: proposalIDPtr, Abstain: abstain}
		engine := e.rolloutEngine(rng)
		engine.fillRolloutGovernanceVotes(projected, bot, ctx, rng)
		acceptedProposalID, resolved := resolveGovernanceProposal(projected)
		total += governanceVoteUtility(projected, bot, acceptedProposalID, resolved, ctx)
	}
	return total / maxInt(1, rollouts)
}

func (e *Engine) rolloutEngine(rng *rand.Rand) *Engine {
	return &Engine{
		rng:                            rng,
		botSimulationMemorandumCount:   e.botSimulationMemorandumCount,
		botSimulationMemorandumType:    e.botSimulationMemorandumType,
		botSimulationMemorandumVariant: e.botSimulationMemorandumVariant,
		botSimulationMemorandums:       e.botSimulationMemorandums,
		botSimulationRollouts:          e.botSimulationRollouts,
		botObjectiveInferenceCache:     e.botObjectiveInferenceCache,
		botBeliefCache:                 e.botBeliefCache,
		botSuspicionProfileCache:       e.botSuspicionProfileCache,
	}
}

func (e *Engine) fillRolloutMajorVotes(state *GameState, perspective *PlayerState, ctx botPolicyContext, rng *rand.Rand) {
	for _, player := range activePlayers(state) {
		if _, ok := state.CurrentVotes[player.UserID]; ok {
			continue
		}
		decision := e.rolloutMajorDecision(state, perspective, player, ctx, rng)
		if decision == "" {
			continue
		}
		state.CurrentVotes[player.UserID] = VoteState{UserID: player.UserID, Decision: stringPtr(decision)}
	}
}

func (e *Engine) rolloutMajorDecision(state *GameState, perspective *PlayerState, actor *PlayerState, ctx botPolicyContext, rng *rand.Rand) string {
	options := currentMajorOptions(state)
	if len(options) == 0 {
		return ""
	}
	best := options[0]
	bestScore := -1 << 30
	targets := stringSet(state.MoleTargets)
	for _, decision := range options {
		score := 0
		if actor.Role == "mole" {
			switch {
			case decision == state.MoleSabotage:
				score += 130
			case targets[decision]:
				score += 90
			default:
				score += 15
			}
		} else {
			if decision == state.MoleSabotage {
				score -= 120
			} else if targets[decision] {
				score -= 60
			} else {
				score += 85
			}
			score -= e.currentVoteSuspicionPressure(state, perspective, decision) / 5
		}
		if e.usesDeterministicBotJitter() {
			scope := "rollout-major:" + strconv.FormatInt(perspective.UserID, 10) + ":" + decision
			score += e.botRandInt(state, actor, scope, 23)
		} else {
			score += rng.Intn(23)
		}
		if score > bestScore {
			bestScore = score
			best = decision
		}
	}
	return best
}

func (e *Engine) fillRolloutGovernanceVotes(state *GameState, perspective *PlayerState, ctx botPolicyContext, rng *rand.Rand) {
	for _, player := range activePlayers(state) {
		if _, ok := state.GovernanceVotes[player.UserID]; ok {
			continue
		}
		proposalID, abstain := e.chooseBotGovernanceVoteFast(state, player)
		if player.IsCEO {
			abstain = false
		}
		var proposalIDPtr *int
		if !abstain && proposalID != 0 {
			proposalIDCopy := proposalID
			proposalIDPtr = &proposalIDCopy
		}
		state.GovernanceVotes[player.UserID] = GovernanceVoteState{UserID: player.UserID, ProposalID: proposalIDPtr, Abstain: abstain}
	}
}

func majorDecisionUtility(state *GameState, bot *PlayerState, decision string, resolved bool) int {
	if !resolved || decision == "" {
		return -20
	}
	targets := stringSet(state.MoleTargets)
	score := 0
	if bot.Role == "mole" {
		switch {
		case decision == state.MoleSabotage:
			score += 760
		case targets[decision]:
			score += 420
		default:
			score -= 460
		}
	} else {
		switch {
		case decision == state.MoleSabotage:
			score -= 780
		case targets[decision]:
			score -= 340
		default:
			score += 520
		}
	}
	if vote, ok := state.CurrentVotes[bot.UserID]; ok && vote.Decision != nil && *vote.Decision == decision {
		if decisionTypes[decision] == DecisionTypeGrowth {
			score += 45
		} else {
			score += 30
		}
	}
	return score
}

func (e *Engine) governanceCandidateUtility(state *GameState, bot *PlayerState, candidate botGovernanceCandidate, ctx botPolicyContext) int {
	if !candidate.Submit {
		return -35
	}
	proposal := &GovernanceProposalState{
		ProposerUserID: bot.UserID,
		AuthorUserIDs:  []int64{bot.UserID},
		ProposalType:   candidate.Payload.ProposalType,
		FromUserID:     candidate.Payload.FromUserID,
		ToUserID:       candidate.Payload.ToUserID,
		TargetUserID:   candidate.Payload.TargetUserID,
		ShareBPS:       candidate.Payload.ShareBPS,
	}
	return e.scoreBotGovernanceProposal(state, bot, proposal)*4 + e.policyGovernanceTrustScore(bot, proposal, ctx)*5
}

func governanceVoteUtility(state *GameState, bot *PlayerState, proposalID int, resolved bool, ctx botPolicyContext) int {
	if !resolved || proposalID == 0 {
		if bot.Role == "mole" {
			return -20
		}
		return 10
	}
	proposal := state.GovernanceProposals[proposalID]
	if proposal == nil {
		return 0
	}
	score := 0
	if bot.Role == "mole" {
		switch proposal.ProposalType {
		case GovernanceProposalShareTransfer:
			if proposal.ToUserID == bot.UserID {
				score += 680
			}
			if proposal.FromUserID == bot.UserID {
				score -= 760
			}
		case GovernanceProposalTreasuryGrant:
			if proposal.TargetUserID == bot.UserID {
				score += 620
			}
		case GovernanceProposalTreasuryBuyback:
			if proposal.TargetUserID == bot.UserID {
				score -= 820
			} else {
				score += 220
			}
		}
		return score
	}
	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		score += ctx.Trust.Suspicion[proposal.FromUserID] * 3
		score += ctx.Trust.Trust[proposal.ToUserID] * 2
		score -= ctx.Trust.Suspicion[proposal.ToUserID] * 2
	case GovernanceProposalTreasuryGrant:
		score += ctx.Trust.Trust[proposal.TargetUserID] * 3
		score -= ctx.Trust.Suspicion[proposal.TargetUserID] * 3
	case GovernanceProposalTreasuryBuyback:
		score += ctx.Trust.Suspicion[proposal.TargetUserID] * 4
	}
	if proposalAuthoredBy(proposal, bot.UserID) {
		score += 90
	}
	return score
}

func (e *Engine) botGovernanceProposalCandidates(state *GameState, bot *PlayerState, ctx botPolicyContext) []botGovernanceCandidate {
	candidates := []botGovernanceCandidate{{Submit: false}}
	seen := map[string]bool{"skip": true}
	add := func(payload SubmitGovernanceProposalActionPayload) {
		payload.ShareBPS = effectiveAuthorityBPS(bot)
		clampGovernanceProposalShareBPS(state, &payload)
		if err := validateGovernanceProposal(state, bot.UserID, payload); err != nil {
			return
		}
		key := governancePayloadKey(payload)
		if seen[key] {
			return
		}
		seen[key] = true
		candidate := botGovernanceCandidate{Payload: payload, Submit: true}
		candidate.Score = e.scoreGovernanceCandidateImmediate(state, bot, candidate, ctx)
		candidates = append(candidates, candidate)
	}

	if fastPayload, ok := e.chooseBotGovernanceProposalFast(state, bot); ok {
		add(fastPayload)
	}
	for _, target := range activePlayers(state) {
		if state.TreasuryShareBPS > 0 {
			add(SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryGrant, TargetUserID: target.UserID})
		}
		if target.ShareBPS > MinPlayerShareBPS {
			add(SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalTreasuryBuyback, TargetUserID: target.UserID})
		}
	}
	for _, from := range activePlayers(state) {
		if from.ShareBPS <= MinPlayerShareBPS {
			continue
		}
		for _, to := range activePlayers(state) {
			if from.UserID == to.UserID {
				continue
			}
			add(SubmitGovernanceProposalActionPayload{ProposalType: GovernanceProposalShareTransfer, FromUserID: from.UserID, ToUserID: to.UserID})
		}
	}
	for i := range candidates {
		if !candidates[i].Submit {
			candidates[i].Score = -5
		}
	}
	return candidates
}

func (e *Engine) scoreGovernanceCandidateImmediate(state *GameState, bot *PlayerState, candidate botGovernanceCandidate, ctx botPolicyContext) int {
	if !candidate.Submit {
		return -5
	}
	proposal := &GovernanceProposalState{
		ProposerUserID: bot.UserID,
		AuthorUserIDs:  []int64{bot.UserID},
		ProposalType:   candidate.Payload.ProposalType,
		FromUserID:     candidate.Payload.FromUserID,
		ToUserID:       candidate.Payload.ToUserID,
		TargetUserID:   candidate.Payload.TargetUserID,
		ShareBPS:       candidate.Payload.ShareBPS,
	}
	return e.scoreBotGovernanceProposal(state, bot, proposal) + e.policyGovernanceTrustScore(bot, proposal, ctx)
}

func (e *Engine) policyGovernanceTrustScore(bot *PlayerState, proposal *GovernanceProposalState, ctx botPolicyContext) int {
	if bot == nil || proposal == nil || bot.Role == "mole" {
		return 0
	}
	score := 0
	switch proposal.ProposalType {
	case GovernanceProposalShareTransfer:
		score += ctx.Trust.Suspicion[proposal.FromUserID] / 2
		score += ctx.Trust.Trust[proposal.ToUserID] / 3
		score -= ctx.Coalition.Link(proposal.ToUserID, mostSuspiciousID(ctx.Trust)) / 3
	case GovernanceProposalTreasuryGrant:
		score += ctx.Trust.Trust[proposal.TargetUserID] / 2
		score -= ctx.Trust.Suspicion[proposal.TargetUserID] / 2
		score -= ctx.Coalition.Link(proposal.TargetUserID, mostSuspiciousID(ctx.Trust)) / 4
	case GovernanceProposalTreasuryBuyback:
		score += ctx.Trust.Suspicion[proposal.TargetUserID] / 2
		if ctx.Trust.Suspicion[proposal.TargetUserID] >= 45 {
			score += 140
		}
	}
	return score
}

func mostSuspiciousID(profile TrustProfile) int64 {
	var out int64
	best := 0
	for userID, score := range profile.Suspicion {
		if score > best {
			best = score
			out = userID
		}
	}
	return out
}

func governancePayloadKey(payload SubmitGovernanceProposalActionPayload) string {
	switch payload.ProposalType {
	case GovernanceProposalShareTransfer:
		return string(payload.ProposalType) + ":" + int64Key(payload.FromUserID) + ":" + int64Key(payload.ToUserID)
	case GovernanceProposalTreasuryGrant, GovernanceProposalTreasuryBuyback:
		return string(payload.ProposalType) + ":" + int64Key(payload.TargetUserID)
	default:
		return string(payload.ProposalType)
	}
}

func int64Key(v int64) string {
	return strconv.FormatInt(v, 10)
}

func applyBeliefWorld(state *GameState, world BotObjectiveWorld) {
	if state == nil || len(world.Targets) == 0 || world.Sabotage == "" {
		return
	}
	state.MoleTargets = append([]string(nil), world.Targets...)
	sort.Strings(state.MoleTargets)
	state.MoleSabotage = world.Sabotage
}

func botPolicySeed(state *GameState, userID int64, salt string, rollout int) int64 {
	h := fnv.New64a()
	writeInt64Hash(h, int64(state.GameID))
	writeInt64Hash(h, userID)
	writeInt64Hash(h, int64(state.CurrentRound))
	writeInt64Hash(h, int64(state.GovernanceRound))
	writeInt64Hash(h, int64(len(state.AcceptedOrder)))
	writeInt64Hash(h, int64(rollout))
	_, _ = h.Write([]byte(salt))
	return int64(h.Sum64())
}

func writeInt64Hash(h interface{ Write([]byte) (int, error) }, v int64) {
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(uint64(v) >> (i * 8))
	}
	_, _ = h.Write(b[:])
}

func stringPtr(value string) *string {
	v := value
	return &v
}
