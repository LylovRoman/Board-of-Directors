import {
  ArrowLeft,
  Bot,
  Briefcase,
  Check,
  Flag,
  MessageCircle,
  Play,
  RefreshCcw,
  Shield,
  UserMinus,
  Users,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Avatar } from "../components/Avatar";
import {
  DECISION_DETAILS,
  DECISION_IDS,
  bpsToPercent,
  decisionLabel,
  decisionTitle,
  decisionType,
  memorandumRule,
  memorandumTitle,
  normalizeList,
  phaseLabel,
  playerName,
  proposalText,
  proposalTypeLabel,
  roleLabel,
  winnerLabel,
} from "../gameText";
import { useNow } from "../hooks/useNow";
import type {
  ActionType,
  GovernanceProposalType,
  MemorandumType,
  PublicGameState,
  PublicGovernanceProposal,
  PublicGovernanceSubmission,
  PublicOwnVoteState,
  PublicPlayerState,
  PublicVoteState,
} from "../types";

interface GameScreenProps {
  state: PublicGameState;
  currentUserId: number;
  liveStatus: string;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
  onBack: () => void;
  onRefresh: () => void;
  onOpenChat: () => void;
  onOpenRules: () => void;
  onOpenReplay: () => void;
  onOpenProfile: (userId?: number) => void;
}

export function GameScreen(props: GameScreenProps) {
  const { state } = props;
  const actions = state.available_actions ?? [];
  const me = state.me;
  const activePlayers = state.players ?? [];
  const canChat = actions.includes("send_chat_message");

  return (
    <main className="mobile-shell game-shell">
      <header className="game-topbar">
        <button className="icon-button" type="button" aria-label="Назад" onClick={props.onBack}>
          <ArrowLeft size={20} />
        </button>
        <div>
          <strong>{state.company_name || state.title}</strong>
          <small>{phaseLabel(state.phase)} · {props.liveStatus}</small>
        </div>
        <button className="icon-button" type="button" aria-label="Обновить" onClick={props.onRefresh}>
          <RefreshCcw size={19} />
        </button>
      </header>

      <section className="company-card">
        <p className="eyebrow">компания</p>
        <h1>{state.title}</h1>
        {state.company_situation ? <p>{state.company_situation}</p> : null}
        <div className="score-strip">
          <div>
            <small>Раунд</small>
            <strong>{state.current_round || 0}</strong>
          </div>
          <div>
            <small>Резерв</small>
            <strong>{bpsToPercent(state.treasury_share_bps)}</strong>
          </div>
          <div>
            <small>Роль</small>
            <strong>{me?.role ? roleLabel(me.role) : "Гость"}</strong>
          </div>
        </div>
        {typeof state.mole_victory_points === "number" && typeof state.players_victory_points === "number" ? (
          <div className="victory-score">
            <span>Крот {state.mole_victory_points}/3</span>
            <span>Совет {state.players_victory_points}/3</span>
          </div>
        ) : null}
      </section>

      <DirectorStrip players={activePlayers} onOpenProfile={props.onOpenProfile} />

      {state.status === "lobby" ? (
        <LobbyPhase
          state={state}
          currentUserId={props.currentUserId}
          isSubmitting={props.isSubmitting}
          onAction={props.onAction}
          onOpenProfile={props.onOpenProfile}
        />
      ) : state.is_finished ? (
        <FinishPhase state={state} me={me} onOpenReplay={props.onOpenReplay} />
      ) : (
        <StartedPhase
          state={state}
          currentUserId={props.currentUserId}
          isSubmitting={props.isSubmitting}
          onAction={props.onAction}
        />
      )}

      <nav className="game-action-bar" aria-label="Действия партии">
        <button type="button" onClick={props.onOpenChat}>
          <MessageCircle size={19} />
          Чат
          {normalizeList(state.chat_messages).length ? <span>{normalizeList(state.chat_messages).length}</span> : null}
        </button>
        <button type="button" onClick={props.onOpenRules}>
          <Shield size={19} />
          Правила
        </button>
        <button type="button" onClick={() => props.onOpenProfile(me?.user_id || props.currentUserId)}>
          <Briefcase size={19} />
          Профиль
        </button>
        {state.is_finished ? (
          <button type="button" onClick={props.onOpenReplay}>
            <Flag size={19} />
            Replay
          </button>
        ) : canChat ? null : (
          <button type="button" disabled>
            <Users size={19} />
            Гость
          </button>
        )}
      </nav>
    </main>
  );
}

