import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { DecisionType, GamePhase, MemorandumType, PublicChatMessage, PublicGameState, PublicGovernanceProposal, PublicGovernanceReport, PublicGovernanceSubmission, PublicOwnVoteState, PublicPlayerState, PublicRoundReport, PublicVoteState } from "../types";
import { BOT_ACTION_DELAY_MS } from "../gameData";
import { decisionLabel, decisionTitle, decisionType, formatCountdown, formatShare, memorandumRule, memorandumTitle } from "../gameText";
import { ChatPanel } from "../components/ChatPanel";
import { DecisionList, DecisionTypeTag } from "../components/DecisionWidgets";
import { ComplianceWatchPanel, GovernanceProposalPhase, GovernanceVotingPhase, MoleObjectiveSelectionPhase } from "../components/PhaseWidgets";
import { UserAvatar } from "../components/UserAvatar";

export function StartedGameScreen(props: {
  state: PublicGameState;
  me?: PublicPlayerState;
  players: PublicPlayerState[];
  phase: GamePhase;
  acceptedDecisions: string[];
  roundReports: PublicRoundReport[];
  governanceProposals: PublicGovernanceProposal[];
  governanceSubmissions: PublicGovernanceSubmission[];
  governanceReports: PublicGovernanceReport[];
  chatMessages: PublicChatMessage[];
  availableDecisions: string[];
  majorVoteOptions: string[];
  decisionTypes: Record<string, DecisionType>;
  moleTargets: string[];
  moleSabotage: string;
  memorandum: PublicGameState["memorandum"] | null;
  memorandumPreference?: MemorandumType;
  moleVictoryPoints?: number;
  playersVictoryPoints?: number;
  currentVotes: PublicVoteState[];
  hasVoted: boolean;
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  canSelectMoleObjectives: boolean;
  canChooseMemorandum: boolean;
  canPlaceComplianceWatch: boolean;
  canSubmitGovernanceProposal: boolean;
  canSkipGovernanceProposal: boolean;
  canSendChatMessage: boolean;
  isSubmitting: boolean;
  onSelectMoleObjectives: (payload: Record<string, unknown>) => void;
  onChooseMemorandum: (type: MemorandumType) => void;
  onPlaceComplianceWatch: (targetUserId: number) => void;
  onVote: (decision: string) => void;
  onVoteProposal: (proposalId: number) => void;
  onAbstain: () => void;
  onSubmitGovernanceProposal: (payload: Record<string, unknown>) => void;
  onSkipGovernanceProposal: () => void;
  onSendChatMessage: (message: string) => Promise<void>;
  onReactChatMessage: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
  onRefresh: () => Promise<void>;
  isLoading: boolean;
  currentUserId: number;
}) {
  const [selectedReport, setSelectedReport] = useState<PublicRoundReport | null>(null);
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");
  const directorRowRefs = useRef(new Map<number, HTMLDivElement>());
  const previousDirectorRects = useRef(new Map<number, DOMRect>());
  const sortedPlayers = useMemo(
      () =>
          [...props.players].sort((left, right) => {
            if (right.share_bps !== left.share_bps) {
              return right.share_bps - left.share_bps;
            }
            const byName = left.name.localeCompare(right.name);
            return byName !== 0 ? byName : left.user_id - right.user_id;
          }),
      [props.players],
  );
  const playerSortSignature = sortedPlayers.map((player) => `${player.user_id}:${player.share_bps}`).join("|");
  useLayoutEffect(() => {
    const nextRects = new Map<number, DOMRect>();
    directorRowRefs.current.forEach((node, userId) => {
      nextRects.set(userId, node.getBoundingClientRect());
    });
    nextRects.forEach((nextRect, userId) => {
      const previousRect = previousDirectorRects.current.get(userId);
      const node = directorRowRefs.current.get(userId);
      if (!previousRect || !node) {
        return;
      }
      const deltaY = previousRect.top - nextRect.top;
      if (Math.abs(deltaY) > 1) {
        node.animate(
            [{ transform: `translateY(${deltaY}px)` }, { transform: "translateY(0)" }],
            { duration: 240, easing: "cubic-bezier(0.22, 1, 0.36, 1)" },
        );
      }
    });
    previousDirectorRects.current = nextRects;
  }, [playerSortSignature]);
  const isWaitingForPlayer = (userId: number) => {
    if (props.phase === "governance_proposal") {
      return !props.governanceSubmissions.some((item) => item.user_id === userId && item.status);
    }
    return !props.currentVotes.some((item) => item.user_id === userId && item.has_voted);
  };
  const displayedMajorOptions = props.majorVoteOptions.length ? props.majorVoteOptions : props.availableDecisions;
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  const majorUnlockMs = props.state.major_vote_unlocked_at ? new Date(props.state.major_vote_unlocked_at).getTime() : 0;
  const majorVoteLocked = props.phase === "major_voting" && majorUnlockMs > nowMs;
  const majorVoteSecondsLeft = Math.max(0, Math.ceil((majorUnlockMs - nowMs) / 1000));
  const phaseStartedMs = props.state.phase_started_at ? new Date(props.state.phase_started_at).getTime() : 0;
  const phaseDeadlineMs = props.state.phase_deadline_at ? new Date(props.state.phase_deadline_at).getTime() : 0;
  const phaseSecondsLeft = Math.max(0, Math.ceil((phaseDeadlineMs - nowMs) / 1000));
  const botReadyMs = props.players.some((player) => player.is_bot) && phaseStartedMs ? phaseStartedMs + BOT_ACTION_DELAY_MS : 0;
  const botDelaySecondsLeft = Math.max(0, Math.ceil((botReadyMs - nowMs) / 1000));
  const canSubmitMajorVote = props.canVote && !majorVoteLocked;
  const scoreRevealed =
      typeof props.moleVictoryPoints === "number" && typeof props.playersVictoryPoints === "number";

  return (
      <section className="game-stage">
        <div className="play-columns">
          <aside className="side-stack">

            <section className="directors-panel">
              <div className="director-list">
                <div className="director-row company-row">
                  <div className="director-identity">
                    <UserAvatar name="Компания" size="small" />
                    <div>
                      <strong>{props.state.company_name || "Компания"}</strong>
                      <span>Казначейский резерв {formatShare(props.state.treasury_share_bps)}</span>
                    </div>
                  </div>
                </div>
                {sortedPlayers.map((player) => (
                    <div
                        key={player.user_id}
                        ref={(node) => {
                          if (node) {
                            directorRowRefs.current.set(player.user_id, node);
                          } else {
                            directorRowRefs.current.delete(player.user_id);
                          }
                        }}
                        className={player.user_id === props.currentUserId ? "director-row is-current" : "director-row"}
                    >
                      <button className="director-identity profile-link" type="button" onClick={() => props.onOpenProfile(player.user_id)} disabled={player.is_bot}>
                        <UserAvatar name={player.name} avatarUrl={player.avatar_url} size="small" />
                        <div>
                          <strong>
                            {player.name}
                            {isWaitingForPlayer(player.user_id) ? (
                                <span className="pending-vote" aria-label="ожидаем голос">
                            ⌛
                          </span>
                            ) : null}
                          </strong>
                          {player.company_position ? <span>{player.company_position}</span> : null}
                          <span>
                        Доля {formatShare(player.share_bps)}
                      </span>
                          <span>
                        Полномочия {formatShare(player.authority_bps)}
                        </span>
                        </div>
                      </button>
                      <div className="badge-row">
                        {player.is_ceo ? <span className="badge accent">CEO</span> : null}
                        {player.is_bot ? <span className="badge bot">Bot</span> : null}
                      </div>
                    </div>
                ))}
              </div>
            </section>

            {props.me?.role === "mole" ? (
                <section className="secret-card">
                  <p className="eyebrow">Подкопы</p>
                  <DecisionList values={props.moleTargets} emptyText="Цели еще не выбраны." />
                  {props.moleSabotage ? (
                      <>
                        <p className="eyebrow">Диверсия</p>
                        <div className="sabotage-secret">
                          <strong>{decisionLabel(props.moleSabotage)}</strong>
                        </div>
                      </>
                  ) : null}
                  <p className="eyebrow">Счёт</p>
                  <div className="score-row">
                    <span>Крот: {props.moleVictoryPoints ?? 0}/3</span>
                    <span>Совет: {props.playersVictoryPoints ?? 0}/3</span>
                  </div>
                </section>
            ) : (
                <section className="secret-card">
                  {props.me?.role === "compliance" ? (
                      <>
                        <p className="eyebrow">Комплаенс</p>
                        <p className="quiet-text">
                          {props.memorandum
                              ? "Диверсия уже прошла, поэтому наблюдение больше недоступно. Теперь ваша приватная информация - меморандум выбранного типа."
                              : "Каждый мажорный раунд вы можете тайно выбрать одного игрока под наблюдение и менять выбор до закрытия голосования. Если выбранный игрок окажется Кротом и лично проголосует за принятую Диверсию, Совет директоров немедленно победит независимо от текущего счёта."}
                        </p>
                      </>
                  ) : null}
                  {props.me?.role === "player" || props.me?.role === "compliance" ? (
                    <>
                    <p className="eyebrow">{props.me?.role === "compliance" && props.memorandum ? "Меморандум после Диверсии" : "Меморандум"}</p>
                    {props.memorandum ? (
                        <>
                          <h3>{memorandumTitle(props.memorandum.type)}</h3>
                          <p className="quiet-text">{memorandumRule(props.memorandum.type, props.memorandum.variant)}</p>
                          <DecisionList values={props.memorandum.decisions} emptyText="Меморандум еще не получен." />
                        </>
                    ) : (
                        <p className="quiet-text">
                          {props.memorandumPreference
                              ? props.me?.role === "compliance"
                                  ? `Выбран профиль: ${memorandumTitle(props.memorandumPreference)}. Комплаенс получит меморандум только если Диверсия пройдет и партия продолжится.`
                                  : `Выбран профиль: ${memorandumTitle(props.memorandumPreference)}.`
                              : props.me?.role === "compliance"
                                  ? "Выберите тип подсказки заранее: он будет использован только если Диверсия пройдет и партия продолжится."
                                  : "Выбери тип меморандума, пока крот формирует цели."}
                        </p>
                    )}
                    </>
                  ) : null}
                  {scoreRevealed ? (
                      <>
                        <p className="eyebrow">Счёт</p>
                        <div className="score-row">
                          <span>Крот: {props.moleVictoryPoints}/3</span>
                          <span>Совет: {props.playersVictoryPoints}/3</span>
                        </div>
                      </>
                  ) : null}
                </section>
            )}

          </aside>

          <div className="main-stack">
            {phaseDeadlineMs && phaseSecondsLeft <= 90 ? (
                <div className={phaseSecondsLeft <= 30 ? "phase-timer urgent" : "phase-timer"}>
                  <span>Ожидаем вашего решения</span>
                  <strong>{formatCountdown(phaseSecondsLeft)}</strong>
                </div>
            ) : null}

            {props.phase === "mole_objective_selection" ? (
                <MoleObjectiveSelectionPhase
                    isMole={props.me?.role === "mole"}
                    viewerRole={props.me?.role}
                    canSelect={props.canSelectMoleObjectives}
                    canChooseMemorandum={props.canChooseMemorandum}
                    memorandumPreference={props.memorandumPreference}
                    isSubmitting={props.isSubmitting}
                    onSubmit={props.onSelectMoleObjectives}
                    onChooseMemorandum={props.onChooseMemorandum}
                />
            ) : props.phase === "governance_proposal" ? (
                <GovernanceProposalPhase
                    players={props.players}
                    submissions={props.governanceSubmissions}
                    treasuryShareBPS={props.state.treasury_share_bps}
                    currentUserId={props.currentUserId}
                    canSubmit={props.canSubmitGovernanceProposal}
                    canSkip={props.canSkipGovernanceProposal}
                    isSubmitting={props.isSubmitting}
                    onSubmit={props.onSubmitGovernanceProposal}
                    onSkip={props.onSkipGovernanceProposal}
                />
            ) : props.phase === "governance_voting" ? (
                <GovernanceVotingPhase
                    players={props.players}
                    proposals={props.governanceProposals}
                    currentVotes={props.currentVotes}
                    myCurrentVote={props.myCurrentVote}
                    canVote={props.canVote}
                    hasVoted={props.hasVoted}
                    isSubmitting={props.isSubmitting}
                    isCEO={Boolean(props.me?.is_ceo)}
                    onVote={props.onVoteProposal}
                    onAbstain={props.onAbstain}
                />
            ) : (
                <>
                {props.me?.role === "compliance" ? (
                      <ComplianceWatchPanel
                          players={props.players}
                          currentUserId={props.currentUserId}
                          watch={props.state.compliance_watch ?? null}
                          canPlace={props.canPlaceComplianceWatch}
                          isSubmitting={props.isSubmitting}
                          onPlace={props.onPlaceComplianceWatch}
                      />
                  ) : null}
                <section className="voting-board">
                  <div className="section-heading compact-heading">
                    <div>
                      <p className="eyebrow">голосование</p>
                    </div>
                    {majorVoteLocked ? <span className="wait-pill">Обсуждение: {majorVoteSecondsLeft}с</span> : props.hasVoted ? <span className="wait-pill">Выбор сохранён, можно изменить</span> : null}
                  </div>

                  <div className="decision-grid">
                    {displayedMajorOptions.map((decision) => {
                      const isMoleTarget = props.me?.role === "mole" && props.moleTargets.includes(decision);
                      const isMoleSabotage = props.me?.role === "mole" && props.moleSabotage === decision;
                      const isSelected = props.myCurrentVote?.decision === decision;
                      const type = decisionType(decision, props.decisionTypes);
                      return (
                          <button
                              type="button"
                              className={["decision-card", "decision-card-button", type, isMoleTarget ? "mole-target" : "", isMoleSabotage ? "mole-sabotage" : "", isSelected ? "selected-vote" : ""]
                                  .filter(Boolean)
                                  .join(" ")}
                              key={decision}
                              onClick={() => props.onVote(decision)}
                              disabled={!canSubmitMajorVote || props.isSubmitting}
                          >
                            <span>{isMoleSabotage ? "Диверсия" : isMoleTarget ? "Подкоп" : "Решение"}</span>
                            <strong>{decisionTitle(decision)}</strong>
                            <div className="decision-meta">
                              <small className="decision-letter">{decision}</small>
                              <DecisionTypeTag type={type} />
                            </div>
                          </button>
                      );
                    })}
                  </div>
                </section>
                </>
            )}

            <ChatPanel
                messages={props.chatMessages}
                players={props.players}
                currentUserId={props.currentUserId}
                canSend={props.canSendChatMessage}
                isSubmitting={props.isSubmitting}
                onSend={props.onSendChatMessage}
                onReact={props.onReactChatMessage}
                onOpenProfile={props.onOpenProfile}
            />
          </div>
        </div>
      </section>
  );
}
