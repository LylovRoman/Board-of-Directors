import { useEffect } from "react";

interface ToastProps {
  message: string | null;
  tone?: "error" | "success";
  onClose: () => void;
}

export function Toast({ message, tone = "error", onClose }: ToastProps) {
  useEffect(() => {
    if (!message) {
      return undefined;
    }
    const id = window.setTimeout(onClose, 4200);
    return () => window.clearTimeout(id);
  }, [message, onClose]);

  if (!message) {
    return null;
  }

  return (
    <div className={`toast toast-${tone}`} role="status">
      <span>{message}</span>
      <button type="button" onClick={onClose}>
        OK
      </button>
    </div>
  );
}
