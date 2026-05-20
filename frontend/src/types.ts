export type ActionType =
  | "join_game"
  | "leave_game"
  | "kick_player"
  | "ban_player"
  | "add_bot"
  | "send_chat_message"
  | "react_chat_message"
  | "start_game"
  | "choose_memorandum"
  | "select_mole_objectives"
  | "place_compliance_watch"
  | "vote"
  | "submit_governance_proposal"
  | "skip_governance_proposal";
export type GameStatus = "lobby" | "started" | "finished";
export type GamePhase = "mole_objective_selection" | "major_voting" | "governance_proposal" | "governance_voting";
export type GovernanceProposalType = "share_transfer" | "treasury_grant" | "treasury_buyback" | "appoint_ceo";
export type DecisionType = "growth" | "empowerment";
export type MemorandumType = "opportunity" | "risk";

export interface User {
  id: number;
  login?: string;
  name: string;
  avatar_url?: string;
  company_position?: string;
  last_seen_at?: string | null;
  created_at: string;
  updated_at?: string | null;
}

export interface AuthUser {
  id: number;
  login: string;
  name: string;
  avatar_url?: string;
  company_position?: string;
  last_seen_at?: string | null;
  created_at: string;
  updated_at?: string | null;
}

export interface Game {
  id: number;
  title: string;
  company_name?: string;
  company_situation?: string;
  created_at: string;
  status?: GameStatus;
  phase?: GamePhase;
  winner?: string;
  current_round?: number;
  player_count?: number;
  started_player_count?: number;
  player_user_ids?: number[];
  players?: GameListPlayer[] | null;
  is_member?: boolean;
}

export interface GameListPlayer {
  user_id: number;
  name: string;
  avatar_url?: string;
  company_position?: string;
  is_host: boolean;
  is_ceo: boolean;
  is_bot: boolean;
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
  avatar_url?: string;
  company_position?: string;
  share_bps: number;
  authority_bps: number;
  is_host: boolean;
  is_ceo: boolean;
  is_bot: boolean;
  role?: string;
}

export interface PublicVoteState {
  user_id: number;
  has_voted: boolean;
  proposal_id?: number;
  abstain?: boolean;
  share_bps?: number;
  authority_bps?: number;
  voting_power_bps?: number;
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
  avatar_url?: string;
  company_position?: string;
  message: string;
  kind?: "user" | "system" | string;
  system_event_type?: string;
  title?: string;
  summary?: string;
  details?: string[] | null;
  tone?: "success" | "warning" | "danger" | string;
  collapsible?: boolean;
  reactions?: PublicChatReaction[] | null;
  created_at: string;
}

export interface PublicChatReaction {
  emoji: string;
  count: number;
  reacted_by_me: boolean;
}

export interface PublicMemorandum {
  type: MemorandumType;
  decisions: string[];
}

export interface PublicComplianceWatch {
  round_number: number;
  compliance_user_id: number;
  target_user_id: number;
}

export interface PublicGovernanceProposal {
  id: number;
  round: number;
  proposer_user_id: number;
  author_user_ids?: number[] | null;
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
  votes?: PublicGovernanceVoteReport[] | null;
}

export interface PublicGovernanceVoteReport {
  proposal_id?: number;
  proposal?: PublicGovernanceProposal;
  proposal_title?: string;
  abstain: boolean;
  share_bps: number;
  authority_bps: number;
  voting_power_bps: number;
  voter_count: number;
  voters?: PublicGovernanceVoterReport[] | null;
}

export interface PublicGovernanceVoterReport {
  user_id: number;
  name: string;
  share_bps: number;
  authority_bps: number;
  voting_power_bps: number;
}

export interface PublicDecisionVoteReport {
  decision: string;
  abstain: boolean;
  share_bps: number;
  voter_count: number;
  voters?: PublicDecisionVoterReport[] | null;
}

export interface PublicDecisionVoterReport {
  user_id: number;
  name: string;
  share_bps: number;
}

export interface PublicRoundReport {
  round: number;
  outcome: "accepted" | "rejected";
  decision?: string;
  reason?: string;
  votes: PublicDecisionVoteReport[];
}

