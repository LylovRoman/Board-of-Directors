import { useCallback, useEffect, useMemo, useState } from "react";
import {
  createGame,
  createUser,
  getGameState,
  listGames,
  listUsers,
  sendGameAction,
  API_BASE_URL,
} from "./api";
import type {
  ActionType,
  Game,
  GamePhase,
  GameStatus,
  GovernanceProposalType,
  PublicGameState,
  PublicGovernanceProposal,
  PublicGovernanceReport,
  PublicGovernanceSubmission,
  PublicOwnVoteState,
  PublicChatMessage,
  PublicPlayerState,
  PublicRoundReport,
  User,
} from "./types";
import {
  normalizeChatMessages,
  normalizeGovernanceProposals,
  normalizeGovernanceReports,
  normalizeGovernanceSubmissions,
  normalizeRoundReports,
  normalizeStringArray,
  normalizeVotes,
} from "./types";

const CURRENT_USER_STORAGE_KEY = "board-of-directors-current-user-id";
const SELECTED_GAME_STORAGE_KEY = "board-of-directors-selected-game-id";
const DECISION_TITLES: Record<string, string> = {
  A: "Враждебное поглощение",
  B: "Экспансия на новый рынок",
  C: "Выплата дивидендов по акциям",
  D: "Запуск экспериментального продукта",
  E: "Сделка слияния",
  F: "Оптимизация неэффективного персонала",
  G: "Агрессивная налоговая стратегия",
  H: "Обратный выкуп акций",
};

interface GameCard {
  game: Game;
  state?: PublicGameState;
}

