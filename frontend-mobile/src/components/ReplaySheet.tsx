import { bpsToPercent, proposalText } from "../gameText";
import type { PublicGameState, PublicReplayStep } from "../types";

interface ReplaySheetContentProps {
  state: PublicGameState;
  steps: PublicReplayStep[];
}

export function ReplaySheetContent({ state, steps }: ReplaySheetContentProps) {
  return (
    <div className="replay-list">
      {steps.map((step) => (
        <article className={`replay-step replay-${step.kind}`} key={step.id}>
          <span>{step.round ? `Раунд ${step.round}` : step.kind}</span>
          <strong>{step.title}</strong>
          <p>{step.proposal ? proposalText(step.proposal, state.players) : step.summary}</p>
          {step.votes?.length ? (
            <div className="vote-bars">
              {step.votes.map((vote) => (
                <div className="vote-bar" key={`${step.id}-${vote.label}`}>
                  <div>
                    <span>{vote.label}</span>
                    <strong>{bpsToPercent(vote.voting_power_bps ?? vote.share_bps)}</strong>
                  </div>
                  <small>{vote.voters.join(", ") || "нет голосов"}</small>
                </div>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  );
}
