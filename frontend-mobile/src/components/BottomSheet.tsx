import { X } from "lucide-react";
import type { ReactNode } from "react";

interface BottomSheetProps {
  title: string;
  eyebrow?: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

export function BottomSheet({ title, eyebrow, open, onClose, children }: BottomSheetProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="sheet-layer" role="presentation">
      <button className="sheet-scrim" type="button" aria-label="Закрыть" onClick={onClose} />
      <section className="bottom-sheet" role="dialog" aria-modal="true" aria-labelledby="sheet-title">
        <header className="sheet-header">
          <div>
            {eyebrow ? <p className="eyebrow">{eyebrow}</p> : null}
            <h2 id="sheet-title">{title}</h2>
          </div>
          <button className="icon-button" type="button" aria-label="Закрыть" onClick={onClose}>
            <X size={20} />
          </button>
        </header>
        <div className="sheet-body">{children}</div>
      </section>
    </div>
  );
}
