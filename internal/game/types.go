package game

import (
	"encoding/json"
	"time"
)

type ActionType string

const (
	ActionJoinGame                 ActionType = "join_game"
	ActionLeaveGame                ActionType = "leave_game"
	ActionKickPlayer               ActionType = "kick_player"
	ActionBanPlayer                ActionType = "ban_player"
	ActionAddBot                   ActionType = "add_bot"
	ActionSendChatMessage          ActionType = "send_chat_message"
	ActionReactChatMessage         ActionType = "react_chat_message"
	ActionStartGame                ActionType = "start_game"
	ActionChooseMemorandum         ActionType = "choose_memorandum"
	ActionSelectMoleObjectives     ActionType = "select_mole_objectives"
	ActionPlaceComplianceWatch     ActionType = "place_compliance_watch"
	ActionVote                     ActionType = "vote"
	ActionSubmitGovernanceProposal ActionType = "submit_governance_proposal"
	ActionSkipGovernanceProposal   ActionType = "skip_governance_proposal"
)

const (
	RolePlayer     = "player"
	RoleMole       = "mole"
	RoleCompliance = "compliance"
)

const (
	WinnerReasonMoleTargetsCollected    = "mole_targets_collected"
	WinnerReasonCleanDecisionsCollected = "three_clean_decisions_collected"
	WinnerReasonMoleCaughtByCompliance  = "mole_caught_by_compliance"
	ComplianceCatchReasonDirectSabotage = "direct_sabotage_vote"
)

type GameStatus string

const (
	GameStatusLobby    GameStatus = "lobby"
	GameStatusStarted  GameStatus = "started"
	GameStatusFinished GameStatus = "finished"
)

type GamePhase string

const (
	GamePhaseMoleObjectiveSelection GamePhase = "mole_objective_selection"
	GamePhaseMajorVoting            GamePhase = "major_voting"
	GamePhaseGovernanceProposal     GamePhase = "governance_proposal"
	GamePhaseGovernanceVoting       GamePhase = "governance_voting"
)

type GovernanceProposalType string

const (
	GovernanceProposalShareTransfer   GovernanceProposalType = "share_transfer"
	GovernanceProposalTreasuryGrant   GovernanceProposalType = "treasury_grant"
	GovernanceProposalTreasuryBuyback GovernanceProposalType = "treasury_buyback"
	GovernanceProposalAppointCEO      GovernanceProposalType = "appoint_ceo"
)

type DecisionType string

const (
	DecisionTypeGrowth      DecisionType = "growth"
	DecisionTypeEmpowerment DecisionType = "empowerment"
)

type MemorandumType string

const (
	MemorandumTypeOpportunity MemorandumType = "opportunity"
	MemorandumTypeRisk        MemorandumType = "risk"
)

const (
	MinPlayers               = 3
	MaxPlayers               = 8
	TotalSharesBPS           = 10000
	InitialPlayerSharesBPS   = 8000
	InitialTreasurySharesBPS = 2000
	MajorDecisionRewardBPS   = 100
	InitialAuthorityBPS      = 300
	CEOAuthorityBonusBPS     = 100
	MajorAuthorityRewardBPS  = 100
	MaxShareChangeBPS        = 500
	MinPlayerShareBPS        = 500
	MaxChatMessageLength     = 500
	MaxPublicChatMessages    = 80
	MaxGovernanceProposals   = 4
	FirstMajorVoteLock       = time.Minute
	PhaseDuration            = 3 * time.Minute
	BotActionDelay           = 10 * time.Second
)

var allDecisions = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

var decisionTypes = map[string]DecisionType{
	"A": DecisionTypeGrowth,
	"B": DecisionTypeGrowth,
	"C": DecisionTypeEmpowerment,
	"D": DecisionTypeGrowth,
	"E": DecisionTypeGrowth,
	"F": DecisionTypeEmpowerment,
	"G": DecisionTypeEmpowerment,
	"H": DecisionTypeEmpowerment,
}

var decisionTitles = map[string]string{
	"A": "Выпуск облигаций",
	"B": "Экспансия на новый рынок",
	"C": "Сделка слияния",
	"D": "Запуск экспериментального продукта",
	"E": "Выплата дивидендов по акциям",
	"F": "Оптимизация неэффективного персонала",
	"G": "Агрессивная налоговая стратегия",
	"H": "Обратный выкуп акций",
}

