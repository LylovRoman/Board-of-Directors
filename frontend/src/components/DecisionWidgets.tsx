import type { DecisionType } from "../types";
import { decisionLabel } from "../gameText";

export function DecisionList({ values, emptyText }: { values: string[]; emptyText: string }) {
  if (!values.length) {
    return <p className="quiet-text">{emptyText}</p>;
  }

  return (
      <div className="decision-list">
        {values.map((value, index) => (
            <span key={`${value}-${index}`}>{decisionLabel(value)}</span>
        ))}
      </div>
  );
}

export function DecisionTypeTag({ type }: { type: DecisionType }) {
  const isEmpowerment = type === "empowerment";
  return (
      <span
          className={isEmpowerment ? "decision-type-tag empowerment" : "decision-type-tag growth"}
          title={isEmpowerment ? "Победители получают +1% к полномочиям" : "Победители получают +1% к доле"}
      >
      +1%
    </span>
  );
}
