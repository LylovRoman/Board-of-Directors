import type { ActionType, DecisionType, Game } from "./types";
import type { SfxName } from "./audio";

export interface GameCard {
  game: Game;
}
export type AuthMode = "login" | "register";
export type LiveStatus = "idle" | "connecting" | "connected" | "reconnecting" | "fallback";
export type LobbySort = "newest" | "players" | "round";

export const SELECTED_GAME_STORAGE_KEY = "board-of-directors-selected-game-id";
export const SOUND_STORAGE_KEY = "board-of-directors-sound-enabled";
export const LEADERBOARD_HIDDEN_STORAGE_KEY = "board-of-directors-leaderboard-hidden";
export const BOT_ACTION_DELAY_MS = 10_000;
export const DECISION_TITLES: Record<string, string> = {
  A: "Выпуск облигаций",
  B: "Экспансия на новый рынок",
  C: "Сделка слияния",
  D: "Запуск экспериментального продукта",
  E: "Выплата дивидендов по акциям",
  F: "Оптимизация неэффективного персонала",
  G: "Агрессивная налоговая стратегия",
  H: "Обратный выкуп акций",
};
export const DECISION_OPTIONS = Object.keys(DECISION_TITLES);
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
export const ACTION_SFX: Partial<Record<ActionType, SfxName>> = {
  join_game: "join",
  leave_game: "close",
  kick_player: "success",
  ban_player: "success",
  add_bot: "success",
  send_chat_message: "chat-send",
  react_chat_message: "reaction",
  start_game: "start",
  choose_memorandum: "success",
  select_mole_objectives: "success",
  place_compliance_watch: "success",
  vote: "vote",
  submit_governance_proposal: "success",
  skip_governance_proposal: "close",
};