var generatedPositions = []string{
	"Финансовый директор",
	"Директор по безопасности",
	"HR-директор",
	"Директор по инновациям",
	"Юрист компании",
	"Антикризисный менеджер",
	"Директор по связям с инвесторами",
}

var sharePresets = map[int][]int{
	3: {3500, 2500, 2000},
	4: {2500, 2000, 2000, 1500},
	5: {2000, 1700, 1600, 1400, 1300},
	6: {1700, 1500, 1300, 1200, 1200, 1100},
	7: {1400, 1200, 1200, 1100, 1100, 1000, 1000},
	8: {1200, 1100, 1100, 1000, 1000, 900, 900, 800},
}

type Action struct {
	UserID  int64           `json:"user_id"`
	Type    ActionType      `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type GameState struct {
	GameID             int64      `json:"game_id"`
	Title              string     `json:"title"`
	CompanyName        string     `json:"company_name"`
	CompanySituation   string     `json:"company_situation"`
	Status             GameStatus `json:"status"`
	Phase              GamePhase  `json:"phase"`
	IsFinished         bool       `json:"is_finished"`
	Winner             string     `json:"winner,omitempty"`
	WinnerReason       string     `json:"winner_reason,omitempty"`
	StartedPlayerCount int        `json:"started_player_count,omitempty"`

	HostUserID              int64
	CEOUserID               int64
	MoleUserID              int64
	ComplianceUserID        int64
	MoleTargets             []string
	MoleSabotage            string
	ComplianceWatches       map[int]ComplianceWatchState
	ComplianceCatch         *ComplianceCatchState
	MemorandumPreferences   map[int64]MemorandumType
	Memorandums             map[int64]MemorandumState
	CurrentRound            int
	GovernanceRound         int
	TreasuryShareBPS        int `json:"treasury_share_bps"`
	Players                 map[int64]*PlayerState
	PlayerOrder             []int64
	CurrentVotes            map[int64]VoteState
	GovernanceProposals     map[int]*GovernanceProposalState
	GovernanceProposalOrder []int
	GovernanceSubmissions   map[int64]GovernanceSubmissionState
	GovernanceVotes         map[int64]GovernanceVoteState
	AcceptedOrder           []string
	RejectedOrder           []string
	RoundReports            []RoundReport
	GovernanceReports       []GovernanceReport
	Available               map[string]bool
	MajorVoteOptions        []string
	MajorVoteUnlockedAt     *time.Time
	PhaseStartedAt          *time.Time
	PhaseDeadlineAt         *time.Time
	ChatMessages            []ChatMessageState
	ChatReactions           map[int64]map[string]map[int64]bool
}

type PlayerState struct {
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Position     string `json:"company_position,omitempty"`
	ShareBPS     int    `json:"share_bps"`
	AuthorityBPS int    `json:"authority_bps"`
	IsHost       bool   `json:"is_host"`
	IsCEO        bool   `json:"is_ceo"`
	IsBot        bool   `json:"is_bot"`
	IsLeft       bool   `json:"is_left"`
	IsKicked     bool   `json:"is_kicked"`
	Role         string `json:"role,omitempty"`
}

type VoteState struct {
	UserID   int64   `json:"user_id"`
	Decision *string `json:"decision,omitempty"`
	Abstain  bool    `json:"abstain"`
}

type ComplianceWatchState struct {
	RoundNumber      int   `json:"round_number"`
	ComplianceUserID int64 `json:"compliance_user_id"`
	TargetUserID     int64 `json:"target_user_id"`
}

type ComplianceCatchState struct {
	RoundNumber      int    `json:"round_number"`
	ComplianceUserID int64  `json:"compliance_user_id"`
	MoleUserID       int64  `json:"mole_user_id"`
	AcceptedDecision string `json:"accepted_decision"`
	Reason           string `json:"reason"`
}

type GovernanceProposalState struct {
	ID             int                    `json:"id"`
	Round          int                    `json:"round"`
	ProposerUserID int64                  `json:"proposer_user_id"`
	AuthorUserIDs  []int64                `json:"author_user_ids,omitempty"`
	ProposalType   GovernanceProposalType `json:"proposal_type"`
	FromUserID     int64                  `json:"from_user_id,omitempty"`
	ToUserID       int64                  `json:"to_user_id,omitempty"`
	TargetUserID   int64                  `json:"target_user_id,omitempty"`
	ShareBPS       int                    `json:"share_bps,omitempty"`
}

type GovernanceSubmissionState struct {
	UserID     int64  `json:"user_id"`
	Status     string `json:"status"`
	ProposalID int    `json:"proposal_id,omitempty"`
}

type GovernanceVoteState struct {
	UserID     int64 `json:"user_id"`
	ProposalID *int  `json:"proposal_id,omitempty"`
	Abstain    bool  `json:"abstain"`
}

type MemorandumState struct {
	UserID    int64          `json:"user_id"`
	Type      MemorandumType `json:"type"`
	Decisions []string       `json:"decisions"`
}

type ChatMessageState struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	UserName        string    `json:"user_name"`
	UserPosition    string    `json:"company_position,omitempty"`
	Message         string    `json:"message"`
	Kind            string    `json:"kind,omitempty"`
	SystemEventType string    `json:"system_event_type,omitempty"`
	Title           string    `json:"title,omitempty"`
	Summary         string    `json:"summary,omitempty"`
	Details         []string  `json:"details,omitempty"`
	Tone            string    `json:"tone,omitempty"`
	Collapsible     bool      `json:"collapsible,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type PublicChatReaction struct {
	Emoji       string `json:"emoji"`
	Count       int    `json:"count"`
	ReactedByMe bool   `json:"reacted_by_me"`
}