export interface PublicFinalSummary {
  winner: string;
  winner_reason?: string;
  winner_user_ids: number[];
  mole_user_id: number;
  compliance_user_id?: number;
  compliance_catch?: PublicComplianceCatch | null;
  mole_targets: string[];
  mole_sabotage: string;
  mole_points: number;
  players_points: number;
  least_mistake_user_ids: number[];
  player_stats: PublicFinalPlayerStats[];
}

export interface PublicComplianceCatch {
  round_number: number;
  compliance_user_id: number;
  mole_user_id: number;
  accepted_decision: string;
  reason: string;
}

export interface PublicFinalPlayerStats {
  user_id: number;
  name: string;
  role: string;
  won: boolean;
  major_votes: number;
  aligned_votes: number;
  mistakes: number;
  accuracy_bps: number;
  xp_earned: number;
  xp_breakdown?: XPAward[] | null;
}

export interface XPAward {
  reason: string;
  points: number;
}

export interface PublicReplayStep {
  id: string;
  kind: "setup" | "major_vote" | "governance" | "final" | string;
  title: string;
  summary: string;
  round?: number;
  outcome?: "accepted" | "rejected" | string;
  decision?: string;
  proposal?: PublicGovernanceProposal;
  winner?: string;
  winner_reason?: string;
  votes?: PublicReplayVote[] | null;
}

export interface PublicReplayVote {
  label: string;
  share_bps?: number;
  voting_power_bps?: number;
  voters: string[];
}

export interface PublicGameState {
  game_id: number;
  title: string;
  company_name?: string;
  company_situation?: string;
  status: GameStatus;
  phase?: GamePhase;
  is_finished: boolean;
  winner?: string;
  winner_reason?: string;
  current_round: number;
  governance_round?: number;
  treasury_share_bps: number;
  available_decisions?: string[] | null;
  major_vote_options?: string[] | null;
  major_vote_unlocked_at?: string | null;
  phase_started_at?: string | null;
  phase_deadline_at?: string | null;
  decision_types?: Record<string, DecisionType> | null;
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
  mole_sabotage?: string;
  memorandum_preference?: MemorandumType;
  memorandum?: PublicMemorandum | null;
  compliance_watch?: PublicComplianceWatch | null;
  mole_victory_points?: number;
  players_victory_points?: number;
  final_summary?: PublicFinalSummary | null;
  replay_steps?: PublicReplayStep[] | null;
  available_actions: ActionType[];
}

export interface GameActionRequest {
  type: ActionType;
  payload?: Record<string, unknown>;
}

export interface CreateGameRequest {
  title: string;
}

export interface ApiErrorResponse {
  error?: string;
}

export interface LoginRequest {
  login: string;
  password: string;
}

export interface RegisterRequest {
  login: string;
  password: string;
  name: string;
  avatar_url?: string;
}

export interface RoleStats {
  games: number;
  wins: number;
  losses: number;
  winrate: number;
  major_votes?: number;
  aligned_votes?: number;
  accuracy_bps?: number;
}

export interface UserStats {
  total: RoleStats;
  mole: RoleStats;
  director: RoleStats;
}

export interface Profile extends User {
  stats: UserStats;
  respect_count: number;
  respected_by_me: boolean;
  xp: number;
  rank_title: string;
}

export interface ProfileResponse {
  profile: Profile;
}

export type LeaderboardPeriod = "week" | "month" | "all";
export type LeaderboardSort = "winrate" | "respect" | "accuracy";

export interface LeaderboardEntry {
  rank: number;
  user: Profile;
  games: number;
  wins: number;
  losses: number;
  winrate: number;
  rating_points: number;
  respect_delta: number;
  accuracy_bps: number;
  xp: number;
  rank_title: string;
}

export interface LeaderboardResponse {
  period: LeaderboardPeriod;
  sort: LeaderboardSort;
  entries: LeaderboardEntry[];
}

export interface UpdateProfileRequest {
  name: string;
  avatar_url: string;
  company_position: string;
}

export interface UpdateProfileResponse {
  user: AuthUser;
}

export interface ChangePasswordRequest {
  current_password: string;
  new_password: string;
}

export interface AuthResponse {
  user: AuthUser;
  token: string;
}

export interface MeResponse {
  user: AuthUser;
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
