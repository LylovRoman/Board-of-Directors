import type {
  CreateGameRequest,
  CreateGameResponse,
  CreateUserResponse,
  GameActionRequest,
  GameActionResponse,
  GamesResponse,
  GameStateResponse,
  PublicGameState,
  User,
  UsersResponse,
} from "./types";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8000";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  const text = await response.text();
  const data = text ? JSON.parse(text) : {};

  if (!response.ok) {
    const message =
      typeof data?.error === "string"
        ? data.error
        : `HTTP ${response.status} ${response.statusText}`;
    throw new Error(message);
  }

  return data as T;
}

export async function listUsers(): Promise<User[]> {
  const data = await request<UsersResponse>("/users/");
  return data.users;
}

export async function createUser(name: string): Promise<User> {
  const data = await request<CreateUserResponse>("/users/", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  return data.user;
}

export async function listGames(): Promise<GamesResponse["games"]> {
  const data = await request<GamesResponse>("/games/");
  return data.games;
}

export async function createGame(input: CreateGameRequest): Promise<CreateGameResponse> {
  return request<CreateGameResponse>("/games/", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getGameState(gameId: number, viewerUserId: number): Promise<PublicGameState> {
  const data = await request<GameStateResponse>(
    `/games/${gameId}/state?viewer_user_id=${viewerUserId}`,
  );
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