type PublicGameState struct {
	GameID                int64                        `json:"game_id"`
	Title                 string                       `json:"title"`
	CompanyName           string                       `json:"company_name"`
	CompanySituation      string                       `json:"company_situation"`
	Status                GameStatus                   `json:"status"`
	Phase                 GamePhase                    `json:"phase"`
	IsFinished            bool                         `json:"is_finished"`
	Winner                string                       `json:"winner,omitempty"`
	WinnerReason          string                       `json:"winner_reason,omitempty"`
	StartedPlayerCount    int                          `json:"started_player_count,omitempty"`
	CurrentRound          int                          `json:"current_round"`
	GovernanceRound       int                          `json:"governance_round"`
	TreasuryShareBPS      int                          `json:"treasury_share_bps"`
	AvailableDecisions    []string                     `json:"available_decisions"`
	MajorVoteOptions      []string                     `json:"major_vote_options"`
	MajorVoteUnlockedAt   *time.Time                   `json:"major_vote_unlocked_at,omitempty"`
	PhaseStartedAt        *time.Time                   `json:"phase_started_at,omitempty"`
	PhaseDeadlineAt       *time.Time                   `json:"phase_deadline_at,omitempty"`
	DecisionTypes         map[string]DecisionType      `json:"decision_types"`
	AcceptedDecisions     []string                     `json:"accepted_decisions"`
	RejectedDecisions     []string                     `json:"rejected_decisions"`
	Players               []PublicPlayerState          `json:"players"`
	Me                    PublicPlayerState            `json:"me"`
	CurrentVotes          []PublicVoteState            `json:"current_votes"`
	MyCurrentVote         *PublicOwnVoteState          `json:"my_current_vote,omitempty"`
	GovernanceProposals   []PublicGovernanceProposal   `json:"governance_proposals"`
	GovernanceSubmissions []PublicGovernanceSubmission `json:"governance_submissions"`
	GovernanceReports     []PublicGovernanceReport     `json:"governance_reports"`
	RoundReports          []PublicRoundReport          `json:"round_reports"`
	ChatMessages          []PublicChatMessage          `json:"chat_messages"`
	MoleTargets           []string                     `json:"mole_targets,omitempty"`
	MoleSabotage          string                       `json:"mole_sabotage,omitempty"`
	MemorandumPreference  MemorandumType               `json:"memorandum_preference,omitempty"`
	Memorandum            *PublicMemorandum            `json:"memorandum,omitempty"`
	ComplianceWatch       *PublicComplianceWatch       `json:"compliance_watch,omitempty"`
	MoleVictoryPoints     *int                         `json:"mole_victory_points,omitempty"`
	PlayersVictoryPoints  *int                         `json:"players_victory_points,omitempty"`
	FinalSummary          *PublicFinalSummary          `json:"final_summary,omitempty"`
	ReplaySteps           []PublicReplayStep           `json:"replay_steps,omitempty"`
	AvailableActions      []ActionType                 `json:"available_actions"`
}

