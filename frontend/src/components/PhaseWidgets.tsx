import { useMemo, useState } from "react";
import type React from "react";
import type { FormEvent } from "react";
import type { MemorandumType, PublicGameState, PublicGovernanceProposal, PublicGovernanceSubmission, PublicOwnVoteState, PublicPlayerState, PublicVoteState } from "../types";
import { DECISION_OPTIONS } from "../gameData";
import { decisionTitle, decisionType, describeGovernanceProposal, formatShare, memorandumRule, memorandumTitle, playerName } from "../gameText";
import { DecisionTypeTag } from "./DecisionWidgets";
import { UserAvatar } from "./UserAvatar";

export function ComplianceWatchPanel(props: {
  players: PublicPlayerState[];
  currentUserId: number;
  watch: PublicGameState["compliance_watch"] | null;
  canPlace: boolean;
  isSubmitting: boolean;
  onPlace: (targetUserId: number) => void;
}) {
  const candidates = props.players.filter((player) => player.user_id !== props.currentUserId);
  const watchedPlayer = props.watch
      ? props.players.find((player) => player.user_id === props.watch?.target_user_id)
      : undefined;
  const watchedUserId = props.watch?.target_user_id ?? null;

  return (
      <section className="compliance-watch-panel">
        <div>
          <p className="eyebrow">Негласное наблюдение</p>
          <p className="quiet-text">
            {props.canPlace
                ? watchedPlayer
                    ? `Сейчас под наблюдением: ${watchedPlayer.name}. Нажмите другого игрока, чтобы сменить цель.`
                    : "Нажмите на игрока, чтобы установить наблюдение в этом раунде."
                : "После принятой Диверсии наблюдение больше недоступно."}
          </p>
        </div>
        <div className="watch-player-grid">
          {candidates.map((player) => (
              <button
                  key={player.user_id}
                  type="button"
                  className={watchedUserId === player.user_id ? "watch-player-option selected" : "watch-player-option"}
                  onClick={() => props.onPlace(player.user_id)}
                  disabled={!props.canPlace || props.isSubmitting}
              >
                <UserAvatar name={player.name} avatarUrl={player.avatar_url} size="small" />
                <span>{player.name}</span>
              </button>
          ))}
        </div>
      </section>
  );
}

