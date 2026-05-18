package models

import "time"

type User struct {
	ID           int64      `json:"id"`
	Login        string     `json:"login,omitempty"`
	Name         string     `json:"name"`
	PasswordHash string     `json:"-"`
	AvatarURL    string     `json:"avatar_url,omitempty"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

type Game struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID         int64     `json:"id"`
	GameID     int64     `json:"game_id"`
	UserID     *int64    `json:"user_id,omitempty"`
	ActorName  string    `json:"actor_name,omitempty"`
	EventType  string    `json:"event_type"`
	EventValue string    `json:"event_value,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

const (
	EventGameCreated     = "game_created"
	EventPlayerJoined    = "player_joined"
	EventPlayerLeft      = "player_left"
	EventPlayerKicked    = "player_kicked"
	EventChatMessageSent = "chat_message_sent"

	EventGameStarted                  = "game_started"
	EventMoleSelected                 = "mole_selected"
	EventMoleTargetsGenerated         = "mole_targets_generated"
	EventMoleObjectivesSelected       = "mole_objectives_selected"
	EventMemorandumPreferenceSelected = "memorandum_preference_selected"
	EventMemorandumAssigned           = "memorandum_assigned"
	EventPlayerReceivedShare          = "player_received_share"
	EventPlayerAuthorityGranted       = "player_authority_granted"
	EventCEOSelected                  = "ceo_selected"
	EventVotingRoundStarted           = "voting_round_started"

	EventVoteSubmitted  = "vote_submitted"
	EventVotingResolved = "voting_resolved"

	EventDecisionAccepted = "decision_accepted"
	EventDecisionRejected = "decision_rejected"
	EventGameFinished     = "game_finished"

	EventGovernanceProposalPhaseStarted = "governance_proposal_phase_started"
	EventGovernanceProposalSubmitted    = "governance_proposal_submitted"
	EventGovernanceProposalSkipped      = "governance_proposal_skipped"
	EventGovernanceVotingStarted        = "governance_voting_started"
	EventGovernanceVoteSubmitted        = "governance_vote_submitted"
	EventGovernanceResolved             = "governance_resolved"
	EventGovernanceProposalAccepted     = "governance_proposal_accepted"
	EventGovernanceProposalRejected     = "governance_proposal_rejected"
	EventPlayerShareTransferred         = "player_share_transferred"
	EventTreasuryShareGranted           = "treasury_share_granted"
	EventTreasuryShareBoughtBack        = "treasury_share_bought_back"
	EventCEOChanged                     = "ceo_changed"
)
