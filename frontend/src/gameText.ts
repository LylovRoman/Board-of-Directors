import type {
  DecisionType,
  GamePhase,
  GameStatus,
  MemorandumType,
  MemorandumVariant,
  PublicChatMessage,
  PublicGameState,
  PublicGovernanceProposal,
  PublicGovernanceReport,
  PublicPlayerState,
} from "./types";
import { DECISION_OPTIONS, DECISION_TITLES, DECISION_TYPE_FALLBACK } from "./gameData";
import type { LiveStatus } from "./gameData";

export function statusLabel(status?: GameStatus): string {
  switch (status) {
    case "lobby":
      return "Ожидает игроков";
    case "started":
      return "Идет заседание";
    case "finished":
      return "Завершена";
    default:
      return "Статус уточняется";
  }
}

export function phaseLabel(phase?: GamePhase): string {
  switch (phase) {
    case "mole_objective_selection":
      return "Выбор целей Крота";
    case "mole_case_breakdown":
      return "Развал дела";
    case "governance_proposal":
    case "governance_voting":
      return "Корпоративные манёвры";
    case "major_voting":
      return "Мажорное голосование";
    default:
      return "Подготовка";
  }
}

export function roleLabel(role?: string): string {
  if (role === "mole") {
    return "Крот";
  }
  if (role === "compliance") {
    return "Комплаенс";
  }
  return "Директор";
}

export function winnerLabel(winner?: string): string {
  if (winner === "mole") {
    return "Крот победил";
  }
  if (winner === "players") {
    return "Совет директоров победил";
  }
  return "Игра завершена";
}

export function formatShare(bps?: number): string {
  const value = typeof bps === "number" ? bps : 0;
  return `${(value / 100).toFixed(value % 100 === 0 ? 0 : 1)}%`;
}

export function formatAccuracy(bps?: number): string {
  return formatShare(bps);
}

export function liveStatusLabel(status: LiveStatus): string {
  switch (status) {
    case "connected":
      return "live";
    case "connecting":
      return "подключение";
    case "reconnecting":
      return "переподключение";
    case "fallback":
      return "обновление";
    default:
      return "offline";
  }
}

export function decisionTitle(decision: string): string {
  return DECISION_TITLES[decision] ?? decision;
}

export function decisionLabel(decision: string): string {
  const title = decisionTitle(decision);
  return title === decision ? decision : `${decision} — ${title}`;
}

export function decisionType(decision: string, decisionTypes?: Record<string, DecisionType> | null): DecisionType {
  return decisionTypes?.[decision] ?? DECISION_TYPE_FALLBACK[decision] ?? "growth";
}

export function finalDecisionClass(decision?: string, summary?: PublicGameState["final_summary"]): string {
  if (!decision || !summary) {
    return "final-decision-clean";
  }
  if (decision === summary.mole_sabotage) {
    return "final-decision-sabotage";
  }
  if (summary.mole_targets.includes(decision)) {
    return "final-decision-podkop";
  }
  return "final-decision-clean";
}

export function memorandumTitle(type?: MemorandumType): string {
  return type === "risk" ? "Учитываю риски" : "Вижу возможности";
}

export function memorandumRule(type?: MemorandumType, variant: MemorandumVariant = "standard"): string {
  const subject = variant === "advanced" ? "паре" : "тройке";
  return type === "risk"
      ? `В этой ${subject} по крайней мере одно решение является целью крота.`
      : `В этой ${subject} по крайней мере одно решение не является целью крота.`;
}

export function percentToBps(value: string): number {
  const normalized = value.replace(",", ".").trim();
  const percent = Number.parseFloat(normalized);
  return Number.isFinite(percent) ? Math.round(percent * 100) : 0;
}

export function formatChatTime(value: string): string {
  if (!value || value.startsWith("0001-")) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
}

export function formatCountdown(seconds: number): string {
  const safeSeconds = Math.max(0, Math.ceil(seconds));
  return `${Math.floor(safeSeconds / 60)}:${String(safeSeconds % 60).padStart(2, "0")}`;
}

export function formatVotesCount(count: number): string {
  if (count % 10 === 1 && count % 100 !== 11) {
    return `${count} голос`;
  }
  if ([2, 3, 4].includes(count % 10) && ![12, 13, 14].includes(count % 100)) {
    return `${count} голоса`;
  }
  return `${count} голосов`;
}

