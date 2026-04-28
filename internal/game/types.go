package game

import "encoding/json"

type ActionType string

const (
	ActionJoinGame                 ActionType = "join_game"
	ActionKickPlayer               ActionType = "kick_player"
	ActionStartGame                ActionType = "start_game"
	ActionVote                     ActionType = "vote"
	ActionSubmitGovernanceProposal ActionType = "submit_governance_proposal"
	ActionSkipGovernanceProposal   ActionType = "skip_governance_proposal"
)

type GameStatus string

const (
	GameStatusLobby    GameStatus = "lobby"
	GameStatusStarted  GameStatus = "started"
	GameStatusFinished GameStatus = "finished"
)

type GamePhase string

const (
	GamePhaseMajorVoting        GamePhase = "major_voting"
	GamePhaseGovernanceProposal GamePhase = "governance_proposal"
	GamePhaseGovernanceVoting   GamePhase = "governance_voting"
)

type GovernanceProposalType string

const (
	GovernanceProposalShareTransfer   GovernanceProposalType = "share_transfer"
	GovernanceProposalTreasuryGrant   GovernanceProposalType = "treasury_grant"
	GovernanceProposalTreasuryBuyback GovernanceProposalType = "treasury_buyback"
	GovernanceProposalAppointCEO      GovernanceProposalType = "appoint_ceo"
)

const (
	MinPlayers               = 3
	MaxPlayers               = 8
	TotalSharesBPS           = 10000
	InitialPlayerSharesBPS   = 8000
	InitialTreasurySharesBPS = 2000
	MajorDecisionRewardBPS   = 100
	MaxShareChangeBPS        = 500
	MinPlayerShareBPS        = 500
)

var allDecisions = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

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
	GameID     int64      `json:"game_id"`
	Title      string     `json:"title"`
	Status     GameStatus `json:"status"`
	Phase      GamePhase  `json:"phase"`
	IsFinished bool       `json:"is_finished"`
	Winner     string     `json:"winner,omitempty"`

	HostUserID              int64
	CEOUserID               int64
	MoleUserID              int64
	MoleTargets             []string
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
}

type PlayerState struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	ShareBPS int    `json:"share_bps"`
	IsHost   bool   `json:"is_host"`
	IsCEO    bool   `json:"is_ceo"`
	IsKicked bool   `json:"is_kicked"`
	Role     string `json:"role,omitempty"`
}

type VoteState struct {
	UserID   int64   `json:"user_id"`
	Decision *string `json:"decision,omitempty"`
	Abstain  bool    `json:"abstain"`
}

type GovernanceProposalState struct {
	ID             int                    `json:"id"`
	Round          int                    `json:"round"`
	ProposerUserID int64                  `json:"proposer_user_id"`
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

type PublicGameState struct {
	GameID                int64                        `json:"game_id"`
	Title                 string                       `json:"title"`
	Status                GameStatus                   `json:"status"`
	Phase                 GamePhase                    `json:"phase"`
	IsFinished            bool                         `json:"is_finished"`
	Winner                string                       `json:"winner,omitempty"`
	CurrentRound          int                          `json:"current_round"`
	GovernanceRound       int                          `json:"governance_round"`
	TreasuryShareBPS      int                          `json:"treasury_share_bps"`
	AvailableDecisions    []string                     `json:"available_decisions"`
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
	MoleTargets           []string                     `json:"mole_targets,omitempty"`
	AvailableActions      []ActionType                 `json:"available_actions"`
}

type PublicPlayerState struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	ShareBPS int    `json:"share_bps"`
	IsHost   bool   `json:"is_host"`
	IsCEO    bool   `json:"is_ceo"`
	Role     string `json:"role,omitempty"`
}

type PublicVoteState struct {
	UserID   int64 `json:"user_id"`
	HasVoted bool  `json:"has_voted"`
}

type PublicOwnVoteState struct {
	Decision   string `json:"decision,omitempty"`
	ProposalID int    `json:"proposal_id,omitempty"`
	Abstain    bool   `json:"abstain"`
}

type PublicGovernanceProposal struct {
	ID             int                    `json:"id"`
	Round          int                    `json:"round"`
	ProposerUserID int64                  `json:"proposer_user_id"`
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
}

type PublicGovernanceReport struct {
	Round    int                       `json:"round"`
	Outcome  string                    `json:"outcome"`
	Proposal *PublicGovernanceProposal `json:"proposal,omitempty"`
	Reason   string                    `json:"reason,omitempty"`
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
}

type PublicRoundReport struct {
	Round    int                        `json:"round"`
	Outcome  string                     `json:"outcome"`
	Decision string                     `json:"decision,omitempty"`
	Reason   string                     `json:"reason,omitempty"`
	Votes    []PublicDecisionVoteReport `json:"votes"`
}

type PublicDecisionVoteReport struct {
	Decision   string `json:"decision"`
	Abstain    bool   `json:"abstain"`
	ShareBPS   int    `json:"share_bps"`
	VoterCount int    `json:"voter_count"`
}

type GameCreatedPayload struct {
	HostUserID int64  `json:"host_user_id"`
	Title      string `json:"title"`
}

type PlayerJoinedPayload struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type PlayerKickedPayload struct {
	UserID int64 `json:"user_id"`
}

type MoleSelectedPayload struct {
	UserID int64 `json:"user_id"`
}

type MoleTargetsGeneratedPayload struct {
	Targets []string `json:"targets"`
}

type PlayerReceivedSharePayload struct {
	UserID   int64 `json:"user_id"`
	ShareBPS int   `json:"share_bps"`
}

type CEOSelectedPayload struct {
	UserID int64 `json:"user_id"`
}

type VotingRoundStartedPayload struct {
	Round int `json:"round"`
}

type VoteSubmittedPayload struct {
	Round    int     `json:"round"`
	UserID   int64   `json:"user_id"`
	Decision *string `json:"decision,omitempty"`
	Abstain  bool    `json:"abstain"`
}

type GovernanceProposalPhaseStartedPayload struct {
	Round int `json:"round"`
}

type GovernanceProposalSubmittedPayload struct {
	Round          int                    `json:"round"`
	ProposalID     int                    `json:"proposal_id"`
	ProposerUserID int64                  `json:"proposer_user_id"`
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
	Round int `json:"round"`
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