type PublicPlayerState struct {
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	Position     string `json:"company_position,omitempty"`
	ShareBPS     int    `json:"share_bps"`
	AuthorityBPS int    `json:"authority_bps"`
	IsHost       bool   `json:"is_host"`
	IsCEO        bool   `json:"is_ceo"`
	IsBot        bool   `json:"is_bot"`
	Role         string `json:"role,omitempty"`
}

type PublicVoteState struct {
	UserID         int64 `json:"user_id"`
	HasVoted       bool  `json:"has_voted"`
	ProposalID     int   `json:"proposal_id,omitempty"`
	Abstain        bool  `json:"abstain,omitempty"`
	ShareBPS       int   `json:"share_bps,omitempty"`
	AuthorityBPS   int   `json:"authority_bps,omitempty"`
	VotingPowerBPS int   `json:"voting_power_bps,omitempty"`
}

type PublicOwnVoteState struct {
	Decision   string `json:"decision,omitempty"`
	ProposalID int    `json:"proposal_id,omitempty"`
	Abstain    bool   `json:"abstain"`
}

type PublicChatMessage struct {
	ID              int64                `json:"id"`
	UserID          int64                `json:"user_id"`
	UserName        string               `json:"user_name"`
	AvatarURL       string               `json:"avatar_url,omitempty"`
	UserPosition    string               `json:"company_position,omitempty"`
	Message         string               `json:"message"`
	Kind            string               `json:"kind,omitempty"`
	SystemEventType string               `json:"system_event_type,omitempty"`
	Title           string               `json:"title,omitempty"`
	Summary         string               `json:"summary,omitempty"`
	Details         []string             `json:"details,omitempty"`
	Tone            string               `json:"tone,omitempty"`
	Collapsible     bool                 `json:"collapsible,omitempty"`
	Reactions       []PublicChatReaction `json:"reactions,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
}

type PublicMemorandum struct {
	Type      MemorandumType `json:"type"`
	Decisions []string       `json:"decisions"`
}

type PublicComplianceWatch struct {
	RoundNumber      int   `json:"round_number"`
	ComplianceUserID int64 `json:"compliance_user_id"`
	TargetUserID     int64 `json:"target_user_id"`
}

type PublicGovernanceProposal struct {
	ID             int                    `json:"id"`
	Round          int                    `json:"round"`
	ProposerUserID int64                  `json:"proposer_user_id"`
	AuthorUserIDs  []int64                `json:"author_user_ids,omitempty"`
	ProposalType   GovernanceProposalType `json:"proposal_type"`
	FromUserID     int64                  `json:"from_user_id,omitempty"`
	ToUserID       int64                  `json:"to_user_id,omitempty"`
	TargetUserID   int64                  `json:"target_user_id,omitempty"`
	ShareBPS       int                    `json:"share_bps,omitempty"`
}

type PublicGovernanceSubmission struct {
	UserID     int64  `json:"user_id"`
	Status     string `json:"status"`
	ProposalID int    `json:"proposal_id,omitempty"`
}

type GovernanceReport struct {
	Round    int
	Outcome  string
	Proposal *GovernanceProposalState
	Reason   string
	Votes    []GovernanceVoteReport
}

type PublicGovernanceReport struct {
	Round    int                          `json:"round"`
	Outcome  string                       `json:"outcome"`
	Proposal *PublicGovernanceProposal    `json:"proposal,omitempty"`
	Reason   string                       `json:"reason,omitempty"`
	Votes    []PublicGovernanceVoteReport `json:"votes,omitempty"`
}

type GovernanceVoteReport struct {
	ProposalID     int
	Proposal       *GovernanceProposalState
	ProposalTitle  string
	Abstain        bool
	ShareBPS       int
	AuthorityBPS   int
	VotingPowerBPS int
	VoterCount     int
	Voters         []GovernanceVoterReport
}

type GovernanceVoterReport struct {
	UserID         int64
	Name           string
	ShareBPS       int
	AuthorityBPS   int
	VotingPowerBPS int
}

type PublicGovernanceVoteReport struct {
	ProposalID     int                           `json:"proposal_id,omitempty"`
	Proposal       *PublicGovernanceProposal     `json:"proposal,omitempty"`
	ProposalTitle  string                        `json:"proposal_title,omitempty"`
	Abstain        bool                          `json:"abstain"`
	ShareBPS       int                           `json:"share_bps"`
	AuthorityBPS   int                           `json:"authority_bps"`
	VotingPowerBPS int                           `json:"voting_power_bps"`
	VoterCount     int                           `json:"voter_count"`
	Voters         []PublicGovernanceVoterReport `json:"voters"`
}

type PublicGovernanceVoterReport struct {
	UserID         int64  `json:"user_id"`
	Name           string `json:"name"`
	ShareBPS       int    `json:"share_bps"`
	AuthorityBPS   int    `json:"authority_bps"`
	VotingPowerBPS int    `json:"voting_power_bps"`
}

type RoundReport struct {
	Round    int
	Outcome  string
	Decision string
	Reason   string
	Votes    []DecisionVoteReport
}

type DecisionVoteReport struct {
	Decision   string
	Abstain    bool
	ShareBPS   int
	VoterCount int
	Voters     []DecisionVoterReport
}

type DecisionVoterReport struct {
	UserID   int64
	Name     string
	ShareBPS int
}

type PublicRoundReport struct {
	Round    int                        `json:"round"`
	Outcome  string                     `json:"outcome"`
	Decision string                     `json:"decision,omitempty"`
	Reason   string                     `json:"reason,omitempty"`
	Votes    []PublicDecisionVoteReport `json:"votes"`
}

type PublicDecisionVoteReport struct {
	Decision   string                      `json:"decision"`
	Abstain    bool                        `json:"abstain"`
	ShareBPS   int                         `json:"share_bps"`
	VoterCount int                         `json:"voter_count"`
	Voters     []PublicDecisionVoterReport `json:"voters"`
}

type PublicDecisionVoterReport struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	ShareBPS int    `json:"share_bps"`
}

type PublicFinalSummary struct {
	Winner              string                   `json:"winner"`
	WinnerReason        string                   `json:"winner_reason,omitempty"`
	WinnerUserIDs       []int64                  `json:"winner_user_ids"`
	MoleUserID          int64                    `json:"mole_user_id"`
	ComplianceUserID    int64                    `json:"compliance_user_id,omitempty"`
	ComplianceCatch     *PublicComplianceCatch   `json:"compliance_catch,omitempty"`
	MoleTargets         []string                 `json:"mole_targets"`
	MoleSabotage        string                   `json:"mole_sabotage"`
	MolePoints          int                      `json:"mole_points"`
	PlayersPoints       int                      `json:"players_points"`
	LeastMistakeUserIDs []int64                  `json:"least_mistake_user_ids"`
	PlayerStats         []PublicFinalPlayerStats `json:"player_stats"`
}

type PublicComplianceCatch struct {
	RoundNumber      int    `json:"round_number"`
	ComplianceUserID int64  `json:"compliance_user_id"`
	MoleUserID       int64  `json:"mole_user_id"`
	AcceptedDecision string `json:"accepted_decision"`
	Reason           string `json:"reason"`
}

type PublicFinalPlayerStats struct {
	UserID       int64     `json:"user_id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	Won          bool      `json:"won"`
	MajorVotes   int       `json:"major_votes"`
	AlignedVotes int       `json:"aligned_votes"`
	Mistakes     int       `json:"mistakes"`
	AccuracyBPS  int       `json:"accuracy_bps"`
	XPEarned     int       `json:"xp_earned"`
	XPBreakdown  []XPAward `json:"xp_breakdown,omitempty"`
}

