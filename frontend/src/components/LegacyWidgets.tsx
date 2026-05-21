import type { PublicPlayerState, PublicRoundReport } from "../types";
import { decisionLabel, formatShare, formatVotesCount } from "../gameText";

export function PlayerSelect(props: {
  players: PublicPlayerState[];
  value: number;
  excludeUserIds?: number[];
  onChange: (userId: number) => void;
}) {
  const exclude = new Set(props.excludeUserIds ?? []);
  const options = props.players.filter((player) => !exclude.has(player.user_id));
  const value = options.some((player) => player.user_id === props.value) ? props.value : (options[0]?.user_id ?? props.value);
  return (
      <select value={value} onChange={(event) => props.onChange(Number(event.target.value))}>
        {options.map((player) => (
            <option key={player.user_id} value={player.user_id}>
              {player.name}
            </option>
        ))}
      </select>
  );
}

export function ShareInput(props: { value: string; onChange: (sharePercent: string) => void }) {
  return (
      <label>
        Доля, %
        <input
            type="text"
            inputMode="decimal"
            value={props.value}
            placeholder="например, 2.5"
            onChange={(event) => props.onChange(event.target.value)}
        />
      </label>
  );
}

export function HudItem({ label, value }: { label: string; value: string }) {
  return (
      <div className="hud-item">
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
  );
}

export function RoundReportList(props: {
  reports: PublicRoundReport[];
  emptyText: string;
  onSelect: (report: PublicRoundReport) => void;
}) {
  if (!props.reports.length) {
    return <p className="quiet-text">{props.emptyText}</p>;
  }

  return (
      <div className="decision-list interactive-list">
        {props.reports.map((report) => (
            <button key={`${report.outcome}-${report.round}`} onClick={() => props.onSelect(report)}>
              <strong>{report.decision ? decisionLabel(report.decision) : `Раунд ${report.round}`}</strong>
              <small>Раунд {report.round}</small>
            </button>
        ))}
      </div>
  );
}

export function RoundReportDetails(props: { report: PublicRoundReport | null; onClose: () => void }) {
  if (!props.report) {
    return null;
  }

  return (
      <aside className="round-report-details">
        <div className="round-report-header">
          <div>
            <p className="eyebrow">отчет раунда</p>
            <h3>
              Раунд {props.report.round}:{" "}
              {props.report.outcome === "accepted" && props.report.decision
                  ? `принято ${decisionLabel(props.report.decision)}`
                  : "решение не принято"}
            </h3>
          </div>
          <button className="mini-button" onClick={props.onClose}>
            Закрыть
          </button>
        </div>
        <div className="round-report-votes">
          {props.report.votes.map((vote) => (
              <div className="round-report-row" key={`${props.report?.round}-${vote.decision}`}>
                <div>
                  <span>{vote.abstain ? "Воздержались" : decisionLabel(vote.decision)}</span>
                  <small>{(vote.voters ?? []).map((voter) => voter.name).join(", ") || formatVotesCount(vote.voter_count)}</small>
                </div>
                <strong>{formatShare(vote.share_bps)}</strong>
              </div>
          ))}
          {!props.report.votes.length ? <p className="quiet-text">Подробных голосов для этого раунда нет.</p> : null}
        </div>
      </aside>
  );
}
