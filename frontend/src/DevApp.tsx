import { useCallback, useEffect, useState, type ReactNode } from "react";
import {
  API_BASE_URL,
  createGame,
  getMe,
  getGameState,
  listGames,
  login,
  register,
  sendGameAction,
} from "./api";
import {
  AUTH_SESSION_CLEARED_EVENT,
  clearAuthSession,
  getAuthToken,
  readStoredAuthSession,
  saveAuthSession,
  saveAuthUser,
} from "./authSession";
import type { ActionType, AuthUser, Game, PublicGameState } from "./types";
import { normalizeStringArray, normalizeVotes } from "./types";

type AuthMode = "login" | "register";

export default function DevApp() {
  const [authSession, setAuthSession] = useState(() => readStoredAuthSession());
  const [isAuthChecking, setIsAuthChecking] = useState(() => Boolean(getAuthToken()));
  const [games, setGames] = useState<Game[]>([]);
  const [selectedGameId, setSelectedGameId] = useState<number | null>(null);
  const [gameState, setGameState] = useState<PublicGameState | null>(null);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [authLogin, setAuthLogin] = useState("");
  const [authName, setAuthName] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [gameTitleInput, setGameTitleInput] = useState("");
  const [isLoadingGames, setIsLoadingGames] = useState(false);
  const [isLoadingGameState, setIsLoadingGameState] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const currentUser: AuthUser | null = authSession?.user ?? null;
  const currentUserId = currentUser?.id ?? null;

  const clearMessages = useCallback(() => {
    setErrorMessage(null);
    setSuccessMessage(null);
  }, []);

  const handleError = useCallback((error: unknown) => {
    const message = error instanceof Error ? error.message : "Неизвестная ошибка";
    setErrorMessage(message);
    setSuccessMessage(null);
  }, []);

  const loadGames = useCallback(async () => {
    setIsLoadingGames(true);
    try {
      const nextGames = await listGames();
      setGames(nextGames);
    } catch (error) {
      handleError(error);
    } finally {
      setIsLoadingGames(false);
    }
  }, [handleError]);

  const loadGameState = useCallback(
    async (gameId: number) => {
      setIsLoadingGameState(true);
      try {
        const state = await getGameState(gameId);
        setGameState(state);
      } catch (error) {
        handleError(error);
      } finally {
        setIsLoadingGameState(false);
      }
    },
    [handleError],
  );

  useEffect(() => {
    let canceled = false;

    async function validateStoredSession() {
      if (!authSession) {
        setIsAuthChecking(false);
        return;
      }

      try {
        const user = await getMe();
        if (canceled) {
          return;
        }
        saveAuthUser(user);
        setAuthSession((session) => (session ? { ...session, user } : null));
      } catch {
        if (!canceled) {
          setAuthSession(null);
        }
      } finally {
        if (!canceled) {
          setIsAuthChecking(false);
        }
      }
    }

    void validateStoredSession();

    return () => {
      canceled = true;
    };
  }, []);

  useEffect(() => {
    const handleSessionCleared = () => {
      setAuthSession(null);
      setSelectedGameId(null);
      setGameState(null);
      setGames([]);
    };

    window.addEventListener(AUTH_SESSION_CLEARED_EVENT, handleSessionCleared);
    return () => window.removeEventListener(AUTH_SESSION_CLEARED_EVENT, handleSessionCleared);
  }, []);

  useEffect(() => {
    if (!isAuthChecking && currentUserId) {
      void loadGames();
    }
  }, [currentUserId, isAuthChecking, loadGames]);

  useEffect(() => {
    if (selectedGameId !== null && currentUserId !== null) {
      void loadGameState(selectedGameId);
    }
  }, [selectedGameId, currentUserId, loadGameState]);

  useEffect(() => {
    if (!autoRefresh || selectedGameId === null || currentUserId === null) {
      return undefined;
    }

    const refreshSelectedGame = () => {
      if (document.visibilityState === "visible") {
        void loadGameState(selectedGameId);
      }
    };

    const intervalId = window.setInterval(refreshSelectedGame, 10000);
    document.addEventListener("visibilitychange", refreshSelectedGame);

    return () => {
      window.clearInterval(intervalId);
      document.removeEventListener("visibilitychange", refreshSelectedGame);
    };
  }, [autoRefresh, currentUserId, loadGameState, selectedGameId]);

  async function handleAuthSubmit(event: React.FormEvent) {
    event.preventDefault();
    const normalizedLogin = authLogin.trim();
    const normalizedName = authName.trim();
    if (!normalizedLogin || !authPassword) {
      setErrorMessage("Enter login and password");
      return;
    }
    if (authMode === "register" && !normalizedName) {
      setErrorMessage("Enter display name");
      return;
    }

    setIsSubmitting(true);
    clearMessages();
    try {
      const response = authMode === "login"
        ? await login({ login: normalizedLogin, password: authPassword })
        : await register({ login: normalizedLogin, name: normalizedName, password: authPassword });
      saveAuthSession(response);
      setAuthSession(response);
      setAuthPassword("");
      setSuccessMessage("Authenticated as " + response.user.name);
      await loadGames();
    } catch (error) {
      handleError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateGame(event: React.FormEvent) {
    event.preventDefault();
    if (!currentUserId) {
      setErrorMessage("Сначала выбери текущего пользователя");
      return;
    }
    if (!gameTitleInput.trim()) {
      setErrorMessage("Введите название игры");
      return;
    }
    setIsSubmitting(true);
    clearMessages();
    try {
      const response = await createGame({ title: gameTitleInput.trim() });
      setGameTitleInput("");
      setSelectedGameId(response.game.id);
      setGameState(response.state);
      setSuccessMessage(`Игра ${response.game.title} создана`);
      await loadGames();
    } catch (error) {
      handleError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleAction(type: ActionType, payload?: Record<string, unknown>) {
    if (!selectedGameId || !currentUserId) {
      setErrorMessage("Выбери пользователя и игру");
      return;
    }
    setIsSubmitting(true);
    clearMessages();
    try {
      const response = await sendGameAction(selectedGameId, {
        type,
        payload,
      });
      if (response.state) {
        setGameState(response.state);
      } else {
        await loadGameState(selectedGameId);
      }
      setSuccessMessage(`Действие ${type} отправлено`);
    } catch (error) {
      handleError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleManualRefresh() {
    if (!selectedGameId || !currentUserId) {
      setErrorMessage("Choose user and game");
      return;
    }
    clearMessages();
    await loadGameState(selectedGameId);
  }

  function handleLogout() {
    clearAuthSession(false);
    setAuthSession(null);
    setSelectedGameId(null);
    setGameState(null);
    setGames([]);
    setAuthPassword("");
    clearMessages();
  }

  const availableDecisions = normalizeStringArray(gameState?.available_decisions);
  const acceptedDecisions = normalizeStringArray(gameState?.accepted_decisions);
  const rejectedDecisions = normalizeStringArray(gameState?.rejected_decisions);
  const currentVotes = normalizeVotes(gameState?.current_votes);
  const availableActions = gameState?.available_actions ?? [];

  if (!currentUser) {
    return (
      <div className="app-shell dev-shell">
        <header className="page-header">
          <div>
            <h1>Board of Directors Dev Frontend</h1>
            <p className="muted">
              API: <code>{API_BASE_URL}</code> · <a href="/play">Player UI</a>
            </p>
          </div>
        </header>
        {errorMessage ? <div className="banner error">{errorMessage}</div> : null}
        <section className="panel auth-panel">
          <div className="panel-header">
            <h2>{authMode === "login" ? "Login" : "Register"}</h2>
            <div className="inline-actions">
              <button type="button" onClick={() => setAuthMode("login")} disabled={authMode === "login"}>Login</button>
              <button type="button" onClick={() => setAuthMode("register")} disabled={authMode === "register"}>Register</button>
            </div>
          </div>
          <form className="stack" onSubmit={handleAuthSubmit}>
            <input value={authLogin} onChange={(event) => setAuthLogin(event.target.value)} placeholder="login" autoComplete="username" />
            {authMode === "register" ? <input value={authName} onChange={(event) => setAuthName(event.target.value)} placeholder="name" autoComplete="name" /> : null}
            <input value={authPassword} onChange={(event) => setAuthPassword(event.target.value)} placeholder="password" type="password" autoComplete={authMode === "login" ? "current-password" : "new-password"} />
            <button type="submit" disabled={isSubmitting || isAuthChecking}>{authMode === "login" ? "Login" : "Register"}</button>
          </form>
        </section>
      </div>
    );
  }

  return (
    <div className="app-shell dev-shell">
      <header className="page-header">
        <div>
          <h1>Board of Directors Dev Frontend</h1>
          <p className="muted">
            API: <code>{API_BASE_URL}</code> · <a href="/play">Игровой интерфейс</a>
          </p>
        </div>
        <div className="current-user-badge">
          Текущий пользователь:{" "}
          <strong>{currentUser ? `${currentUser.name} (#${currentUser.id})` : "не выбран"}</strong>
        </div>
      </header>

      {errorMessage ? <div className="banner error">{errorMessage}</div> : null}
      {successMessage ? <div className="banner success">{successMessage}</div> : null}

      <div className="grid">
        <section className="panel">
          <div className="panel-header">
            <h2>Auth</h2>
            <button onClick={handleLogout}>Logout</button>
          </div>
          <div className="stack">
            <InfoRow label="User" value={currentUser.name + " (#" + currentUser.id + ")"} />
            <InfoRow label="Login" value={currentUser.login} />
          </div>
          <p className="muted">Dev actions use the current JWT user. Use another browser profile to test another player.</p>
        </section>

        <section className="panel">
          <div className="panel-header">
            <h2>Игры</h2>
            <button onClick={() => void loadGames()} disabled={isLoadingGames || isSubmitting}>
              Обновить
            </button>
          </div>
          <form className="stack" onSubmit={handleCreateGame}>
            <input
              value={gameTitleInput}
              onChange={(event) => setGameTitleInput(event.target.value)}
              placeholder="Название игры"
            />
            <button type="submit" disabled={isSubmitting || !currentUserId}>
              Создать игру от текущего пользователя
            </button>
          </form>
          <div className="list">
            {games.map((game) => (
              <button
                key={game.id}
                className={game.id === selectedGameId ? "list-item selected" : "list-item"}
                onClick={() => {
                  clearMessages();
                  setSelectedGameId(game.id);
                }}
              >
                <span>{game.title}</span>
                <span className="muted">#{game.id}</span>
              </button>
            ))}
            {!games.length && <p className="muted">Игр пока нет.</p>}
          </div>
        </section>
      </div>

      <section className="panel game-panel">
        <div className="panel-header">
          <h2>Партия</h2>
          <div className="inline-actions">
            <label className="checkbox">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(event) => setAutoRefresh(event.target.checked)}
              />
              Auto-refresh 10s
            </label>
            <button
              onClick={() => void handleManualRefresh()}
              disabled={!selectedGameId || !currentUserId || isLoadingGameState}
            >
              Обновить состояние
            </button>
          </div>
        </div>

        {!selectedGameId ? (
          <p className="muted">Выбери игру из списка, чтобы увидеть состояние партии.</p>
        ) : (
          <>
            {isLoadingGameState && !gameState ? <p className="muted">Загрузка состояния...</p> : null}

            {gameState ? (
              <div className="game-layout">
                <div className="info-grid">
                  <InfoRow label="Game ID" value={String(gameState.game_id)} />
                  <InfoRow label="Title" value={gameState.title} />
                  <InfoRow label="Status" value={gameState.status} />
                  <InfoRow label="Current round" value={String(gameState.current_round)} />
                  <InfoRow label="Treasury share bps" value={String(gameState.treasury_share_bps)} />
                  <InfoRow label="Finished" value={gameState.is_finished ? "yes" : "no"} />
                  <InfoRow label="Winner" value={gameState.winner || "-"} />
                  <InfoRow label="My role" value={gameState.me?.role || "-"} />
                </div>

                <Block title="Available actions">
                  <TagList values={availableActions} emptyText="Нет доступных действий" />
                </Block>

                <div className="action-groups">
                  {availableActions.includes("join_game") ? (
                    <button disabled={isSubmitting || !currentUserId} onClick={() => void handleAction("join_game")}>
                      join_game
                    </button>
                  ) : null}
                  {availableActions.includes("start_game") ? (
                    <button disabled={isSubmitting || !currentUserId} onClick={() => void handleAction("start_game")}>
                      start_game
                    </button>
                  ) : null}
                </div>

                {availableActions.includes("vote") ? (
                  <Block title="Голосование">
                    <div className="action-groups">
                      {availableDecisions.map((decision) => (
                        <button
                          key={decision}
                          disabled={isSubmitting}
                          onClick={() => void handleAction("vote", { decision, abstain: false })}
                        >
                          vote {decision}
                        </button>
                      ))}
                      <button disabled={isSubmitting} onClick={() => void handleAction("vote", { abstain: true })}>
                        abstain
                      </button>
                    </div>
                  </Block>
                ) : null}

                <Block title="Players">
                  <div className="table">
                    <div className="table-row table-head">
                      <span>Name</span>
                      <span>User ID</span>
                      <span>Share</span>
                      <span>Host</span>
                      <span>CEO</span>
                      <span>Action</span>
                    </div>
                    {gameState.players.map((player) => {
                      const canKick =
                        availableActions.includes("kick_player") &&
                        currentUserId !== null &&
                        player.user_id !== currentUserId;
                      return (
                        <div key={player.user_id} className="table-row">
                          <span>{player.name}</span>
                          <span>#{player.user_id}</span>
                          <span>{player.share_bps}</span>
                          <span>{player.is_host ? "yes" : "no"}</span>
                          <span>{player.is_ceo ? "yes" : "no"}</span>
                          <span>
                            {canKick ? (
                              <button
                                className="danger-button"
                                disabled={isSubmitting}
                                onClick={() => void handleAction("kick_player", { user_id: player.user_id })}
                              >
                                kick
                              </button>
                            ) : (
                              "-"
                            )}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </Block>

                <div className="columns">
                  <Block title="Available decisions">
                    <TagList values={availableDecisions} emptyText="Пусто" />
                  </Block>
                  <Block title="Accepted decisions">
                    <TagList values={acceptedDecisions} emptyText="Пусто" />
                  </Block>
                  <Block title="Rejected decisions">
                    <TagList values={rejectedDecisions} emptyText="Пусто" />
                  </Block>
                </div>

                <Block title="Current votes">
                  <div className="table">
                    <div className="table-row table-head">
                      <span>User ID</span>
                      <span>Has voted</span>
                    </div>
                    {currentVotes.map((vote) => (
                      <div key={vote.user_id} className="table-row">
                        <span>#{vote.user_id}</span>
                        <span>{vote.has_voted ? "yes" : "no"}</span>
                      </div>
                    ))}
                    {!currentVotes.length ? <p className="muted">Голоса отсутствуют.</p> : null}
                  </div>
                </Block>

                <Block title="Raw state JSON">
                  <pre className="raw-json">{JSON.stringify(gameState, null, 2)}</pre>
                </Block>
              </div>
            ) : (
              <p className="muted">Состояние партии еще не загружено.</p>
            )}
          </>
        )}
      </section>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="info-row">
      <span className="muted">{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Block(props: { title: string; children: ReactNode }) {
  return (
    <section className="block">
      <h3>{props.title}</h3>
      {props.children}
    </section>
  );
}

function TagList(props: { values: string[]; emptyText: string }) {
  if (!props.values.length) {
    return <p className="muted">{props.emptyText}</p>;
  }
  return (
    <div className="tag-list">
      {props.values.map((value) => (
        <span key={value} className="tag">
          {value}
        </span>
      ))}
    </div>
  );
}