type XPAward struct {
	Reason string `json:"reason"`
	Points int    `json:"points"`
}

type PublicReplayStep struct {
	ID           string                    `json:"id"`
	Kind         string                    `json:"kind"`
	Title        string                    `json:"title"`
	Summary      string                    `json:"summary"`
	Round        int                       `json:"round,omitempty"`
	Outcome      string                    `json:"outcome,omitempty"`
	Decision     string                    `json:"decision,omitempty"`
	Proposal     *PublicGovernanceProposal `json:"proposal,omitempty"`
	Winner       string                    `json:"winner,omitempty"`
	WinnerReason string                    `json:"winner_reason,omitempty"`
	Reason       string                    `json:"reason,omitempty"`
	Votes        []PublicReplayVote        `json:"votes,omitempty"`
}

type PublicReplayVote struct {
	Label          string   `json:"label"`
	ShareBPS       int      `json:"share_bps,omitempty"`
	VotingPowerBPS int      `json:"voting_power_bps,omitempty"`
	Voters         []string `json:"voters"`
}

type GameCreatedPayload struct {
	HostUserID       int64  `json:"host_user_id"`
	Title            string `json:"title"`
	CompanyName      string `json:"company_name,omitempty"`
	CompanySituation string `json:"company_situation,omitempty"`
}

