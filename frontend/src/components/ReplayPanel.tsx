import { useEffect, useState } from "react";
import type { PublicGameState } from "../types";
import { decisionLabel, formatShare, winnerLabel } from "../gameText";

export function ReplayPanel(props: { state: PublicGameState; steps: NonNullable<PublicGameState["replay_steps"]>; onBack: () => void }) {
  const steps = props.steps.length
      ? props.steps
      : [{ id: "final", kind: "final", title: "Финал", summary: winnerLabel(props.state.winner), winner: props.state.winner }];
  const [index, setIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState(1200);
  const step = steps[Math.min(index, steps.length - 1)];

  useEffect(() => {
    if (!isPlaying) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      setIndex((value) => {
        if (value >= steps.length - 1) {
          setIsPlaying(false);
          return value;
        }
        return value + 1;
      });
    }, speed);
    return () => window.clearInterval(intervalId);
  }, [isPlaying, speed, steps.length]);

  return (
      <section className="replay-screen">
        <div className="section-heading">
          <div>
            <p className="eyebrow">реплей</p>
            <h1>{props.state.title}</h1>
            {props.state.company_name ? <p className="quiet-text">{props.state.company_name}</p> : null}
          </div>
          <div className="toolbar-actions">
            <button className="secondary-action" onClick={props.onBack}>К финалу</button>
          </div>
        </div>
        <div className="replay-layout">
          <aside className="replay-timeline">
            {steps.map((item, itemIndex) => (
                <button key={item.id} className={itemIndex === index ? "replay-step active" : "replay-step"} onClick={() => setIndex(itemIndex)}>
                  <span>{itemIndex + 1}</span>
                  <strong>{item.title}</strong>
                </button>
            ))}
          </aside>
          <section className="replay-detail">
            <p className="eyebrow">{step.kind}</p>
            <h2>{step.title}</h2>
            <p>{step.summary}</p>
            {step.decision ? <p>Решение: {decisionLabel(step.decision)}</p> : null}
            {step.winner ? <p>Победитель: {winnerLabel(step.winner)}</p> : null}
            {step.winner_reason === "mole_caught_by_compliance" ? (
                <p>Совет победил, потому что Комплаенс поймал Крота на личной поддержке принятой Диверсии.</p>
            ) : null}
            {step.votes?.length ? (
                <div className="replay-votes">
                  {step.votes.map((vote) => (
                      <div className="round-report-row" key={`${step.id}-${vote.label}`}>
                        <div>
                          <span>{vote.label}</span>
                          <small>{vote.voters.join(", ") || "без голосов"}</small>
                        </div>
                        <strong>{formatShare(vote.voting_power_bps ?? vote.share_bps)}</strong>
                      </div>
                  ))}
                </div>
            ) : null}
          </section>
        </div>
        <div className="replay-controls">
          <button className="secondary-action" onClick={() => setIndex((value) => Math.max(0, value - 1))} disabled={index === 0}>Назад</button>
          <button className="primary-action" onClick={() => setIsPlaying((value) => !value)}>{isPlaying ? "Пауза" : "Play"}</button>
          <button className="secondary-action" onClick={() => setIndex((value) => Math.min(steps.length - 1, value + 1))} disabled={index === steps.length - 1}>Вперед</button>
          <select value={speed} onChange={(event) => setSpeed(Number(event.target.value))} aria-label="Скорость реплея">
            <option value={1800}>0.75x</option>
            <option value={1200}>1x</option>
            <option value={700}>1.5x</option>
          </select>
        </div>
      </section>
  );
}
