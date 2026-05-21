import { useMemo, useState } from "react";
import type { PublicChatMessage, PublicGameState, PublicGovernanceReport, PublicPlayerState, PublicRoundReport } from "../types";
import { decisionLabel, finalDecisionClass, formatAccuracy, roleLabel, winnerLabel } from "../gameText";
import { ChatPanel } from "../components/ChatPanel";
import { ReplayPanel } from "../components/ReplayPanel";

export function FinishScreen(props: {
  state: PublicGameState;
  me?: PublicPlayerState;
  acceptedDecisions: string[];
  roundReports: PublicRoundReport[];
  governanceReports: PublicGovernanceReport[];
  chatMessages: PublicChatMessage[];
  canSendChatMessage: boolean;
  currentUserId: number;
  isSubmitting: boolean;
  onSendChatMessage: (message: string) => Promise<void>;
  onReactChatMessage: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
  onRefresh: () => Promise<void>;
  onBack: () => void;
  isLoading: boolean;
}) {
  const [replayOpen, setReplayOpen] = useState(false);
  const playerWon =
      props.state.winner === "mole" ? props.me?.role === "mole" : props.state.winner === "players" && props.me?.role !== "mole";
  const summary = props.state.final_summary;
  const playerStats = [...(summary?.player_stats ?? [])].sort((left, right) => {
    if (left.mistakes !== right.mistakes) {
      return left.mistakes - right.mistakes;
    }
    return right.accuracy_bps - left.accuracy_bps;
  });
  const winners = playerStats.filter((stat) => stat.won);
  const leastMistakes = new Set(summary?.least_mistake_user_ids ?? []);
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");
  const riskyReports = props.roundReports.filter((report) => report.outcome !== "accepted");
  const myFinalStats = playerStats.find((stat) => stat.user_id === props.currentUserId);
  const myPlayerName =
      props.state.players?.find((player) => player.user_id === props.currentUserId)?.name ?? props.me?.name ?? "";
  const myRoundDecisions = props.roundReports.flatMap((report) => {
    const myVote = report.votes.find(
        (vote) => !vote.abstain && Boolean(vote.decision) && (vote.voters ?? []).some((voter) => voter.name === myPlayerName),
    );
    return myVote?.decision ? [{ round: report.round, decision: myVote.decision }] : [];
  });

  if (replayOpen) {
    return <ReplayPanel state={props.state} steps={props.state.replay_steps ?? []} onBack={() => setReplayOpen(false)} />;
  }

  const acceptedDecisionByRound = useMemo(
      () =>
          new Map(
              acceptedReports
                  .filter((report) => report.decision)
                  .map((report) => [report.round, report.decision]),
          ),
      [acceptedReports],
  );
  const winnerReason = summary?.winner_reason ?? props.state.winner_reason;
  const complianceVictory = winnerReason === "mole_caught_by_compliance";
  const complianceUserId = summary?.compliance_user_id;
  const complianceName = complianceUserId
      ? playerStats.find((stat) => stat.user_id === complianceUserId)?.name ??
      props.state.players?.find((player) => player.user_id === complianceUserId)?.name
      : "";

  return (
      <section className="finish-screen">
        <p className="eyebrow">финал</p>
        {props.me?.role ? (
            <p className="personal-result">
              {roleLabel(props.me.role)}: {playerWon ? "Ты победил" : "Ты проиграл"}
            </p>
        ) : null}
        <div className="final-score-row">
          <span>Крот {summary?.mole_points ?? props.state.mole_victory_points ?? 0}/3</span>
          <span>Совет {summary?.players_points ?? props.state.players_victory_points ?? 0}/3</span>
        </div>
        {complianceVictory ? (
            <section className="final-panel compliance-victory-panel">
              <p className="eyebrow">Саботаж раскрыт</p>
              <strong>Совет директоров побеждает</strong>
              <p>
                Крот был пойман Комплаенсом в момент попытки провести Диверсию
                {summary?.compliance_catch?.accepted_decision ? ` ${decisionLabel(summary.compliance_catch.accepted_decision)}` : ""}.
              </p>
            </section>
        ) : null}
        <div className="final-grid">
          {myFinalStats ? (
              <section className="final-panel xp-panel">
                <p className="eyebrow">Твоя награда</p>
                <strong>+{myFinalStats.xp_earned ?? 0} XP</strong>
                <div className="xp-breakdown">
                  {(myFinalStats.xp_breakdown ?? []).map((award) => (
                      <span key={`${award.reason}-${award.points}`}>{award.reason}: +{award.points}</span>
                  ))}
                </div>
              </section>
          ) : null}
          <section className="final-panel">
            <p className="eyebrow">победители</p>
            <div className="winner-list">
              {winners.map((winner) => (
                  <span key={winner.user_id}>{winner.name} · {roleLabel(winner.role)}</span>
              ))}
              {!winners.length ? <span>{winnerLabel(props.state.winner)}</span> : null}
            </div>
          </section>
          <section className="final-panel final-table-panel">
            <p className="eyebrow">точность голосов</p>
            <div className="final-stats-table">
              {playerStats.map((stat) => (
                  <div className={leastMistakes.has(stat.user_id) ? "final-stat-row best" : "final-stat-row"} key={stat.user_id}>
                    <span>{stat.name}</span>
                    <span>{stat.mistakes} ошибок</span>
                    <strong>{formatAccuracy(stat.accuracy_bps)}</strong>
                  </div>
              ))}
            </div>
          </section>
          <section className="final-panel">
            <p className="eyebrow">раскрытие</p>
            <p>Крот: {playerStats.find((stat) => stat.user_id === summary?.mole_user_id)?.name ?? "неизвестно"}</p>
            {complianceUserId ? <p>Комплаенс: {complianceName || "неизвестно"}</p> : null}
            <p style={{ whiteSpace: 'pre-line' }}>
              Цели: {(summary?.mole_targets ?? []).map(decisionLabel).join("\n") || "нет данных"}
            </p>
            <p>Диверсия: {summary?.mole_sabotage ? decisionLabel(summary.mole_sabotage) : "нет данных"}</p>
          </section>
          {myRoundDecisions.length ? (
              <section className="final-panel">
                <p className="eyebrow">Твои решения</p>
                <div className="final-decision-list">
                  {myRoundDecisions.map((item) => {
                    const matchedAcceptedDecision =
                        acceptedDecisionByRound.get(item.round) === item.decision;

                    return (
                        <span
                            key={`my-decision-${item.round}`}
                            className={matchedAcceptedDecision ? "my-decision accepted-match" : "my-decision"}
                        >
            Раунд {item.round}: {decisionLabel(item.decision)}
          </span>
                    );
                  })}
                </div>
              </section>
          ) : null}
          <section className="final-panel">
            <p className="eyebrow">принятые решения</p>
            <div className="final-decision-list">
              {acceptedReports.slice(-5).map((report, index) => (
                  <span className={finalDecisionClass(report.decision, summary)} key={`accepted-${report.round}`}>Решение {index + 1}: {report.decision ? decisionLabel(report.decision) : "принято"}</span>
              ))}
              {riskyReports.slice(-2).map((report, index) => (
                  <span key={`risky-${report.round}`}>Спорное решение {index + 1}: {report.reason ?? "ничья"}</span>
              ))}
            </div>
          </section>
        </div>
        <ChatPanel
            messages={props.chatMessages}
            players={props.state.players ?? []}
            currentUserId={props.currentUserId}
            canSend={props.canSendChatMessage}
            isSubmitting={props.isSubmitting}
            onSend={props.onSendChatMessage}
            onReact={props.onReactChatMessage}
            onOpenProfile={props.onOpenProfile}
        />
        <div className="toolbar-actions centered-actions">
          <button className="primary-action" onClick={() => setReplayOpen(true)}>
            Смотреть реплей
          </button>
          <button className="secondary-action" onClick={props.onBack}>
            К списку игр
          </button>
        </div>
      </section>
  );
}
