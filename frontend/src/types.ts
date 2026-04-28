export type ActionType =
  | "join_game"
  | "leave_game"
  | "kick_player"
  | "send_chat_message"
  | "start_game"
  | "vote"
  | "submit_governance_proposal"
  | "skip_governance_proposal";
export type GameStatus = "lobby" | "started" | "finished";
export type GamePhase = "major_voting" | "governance_proposal" | "governance_voting";
export type GovernanceProposalType = "share_transfer" | "treasury_grant" | "treasury_buyback" | "appoint_ceo";

export interface User {
  id: number;
  name: string;
  created_at: string;
}

export interface Game {
  id: number;
  title: string;
  created_at: string;
}

export interface Event {
  id: number;
  game_id: number;
  user_id?: number | null;
  actor_name?: string;
  event_type: string;
  event_value?: string;
  created_at: string;
}

export interface PublicPlayerState {
  user_id: number;
  name: string;
  share_bps: number;
  is_host: boolean;
  is_ceo: boolean;
  role?: string;
}

export interface PublicVoteState {
  user_id: number;
  has_voted: boolean;
}

export interface PublicOwnVoteState {
  decision?: string;
  proposal_id?: number;
  abstain: boolean;
}

export interface PublicChatMessage {
  id: number;
  user_id: number;
  user_name: string;
  message: string;
  created_at: string;
}

export interface PublicGovernanceProposal {
  id: number;
  round: number;
  proposer_user_id: number;
  proposal_type: GovernanceProposalType;
  from_user_id?: number;
  to_user_id?: number;
  target_user_id?: number;
  share_bps?: number;
}

export interface PublicGovernanceSubmission {
  user_id: number;
  status?: "submitted" | "skipped" | "";
  proposal_id?: number;
}

export interface PublicGovernanceReport {
  round: number;
  outcome: "accepted" | "rejected";
  proposal?: PublicGovernanceProposal;
  reason?: string;
}

export interface PublicDecisionVoteReport {
  decision: string;
  abstain: boolean;
  share_bps: number;
  voter_count: number;
}

export interface PublicRoundReport {
  round: number;
  outcome: "accepted" | "rejected";
  decision?: string;
  reason?: string;
  votes: PublicDecisionVoteReport[];
}

export interface PublicGameState {
  game_id: number;
  title: string;
  status: GameStatus;
  phase?: GamePhase;
  is_finished: boolean;
  winner?: string;
  current_round: number;
  governance_round?: number;
  treasury_share_bps: number;
  available_decisions?: string[] | null;
  accepted_decisions?: string[] | null;
  rejected_decisions?: string[] | null;
  players: PublicPlayerState[];
  me?: PublicPlayerState;
  current_votes?: PublicVoteState[] | null;
  my_current_vote?: PublicOwnVoteState | null;
  governance_proposals?: PublicGovernanceProposal[] | null;
  governance_submissions?: PublicGovernanceSubmission[] | null;
  governance_reports?: PublicGovernanceReport[] | null;
  round_reports?: PublicRoundReport[] | null;
  chat_messages?: PublicChatMessage[] | null;
  mole_targets?: string[];
  available_actions: ActionType[];
}

export interface GameActionRequest {
  user_id: number;
  type: ActionType;
  payload?: Record<string, unknown>;
}

export interface CreateGameRequest {
  title: string;
  host_user_id: number;
}

export interface ApiErrorResponse {
  error?: string;
}

export interface UsersResponse {
  users: User[];
}

export interface CreateUserResponse {
  user: User;
}

export interface GamesResponse {
  games: Game[];
}

export interface CreateGameResponse {
  game: Game;
  state: PublicGameState;
}

export interface GameStateResponse {
  state: PublicGameState;
}

export interface GameActionResponse {
  events: Event[];
  state?: PublicGameState;
}

export function normalizeStringArray(value?: string[] | null): string[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeVotes(value?: PublicVoteState[] | null): PublicVoteState[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeRoundReports(value?: PublicRoundReport[] | null): PublicRoundReport[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeChatMessages(value?: PublicChatMessage[] | null): PublicChatMessage[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeGovernanceProposals(value?: PublicGovernanceProposal[] | null): PublicGovernanceProposal[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeGovernanceSubmissions(value?: PublicGovernanceSubmission[] | null): PublicGovernanceSubmission[] {
  return Array.isArray(value) ? value : [];
}

export function normalizeGovernanceReports(value?: PublicGovernanceReport[] | null): PublicGovernanceReport[] {
  return Array.isArray(value) ? value : [];
}