export function MoleObjectiveSelectionPhase(props: {
  isMole: boolean;
  viewerRole?: PublicPlayerState["role"];
  canSelect: boolean;
  canChooseMemorandum: boolean;
  memorandumPreference?: MemorandumType;
  isSubmitting: boolean;
  onSubmit: (payload: Record<string, unknown>) => void;
  onChooseMemorandum: (type: MemorandumType) => void;
}) {
  const [targets, setTargets] = useState<string[]>([]);
  const [sabotage, setSabotage] = useState("");
  const selectedTargets = new Set(targets);
  const canSubmit = props.canSelect && targets.length === 3 && Boolean(sabotage) && !selectedTargets.has(sabotage);

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

  function chooseSabotage(decision: string) {
    setSabotage(decision);
    setTargets((current) => current.filter((item) => item !== decision));
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    props.onSubmit({ targets, sabotage });
  }

  if (!props.isMole) {
    const isCompliance = props.viewerRole === "compliance";
    return (
        <section className="objective-selection waiting-selection">
          <p className="eyebrow">подготовка</p>
          <p className="quiet-text">
            {isCompliance
                ? "Выберите предпочтительный тип подсказки. Комплаенс не получает стартовый меморандум: этот выбор будет использован только для позднего меморандума после принятой Диверсии, если партия продолжится."
                : "Выберите тип стартового меморандума. Конкретная тройка решений будет сгенерирована случайно после того, как Крот подтвердит цели."}
          </p>
          {props.memorandumPreference ? (
              <div className="memorandum-choice selected">
                <strong>{memorandumTitle(props.memorandumPreference)}</strong>
                <span>{memorandumRule(props.memorandumPreference)}</span>
              </div>
          ) : (
              <div className="memorandum-choice-grid">
                <button
                    type="button"
                    className="memorandum-choice"
                    disabled={!props.canChooseMemorandum || props.isSubmitting}
                    onClick={() => props.onChooseMemorandum("opportunity")}
                >
                  <strong>Принимая решения, я часто вижу возможности</strong>
                  <span>{memorandumRule("opportunity")}</span>
                </button>
                <button
                    type="button"
                    className="memorandum-choice"
                    disabled={!props.canChooseMemorandum || props.isSubmitting}
                    onClick={() => props.onChooseMemorandum("risk")}
                >
                  <strong>Принимая решения, я часто учитываю риски</strong>
                  <span>{memorandumRule("risk")}</span>
                </button>
              </div>
          )}
          <p className="quiet-text">Первое голосование начнется, когда тайный Крот выберет три Подкопа и одну Диверсию.</p>
        </section>
    );
  }

  return (
      <form className="objective-selection" onSubmit={submit}>
        <div className="section-heading compact-heading">
          <div>
            <p className="eyebrow">секретный выбор</p>
            <h2>Выбери Подкопы и Диверсию</h2>
          </div>
          <span className="wait-pill">{targets.length}/3 Подкопа</span>
        </div>
        <div className="objective-grid">
          {DECISION_OPTIONS.map((decision) => {
            const isTarget = selectedTargets.has(decision);
            const isSabotage = sabotage === decision;
            const type = decisionType(decision);
            return (
                <article
                    className={["objective-card", type, isTarget ? "is-target" : "", isSabotage ? "is-sabotage" : ""]
                        .filter(Boolean)
                        .join(" ")}
                    key={decision}
                >
                  <span>{isSabotage ? "Диверсия" : isTarget ? "Подкоп" : "Решение"}</span>
                  <strong>{decisionTitle(decision)}</strong>
                  <div className="decision-meta">
                    <small>{decision}</small>
                    <DecisionTypeTag type={type} />
                  </div>
                  <div className="objective-actions">
                    <button type="button" className="secondary-action" onClick={() => toggleTarget(decision)} disabled={props.isSubmitting || (!isTarget && (targets.length >= 3 || isSabotage))}>
                      Подкоп
                    </button>
                    <button type="button" className="sabotage-pick-action" onClick={() => chooseSabotage(decision)} disabled={props.isSubmitting}>
                      Диверсия
                    </button>
                  </div>
                </article>
            );
          })}
        </div>
        <button className="primary-action" type="submit" disabled={!canSubmit || props.isSubmitting}>
          Подтвердить цели
        </button>
      </form>
  );
}