type PlayerJoinedPayload struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Position string `json:"company_position,omitempty"`
	IsBot    bool   `json:"is_bot,omitempty"`
}

type PlayerKickedPayload struct {
	UserID int64 `json:"user_id"`
}

type PlayerLeftPayload struct {
	UserID int64 `json:"user_id"`
}

type PlayerReplacedByBotPayload struct {
	UserID       int64  `json:"user_id"`
	BotUserID    int64  `json:"bot_user_id"`
	Name         string `json:"name"`
	Position     string `json:"company_position,omitempty"`
	ShareBPS     int    `json:"share_bps"`
	AuthorityBPS int    `json:"authority_bps"`
	IsCEO        bool   `json:"is_ceo,omitempty"`
	Role         string `json:"role,omitempty"`
}

type ChatMessageSentPayload struct {
	UserID          int64    `json:"user_id"`
	Message         string   `json:"message"`
	Kind            string   `json:"kind,omitempty"`
	SystemEventType string   `json:"system_event_type,omitempty"`
	Title           string   `json:"title,omitempty"`
	Summary         string   `json:"summary,omitempty"`
	Details         []string `json:"details,omitempty"`
	Tone            string   `json:"tone,omitempty"`
	Collapsible     bool     `json:"collapsible,omitempty"`
}

type MoleSelectedPayload struct {
	UserID int64 `json:"user_id"`
}

type ComplianceSelectedPayload struct {
	UserID int64 `json:"user_id"`
}

type MoleTargetsGeneratedPayload struct {
	Targets []string `json:"targets"`
}

type MoleObjectivesSelectedPayload struct {
	Targets  []string `json:"targets"`
	Sabotage string   `json:"sabotage"`
}

type MemorandumPreferenceSelectedPayload struct {
	UserID int64          `json:"user_id"`
	Type   MemorandumType `json:"type"`
}

type MemorandumAssignedPayload struct {
	UserID    int64          `json:"user_id"`
	Type      MemorandumType `json:"type"`
	Decisions []string       `json:"decisions"`
}

type PlayerReceivedSharePayload struct {
	UserID   int64 `json:"user_id"`
	ShareBPS int   `json:"share_bps"`
}

type PlayerAuthorityGrantedPayload struct {
	UserID       int64 `json:"user_id"`
	AuthorityBPS int   `json:"authority_bps"`
}

type PlayerPositionAssignedPayload struct {
	UserID   int64  `json:"user_id"`
	Position string `json:"company_position"`
}

type CEOSelectedPayload struct {
	UserID int64 `json:"user_id"`
}

type GameStartedPayload struct {
	PlayerCount int `json:"player_count,omitempty"`
}

type VotingRoundStartedPayload struct {
	Round             int        `json:"round"`
	ShowcaseDecisions []string   `json:"showcase_decisions,omitempty"`
	UnlockedAt        *time.Time `json:"unlocked_at,omitempty"`
}

type VoteSubmittedPayload struct {
	Round    int     `json:"round"`
	UserID   int64   `json:"user_id"`
	Decision *string `json:"decision,omitempty"`
	Abstain  bool    `json:"abstain"`
}

type ComplianceWatchPlacedPayload struct {
	RoundNumber      int   `json:"round_number"`
	ComplianceUserID int64 `json:"compliance_user_id"`
	TargetUserID     int64 `json:"target_user_id"`
}

type MoleExposedByCompliancePayload struct {
	RoundNumber      int    `json:"round_number"`
	ComplianceUserID int64  `json:"compliance_user_id"`
	MoleUserID       int64  `json:"mole_user_id"`
	AcceptedDecision string `json:"accepted_decision"`
	Reason           string `json:"reason"`
}

