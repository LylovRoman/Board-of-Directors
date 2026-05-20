package game

type Rank struct {
	Title string `json:"title"`
	XP    int    `json:"xp"`
}

var rankThresholds = []Rank{
	{Title: "Стажер совета", XP: 0},
	{Title: "Младший директор", XP: 100},
	{Title: "Директор", XP: 250},
	{Title: "Старший директор", XP: 500},
	{Title: "Член комитета", XP: 900},
	{Title: "Вице-президент совета", XP: 1400},
	{Title: "Управляющий партнер", XP: 2200},
	{Title: "Председатель совета", XP: 3200},
	{Title: "Легенда совета", XP: 5000},
}

func RankForXP(xp int) Rank {
	current := rankThresholds[0]
	for _, rank := range rankThresholds {
		if xp >= rank.XP {
			current = rank
		}
	}
	return current
}

func XPTotal(breakdown []XPAward) int {
	total := 0
	for _, award := range breakdown {
		total += award.Points
	}
	return total
}

func xpBreakdownForPlayer(state *GameState, player *PlayerState, stat PublicFinalPlayerStats) []XPAward {
	if state == nil || player == nil || player.IsBot || player.UserID <= 0 || player.IsLeft || player.IsKicked {
		return nil
	}

	awards := []XPAward{{Reason: "Завершил партию", Points: 10}}
	if stat.Won {
		if player.Role == "mole" {
			awards = append(awards, XPAward{Reason: "Победа за Крота", Points: 55})
		} else {
			awards = append(awards, XPAward{Reason: "Победа за Совет", Points: 40})
		}
		if player.IsCEO {
			awards = append(awards, XPAward{Reason: "Победа в должности CEO", Points: 20})
		}
	}
	if stat.MajorVotes > 0 && stat.AccuracyBPS >= 8000 {
		awards = append(awards, XPAward{Reason: "Точность голосований выше 80%", Points: 15})
	}
	if stat.MajorVotes >= 2 && stat.AccuracyBPS == TotalSharesBPS {
		awards = append(awards, XPAward{Reason: "Идеальная точность", Points: 10})
	}
	for _, report := range state.GovernanceReports {
		if report.Outcome != "accepted" || !proposalAuthoredBy(report.Proposal, player.UserID) {
			continue
		}
		awards = append(awards, XPAward{Reason: "Принятое governance-предложение", Points: 8})
	}
	return awards
}
