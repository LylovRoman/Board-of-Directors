import type {
  AuthResponse,
  ChangePasswordRequest,
  CreateGameRequest,
  CreateGameResponse,
  GameActionRequest,
  GameActionResponse,
  GamesResponse,
  GameStateResponse,
  LeaderboardPeriod,
  LeaderboardResponse,
  LoginRequest,
  MeResponse,
  ProfileResponse,
  PublicGameState,
  RegisterRequest,
  UpdateProfileRequest,
  UpdateProfileResponse,
  User,
  UsersResponse,
} from "./types";
import { clearAuthSession, getAuthToken } from "./authSession";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8000";
export const WS_BASE_URL = API_BASE_URL.replace(/^http/i, (scheme: string) =>
  scheme.toLowerCase() === "https" ? "wss" : "ws",
);

interface RequestOptions {
  auth?: boolean;
}

async function request<T>(path: string, init?: RequestInit, options: RequestOptions = {}): Promise<T> {
  const shouldAuthorize = options.auth ?? true;
  const token = shouldAuthorize ? getAuthToken() : null;
  const headers = new Headers(init?.headers);
  headers.set("Content-Type", "application/json");
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      ...Object.fromEntries(headers.entries()),
    },
  });

  const text = await response.text();
  const data = text ? JSON.parse(text) : {};

  if (!response.ok) {
    if (response.status === 401 && shouldAuthorize) {
      clearAuthSession();
    }
    const message =
      typeof data?.error === "string"
        ? data.error
        : `HTTP ${response.status} ${response.statusText}`;
    throw new Error(message);
  }

  return data as T;
}

export async function login(input: LoginRequest): Promise<AuthResponse> {
  return request<AuthResponse>(
    "/auth/login",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
    { auth: false },
  );
}

export async function register(input: RegisterRequest): Promise<AuthResponse> {
  return request<AuthResponse>(
    "/auth/register",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
    { auth: false },
  );
}

export async function getMe(): Promise<AuthResponse["user"]> {
  const data = await request<MeResponse>("/auth/me");
  return data.user;
}

export async function getMyProfile(): Promise<ProfileResponse["profile"]> {
  const data = await request<ProfileResponse>("/users/me/profile");
  return data.profile;
}

export async function getUserProfile(userId: number): Promise<ProfileResponse["profile"]> {
  const data = await request<ProfileResponse>(`/users/${userId}/profile`);
  return data.profile;
}

export async function respectUser(userId: number): Promise<ProfileResponse["profile"]> {
  const data = await request<ProfileResponse>(`/users/${userId}/respect`, {
    method: "POST",
    body: JSON.stringify({}),
  });
  return data.profile;
}

export async function updateMyProfile(input: UpdateProfileRequest): Promise<AuthResponse["user"]> {
  const data = await request<UpdateProfileResponse>("/users/me/profile", {
    method: "PUT",
    body: JSON.stringify(input),
  });
  return data.user;
}

export async function changePassword(input: ChangePasswordRequest): Promise<void> {
  await request<{ status: string }>("/auth/password", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function listUsers(): Promise<User[]> {
  const data = await request<UsersResponse>("/users/");
  return data.users;
}

export async function listGames(): Promise<GamesResponse["games"]> {
  const data = await request<GamesResponse>("/games/");
  return data.games;
}

export async function getLeaderboard(period: LeaderboardPeriod = "week"): Promise<LeaderboardResponse> {
  return request<LeaderboardResponse>(`/leaderboard?period=${encodeURIComponent(period)}`);
}

export async function createGame(input: CreateGameRequest): Promise<CreateGameResponse> {
  return request<CreateGameResponse>("/games/", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getGameState(gameId: number): Promise<PublicGameState> {
  const data = await request<GameStateResponse>(`/games/${gameId}/state`);
  return data.state;
}

export async function sendGameAction(
  gameId: number,
  input: GameActionRequest,
): Promise<GameActionResponse> {
  return request<GameActionResponse>(`/games/${gameId}/actions`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export { API_BASE_URL };
