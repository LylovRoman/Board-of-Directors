import type {
  DecisionType,
  GamePhase,
  GameStatus,
  GovernanceProposalType,
  MemorandumType,
  MemorandumVariant,
  PublicGameState,
  PublicGovernanceProposal,
  PublicPlayerState,
} from "./types";

export const DECISION_TITLES: Record<string, string> = {
  A: "Выпуск облигаций",
  B: "Экспансия на новый рынок",
  C: "Сделка слияния",
  D: "Экспериментальный продукт",
  E: "Выплата дивидендов",
  F: "Оптимизация персонала",
  G: "Налоговая стратегия",
  H: "Обратный выкуп",
};

export const DECISION_DETAILS: Record<string, string> = {
  A: "Привлечь капитал через облигации.",
  B: "Выйти на новый рынок с высоким потенциалом.",
  C: "Ускорить рост через M&A.",
  D: "Запустить рискованный новый продукт.",
  E: "Направить прибыль акционерам.",
  F: "Сократить издержки и перераспределить ресурсы.",
  G: "Снизить налоговую нагрузку агрессивной схемой.",
  H: "Сконцентрировать капитал через buyback.",
};

export const DECISION_TYPE_FALLBACK: Record<string, DecisionType> = {
  A: "growth",
  B: "growth",
  C: "growth",
  D: "growth",
  E: "empowerment",
  F: "empowerment",
  G: "empowerment",
  H: "empowerment",
};

export const DECISION_IDS = Object.keys(DECISION_TITLES);

export const ALLOWED_REACTIONS = ["👍", "🤝", "💼", "📈", "⚠️", "🕵️", "✅", "🔥"];

export function statusLabel(status?: GameStatus): string {
  switch (status) {
    case "lobby":
      return "Лобби";
    case "started":
      return "Идет заседание";
    case "finished":
      return "Финал";
    default:
      return "Статус уточняется";
  }
}

export function phaseLabel(phase?: GamePhase): string {
  switch (phase) {
    case "mole_objective_selection":
      return "Тайный брифинг";
    case "major_voting":
      return "Major vote";
    case "governance_proposal":
      return "Предложения";
    case "governance_voting":
      return "Governance vote";
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
    return "Совет победил";
  }
  return "Игра завершена";
}

export function bpsToPercent(bps?: number): string {
  const value = typeof bps === "number" ? bps : 0;
  return `${(value / 100).toFixed(value % 100 === 0 ? 0 : 1)}%`;
}

export function percentToBps(value: string): number {
  const parsed = Number.parseFloat(value.replace(",", ".").trim());
  return Number.isFinite(parsed) ? Math.max(0, Math.round(parsed * 100)) : 0;
}

export function decisionTitle(decision: string): string {
  return DECISION_TITLES[decision] ?? decision;
}

export function decisionLabel(decision: string): string {
  const title = decisionTitle(decision);
  return title === decision ? decision : `${decision} · ${title}`;
}

export function decisionType(decision: string, state?: PublicGameState | null): DecisionType {
  return state?.decision_types?.[decision] ?? DECISION_TYPE_FALLBACK[decision] ?? "growth";
}

export function memorandumTitle(type?: MemorandumType): string {
  return type === "risk" ? "Риски" : "Возможности";
}

export function memorandumRule(type?: MemorandumType, variant: MemorandumVariant = "standard"): string {
  const subject = variant === "advanced" ? "паре" : "тройке";
  return type === "risk"
    ? `В этой ${subject} есть хотя бы одна цель Крота.`
    : `В этой ${subject} есть хотя бы одно чистое решение.`;
}

export function playerName(players: PublicPlayerState[], userId?: number): string {
  if (!userId) {
    return "игрок";
  }
  return players.find((player) => player.user_id === userId)?.name ?? `Игрок #${userId}`;
}

export function proposalTypeLabel(type: GovernanceProposalType): string {
  switch (type) {
    case "share_transfer":
      return "Передача доли";
    case "treasury_grant":
      return "Грант из резерва";
    case "treasury_buyback":
      return "Выкуп в резерв";
    case "appoint_ceo":
      return "Назначить CEO";
    default:
      return "Маневр";
  }
}

export function proposalText(proposal: PublicGovernanceProposal, players: PublicPlayerState[]): string {
  switch (proposal.proposal_type) {
    case "share_transfer":
      return `${playerName(players, proposal.from_user_id)} → ${playerName(players, proposal.to_user_id)} · ${bpsToPercent(proposal.share_bps)}`;
    case "treasury_grant":
      return `Резерв → ${playerName(players, proposal.target_user_id)} · ${bpsToPercent(proposal.share_bps)}`;
    case "treasury_buyback":
      return `${playerName(players, proposal.target_user_id)} → резерв · ${bpsToPercent(proposal.share_bps)}`;
    case "appoint_ceo":
      return `CEO: ${playerName(players, proposal.target_user_id)}`;
    default:
      return "Корпоративный маневр";
  }
}

export function formatTime(value?: string): string {
  if (!value || value.startsWith("0001-")) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
}

export function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Что-то пошло не так";
}

export function normalizeList<T>(value?: T[] | null): T[] {
  return Array.isArray(value) ? value : [];
}
