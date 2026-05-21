export function Toast({ message, tone = "error", onClose }: { message: string | null; tone?: "error" | "success"; onClose: () => void }) {
  if (!message) {
    return null;
  }

  return (
      <div className={`toast toast-${tone}`} role="alert">
        <span>{message}</span>
        <button onClick={onClose}>Закрыть</button>
      </div>
  );
}