export function GovernanceProposalPhase(props: {
  players: PublicPlayerState[];
  submissions: PublicGovernanceSubmission[];
  treasuryShareBPS: number;
  currentUserId: number;
  canSubmit: boolean;
  canSkip: boolean;
  isSubmitting: boolean;
  onSubmit: (payload: Record<string, unknown>) => void;
  onSkip: () => void;
}) {
  const [plusUserId, setPlusUserId] = useState<number | null>(null);
  const [minusUserId, setMinusUserId] = useState<number | null>(null);

  const mySubmission = props.submissions.find((submission) => submission.user_id === props.currentUserId);
  const canAct = props.canSubmit || props.canSkip;
  const currentPlayer = props.players.find((player) => player.user_id === props.currentUserId);
  const proposalStrengthBPS = currentPlayer?.authority_bps ?? 0;
  const effectiveShareBPS = useMemo(() => {
    if (!proposalStrengthBPS || plusUserId === minusUserId) {
      return 0;
    }
    if (plusUserId && minusUserId) {
      const from = props.players.find((player) => player.user_id === minusUserId);
      return Math.min(proposalStrengthBPS, Math.max(0, (from?.share_bps ?? 0) - 500));
    }
    if (plusUserId) {
      return Math.min(proposalStrengthBPS, Math.max(0, props.treasuryShareBPS));
    }
    if (minusUserId) {
      const target = props.players.find((player) => player.user_id === minusUserId);
      return Math.min(proposalStrengthBPS, Math.max(0, (target?.share_bps ?? 0) - 500));
    }
    return 0;
  }, [minusUserId, plusUserId, proposalStrengthBPS, props.players, props.treasuryShareBPS]);
  const isPartialProposal = effectiveShareBPS > 0 && effectiveShareBPS < proposalStrengthBPS;
  const canSubmitForm = props.canSubmit && (Boolean(plusUserId) || Boolean(minusUserId)) && plusUserId !== minusUserId && effectiveShareBPS > 0;

  function togglePlus(userId: number) {
    setPlusUserId((current) => (current === userId ? null : userId));
    setMinusUserId((current) => (current === userId ? null : current));
  }

  function toggleMinus(userId: number) {
    setMinusUserId((current) => (current === userId ? null : userId));
    setPlusUserId((current) => (current === userId ? null : current));
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmitForm) {
      return;
    }
    if (plusUserId && minusUserId) {
      props.onSubmit({
        proposal_type: "share_transfer",
        from_user_id: minusUserId,
        to_user_id: plusUserId,
      });
      return;
    }
    if (plusUserId) {
      props.onSubmit({
        proposal_type: "treasury_grant",
        target_user_id: plusUserId,
      });
      return;
    }
    if (minusUserId) {
      props.onSubmit({
        proposal_type: "treasury_buyback",
        target_user_id: minusUserId,
      });
    }
  }

  return (
      <section className="voting-board governance-board">
        <div className="section-heading compact-heading">
          <div>
            <p className="eyebrow">Корпоративные манёвры</p>
          </div>
          {!canAct ? <span className="wait-pill">Ждем остальных</span> : null}
        </div>

        {mySubmission?.status ? (
            <p className="quiet-text">Ты уже {mySubmission.status === "submitted" ? "подал предложение" : "пропустил манёвр"}.</p>
        ) : (
            <form className="governance-form" onSubmit={submit}>
              <div className="governance-proposal-summary">
                <span>Сила предложения</span>
                <strong>{formatShare(currentPlayer?.authority_bps)} полномочия</strong>
                {plusUserId || minusUserId ? (
                    <span>
                Применится: <strong>{formatShare(effectiveShareBPS)}</strong>
              </span>
                ) : null}
                {isPartialProposal ? (
                    <small className="governance-warning">Будет передана не вся доля: останется минимум 5% у игрока или 0% в резерве.</small>
                ) : null}
              </div>

              <div className="governance-pick-grid">
                {props.players.map((player) => (
                    <article
                        className={[
                          "governance-player-card",
                          plusUserId === player.user_id ? "plus-selected" : "",
                          minusUserId === player.user_id ? "minus-selected" : "",
                        ]
                            .filter(Boolean)
                            .join(" ")}
                        key={player.user_id}
                    >
                      <div className="governance-player-main">
                        <UserAvatar name={player.name} avatarUrl={player.avatar_url} size="small" />
                        <div>
                          <strong>{player.name}</strong>
                          <span>{formatShare(player.share_bps)} · {formatShare(player.authority_bps)}</span>
                        </div>
                      </div>
                      <div className="governance-icon-actions">
                        <button
                            type="button"
                            className="icon-action plus-action"
                            onClick={() => togglePlus(player.user_id)}
                            disabled={!props.canSubmit || props.isSubmitting}
                            aria-label={`Дать долю: ${player.name}`}
                            title="Дать долю"
                        >
                          +
                        </button>
                        <button
                            type="button"
                            className="icon-action minus-action"
                            onClick={() => toggleMinus(player.user_id)}
                            disabled={!props.canSubmit || props.isSubmitting}
                            aria-label={`Оштрафовать: ${player.name}`}
                            title="Оштрафовать"
                        >
                          −
                        </button>
                      </div>
                    </article>
                ))}
              </div>

              <div className="governance-actions">
                <button className="primary-action" type="submit" disabled={!canSubmitForm || props.isSubmitting}>
                  Подать предложение
                </button>
                <button
                    className="secondary-action"
                    type="button"
                    onClick={props.onSkip}
                    disabled={!props.canSkip || props.isSubmitting}
                >
                  Пропустить
                </button>
              </div>
            </form>
        )}

      </section>
  );
}