export function playerName(players: PublicPlayerState[], userId?: number): string {
  if (!userId) {
    return "игрок";
  }
  return players.find((player) => player.user_id === userId)?.name ?? `Игрок #${userId}`;
}

export function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function effectivePlayerPositionLabel(player: PublicPlayerState): string {
  return player.is_ceo ? "CEO" : (player.company_position ?? "").trim();
}

export function stripKnownPositionSuffixes(text: string, players: PublicPlayerState[]): string {
  return players.reduce((current, player) => {
    const position = effectivePlayerPositionLabel(player);
    const name = player.name?.trim();
    if (!name || !position) {
      return current;
    }
    const pattern = new RegExp(`${escapeRegExp(name)},\\s*${escapeRegExp(position)}(?=\\)|,|\\.|;|$)`, "g");
    return current.replace(pattern, name);
  }, text);
}

export function majorVoteDecisionFromDetails(details?: string[] | null): string | null {
  let winner: { decision: string; share: number } | null = null;
  for (const detail of details ?? []) {
    const match = detail.match(/^([A-H])\s+[—-].*?:\s+(\d+(?:[.,]\d+)?)%/);
    if (!match) {
      continue;
    }
    const share = Number(match[2].replace(",", "."));
    if (!Number.isFinite(share)) {
      continue;
    }
    if (!winner || share > winner.share) {
      winner = { decision: match[1], share };
    }
  }
  return winner?.decision ?? null;
}

export function systemChatTitle(message: PublicChatMessage): string {
  const title = message.title?.trim() || "";
  switch (message.system_event_type) {
    case "company_briefing":
      return title && !title.startsWith("Компания:") ? title : "Брифинг компании";
    case "sabotage_accepted":
      return title && !title.startsWith("Тревожный сигнал:") ? title : "Тревожный сигнал";
    case "mole_revealed":
      return title && !title.startsWith("Крот раскрыт:") ? title : "Крот раскрыт";
    case "major_vote_accepted":
    case "major_vote_rejected": {
      const value = title.startsWith("Итоги major vote:") ? title.slice("Итоги major vote:".length).trim() : "";
      if (DECISION_OPTIONS.includes(value) || value === "не принято") {
        return `Итоги major vote: ${value}`;
      }
      const decision = majorVoteDecisionFromDetails(message.details);
      if (decision) {
        return `Итоги major vote: ${decision}`;
      }
      return message.system_event_type === "major_vote_rejected" ? "Итоги major vote: не принято" : "Итоги major vote: принято";
    }
    default:
      return title || "Системное сообщение";
  }
}

export function describeGovernanceProposal(proposal: PublicGovernanceProposal, players: PublicPlayerState[]): string {
  switch (proposal.proposal_type) {
    case "share_transfer":
      return `${playerName(players, proposal.from_user_id)} → ${playerName(players, proposal.to_user_id)}`;
    case "treasury_grant":
      return `Резерв → ${playerName(players, proposal.target_user_id)}`;
    case "treasury_buyback":
      return `${playerName(players, proposal.target_user_id)} → резерв`;
    case "appoint_ceo":
      return `CEO: ${playerName(players, proposal.target_user_id)}`;
    default:
      return "Корпоративный маневр";
  }
}

export function governanceVoteTitle(
    vote: { abstain?: boolean; proposal_title?: string; proposal?: PublicGovernanceProposal; proposal_id?: number },
    players: PublicPlayerState[],
): string {
  if (vote.abstain) {
    return "Воздержались";
  }
  if (vote.proposal_title) {
    return vote.proposal_title;
  }
  if (vote.proposal) {
    return describeGovernanceProposal(vote.proposal, players);
  }
  return "Корпоративный маневр";
}

export function governanceReportText(report: PublicGovernanceReport, players: PublicPlayerState[]): string {
  if (report.outcome === "accepted" && report.proposal) {
    return `Принято: ${describeGovernanceProposal(report.proposal, players)}`;
  }
  return "Манёвр не принят: следующий мажорный раунд начался без изменений";
}

export function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Неизвестная ошибка";
}

export function formatWinRate(value: number): string {
  return `${Math.round(value * 100)}%`;
}
