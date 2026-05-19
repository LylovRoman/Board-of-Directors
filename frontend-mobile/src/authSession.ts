import type { AuthUser } from "./types";

export const AUTH_TOKEN_STORAGE_KEY = "board-of-directors-auth-token";
export const AUTH_USER_STORAGE_KEY = "board-of-directors-auth-user";
export const LEGACY_CURRENT_USER_STORAGE_KEY = "board-of-directors-current-user-id";
export const AUTH_SESSION_CLEARED_EVENT = "board-of-directors-auth-session-cleared";

export interface AuthSession {
  token: string;
  user: AuthUser;
}

function isAuthUser(value: unknown): value is AuthUser {
  if (!value || typeof value !== "object") {
    return false;
  }
  const user = value as Partial<AuthUser>;
  return typeof user.id === "number" && typeof user.login === "string" && typeof user.name === "string";
}

export function readStoredAuthSession(): AuthSession | null {
  window.localStorage.removeItem(LEGACY_CURRENT_USER_STORAGE_KEY);

  const token = window.localStorage.getItem(AUTH_TOKEN_STORAGE_KEY);
  const rawUser = window.localStorage.getItem(AUTH_USER_STORAGE_KEY);
  if (!token || !rawUser) {
    return null;
  }

  try {
    const user = JSON.parse(rawUser) as unknown;
    return isAuthUser(user) ? { token, user } : null;
  } catch {
    return null;
  }
}

export function getAuthToken(): string | null {
  return window.localStorage.getItem(AUTH_TOKEN_STORAGE_KEY);
}

export function saveAuthSession(session: AuthSession): void {
  window.localStorage.removeItem(LEGACY_CURRENT_USER_STORAGE_KEY);
  window.localStorage.setItem(AUTH_TOKEN_STORAGE_KEY, session.token);
  window.localStorage.setItem(AUTH_USER_STORAGE_KEY, JSON.stringify(session.user));
}

export function saveAuthUser(user: AuthUser): void {
  window.localStorage.setItem(AUTH_USER_STORAGE_KEY, JSON.stringify(user));
}

export function clearAuthSession(notify = true): void {
  window.localStorage.removeItem(AUTH_TOKEN_STORAGE_KEY);
  window.localStorage.removeItem(AUTH_USER_STORAGE_KEY);
  window.localStorage.removeItem(LEGACY_CURRENT_USER_STORAGE_KEY);

  if (notify) {
    window.dispatchEvent(new Event(AUTH_SESSION_CLEARED_EVENT));
  }
}