function DirectorStrip(props: { players: PublicPlayerState[]; onOpenProfile: (userId?: number) => void }) {
  return (
    <section className="director-strip" aria-label="Директора">
      {props.players.map((player) => (
        <button className="director-chip" type="button" key={player.user_id} onClick={() => props.onOpenProfile(player.user_id)}>
          <Avatar name={player.name} avatarUrl={player.avatar_url} size="sm" />
          <span>
            <strong>{player.name}</strong>
            <small>
              {bpsToPercent(player.share_bps)} · {player.is_ceo ? "CEO" : player.company_position || "директор"}
            </small>
          </span>
        </button>
      ))}
    </section>
  );
}

function LobbyPhase(props: {
  state: PublicGameState;
  currentUserId: number;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
  onOpenProfile: (userId?: number) => void;
}) {
  const actions = props.state.available_actions ?? [];
  const players = props.state.players ?? [];
  const canJoin = actions.includes("join_game");
  const canLeave = actions.includes("leave_game");
  const canStart = actions.includes("start_game");
  const canKick = actions.includes("kick_player");
  const canBan = actions.includes("ban_player");
  const canAddBot = actions.includes("add_bot");

  return (
    <section className="phase-card">
      <div className="section-heading-row">
        <div>
          <p className="eyebrow">лобби</p>
          <h2>{players.length}/8 директоров</h2>
        </div>
        {canStart ? <span className="ready-pill">Host</span> : null}
      </div>

      <div className="lobby-actions-grid">
        {canJoin ? (
          <button className="primary-action wide-action" type="button" disabled={props.isSubmitting} onClick={() => props.onAction("join_game")}>
            Войти в комнату
          </button>
        ) : null}
        {canLeave ? (
          <button className="secondary-action wide-action" type="button" disabled={props.isSubmitting} onClick={() => props.onAction("leave_game")}>
            Выйти из лобби
          </button>
        ) : null}
        {canStart ? (
          <button className="primary-action wide-action" type="button" disabled={props.isSubmitting || players.length < 3} onClick={() => props.onAction("start_game")}>
            <Play size={18} />
            Начать игру
          </button>
        ) : null}
        {canAddBot ? (
          <button className="secondary-action wide-action" type="button" disabled={props.isSubmitting} onClick={() => props.onAction("add_bot", { count: 1 })}>
            <Bot size={18} />
            Добавить бота
          </button>
        ) : null}
      </div>

      <div className="player-list">
        {players.map((player) => (
          <article className="player-list-card" key={player.user_id}>
            <button type="button" className="player-identity-button" onClick={() => props.onOpenProfile(player.user_id)} disabled={player.is_bot}>
              <Avatar name={player.name} avatarUrl={player.avatar_url} size="md" />
              <span>
                <strong>{player.name}</strong>
                <small>{player.is_host ? "Host" : player.company_position || "Директор"}</small>
              </span>
            </button>
            {player.user_id !== props.currentUserId && (canKick || canBan) ? (
              <div className="host-actions">
                {canKick ? (
                  <button
                    className="icon-danger-button"
                    type="button"
                    disabled={props.isSubmitting}
                    onClick={() => props.onAction("kick_player", { user_id: player.user_id })}
                    aria-label={`Убрать ${player.name}`}
                  >
                    <UserMinus size={17} />
                  </button>
                ) : null}
                {canBan ? (
                  <button
                    className="mini-button danger"
                    type="button"
                    disabled={props.isSubmitting}
                    onClick={() => props.onAction("ban_player", { user_id: player.user_id })}
                  >
                    ban
                  </button>
                ) : null}
              </div>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}

function StartedPhase(props: {
  state: PublicGameState;
  currentUserId: number;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
}) {
  const phase = props.state.phase;
  const actions = props.state.available_actions ?? [];

  if (phase === "mole_objective_selection") {
    return (
      <MoleObjectivePhase
        state={props.state}
        canSelect={actions.includes("select_mole_objectives")}
        canChooseMemorandum={actions.includes("choose_memorandum")}
        isSubmitting={props.isSubmitting}
        onAction={props.onAction}
      />
    );
  }

  if (phase === "governance_proposal") {
    return (
      <GovernanceProposalPhase
        state={props.state}
        currentUserId={props.currentUserId}
        canSubmit={actions.includes("submit_governance_proposal")}
        canSkip={actions.includes("skip_governance_proposal")}
        isSubmitting={props.isSubmitting}
        onAction={props.onAction}
      />
    );
  }

  if (phase === "governance_voting") {
    return (
      <GovernanceVotingPhase
        players={props.state.players}
        proposals={normalizeList(props.state.governance_proposals)}
        votes={normalizeList(props.state.current_votes)}
        myVote={props.state.my_current_vote ?? null}
        me={props.state.me}
        canVote={actions.includes("vote")}
        isSubmitting={props.isSubmitting}
        onAction={props.onAction}
      />
    );
  }

  return (
    <MajorVotingPhase
      state={props.state}
      canVote={actions.includes("vote")}
      isSubmitting={props.isSubmitting}
      onAction={props.onAction}
    />
  );
}

function MoleObjectivePhase(props: {
  state: PublicGameState;
  canSelect: boolean;
  canChooseMemorandum: boolean;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
}) {
  const [targets, setTargets] = useState<string[]>([]);
  const [sabotage, setSabotage] = useState("");
  const isMole = props.state.me?.role === "mole";
  const selectedTargets = new Set(targets);
  const canSubmit = props.canSelect && targets.length === 3 && sabotage && !selectedTargets.has(sabotage);

  function toggleTarget(decision: string) {
    setTargets((current) => {
      if (current.includes(decision)) {
        return current.filter((item) => item !== decision);
      }
      if (current.length >= 3 || sabotage === decision) {
        return current;
      }
      return [...current, decision].sort();
    });
  }

  if (!isMole) {
    const preference = props.state.memorandum_preference;
    return (
      <section className="phase-card">
        <p className="eyebrow">тайный брифинг</p>
        <h2>Крот выбирает цели</h2>
        {preference ? (
          <div className="memorandum-card selected">
            <strong>{memorandumTitle(preference)}</strong>
            <span>{memorandumRule(preference)}</span>
          </div>
        ) : (
          <div className="decision-grid two">
            {(["opportunity", "risk"] as MemorandumType[]).map((type) => (
              <button
                className="decision-card memorandum"
                type="button"
                key={type}
                disabled={!props.canChooseMemorandum || props.isSubmitting}
                onClick={() => props.onAction("choose_memorandum", { type })}
              >
                <strong>{memorandumTitle(type)}</strong>
                <span>{memorandumRule(type)}</span>
              </button>
            ))}
          </div>
        )}
      </section>
    );
  }

  return (
    <section className="phase-card">
      <div className="section-heading-row">
        <div>
          <p className="eyebrow">секретный выбор</p>
          <h2>Подкопы и Диверсия</h2>
        </div>
        <span className="ready-pill">{targets.length}/3</span>
      </div>
      <div className="objective-list">
        {DECISION_IDS.map((decision) => {
          const isTarget = selectedTargets.has(decision);
          const isSabotage = sabotage === decision;
          return (
            <article className={`objective-mobile-card ${decisionType(decision, props.state)} ${isTarget ? "is-target" : ""} ${isSabotage ? "is-sabotage" : ""}`} key={decision}>
              <div>
                <small>{decision}</small>
                <strong>{decisionTitle(decision)}</strong>
                <span>{DECISION_DETAILS[decision]}</span>
              </div>
              <div className="objective-buttons">
                <button type="button" onClick={() => toggleTarget(decision)} disabled={props.isSubmitting || (!isTarget && (targets.length >= 3 || isSabotage))}>
                  Подкоп
                </button>
                <button type="button" onClick={() => { setSabotage(decision); setTargets((current) => current.filter((item) => item !== decision)); }} disabled={props.isSubmitting}>
                  Диверсия
                </button>
              </div>
            </article>
          );
        })}
      </div>
      <button
        className="primary-action wide-action sticky-submit"
        type="button"
        disabled={!canSubmit || props.isSubmitting}
        onClick={() => props.onAction("select_mole_objectives", { targets, sabotage })}
      >
        Подтвердить цели
      </button>
    </section>
  );
}

function MajorVotingPhase(props: {
  state: PublicGameState;
  canVote: boolean;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
}) {
  const now = useNow();
  const options = normalizeList(props.state.major_vote_options).length
    ? normalizeList(props.state.major_vote_options)
    : normalizeList(props.state.available_decisions);
  const unlockAt = props.state.major_vote_unlocked_at ? new Date(props.state.major_vote_unlocked_at).getTime() : 0;
  const locked = Boolean(unlockAt && unlockAt > now);
  const secondsLeft = Math.max(0, Math.ceil((unlockAt - now) / 1000));
  const selectedDecision = props.state.my_current_vote?.decision;
  const moleTargets = new Set(normalizeList(props.state.mole_targets));
  const moleSabotage = props.state.mole_sabotage;

  return (
    <section className="phase-card">
      <div className="section-heading-row">
        <div>
          <p className="eyebrow">major vote</p>
          <h2>Выбери решение</h2>
        </div>
        {locked ? <span className="ready-pill">обсуждение {secondsLeft}с</span> : selectedDecision ? <span className="ready-pill">можно изменить</span> : null}
      </div>

      {props.state.memorandum ? (
        <div className="memorandum-card">
          <strong>{memorandumTitle(props.state.memorandum.type)}</strong>
          <span>{memorandumRule(props.state.memorandum.type)}</span>
          <div>{props.state.memorandum.decisions.map((decision) => <em key={decision}>{decision}</em>)}</div>
        </div>
      ) : null}

      <div className="decision-grid">
        {options.map((decision) => {
          const selected = selectedDecision === decision;
          const isMoleTarget = props.state.me?.role === "mole" && moleTargets.has(decision);
          const isSabotage = props.state.me?.role === "mole" && moleSabotage === decision;
          return (
            <button
              className={`decision-card ${decisionType(decision, props.state)} ${selected ? "selected" : ""} ${isMoleTarget ? "mole-target" : ""} ${isSabotage ? "mole-sabotage" : ""}`}
              type="button"
              key={decision}
              disabled={!props.canVote || locked || props.isSubmitting}
              onClick={() => props.onAction("vote", { decision })}
            >
              <span>{isSabotage ? "Диверсия" : isMoleTarget ? "Подкоп" : decision}</span>
              <strong>{decisionTitle(decision)}</strong>
              <small>{DECISION_DETAILS[decision]}</small>
              {selected ? <Check size={18} /> : null}
            </button>
          );
        })}
      </div>
    </section>
  );
}

function GovernanceProposalPhase(props: {
  state: PublicGameState;
  currentUserId: number;
  canSubmit: boolean;
  canSkip: boolean;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
}) {
  const players = props.state.players;
  const submissions = normalizeList(props.state.governance_submissions);
  const [plusUserId, setPlusUserId] = useState<number | null>(null);
  const [minusUserId, setMinusUserId] = useState<number | null>(null);
  const mySubmission = submissions.find((submission) => submission.user_id === props.currentUserId);
  const me = players.find((player) => player.user_id === props.currentUserId);
  const strengthBps = me?.authority_bps ?? 0;

  const effectiveShareBps = useMemo(() => {
    if (!strengthBps || plusUserId === minusUserId) {
      return 0;
    }
    if (plusUserId && minusUserId) {
      const from = players.find((player) => player.user_id === minusUserId);
      return Math.min(strengthBps, Math.max(0, (from?.share_bps ?? 0) - 500));
    }
    if (plusUserId) {
      return Math.min(strengthBps, Math.max(0, props.state.treasury_share_bps));
    }
    if (minusUserId) {
      const target = players.find((player) => player.user_id === minusUserId);
      return Math.min(strengthBps, Math.max(0, (target?.share_bps ?? 0) - 500));
    }
    return 0;
  }, [minusUserId, players, plusUserId, props.state.treasury_share_bps, strengthBps]);

  const canSubmit = props.canSubmit && effectiveShareBps > 0 && (Boolean(plusUserId) || Boolean(minusUserId));

  function submit() {
    if (!canSubmit) {
      return;
    }
    if (plusUserId && minusUserId) {
      props.onAction("submit_governance_proposal", {
        proposal_type: "share_transfer",
        from_user_id: minusUserId,
        to_user_id: plusUserId,
      });
    } else if (plusUserId) {
      props.onAction("submit_governance_proposal", {
        proposal_type: "treasury_grant",
        target_user_id: plusUserId,
      });
    } else if (minusUserId) {
      props.onAction("submit_governance_proposal", {
        proposal_type: "treasury_buyback",
        target_user_id: minusUserId,
      });
    }
  }

  return (
    <section className="phase-card">
      <div className="section-heading-row">
        <div>
          <p className="eyebrow">governance</p>
          <h2>Маневр с долями</h2>
        </div>
        {mySubmission?.status ? <span className="ready-pill">готово</span> : null}
      </div>

      {mySubmission?.status ? (
        <div className="empty-state">
          <strong>{mySubmission.status === "submitted" ? "Предложение подано" : "Раунд пропущен"}</strong>
          <span>Ждем остальных директоров.</span>
        </div>
      ) : (
        <>
          <div className="governance-summary-card">
            <span>Сила предложения</span>
            <strong>{bpsToPercent(strengthBps)}</strong>
            <small>Применится: {bpsToPercent(effectiveShareBps)}. Backend сам ограничит маневр минимумом 5% у игрока и 0% в резерве.</small>
          </div>
          <div className="governance-player-list">
            {players.map((player) => {
              const plus = plusUserId === player.user_id;
              const minus = minusUserId === player.user_id;
              return (
                <article className={`governance-player ${plus ? "plus" : ""} ${minus ? "minus" : ""}`} key={player.user_id}>
                  <div>
                    <Avatar name={player.name} avatarUrl={player.avatar_url} size="sm" />
                    <span>
                      <strong>{player.name}</strong>
                      <small>{bpsToPercent(player.share_bps)} доля · {bpsToPercent(player.authority_bps)} полномочия</small>
                    </span>
                  </div>
                  <div className="plus-minus">
                    <button
                      type="button"
                      disabled={!props.canSubmit || props.isSubmitting}
                      onClick={() => {
                        setPlusUserId((current) => (current === player.user_id ? null : player.user_id));
                        setMinusUserId((current) => (current === player.user_id ? null : current));
                      }}
                    >
                      +
                    </button>
                    <button
                      type="button"
                      disabled={!props.canSubmit || props.isSubmitting}
                      onClick={() => {
                        setMinusUserId((current) => (current === player.user_id ? null : player.user_id));
                        setPlusUserId((current) => (current === player.user_id ? null : current));
                      }}
                    >
                      −
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
          <div className="button-row sticky-submit">
            <button className="primary-action" type="button" disabled={!canSubmit || props.isSubmitting} onClick={submit}>
              Подать
            </button>
            <button className="secondary-action" type="button" disabled={!props.canSkip || props.isSubmitting} onClick={() => props.onAction("skip_governance_proposal")}>
              Пропустить
            </button>
          </div>
        </>
      )}
    </section>
  );
}

function GovernanceVotingPhase(props: {
  players: PublicPlayerState[];
  proposals: PublicGovernanceProposal[];
  votes: PublicVoteState[];
  myVote: PublicOwnVoteState | null;
  me?: PublicPlayerState;
  canVote: boolean;
  isSubmitting: boolean;
  onAction: (type: ActionType, payload?: Record<string, unknown>) => void;
}) {
  const voted = Boolean(props.myVote);
  const canAbstain = props.canVote && !props.me?.is_ceo;

  return (
    <section className="phase-card">
      <div className="section-heading-row">
        <div>
          <p className="eyebrow">governance vote</p>
          <h2>Выбери предложение</h2>
        </div>
        {voted ? <span className="ready-pill">можно изменить</span> : null}
      </div>
      <div className="proposal-list">
        {props.proposals.map((proposal) => (
          <GovernanceProposalCard
            key={proposal.id}
            proposal={proposal}
            players={props.players}
            votes={props.votes}
            selected={props.myVote?.proposal_id === proposal.id}
            disabled={!props.canVote || props.isSubmitting}
            onVote={() => props.onAction("vote", { proposal_id: proposal.id })}
          />
        ))}
      </div>
      <button
        className={props.myVote?.abstain ? "secondary-action wide-action selected" : "secondary-action wide-action"}
        type="button"
        disabled={!canAbstain || props.isSubmitting}
        onClick={() => props.onAction("vote", { abstain: true })}
      >
        Воздержаться
      </button>
    </section>
  );
}

function GovernanceProposalCard(props: {
  proposal: PublicGovernanceProposal;
  players: PublicPlayerState[];
  votes: PublicVoteState[];
  selected: boolean;
  disabled: boolean;
  onVote: () => void;
}) {
  const proposalVotes = props.votes.filter((vote) => vote.has_voted && vote.proposal_id === props.proposal.id);
  const totalPower = proposalVotes.reduce((sum, vote) => sum + (vote.voting_power_bps ?? 0), 0);

  return (
    <button className={`proposal-mobile-card ${props.selected ? "selected" : ""}`} type="button" disabled={props.disabled} onClick={props.onVote}>
      <span>{proposalTypeLabel(props.proposal.proposal_type as GovernanceProposalType)}</span>
      <strong>{proposalText(props.proposal, props.players)}</strong>
      <small>Сила предложения {bpsToPercent(props.proposal.share_bps)} · голосов {bpsToPercent(totalPower)}</small>
      <div className="mini-voters">
        {proposalVotes.slice(0, 4).map((vote) => (
          <em key={vote.user_id}>{playerName(props.players, vote.user_id)}</em>
        ))}
      </div>
    </button>
  );
}

function FinishPhase(props: { state: PublicGameState; me?: PublicPlayerState; onOpenReplay: () => void }) {
  const summary = props.state.final_summary;
  const myStat = summary?.player_stats.find((stat) => stat.user_id === props.me?.user_id);
  const playerWon =
    props.state.winner === "mole" ? props.me?.role === "mole" : props.state.winner === "players" && props.me?.role !== "mole";

  return (
    <section className="phase-card finish-card">
      <p className="eyebrow">финал</p>
      <h2>{winnerLabel(props.state.winner)}</h2>
      <div className={playerWon ? "personal-result win" : "personal-result lose"}>
        <strong>{playerWon ? "Ты в победившей стороне" : "Эта партия ушла другой стороне"}</strong>
        <span>{props.me?.role ? roleLabel(props.me.role) : "Роль неизвестна"}</span>
      </div>
      <div className="score-strip">
        <div>
          <small>Крот</small>
          <strong>{summary?.mole_points ?? props.state.mole_victory_points ?? 0}/3</strong>
        </div>
        <div>
          <small>Совет</small>
          <strong>{summary?.players_points ?? props.state.players_victory_points ?? 0}/3</strong>
        </div>
        <div>
          <small>Точность</small>
          <strong>{bpsToPercent(myStat?.accuracy_bps)}</strong>
        </div>
      </div>
      {summary ? (
        <div className="final-reveal">
          <span>Крот: {playerName(props.state.players, summary.mole_user_id)}</span>
          <span>Подкопы: {summary.mole_targets.map(decisionLabel).join(", ")}</span>
          <span>Диверсия: {decisionLabel(summary.mole_sabotage)}</span>
        </div>
      ) : null}
      <button className="primary-action wide-action" type="button" onClick={props.onOpenReplay}>
        Открыть replay
      </button>
    </section>
  );
}