function readStoredNumber(key: string): number | null {
  const raw = window.localStorage.getItem(key);
  if (!raw) {
    return null;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function statusLabel(status?: GameStatus): string {
  switch (status) {
    case "lobby":
      return "Ожидает игроков";
    case "started":
      return "Идет заседание";
    case "finished":
      return "Завершена";
    default:
      return "Статус уточняется";
  }
}

function phaseLabel(phase?: GamePhase): string {
  switch (phase) {
    case "governance_proposal":
    case "governance_voting":
      return "Корпоративные манёвры";
    case "major_voting":
      return "Мажорное голосование";
    default:
      return "Подготовка";
  }
}

function roleLabel(role?: string): string {
  return role === "mole" ? "Крот" : "Директор";
}

function winnerLabel(winner?: string): string {
  if (winner === "mole") {
    return "Крот победил";
  }
  if (winner === "players") {
    return "Совет директоров победил";
  }
  return "Игра завершена";
}

function formatShare(bps?: number): string {
  const value = typeof bps === "number" ? bps : 0;
  return `${(value / 100).toFixed(value % 100 === 0 ? 0 : 1)}%`;
}

function decisionTitle(decision: string): string {
  return DECISION_TITLES[decision] ?? decision;
}

function decisionLabel(decision: string): string {
  const title = decisionTitle(decision);
  return title === decision ? decision : `${decision} — ${title}`;
}

function percentToBps(value: string): number {
  const normalized = value.replace(",", ".").trim();
  const percent = Number.parseFloat(normalized);
  return Number.isFinite(percent) ? Math.round(percent * 100) : 0;
}

function formatChatTime(value: string): string {
  if (!value || value.startsWith("0001-")) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
}

function formatVotesCount(count: number): string {
  if (count % 10 === 1 && count % 100 !== 11) {
    return `${count} голос`;
  }
  if ([2, 3, 4].includes(count % 10) && ![12, 13, 14].includes(count % 100)) {
    return `${count} голоса`;
  }
  return `${count} голосов`;
}

function playerName(players: PublicPlayerState[], userId?: number): string {
  if (!userId) {
    return "игрок";
  }
  return players.find((player) => player.user_id === userId)?.name ?? `Игрок #${userId}`;
}

function describeGovernanceProposal(proposal: PublicGovernanceProposal, players: PublicPlayerState[]): string {
  switch (proposal.proposal_type) {
    case "share_transfer":
      return `${playerName(players, proposal.from_user_id)} передает ${formatShare(proposal.share_bps)} игроку ${playerName(
        players,
        proposal.to_user_id,
      )}`;
    case "treasury_grant":
      return `Выдать ${formatShare(proposal.share_bps)} из резерва игроку ${playerName(players, proposal.target_user_id)}`;
    case "treasury_buyback":
      return `Выкупить ${formatShare(proposal.share_bps)} у игрока ${playerName(players, proposal.target_user_id)} в резерв`;
    case "appoint_ceo":
      return `Назначить CEO: ${playerName(players, proposal.target_user_id)}`;
    default:
      return "Корпоративный манёвр";
  }
}

function governanceReportText(report: PublicGovernanceReport, players: PublicPlayerState[]): string {
  if (report.outcome === "accepted" && report.proposal) {
    return `Принято: ${describeGovernanceProposal(report.proposal, players)}`;
  }
  return "Манёвр не принят: следующий мажорный раунд начался без изменений";
}

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Неизвестная ошибка";
}

export default function PlayerApp() {
  const [users, setUsers] = useState<User[]>([]);
  const [games, setGames] = useState<Game[]>([]);
  const [gameCards, setGameCards] = useState<GameCard[]>([]);
  const [currentUserId, setCurrentUserId] = useState<number | null>(() =>
    readStoredNumber(CURRENT_USER_STORAGE_KEY),
  );
  const [selectedGameId, setSelectedGameId] = useState<number | null>(() =>
    readStoredNumber(SELECTED_GAME_STORAGE_KEY),
  );
  const [gameState, setGameState] = useState<PublicGameState | null>(null);
  const [playerName, setPlayerName] = useState("");
  const [newGameTitle, setNewGameTitle] = useState("");
  const [lobbyFilter, setLobbyFilter] = useState("");
  const [lobbyStatusFilter, setLobbyStatusFilter] = useState<GameStatus | "all">("all");
  const [onlyMyGames, setOnlyMyGames] = useState(false);
  const [isCreatingGame, setIsCreatingGame] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const currentUser = useMemo(
    () => users.find((user) => user.id === currentUserId) ?? null,
    [users, currentUserId],
  );
  const availableActions = gameState?.available_actions ?? [];
  const players = gameState?.players ?? [];
  const currentVotes = normalizeVotes(gameState?.current_votes);
  const acceptedDecisions = normalizeStringArray(gameState?.accepted_decisions);
  const availableDecisions = normalizeStringArray(gameState?.available_decisions);
  const roundReports = normalizeRoundReports(gameState?.round_reports);
  const chatMessages = normalizeChatMessages(gameState?.chat_messages);
  const governanceProposals = normalizeGovernanceProposals(gameState?.governance_proposals);
  const governanceSubmissions = normalizeGovernanceSubmissions(gameState?.governance_submissions);
  const governanceReports = normalizeGovernanceReports(gameState?.governance_reports);
  const moleTargets = normalizeStringArray(gameState?.mole_targets);
  const me = gameState?.me;
  const hasMe = Boolean(me && me.user_id === currentUserId);
  const hasVoted = currentVotes.some((vote) => vote.user_id === currentUserId && vote.has_voted);
  const canVote = availableActions.includes("vote");
  const canSubmitGovernanceProposal = availableActions.includes("submit_governance_proposal");
  const canSkipGovernanceProposal = availableActions.includes("skip_governance_proposal");
  const canJoin = availableActions.includes("join_game");
  const canStart = availableActions.includes("start_game");
  const canKick = availableActions.includes("kick_player");
  const canSendChatMessage = availableActions.includes("send_chat_message");
  const filteredGameCards = useMemo(() => {
    const normalizedFilter = lobbyFilter.trim().toLowerCase();
    return gameCards.filter(({ game, state }) => {
      const title = (state?.title ?? game.title).toLowerCase();
      const matchesText = !normalizedFilter || title.includes(normalizedFilter);
      const matchesStatus = lobbyStatusFilter === "all" || state?.status === lobbyStatusFilter;
      const matchesOwner =
        !onlyMyGames ||
        Boolean(currentUserId && state?.players?.some((player) => player.user_id === currentUserId));
      return matchesText && matchesStatus && matchesOwner;
    });
  }, [currentUserId, gameCards, lobbyFilter, lobbyStatusFilter, onlyMyGames]);

  const showError = useCallback((error: unknown) => {
    setErrorMessage(getErrorMessage(error));
  }, []);

  const loadUsers = useCallback(async () => {
    try {
      const nextUsers = await listUsers();
      setUsers(nextUsers);
      if (currentUserId !== null && !nextUsers.some((user) => user.id === currentUserId)) {
        setCurrentUserId(null);
        setSelectedGameId(null);
        window.localStorage.removeItem(CURRENT_USER_STORAGE_KEY);
        window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
      }
    } catch (error) {
      showError(error);
    }
  }, [currentUserId, showError]);

  const loadGames = useCallback(async () => {
    try {
      const nextGames = await listGames();
      setGames(nextGames);
      return nextGames;
    } catch (error) {
      showError(error);
      return [];
    }
  }, [showError]);

  const loadGameState = useCallback(
    async (gameId: number, viewerUserId: number) => {
      try {
        const state = await getGameState(gameId, viewerUserId);
        setGameState(state);
        return state;
      } catch (error) {
        showError(error);
        return null;
      }
    },
    [showError],
  );

  const loadGameCards = useCallback(
    async (sourceGames = games, viewerUserId = currentUserId) => {
      if (!viewerUserId) {
        setGameCards(sourceGames.map((game) => ({ game })));
        return;
      }

      const settled = await Promise.allSettled(
        sourceGames.map(async (game) => ({
          game,
          state: await getGameState(game.id, viewerUserId),
        })),
      );

      setGameCards(
        settled.map((result, index) =>
          result.status === "fulfilled" ? result.value : { game: sourceGames[index] },
        ),
      );
    },
    [currentUserId, games],
  );

  const refreshEverything = useCallback(async () => {
    setIsLoading(true);
    setErrorMessage(null);
    try {
      await loadUsers();
      const nextGames = await loadGames();
      await loadGameCards(nextGames, currentUserId);
      if (selectedGameId && currentUserId) {
        await loadGameState(selectedGameId, currentUserId);
      }
    } finally {
      setIsLoading(false);
    }
  }, [currentUserId, loadGameCards, loadGameState, loadGames, loadUsers, selectedGameId]);

  useEffect(() => {
    void refreshEverything();
  }, []);

  useEffect(() => {
    if (currentUserId !== null) {
      window.localStorage.setItem(CURRENT_USER_STORAGE_KEY, String(currentUserId));
    }
  }, [currentUserId]);

  useEffect(() => {
    if (selectedGameId !== null) {
      window.localStorage.setItem(SELECTED_GAME_STORAGE_KEY, String(selectedGameId));
    } else {
      window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
    }
  }, [selectedGameId]);

  useEffect(() => {
    if (!currentUserId) {
      return undefined;
    }

    const intervalId = window.setInterval(() => {
      void (async () => {
        const nextGames = await loadGames();
        await loadGameCards(nextGames, currentUserId);
        if (selectedGameId) {
          await loadGameState(selectedGameId, currentUserId);
        }
      })();
    }, 2000);

    return () => window.clearInterval(intervalId);
  }, [currentUserId, loadGameCards, loadGameState, loadGames, selectedGameId]);

  useEffect(() => {
    const isJoinedLobby =
      Boolean(selectedGameId && currentUserId) &&
      gameState?.status === "lobby" &&
      gameState.players?.some((player) => player.user_id === currentUserId) === true;
    if (!isJoinedLobby || !selectedGameId || !currentUserId) {
      return undefined;
    }

    const leaveLobby = () => {
      const body = JSON.stringify({
        user_id: currentUserId,
        type: "leave_game",
      });
      const url = `${API_BASE_URL}/games/${selectedGameId}/actions`;
      if (navigator.sendBeacon?.(url, body)) {
        return;
      }
      void fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        keepalive: true,
      });
    };

    window.addEventListener("pagehide", leaveLobby);
    return () => window.removeEventListener("pagehide", leaveLobby);
  }, [currentUserId, gameState, selectedGameId]);

  async function handleWelcomeSubmit(event: React.FormEvent) {
    event.preventDefault();
    const name = playerName.trim();
    if (!name) {
      setErrorMessage("Введите имя, чтобы войти в игру.");
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const nextUsers = await listUsers();
      const existing = nextUsers.find((user) => user.name.trim().toLowerCase() === name.toLowerCase());
      const user = existing ?? (await createUser(name));
      setUsers(existing ? nextUsers : [...nextUsers, user]);
      setCurrentUserId(user.id);
      setPlayerName(user.name);
      const nextGames = await loadGames();
      await loadGameCards(nextGames, user.id);
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateGame(event: React.FormEvent) {
    event.preventDefault();
    if (!currentUserId) {
      setErrorMessage("Сначала войдите под своим именем.");
      return;
    }

    const title = newGameTitle.trim() || `Заседание совета #${games.length + 1}`;
    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const response = await createGame({ title, host_user_id: currentUserId });
      setNewGameTitle("");
      setIsCreatingGame(false);
      setSelectedGameId(response.game.id);
      setGameState(response.state);
      const nextGames = await loadGames();
      await loadGameCards(nextGames, currentUserId);
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function openGame(gameId: number) {
    if (!currentUserId) {
      setErrorMessage("Сначала войдите под своим именем.");
      return;
    }
    setIsLoading(true);
    setErrorMessage(null);
    setSelectedGameId(gameId);
    await loadGameState(gameId, currentUserId);
    setIsLoading(false);
  }

  async function handleAction(type: ActionType, payload?: Record<string, unknown>) {
    if (!selectedGameId || !currentUserId) {
      setErrorMessage("Игра или игрок не выбраны.");
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
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
      const nextGames = await loadGames();
      await loadGameCards(nextGames, currentUserId);
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleManualRefresh() {
    await refreshEverything();
  }

  function handleLogout() {
    setCurrentUserId(null);
    setSelectedGameId(null);
    setGameState(null);
    setPlayerName("");
    window.localStorage.removeItem(CURRENT_USER_STORAGE_KEY);
    window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
  }

  if (!currentUserId || !currentUser) {
    return (
      <main className="play-shell welcome-screen">
        <div className="aurora" />
        <section className="welcome-hero">
          <p className="eyebrow">корпоративная тайная игра</p>
          <h1>Board of Directors</h1>
          <p className="welcome-copy">
            Войди в совет, голосуй за решения и попробуй понять, кто ведет компанию не туда.
          </p>
          <form className="welcome-form" onSubmit={handleWelcomeSubmit}>
            <label htmlFor="player-name">Твое имя</label>
            <input
              id="player-name"
              value={playerName}
              onChange={(event) => setPlayerName(event.target.value)}
              placeholder="Например, Alice"
              autoComplete="name"
            />
            <button type="submit" className="primary-action" disabled={isSubmitting}>
              Продолжить
            </button>
          </form>
        </section>
        <Toast message={errorMessage} onClose={() => setErrorMessage(null)} />
      </main>
    );
  }

  return (
    <main className="play-shell">
      <div className="aurora" />
      <header className="play-topbar">
        <button className="ghost-button" onClick={() => setSelectedGameId(null)}>
          Игры
        </button>
        <div className="brand-lockup">
          <span>Board of Directors</span>
          <small>тайное заседание</small>
        </div>
        <div className="player-chip">
          <span>{currentUser.name}</span>
          <button className="mini-button" onClick={handleLogout}>
            Сменить
          </button>
        </div>
      </header>

      <Toast message={errorMessage} onClose={() => setErrorMessage(null)} />

      {!selectedGameId ? (
        <section className="lobby-browser">
          <div className="section-heading">
            <div>
              <p className="eyebrow">лобби</p>
              <h1>Выбери заседание</h1>
            </div>
            <div className="toolbar-actions">
              <button className="secondary-action" onClick={() => void handleManualRefresh()} disabled={isLoading}>
                Обновить
              </button>
              <button className="primary-action" onClick={() => setIsCreatingGame((value) => !value)}>
                Создать новую игру
              </button>
            </div>
          </div>

          {isCreatingGame ? (
            <form className="create-game-strip" onSubmit={handleCreateGame}>
              <input
                value={newGameTitle}
                onChange={(event) => setNewGameTitle(event.target.value)}
                placeholder="Название комнаты"
              />
              <button className="primary-action" type="submit" disabled={isSubmitting}>
                Создать
              </button>
            </form>
          ) : null}

          <div className="lobby-filters">
            <input
              value={lobbyFilter}
              onChange={(event) => setLobbyFilter(event.target.value)}
              placeholder="Найти игру по названию"
            />
            <select
              value={lobbyStatusFilter}
              onChange={(event) => setLobbyStatusFilter(event.target.value as GameStatus | "all")}
              aria-label="Фильтр по статусу"
            >
              <option value="all">Все статусы</option>
              <option value="lobby">Ожидают игроков</option>
              <option value="started">Идут сейчас</option>
              <option value="finished">Завершены</option>
            </select>
            <label className="checkbox filter-checkbox">
              <input
                type="checkbox"
                checked={onlyMyGames}
                onChange={(event) => setOnlyMyGames(event.target.checked)}
              />
              Только мои игры
            </label>
          </div>

          <div className="game-card-grid">
            {filteredGameCards.map(({ game, state }) => (
              <article className="room-card" key={game.id}>
                <div>
                  <span className={`status-pill status-${state?.status ?? "unknown"}`}>
                    {statusLabel(state?.status)}
                  </span>
                  <h2>{state?.title ?? game.title}</h2>
                </div>
                <div className="room-meta">
                  <span>{state?.players?.length ?? "?"} игроков</span>
                  <span>{state?.current_round ? `Раунд ${state.current_round}` : "Перед стартом"}</span>
                </div>
                <button className="primary-action" onClick={() => void openGame(game.id)}>
                  Войти
                </button>
              </article>
            ))}
            {!gameCards.length ? (
              <div className="empty-state">
                <h2>Комнат пока нет</h2>
                <p>Создай первое заседание и пригласи остальных директоров.</p>
              </div>
            ) : !filteredGameCards.length ? (
              <div className="empty-state">
                <h2>Ничего не найдено</h2>
                <p>Попробуй другой текст или статус.</p>
              </div>
            ) : null}
          </div>
        </section>
      ) : gameState?.is_finished ? (
        <FinishScreen
          state={gameState}
          me={me}
          acceptedDecisions={acceptedDecisions}
          roundReports={roundReports}
          governanceReports={governanceReports}
          chatMessages={chatMessages}
          canSendChatMessage={canSendChatMessage}
          currentUserId={currentUserId}
          isSubmitting={isSubmitting}
          onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
          onRefresh={handleManualRefresh}
          onBack={() => setSelectedGameId(null)}
          isLoading={isLoading}
        />
      ) : gameState?.status === "started" ? (
        <StartedGameScreen
          state={gameState}
          me={me}
          players={players}
          phase={gameState.phase ?? "major_voting"}
          acceptedDecisions={acceptedDecisions}
          roundReports={roundReports}
          governanceProposals={governanceProposals}
          governanceSubmissions={governanceSubmissions}
          governanceReports={governanceReports}
          chatMessages={chatMessages}
          availableDecisions={availableDecisions}
          moleTargets={moleTargets}
          currentVotes={currentVotes}
          hasVoted={hasVoted}
          myCurrentVote={gameState.my_current_vote ?? null}
          canVote={canVote}
          canSubmitGovernanceProposal={canSubmitGovernanceProposal}
          canSkipGovernanceProposal={canSkipGovernanceProposal}
          canSendChatMessage={canSendChatMessage}
          isSubmitting={isSubmitting}
          onVote={(decision) => void handleAction("vote", { decision, abstain: false })}
          onVoteProposal={(proposalId) => void handleAction("vote", { proposal_id: proposalId, abstain: false })}
          onAbstain={() => void handleAction("vote", { abstain: true })}
          onSubmitGovernanceProposal={(payload) => void handleAction("submit_governance_proposal", payload)}
          onSkipGovernanceProposal={() => void handleAction("skip_governance_proposal")}
          onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
          onRefresh={handleManualRefresh}
          isLoading={isLoading}
          currentUserId={currentUserId}
        />
      ) : (
        <GameLobbyScreen
          state={gameState}
          currentUserId={currentUserId}
          canJoin={canJoin}
          canStart={canStart}
          canKick={canKick}
          hasMe={hasMe}
          chatMessages={chatMessages}
          canSendChatMessage={canSendChatMessage}
          isLoading={isLoading}
          isSubmitting={isSubmitting}
          onJoin={() => void handleAction("join_game")}
          onStart={() => void handleAction("start_game")}
          onKick={(userId) => void handleAction("kick_player", { user_id: userId })}
          onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
          onRefresh={handleManualRefresh}
        />
      )}
    </main>
  );
}

function GameLobbyScreen(props: {
  state: PublicGameState | null;
  currentUserId: number;
  canJoin: boolean;
  canStart: boolean;
  canKick: boolean;
  hasMe: boolean;
  chatMessages: PublicChatMessage[];
  canSendChatMessage: boolean;
  isLoading: boolean;
  isSubmitting: boolean;
  onJoin: () => void;
  onStart: () => void;
  onKick: (userId: number) => void;
  onSendChatMessage: (message: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const state = props.state;

  return (
    <section className="game-stage">
      <div className="section-heading">
        <div>
          <p className="eyebrow">комната</p>
          <h1>{state?.title ?? "Загрузка комнаты"}</h1>
        </div>
        <div className="toolbar-actions">
          <button className="secondary-action" onClick={() => void props.onRefresh()} disabled={props.isLoading}>
            Обновить
          </button>
          {props.canJoin && !props.hasMe ? (
            <button className="primary-action" onClick={props.onJoin} disabled={props.isSubmitting}>
              Присоединиться
            </button>
          ) : null}
          {props.canStart ? (
            <button className="primary-action" onClick={props.onStart} disabled={props.isSubmitting}>
              Начать игру
            </button>
          ) : null}
        </div>
      </div>

      <div className="players-grid">
        {(state?.players ?? []).map((player) => (
          <PlayerCard
            key={player.user_id}
            player={player}
            currentUserId={props.currentUserId}
            canKick={props.canKick && player.user_id !== props.currentUserId}
            onKick={() => props.onKick(player.user_id)}
            isSubmitting={props.isSubmitting}
          />
        ))}
        {!(state?.players ?? []).length ? (
          <div className="empty-state">
            <h2>Пока никого нет</h2>
            <p>Первый директор уже почти в лифте.</p>
          </div>
        ) : null}
      </div>

      <ChatPanel
        messages={props.chatMessages}
        currentUserId={props.currentUserId}
        canSend={props.canSendChatMessage}
        isSubmitting={props.isSubmitting}
        onSend={props.onSendChatMessage}
      />
    </section>
  );
}

function StartedGameScreen(props: {
  state: PublicGameState;
  me?: PublicPlayerState;
  players: PublicPlayerState[];
  phase: GamePhase;
  acceptedDecisions: string[];
  roundReports: PublicRoundReport[];
  governanceProposals: PublicGovernanceProposal[];
  governanceSubmissions: PublicGovernanceSubmission[];
  governanceReports: PublicGovernanceReport[];
  chatMessages: PublicChatMessage[];
  availableDecisions: string[];
  moleTargets: string[];
  currentVotes: { user_id: number; has_voted: boolean }[];
  hasVoted: boolean;
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  canSubmitGovernanceProposal: boolean;
  canSkipGovernanceProposal: boolean;
  canSendChatMessage: boolean;
  isSubmitting: boolean;
  onVote: (decision: string) => void;
  onVoteProposal: (proposalId: number) => void;
  onAbstain: () => void;
  onSubmitGovernanceProposal: (payload: Record<string, unknown>) => void;
  onSkipGovernanceProposal: () => void;
  onSendChatMessage: (message: string) => Promise<void>;
  onRefresh: () => Promise<void>;
  isLoading: boolean;
  currentUserId: number;
}) {
  const [selectedReport, setSelectedReport] = useState<PublicRoundReport | null>(null);
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");
  const canAbstain = props.canVote && !props.me?.is_ceo;
  const isWaitingForPlayer = (userId: number) => {
    if (props.phase === "governance_proposal") {
      return !props.governanceSubmissions.some((item) => item.user_id === userId && item.status);
    }
    return !props.currentVotes.some((item) => item.user_id === userId && item.has_voted);
  };

  return (
    <section className="game-stage">
      <div className="game-hud">
        <HudItem label="Раунд" value={String(props.state.current_round || 1)} />
        <HudItem label="Фаза" value={phaseLabel(props.phase)} />
        <HudItem label="Казначейский резерв" value={formatShare(props.state.treasury_share_bps)} />
        <HudItem label="Принято решений" value={String(props.acceptedDecisions.length)} />
        <button className="secondary-action" onClick={() => void props.onRefresh()} disabled={props.isLoading}>
          Обновить
        </button>
      </div>

      <div className="play-columns">
        <aside className="side-stack">
          <section className="identity-card">
            <p className="eyebrow">Ты</p>
            <h2>{props.me?.name ?? "Наблюдатель"}</h2>
            <div className="identity-meta">
              <span>{formatShare(props.me?.share_bps)} доля</span>
              <span>{roleLabel(props.me?.role)}</span>
              {props.me?.is_ceo ? <strong>CEO</strong> : null}
            </div>
          </section>

          {props.me?.role === "mole" ? (
            <section className="secret-card">
              <p className="eyebrow">Твои цели</p>
              <DecisionList values={props.moleTargets} emptyText="Цели еще не раскрыты." />
            </section>
          ) : null}

          <section className="directors-panel">
            <h2>Совет директоров</h2>
            <div className="director-list">
              {props.players.map((player) => (
                <div
                  key={player.user_id}
                  className={player.user_id === props.currentUserId ? "director-row is-current" : "director-row"}
                >
                  <div>
                    <strong>
                      {player.name}
                      {isWaitingForPlayer(player.user_id) ? (
                        <span className="pending-vote" aria-label="ожидаем голос">
                          ⌛
                        </span>
                      ) : null}
                    </strong>
                    <span>{formatShare(player.share_bps)}</span>
                  </div>
                  <div className="badge-row">
                    {player.is_host ? <span className="badge">Host</span> : null}
                    {player.is_ceo ? <span className="badge accent">CEO</span> : null}
                  </div>
                </div>
              ))}
            </div>
          </section>

        </aside>

        <div className="main-stack">
          <ChatPanel
            messages={props.chatMessages}
            currentUserId={props.currentUserId}
            canSend={props.canSendChatMessage}
            isSubmitting={props.isSubmitting}
            onSend={props.onSendChatMessage}
          />

          {props.phase === "governance_proposal" ? (
            <GovernanceProposalPhase
              players={props.players}
              submissions={props.governanceSubmissions}
              currentUserId={props.currentUserId}
              canSubmit={props.canSubmitGovernanceProposal}
              canSkip={props.canSkipGovernanceProposal}
              isSubmitting={props.isSubmitting}
              onSubmit={props.onSubmitGovernanceProposal}
              onSkip={props.onSkipGovernanceProposal}
            />
          ) : props.phase === "governance_voting" ? (
            <GovernanceVotingPhase
              players={props.players}
              proposals={props.governanceProposals}
              myCurrentVote={props.myCurrentVote}
              canVote={props.canVote}
              hasVoted={props.hasVoted}
              isSubmitting={props.isSubmitting}
              isCEO={Boolean(props.me?.is_ceo)}
              onVote={props.onVoteProposal}
              onAbstain={props.onAbstain}
            />
          ) : (
            <section className="voting-board">
              <div className="section-heading compact-heading">
                <div>
                  <p className="eyebrow">голосование</p>
                  <h2>Выбери решение</h2>
                </div>
                {props.hasVoted ? <span className="wait-pill">Вы проголосовали, ждем остальных</span> : null}
              </div>

              <div className="decision-grid">
                {props.availableDecisions.map((decision) => {
                  const isMoleTarget = props.me?.role === "mole" && props.moleTargets.includes(decision);
                  const isSelected = props.myCurrentVote?.decision === decision;
                  return (
                    <article
                      className={["decision-card", isMoleTarget ? "mole-target" : "", isSelected ? "selected-vote" : ""]
                        .filter(Boolean)
                        .join(" ")}
                      key={decision}
                    >
                      <span>{isMoleTarget ? "Твоя цель" : "Решение"}</span>
                      <strong>{decisionTitle(decision)}</strong>
                      <small>{decision}</small>
                      <button
                        className={isMoleTarget ? "primary-action mole-vote-action" : "primary-action"}
                        onClick={() => props.onVote(decision)}
                        disabled={!props.canVote || props.hasVoted || props.isSubmitting}
                      >
                        Голосовать
                      </button>
                    </article>
                  );
                })}
              </div>

              {props.me?.is_ceo ? null : (
                <button
                  className={
                    props.myCurrentVote?.abstain
                      ? "secondary-action abstain-button selected-abstain"
                      : "secondary-action abstain-button"
                  }
                  onClick={props.onAbstain}
                  disabled={!canAbstain || props.hasVoted || props.isSubmitting}
                >
                  Воздержаться
                </button>
              )}
            </section>
          )}

          <section className="history-panel">
            <div>
              <h2>Принятые решения</h2>
              <RoundReportList
                reports={acceptedReports}
                emptyText="Совет еще ничего не принял."
                onSelect={setSelectedReport}
              />
            </div>
            <GovernanceReportList reports={props.governanceReports} players={props.players} />
            <RoundReportDetails report={selectedReport} onClose={() => setSelectedReport(null)} />
          </section>
        </div>
      </div>
    </section>
  );
}

function FinishScreen(props: {
  state: PublicGameState;
  me?: PublicPlayerState;
  acceptedDecisions: string[];
  roundReports: PublicRoundReport[];
  governanceReports: PublicGovernanceReport[];
  chatMessages: PublicChatMessage[];
  canSendChatMessage: boolean;
  currentUserId: number;
  isSubmitting: boolean;
  onSendChatMessage: (message: string) => Promise<void>;
  onRefresh: () => Promise<void>;
  onBack: () => void;
  isLoading: boolean;
}) {
  const [selectedReport, setSelectedReport] = useState<PublicRoundReport | null>(null);
  const playerWon =
    props.state.winner === "mole" ? props.me?.role === "mole" : props.state.winner === "players" && props.me?.role !== "mole";
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");

  return (
    <section className="finish-screen">
      <p className="eyebrow">финал</p>
      <h1>{winnerLabel(props.state.winner)}</h1>
      {props.me?.role ? (
        <p className="personal-result">
          {roleLabel(props.me.role)}: {playerWon ? "Ты победил" : "Ты проиграл"}
        </p>
      ) : null}
      <section className="history-panel final-history">
        <div>
          <h2>Финальные принятые решения</h2>
          <RoundReportList reports={acceptedReports} emptyText="Решений нет." onSelect={setSelectedReport} />
        </div>
        <GovernanceReportList reports={props.governanceReports} players={props.state.players} />
        <RoundReportDetails report={selectedReport} onClose={() => setSelectedReport(null)} />
      </section>
      <ChatPanel
        messages={props.chatMessages}
        currentUserId={props.currentUserId}
        canSend={props.canSendChatMessage}
        isSubmitting={props.isSubmitting}
        onSend={props.onSendChatMessage}
      />
      <div className="toolbar-actions centered-actions">
        <button className="secondary-action" onClick={() => void props.onRefresh()} disabled={props.isLoading}>
          Обновить
        </button>
        <button className="primary-action" onClick={props.onBack}>
          К списку игр
        </button>
      </div>
    </section>
  );
}

function GovernanceProposalPhase(props: {
  players: PublicPlayerState[];
  submissions: PublicGovernanceSubmission[];
  currentUserId: number;
  canSubmit: boolean;
  canSkip: boolean;
  isSubmitting: boolean;
  onSubmit: (payload: Record<string, unknown>) => void;
  onSkip: () => void;
}) {
  const [proposalType, setProposalType] = useState<GovernanceProposalType>("share_transfer");
  const [fromUserId, setFromUserId] = useState(() => props.currentUserId);
  const [toUserId, setToUserId] = useState(() => props.players.find((player) => player.user_id !== props.currentUserId)?.user_id ?? props.currentUserId);
  const [targetUserId, setTargetUserId] = useState(() => props.currentUserId);
  const [sharePercent, setSharePercent] = useState("5");

  const mySubmission = props.submissions.find((submission) => submission.user_id === props.currentUserId);
  const canAct = props.canSubmit || props.canSkip;
  const currentCEO = props.players.find((player) => player.is_ceo);
  const fallbackNonCEOId = props.players.find((player) => !player.is_ceo)?.user_id ?? targetUserId;
  const appointTargetUserId = targetUserId === currentCEO?.user_id ? fallbackNonCEOId : targetUserId;
  const canSubmitForm =
    props.canSubmit &&
    (proposalType !== "share_transfer" || fromUserId !== toUserId) &&
    (proposalType !== "appoint_ceo" || appointTargetUserId !== currentCEO?.user_id);

  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmitForm) {
      return;
    }
    const shareBps = percentToBps(sharePercent);

    if (proposalType === "share_transfer") {
      props.onSubmit({
        proposal_type: proposalType,
        from_user_id: fromUserId,
        to_user_id: toUserId,
        share_bps: shareBps,
      });
      return;
    }

    if (proposalType === "treasury_grant") {
      props.onSubmit({
        proposal_type: proposalType,
        target_user_id: targetUserId,
        share_bps: shareBps,
      });
      return;
    }

    if (proposalType === "treasury_buyback") {
      props.onSubmit({
        proposal_type: proposalType,
        target_user_id: targetUserId,
        share_bps: shareBps,
      });
      return;
    }

    props.onSubmit({
      proposal_type: proposalType,
      target_user_id: appointTargetUserId,
    });
  }

  return (
    <section className="voting-board governance-board">
      <div className="section-heading compact-heading">
        <div>
          <p className="eyebrow">Корпоративные манёвры</p>
          <h2>Подай предложение или пропусти</h2>
        </div>
        {!canAct ? <span className="wait-pill">Ждем остальных</span> : null}
      </div>

      {mySubmission?.status ? (
        <p className="quiet-text">Ты уже {mySubmission.status === "submitted" ? "подал предложение" : "пропустил манёвр"}.</p>
      ) : (
        <form className="governance-form" onSubmit={submit}>
          <label>
            Тип манёвра
            <select value={proposalType} onChange={(event) => setProposalType(event.target.value as GovernanceProposalType)}>
              <option value="share_transfer">Передача доли</option>
              <option value="treasury_grant">Грант из резерва</option>
              <option value="treasury_buyback">Выкуп доли в резерв</option>
              <option value="appoint_ceo">Назначить CEO</option>
            </select>
          </label>

          {proposalType === "share_transfer" ? (
            <>
              <label>
                От кого
                <PlayerSelect
                  players={props.players}
                  value={fromUserId}
                  excludeUserIds={[toUserId]}
                  onChange={(nextUserId) => {
                    setFromUserId(nextUserId);
                    if (nextUserId === toUserId) {
                      setToUserId(props.players.find((player) => player.user_id !== nextUserId)?.user_id ?? nextUserId);
                    }
                  }}
                />
              </label>
              <label>
                Кому
                <PlayerSelect
                  players={props.players}
                  value={toUserId}
                  excludeUserIds={[fromUserId]}
                  onChange={(nextUserId) => {
                    setToUserId(nextUserId);
                    if (nextUserId === fromUserId) {
                      setFromUserId(props.players.find((player) => player.user_id !== nextUserId)?.user_id ?? nextUserId);
                    }
                  }}
                />
              </label>
              <ShareInput value={sharePercent} onChange={setSharePercent} />
            </>
          ) : proposalType === "treasury_grant" || proposalType === "treasury_buyback" ? (
            <>
              <label>
                {proposalType === "treasury_grant" ? "Получатель" : "У кого выкупить"}
                <PlayerSelect players={props.players} value={targetUserId} onChange={setTargetUserId} />
              </label>
              <ShareInput value={sharePercent} onChange={setSharePercent} />
            </>
          ) : (
            <label>
              Новый CEO
              <PlayerSelect
                players={props.players}
                value={appointTargetUserId}
                excludeUserIds={currentCEO ? [currentCEO.user_id] : []}
                onChange={setTargetUserId}
              />
            </label>
          )}

          <div className="governance-actions">
            <button className="primary-action" type="submit" disabled={!canSubmitForm || props.isSubmitting}>
              Подать предложение
            </button>
            <button
              className="secondary-action"
              type="button"
              onClick={props.onSkip}
              disabled={!props.canSkip || props.isSubmitting}
            >
              Пропустить
            </button>
          </div>
        </form>
      )}

    </section>
  );
}

function GovernanceVotingPhase(props: {
  players: PublicPlayerState[];
  proposals: PublicGovernanceProposal[];
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  hasVoted: boolean;
  isSubmitting: boolean;
  isCEO: boolean;
  onVote: (proposalId: number) => void;
  onAbstain: () => void;
}) {
  const canAbstain = props.canVote && !props.isCEO;

  return (
    <section className="voting-board governance-board">
      <div className="section-heading compact-heading">
        <div>
          <p className="eyebrow">Корпоративные манёвры</p>
          <h2>Выбери предложение</h2>
        </div>
        {props.hasVoted ? <span className="wait-pill">Вы проголосовали, ждем остальных</span> : null}
      </div>

      <div className="proposal-grid">
        {props.proposals.map((proposal) => (
          <GovernanceProposalCard
            key={proposal.id}
            proposal={proposal}
            players={props.players}
            selected={props.myCurrentVote?.proposal_id === proposal.id}
            disabled={!props.canVote || props.hasVoted || props.isSubmitting}
            onVote={() => props.onVote(proposal.id)}
          />
        ))}
      </div>

      {props.isCEO ? null : (
        <button
          className={props.myCurrentVote?.abstain ? "secondary-action abstain-button selected-abstain" : "secondary-action abstain-button"}
          onClick={props.onAbstain}
          disabled={!canAbstain || props.hasVoted || props.isSubmitting}
        >
          Воздержаться
        </button>
      )}
    </section>
  );
}

function GovernanceProposalCard(props: {
  proposal: PublicGovernanceProposal;
  players: PublicPlayerState[];
  selected: boolean;
  disabled: boolean;
  onVote: () => void;
}) {
  return (
    <article className={props.selected ? "proposal-card selected-vote" : "proposal-card"}>
      <span>Предложение #{props.proposal.id}</span>
      <strong>{describeGovernanceProposal(props.proposal, props.players)}</strong>
      <small>Автор: {playerName(props.players, props.proposal.proposer_user_id)}</small>
      <button className="primary-action" onClick={props.onVote} disabled={props.disabled}>
        Голосовать
      </button>
    </article>
  );
}

function GovernanceReportList(props: { reports: PublicGovernanceReport[]; players: PublicPlayerState[] }) {
  if (!props.reports.length) {
    return null;
  }

  return (
    <div className="governance-report-list">
      <h2>Корпоративные манёвры</h2>
      {props.reports.slice(-3).map((report) => (
        <div className={report.outcome === "accepted" ? "governance-report accepted" : "governance-report"} key={report.round}>
          <span>Раунд {report.round}</span>
          <strong>{governanceReportText(report, props.players)}</strong>
        </div>
      ))}
    </div>
  );
}

function ChatPanel(props: {
  messages: PublicChatMessage[];
  currentUserId: number;
  canSend: boolean;
  isSubmitting: boolean;
  onSend: (message: string) => Promise<void>;
}) {
  const [draft, setDraft] = useState("");

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const message = draft.trim();
    if (!message || !props.canSend || props.isSubmitting) {
      return;
    }
    await props.onSend(message);
    setDraft("");
  }

  return (
    <section className="chat-panel">
      <div className="chat-heading">
        <div>
          <p className="eyebrow">чат</p>
          <h2>Переговорная</h2>
        </div>
        <span>{props.messages.length}</span>
      </div>

      <div className="chat-messages">
        {props.messages.map((message) => {
          const isMine = message.user_id === props.currentUserId;
          return (
            <article className={isMine ? "chat-message is-mine" : "chat-message"} key={`${message.id}-${message.created_at}`}>
              <div>
                <strong>{isMine ? "Ты" : message.user_name}</strong>
                <small>{formatChatTime(message.created_at)}</small>
              </div>
              <p>{message.message}</p>
            </article>
          );
        })}
        {!props.messages.length ? <p className="quiet-text">В переговорной пока тихо.</p> : null}
      </div>

      <form className="chat-form" onSubmit={submit}>
        <input
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder={props.canSend ? "Сообщение совету" : "Чат доступен участникам комнаты"}
          maxLength={500}
          disabled={!props.canSend || props.isSubmitting}
        />
        <button className="primary-action" type="submit" disabled={!draft.trim() || !props.canSend || props.isSubmitting}>
          Отправить
        </button>
      </form>
    </section>
  );
}

function PlayerSelect(props: {
  players: PublicPlayerState[];
  value: number;
  excludeUserIds?: number[];
  onChange: (userId: number) => void;
}) {
  const exclude = new Set(props.excludeUserIds ?? []);
  const options = props.players.filter((player) => !exclude.has(player.user_id));
  const value = options.some((player) => player.user_id === props.value) ? props.value : (options[0]?.user_id ?? props.value);
  return (
    <select value={value} onChange={(event) => props.onChange(Number(event.target.value))}>
      {options.map((player) => (
        <option key={player.user_id} value={player.user_id}>
          {player.name}
        </option>
      ))}
    </select>
  );
}

function ShareInput(props: { value: string; onChange: (sharePercent: string) => void }) {
  return (
    <label>
      Доля, %
      <input
        type="text"
        inputMode="decimal"
        value={props.value}
        placeholder="например, 2.5"
        onChange={(event) => props.onChange(event.target.value)}
      />
    </label>
  );
}

function PlayerCard(props: {
  player: PublicPlayerState;
  currentUserId: number;
  canKick: boolean;
  isSubmitting: boolean;
  onKick: () => void;
}) {
  return (
    <article className={props.player.user_id === props.currentUserId ? "player-card is-current" : "player-card"}>
      <div>
        <h2>{props.player.name}</h2>
        <p>{formatShare(props.player.share_bps)} доля</p>
      </div>
      <div className="badge-row">
        {props.player.is_host ? <span className="badge">Host</span> : null}
        {props.player.is_ceo ? <span className="badge accent">CEO</span> : null}
        {props.player.user_id === props.currentUserId ? <span className="badge current">Вы</span> : null}
      </div>
      {props.canKick ? (
        <button className="kick-button" onClick={props.onKick} disabled={props.isSubmitting}>
          Убрать
        </button>
      ) : null}
    </article>
  );
}

function HudItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="hud-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function RoundReportList(props: {
  reports: PublicRoundReport[];
  emptyText: string;
  onSelect: (report: PublicRoundReport) => void;
}) {
  if (!props.reports.length) {
    return <p className="quiet-text">{props.emptyText}</p>;
  }

  return (
    <div className="decision-list interactive-list">
      {props.reports.map((report) => (
        <button key={`${report.outcome}-${report.round}`} onClick={() => props.onSelect(report)}>
          <strong>{report.decision ? decisionLabel(report.decision) : `Раунд ${report.round}`}</strong>
          <small>Раунд {report.round}</small>
        </button>
      ))}
    </div>
  );
}

function RoundReportDetails(props: { report: PublicRoundReport | null; onClose: () => void }) {
  if (!props.report) {
    return null;
  }

  return (
    <aside className="round-report-details">
      <div className="round-report-header">
        <div>
          <p className="eyebrow">отчет раунда</p>
          <h3>
            Раунд {props.report.round}:{" "}
            {props.report.outcome === "accepted" && props.report.decision
              ? `принято ${decisionLabel(props.report.decision)}`
              : "решение не принято"}
          </h3>
        </div>
        <button className="mini-button" onClick={props.onClose}>
          Закрыть
        </button>
      </div>
      <div className="round-report-votes">
        {props.report.votes.map((vote) => (
          <div className="round-report-row" key={`${props.report?.round}-${vote.decision}`}>
            <span>{vote.abstain ? "Воздержались" : decisionLabel(vote.decision)}</span>
            <strong>{formatShare(vote.share_bps)}</strong>
            <small>{formatVotesCount(vote.voter_count)}</small>
          </div>
        ))}
        {!props.report.votes.length ? <p className="quiet-text">Подробных голосов для этого раунда нет.</p> : null}
      </div>
    </aside>
  );
}

function DecisionList({ values, emptyText }: { values: string[]; emptyText: string }) {
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

function Toast({ message, onClose }: { message: string | null; onClose: () => void }) {
  if (!message) {
    return null;
  }

  return (
    <div className="toast" role="alert">
      <span>{message}</span>
      <button onClick={onClose}>Закрыть</button>
    </div>
  );
}
