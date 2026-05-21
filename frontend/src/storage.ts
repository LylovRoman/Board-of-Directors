export function readStoredNumber(key: string): number | null {
  const raw = window.localStorage.getItem(key);
  if (!raw) {
    return null;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export function readStoredBoolean(key: string): boolean {
  return window.localStorage.getItem(key) === "true";
}
