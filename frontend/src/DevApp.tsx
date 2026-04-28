import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  API_BASE_URL,
  createGame,
  createUser,
  getGameState,
  listGames,
  listUsers,
  sendGameAction,
} from "./api";
import type { ActionType, Game, PublicGameState, User } from "./types";
import { normalizeStringArray, normalizeVotes } from "./types";

const CURRENT_USER_STORAGE_KEY = "board-of-directors-current-user-id";

function readStoredUserId(): number | null {
  const raw = window.localStorage.getItem(CURRENT_USER_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export default function DevApp() {
  const [users, setUsers] = useState<User[]>([]);
  const [games, setGames] = useState<Game[]>([]);
  const [currentUserId, setCurrentUserId] = useState<number | null>(() => readStoredUserId());
  const [selectedGameId, setSelectedGameId] = useState<number | null>(null);
  const [gameState, setGameState] = useState<PublicGameState | null>(null);
  const [userNameInput, setUserNameInput] = useState("");
  const [gameTitleInput, setGameTitleInput] = useState("");
  const [isLoadingUsers, setIsLoadingUsers] = useState(false);
  const [isLoadingGames, setIsLoadingGames] = useState(false);
  const [isLoadingGameState, setIsLoadingGameState] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const currentUser = useMemo(
    () => users.find((user) => user.id === currentUserId) ?? null,
    [users, currentUserId],
  );

  const clearMessages = useCallback(() => {
    setErrorMessage(null);
    setSuccessMessage(null);
  }, []);

  const handleError = useCallback((error: unknown) => {
    const message = error instanceof Error ? error.message : "Неизвестная ошибка";
    setErrorMessage(message);
    setSuccessMessage(null);
  }, []);

  const loadUsers = useCallback(async () => {
    setIsLoadingUsers(true);
    try {
      const nextUsers = await listUsers();
      setUsers(nextUsers);
      if (currentUserId !== null && !nextUsers.some((user) => user.id === currentUserId)) {
        setCurrentUserId(null);
        window.localStorage.removeItem(CURRENT_USER_STORAGE_KEY);
      }
    } catch (error) {
      handleError(error);
    } finally {
      setIsLoadingUsers(false);
    }
  }, [currentUserId, handleError]);

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
    async (gameId: number, viewerUserId: number) => {
      setIsLoadingGameState(true);
      try {
        const state = await getGameState(gameId, viewerUserId);
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
    void loadUsers();
    void loadGames();
  }, [loadGames, loadUsers]);

  useEffect(() => {
    if (currentUserId !== null) {
      window.localStorage.setItem(CURRENT_USER_STORAGE_KEY, String(currentUserId));
    }
  }, [currentUserId]);

  useEffect(() => {
    if (selectedGameId !== null && currentUserId !== null) {
      void loadGameState(selectedGameId, currentUserId);
    }
  }, [selectedGameId, currentUserId, loadGameState]);

  useEffect(() => {
    if (!autoRefresh || selectedGameId === null || currentUserId === null) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void loadGameState(selectedGameId, currentUserId);
    }, 2000);
    return () => window.clearInterval(intervalId);
  }, [autoRefresh, currentUserId, loadGameState, selectedGameId]);

  async function handleCreateUser(event: React.FormEvent) {
    event.preventDefault();
    if (!userNameInput.trim()) {
      setErrorMessage("Введите имя пользователя");
      return;
    }
    setIsSubmitting(true);
    clearMessages();
    try {
      const user = await createUser(userNameInput.trim());
      setUserNameInput("");
      setSuccessMessage(`Пользователь ${user.name} создан`);
      await loadUsers();
      setCurrentUserId(user.id);
    } catch (error) {
      handleError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreatePresetUsers() {
    setIsSubmitting(true);
    clearMessages();
    try {
      const existingNames = new Set(users.map((user) => user.name.toLowerCase()));
      for (const name of ["Alice", "Bob", "Carol"]) {
        if (!existingNames.has(name.toLowerCase())) {
          await createUser(name);
        }
      }
      setSuccessMessage("Тестовые пользователи Alice, Bob, Carol готовы");
      await loadUsers();
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
      const response = await createGame({
        title: gameTitleInput.trim(),
        host_user_id: currentUserId,
      });
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
        user_id: currentUserId,
        type,
        payload,
      });
      if (response.state) {
        setGameState(response.state);
      } else {
        await loadGameState(selectedGameId, currentUserId);
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
      setErrorMessage("Выбери пользователя и игру");
      return;
    }
    clearMessages();
    await loadGameState(selectedGameId, currentUserId);
  }

  const availableDecisions = normalizeStringArray(gameState?.available_decisions);
  const acceptedDecisions = normalizeStringArray(gameState?.accepted_decisions);
  const rejectedDecisions = normalizeStringArray(gameState?.rejected_decisions);
  const currentVotes = normalizeVotes(gameState?.current_votes);
  const availableActions = gameState?.available_actions ?? [];

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
            <h2>Пользователи</h2>
            <button onClick={() => void loadUsers()} disabled={isLoadingUsers || isSubmitting}>
              Обновить
            </button>
          </div>
          <form className="stack" onSubmit={handleCreateUser}>
            <input
              value={userNameInput}
              onChange={(event) => setUserNameInput(event.target.value)}
              placeholder="Имя пользователя"
            />
            <div className="inline-actions">
              <button type="submit" disabled={isSubmitting}>
                Создать пользователя
              </button>
              <button type="button" disabled={isSubmitting} onClick={() => void handleCreatePresetUsers()}>
                Создать Alice, Bob, Carol
              </button>
            </div>
          </form>
          <div className="list">
            {users.map((user) => (
              <button
                key={user.id}
                className={user.id === currentUserId ? "list-item selected" : "list-item"}
                onClick={() => {
                  clearMessages();
                  setCurrentUserId(user.id);
                }}
              >
                <span>{user.name}</span>
                <span className="muted">#{user.id}</span>
              </button>
            ))}
            {!users.length && <p className="muted">Пользователей пока нет.</p>}
          </div>
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
              Auto-refresh 2s
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
            <div className="inline-actions compact">
              <span className="muted">Смотреть как:</span>
              <select
                value={currentUserId ?? ""}
                onChange={(event) => setCurrentUserId(Number(event.target.value))}
              >
                <option value="" disabled>
                  Выбери пользователя
                </option>
                {users.map((user) => (
                  <option key={user.id} value={user.id}>
                    {user.name} (#{user.id})
                  </option>
                ))}
              </select>
            </div>

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