type GovernanceProposalPhaseStartedPayload struct {
	Round int `json:"round"`
}

type GovernanceProposalSubmittedPayload struct {
	Round          int                    `json:"round"`
	ProposalID     int                    `json:"proposal_id"`
	ProposerUserID int64                  `json:"proposer_user_id"`
	AuthorUserIDs  []int64                `json:"author_user_ids,omitempty"`
	ProposalType   GovernanceProposalType `json:"proposal_type"`
	FromUserID     int64                  `json:"from_user_id,omitempty"`
	ToUserID       int64                  `json:"to_user_id,omitempty"`
	TargetUserID   int64                  `json:"target_user_id,omitempty"`
	ShareBPS       int                    `json:"share_bps,omitempty"`
}

type GovernanceProposalSkippedPayload struct {
	Round  int   `json:"round"`
	UserID int64 `json:"user_id"`
}

type GovernanceVotingStartedPayload struct {
	Round       int   `json:"round"`
	ProposalIDs []int `json:"proposal_ids,omitempty"`
}

type GovernanceVoteSubmittedPayload struct {
	Round      int   `json:"round"`
	UserID     int64 `json:"user_id"`
	ProposalID *int  `json:"proposal_id,omitempty"`
	Abstain    bool  `json:"abstain"`
}

type GovernanceResolvedPayload struct {
	Round int `json:"round"`
}

type GovernanceProposalAcceptedPayload struct {
	Round      int `json:"round"`
	ProposalID int `json:"proposal_id"`
}

type GovernanceProposalRejectedPayload struct {
	Round  int    `json:"round"`
	Reason string `json:"reason"`
}

type PlayerShareTransferredPayload struct {
	FromUserID int64 `json:"from_user_id"`
	ToUserID   int64 `json:"to_user_id"`
	ShareBPS   int   `json:"share_bps"`
}

type TreasuryShareGrantedPayload struct {
	TargetUserID int64 `json:"target_user_id"`
	ShareBPS     int   `json:"share_bps"`
}

type TreasuryShareBoughtBackPayload struct {
	TargetUserID int64 `json:"target_user_id"`
	ShareBPS     int   `json:"share_bps"`
}

type CEOChangedPayload struct {
	TargetUserID int64 `json:"target_user_id"`
}

type DecisionAcceptedPayload struct {
	Round    int    `json:"round"`
	Decision string `json:"decision"`
}

type DecisionRejectedPayload struct {
	Round   int      `json:"round"`
	Options []string `json:"options,omitempty"`
	Reason  string   `json:"reason"`
}

type GameFinishedPayload struct {
	Winner string `json:"winner"`
	Reason string `json:"reason"`
}

type KickPlayerActionPayload struct {
	UserID int64 `json:"user_id"`
}

type BanPlayerActionPayload struct {
	UserID int64 `json:"user_id"`
}

type AddBotActionPayload struct {
	Count int    `json:"count,omitempty"`
	Name  string `json:"name,omitempty"`
}

type SendChatMessageActionPayload struct {
	Message string `json:"message"`
}

type ReactChatMessageActionPayload struct {
	MessageID int64  `json:"message_id"`
	Emoji     string `json:"emoji"`
}

type ChatReactionToggledPayload struct {
	MessageID int64  `json:"message_id"`
	UserID    int64  `json:"user_id"`
	Emoji     string `json:"emoji"`
}

type SelectMoleObjectivesActionPayload struct {
	Targets  []string `json:"targets"`
	Sabotage string   `json:"sabotage"`
}

type ChooseMemorandumActionPayload struct {
	Type MemorandumType `json:"type"`
}

type PlaceComplianceWatchActionPayload struct {
	TargetUserID int64 `json:"target_user_id"`
}

type VoteActionPayload struct {
	Decision   *string `json:"decision,omitempty"`
	ProposalID *int    `json:"proposal_id,omitempty"`
	Abstain    bool    `json:"abstain"`
}

type SubmitGovernanceProposalActionPayload struct {
	ProposalType GovernanceProposalType `json:"proposal_type"`
	FromUserID   int64                  `json:"from_user_id,omitempty"`
	ToUserID     int64                  `json:"to_user_id,omitempty"`
	TargetUserID int64                  `json:"target_user_id,omitempty"`
	ShareBPS     int                    `json:"share_bps,omitempty"`
}