export function GovernanceVotingPhase(props: {
  players: PublicPlayerState[];
  proposals: PublicGovernanceProposal[];
  currentVotes: PublicVoteState[];
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  hasVoted: boolean;
  isSubmitting: boolean;
  isCEO: boolean;
  onVote: (proposalId: number) => void;
  onAbstain: () => void;
}) {
  const canAbstain = props.canVote && !props.isCEO;

  return (
      <section className="voting-board governance-board">
        <div className="section-heading compact-heading">
          <div>
            <p className="eyebrow">Корпоративные манёвры</p>
          </div>
          {props.hasVoted ? <span className="wait-pill">Выбор сохранен, можно изменить</span> : null}
        </div>

        <div className="proposal-grid">
          {props.proposals.map((proposal) => (
              <GovernanceProposalCard
                  key={proposal.id}
                  proposal={proposal}
                  players={props.players}
                  currentVotes={props.currentVotes}
                  selected={props.myCurrentVote?.proposal_id === proposal.id}
                  disabled={!props.canVote || props.isSubmitting}
                  onVote={() => props.onVote(proposal.id)}
              />
          ))}
        </div>

        {props.isCEO ? null : (
            <button
                className={props.myCurrentVote?.abstain ? "secondary-action abstain-button selected-abstain" : "secondary-action abstain-button"}
                onClick={props.onAbstain}
                disabled={!canAbstain || props.isSubmitting}
            >
              Воздержаться
            </button>
        )}
      </section>
  );
}

export function GovernanceProposalCard(props: {
  proposal: PublicGovernanceProposal;
  players: PublicPlayerState[];
  currentVotes: PublicVoteState[];
  selected: boolean;
  disabled: boolean;
  onVote: () => void;
}) {
  const proposer = props.players.find((player) => player.user_id === props.proposal.proposer_user_id);
  const proposerName = proposer?.name ?? playerName(props.players, props.proposal.proposer_user_id);
  const authorIds = props.proposal.author_user_ids?.length ? props.proposal.author_user_ids : [props.proposal.proposer_user_id];
  const proposalVotes = props.currentVotes.filter((vote) => vote.has_voted && vote.proposal_id === props.proposal.id);

  return (
      <article
          className={["proposal-card", "proposal-card-button", props.selected ? "selected-vote" : "", props.disabled ? "is-disabled" : ""]
              .filter(Boolean)
              .join(" ")}
          role="button"
          tabIndex={props.disabled ? -1 : 0}
          onClick={() => {
            if (!props.disabled) {
              props.onVote();
            }
          }}
          onKeyDown={(event) => {
            if (!props.disabled && (event.key === "Enter" || event.key === " ")) {
              event.preventDefault();
              props.onVote();
            }
          }}
      >
        <strong>{describeGovernanceProposal(props.proposal, props.players)}</strong>
        <small>Сила: {formatShare(props.proposal.share_bps)}</small>
        <div className="proposal-authors">
          {authorIds.map((authorId) => {
            const author = props.players.find((player) => player.user_id === authorId);
            const name = author?.name ?? playerName(props.players, authorId);
            return (
                <span className="proposal-author" key={authorId}>
              <UserAvatar name={name} avatarUrl={author?.avatar_url} size="small" />
                  {name}
            </span>
            );
          })}
        </div>
      </article>
  );
}
