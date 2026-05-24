package game

import (
	"fmt"
	"strings"
	"time"

	"agentbackend/internal/models"
)

func (e *Engine) automaticEvents(state *GameState, now time.Time) ([]models.Event, error) {
	if state == nil || state.Status != GameStatusStarted || state.IsFinished {
		return nil, nil
	}

	events, err := e.timeoutEvents(state, now)
	if err != nil {
		return nil, err
	}

	botEvents, err := e.botTurnEvents(state, now)
	if err != nil {
		return nil, err
	}
	events = append(events, botEvents...)
	return events, nil
}

func (e *Engine) timeoutEvents(state *GameState, now time.Time) ([]models.Event, error) {
	if state == nil || state.Status != GameStatusStarted || state.IsFinished || !phaseDeadlineExpired(state, now) {
		return nil, nil
	}
	replacements := e.timeoutReplacementEvents(state)
	for _, event := range replacements {
		if err := ApplyEvent(state, event); err != nil {
			return nil, err
		}
	}
	return replacements, nil
}

func phaseDeadlineExpired(state *GameState, now time.Time) bool {
	return state.PhaseDeadlineAt != nil && !now.Before(*state.PhaseDeadlineAt)
}

func (e *Engine) timeoutReplacementEvents(state *GameState) []models.Event {
	pending := pendingHumanPlayersForPhase(state)
	if len(pending) == 0 {
		return nil
	}

	events := make([]models.Event, 0, len(pending)*2)
	usedNames := activePlayerNames(state)
	nextID := nextBotUserID(state)
	for index, player := range pending {
		botID := nextID - int64(index)
		name := e.replacementBotName(player, usedNames)
		usedNames[strings.ToLower(name)] = true
		position := player.Position
		if strings.TrimSpace(position) == "" {
			position = e.randomGeneratedPosition()
		}
		events = append(events, models.Event{
			GameID:    state.GameID,
			ActorName: "Система",
			EventType: models.EventPlayerReplacedByBot,
			EventValue: mustJSON(PlayerReplacedByBotPayload{
				UserID:       player.UserID,
				BotUserID:    botID,
				Name:         name,
				Position:     position,
				ShareBPS:     player.ShareBPS,
				AuthorityBPS: player.AuthorityBPS,
				IsCEO:        player.IsCEO,
				Role:         player.Role,
			}),
		})
		events = append(events, systemChatPayloadEvents(state.GameID, 0, ChatMessageSentPayload{
			SystemEventType: "player_replaced_by_bot",
			Title:           "Игрок заменен ботом",
			Summary:         fmt.Sprintf("%s не успел сделать ход за 3 минуты. Его место занял %s.", player.Name, name),
			Tone:            "warning",
		})...)
	}
	return events
}

func pendingHumanPlayersForPhase(state *GameState) []*PlayerState {
	out := []*PlayerState{}
	for _, player := range activePlayers(state) {
		if player.IsBot || player.UserID <= 0 {
			continue
		}
		if playerNeedsActionForPhase(state, player.UserID) {
			out = append(out, player)
		}
	}
	return out
}

func playerNeedsActionForPhase(state *GameState, userID int64) bool {
	player := activePlayerByID(state, userID)
	if player == nil {
		return false
	}
	switch state.Phase {
	case GamePhaseMoleObjectiveSelection:
		if player.Role == "mole" {
			return len(state.MoleTargets) == 0 && state.MoleSabotage == ""
		}
		if player.Role == RoleCompliance {
			return false
		}
		return state.MemorandumPreferences[userID] == "" && len(state.MoleTargets) == 0 && state.MoleSabotage == ""
	case GamePhaseMajorVoting:
		_, ok := state.CurrentVotes[userID]
		return !ok
	case GamePhaseMoleCaseBreakdown:
		return player.Role == RoleMole && state.CaseBreakdown != nil
	case GamePhaseGovernanceProposal:
		_, ok := state.GovernanceSubmissions[userID]
		return !ok
	case GamePhaseGovernanceVoting:
		_, ok := state.GovernanceVotes[userID]
		return !ok
	default:
		return false
	}
}

func (e *Engine) replacementBotName(player *PlayerState, used map[string]bool) string {
	base := strings.TrimSpace(player.Position)
	if base == "" {
		base = strings.TrimSpace(player.Name)
	}
	if base == "" {
		base = "директора"
	}
	name := "Заместитель " + base
	if !used[strings.ToLower(name)] {
		return name
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s %d", name, i)
		if !used[strings.ToLower(candidate)] {
			return candidate
		}
	}
	return e.randomBotName(used, -time.Now().UnixNano())
}
