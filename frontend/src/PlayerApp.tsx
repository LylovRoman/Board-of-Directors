import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  changePassword,
  createGame,
  getMe,
  getGameState,
  getMyProfile,
  listGames,
  login,
  register,
  sendGameAction,
  updateMyProfile,
  API_BASE_URL,
} from "./api";
import {
  AUTH_SESSION_CLEARED_EVENT,
  clearAuthSession,
  getAuthToken,
  readStoredAuthSession,
  saveAuthSession,
  saveAuthUser,
} from "./authSession";
import type {
  ActionType,
  AuthUser,
  DecisionType,
  Game,
  GamePhase,
  GameStatus,
  GovernanceProposalType,
  Profile,
  PublicGameState,
  PublicGovernanceProposal,
  PublicGovernanceReport,
  PublicGovernanceSubmission,
  PublicOwnVoteState,
  PublicChatMessage,
  PublicPlayerState,
  PublicVoteState,
  PublicRoundReport,
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

const SELECTED_GAME_STORAGE_KEY = "board-of-directors-selected-game-id";
const DECISION_TITLES: Record<string, string> = {
  A: "Выпуск облигаций",
  B: "Экспансия на новый рынок",
  C: "Выплата дивидендов по акциям",
  D: "Запуск экспериментального продукта",
  E: "Сделка слияния",
  F: "Оптимизация неэффективного персонала",
  G: "Агрессивная налоговая стратегия",
  H: "Обратный выкуп акций",
};
const DECISION_OPTIONS = Object.keys(DECISION_TITLES);
const DECISION_TYPE_FALLBACK: Record<string, DecisionType> = {
  A: "growth",
  B: "growth",
  C: "empowerment",
  D: "growth",
  E: "growth",
  F: "empowerment",
  G: "empowerment",
  H: "empowerment",
};

interface GameCard {
  game: Game;
}

type AuthMode = "login" | "register";

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
    case "mole_objective_selection":
      return "Выбор целей Крота";
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

function decisionType(decision: string, decisionTypes?: Record<string, DecisionType> | null): DecisionType {
  return decisionTypes?.[decision] ?? DECISION_TYPE_FALLBACK[decision] ?? "growth";
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
      return `Оштрафовать ${playerName(players, proposal.target_user_id)} на ${formatShare(proposal.share_bps)} в резерв`;
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
  const [authSession, setAuthSession] = useState(() => readStoredAuthSession());
  const [isAuthChecking, setIsAuthChecking] = useState(() => Boolean(getAuthToken()));
  const [games, setGames] = useState<Game[]>([]);
  const [gameCards, setGameCards] = useState<GameCard[]>([]);
  const [selectedGameId, setSelectedGameId] = useState<number | null>(() =>
    readStoredNumber(SELECTED_GAME_STORAGE_KEY),
  );
  const [gameState, setGameState] = useState<PublicGameState | null>(null);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [authLogin, setAuthLogin] = useState("");
  const [authName, setAuthName] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [newGameTitle, setNewGameTitle] = useState("");
  const [lobbyFilter, setLobbyFilter] = useState("");
  const [lobbyStatusFilter, setLobbyStatusFilter] = useState<GameStatus | "all">("all");
  const [onlyMyGames, setOnlyMyGames] = useState(false);
  const [isCreatingGame, setIsCreatingGame] = useState(false);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [profileName, setProfileName] = useState("");
  const [profileAvatarUrl, setProfileAvatarUrl] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const currentUser: AuthUser | null = authSession?.user ?? null;
  const currentUserId = currentUser?.id ?? null;
  const availableActions = gameState?.available_actions ?? [];
  const players = gameState?.players ?? [];
  const currentVotes = normalizeVotes(gameState?.current_votes);
  const acceptedDecisions = normalizeStringArray(gameState?.accepted_decisions);
  const availableDecisions = normalizeStringArray(gameState?.available_decisions);
  const majorVoteOptions = normalizeStringArray(gameState?.major_vote_options);
  const decisionTypes = gameState?.decision_types ?? DECISION_TYPE_FALLBACK;
  const roundReports = normalizeRoundReports(gameState?.round_reports);
  const chatMessages = normalizeChatMessages(gameState?.chat_messages);
  const governanceProposals = normalizeGovernanceProposals(gameState?.governance_proposals);
  const governanceSubmissions = normalizeGovernanceSubmissions(gameState?.governance_submissions);
  const governanceReports = normalizeGovernanceReports(gameState?.governance_reports);
  const moleTargets = normalizeStringArray(gameState?.mole_targets);
  const moleSabotage = gameState?.mole_sabotage ?? "";
  const me = gameState?.me;
  const hasMe = Boolean(me && me.user_id === currentUserId);
  const hasVoted = currentVotes.some((vote) => vote.user_id === currentUserId && vote.has_voted);
  const canVote = availableActions.includes("vote");
  const canSelectMoleObjectives = availableActions.includes("select_mole_objectives");
  const canSubmitGovernanceProposal = availableActions.includes("submit_governance_proposal");
  const canSkipGovernanceProposal = availableActions.includes("skip_governance_proposal");
  const canJoin = availableActions.includes("join_game");
  const canLeave = availableActions.includes("leave_game");
  const canStart = availableActions.includes("start_game");
  const canKick = availableActions.includes("kick_player");
  const canBan = availableActions.includes("ban_player");
  const canSendChatMessage = availableActions.includes("send_chat_message");
  const filteredGameCards = useMemo(() => {
    const normalizedFilter = lobbyFilter.trim().toLowerCase();
    return gameCards.filter(({ game }) => {
      const title = game.title.toLowerCase();
      const matchesText = !normalizedFilter || title.includes(normalizedFilter);
      const matchesStatus = lobbyStatusFilter === "all" || game.status === lobbyStatusFilter;
      const matchesOwner =
        !onlyMyGames ||
        Boolean(currentUserId && game.player_user_ids?.includes(currentUserId));
      return matchesText && matchesStatus && matchesOwner;
    });
  }, [currentUserId, gameCards, lobbyFilter, lobbyStatusFilter, onlyMyGames]);

  const showError = useCallback((error: unknown) => {
    setSuccessMessage(null);
    setErrorMessage(getErrorMessage(error));
  }, []);

  const loadGames = useCallback(async () => {
    try {
      const nextGames = await listGames();
      setGames(nextGames);
      setGameCards(nextGames.map((game) => ({ game })));
      return nextGames;
    } catch (error) {
      showError(error);
      return [];
    }
  }, [showError]);

  const loadGameState = useCallback(
    async (gameId: number) => {
      try {
        const state = await getGameState(gameId);
        setGameState(state);
        return state;
      } catch (error) {
        showError(error);
        return null;
      }
    },
    [showError],
  );

  const refreshGameList = useCallback(async () => {
    if (!currentUserId) {
      return;
    }
    setIsLoading(true);
    setErrorMessage(null);
    try {
      await loadGames();
    } finally {
      setIsLoading(false);
    }
  }, [currentUserId, loadGames]);

  const refreshSelectedGame = useCallback(async () => {
    if (!currentUserId || !selectedGameId) {
      return;
    }
    setIsLoading(true);
    setErrorMessage(null);
    try {
      await loadGameState(selectedGameId);
    } finally {
      setIsLoading(false);
    }
  }, [currentUserId, loadGameState, selectedGameId]);

  const refreshCurrentView = useCallback(async () => {
    if (selectedGameId) {
      await refreshSelectedGame();
      return;
    }
    await refreshGameList();
  }, [refreshGameList, refreshSelectedGame, selectedGameId]);

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
          setSelectedGameId(null);
          setGameState(null);
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
      setGameCards([]);
      window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
    };

    window.addEventListener(AUTH_SESSION_CLEARED_EVENT, handleSessionCleared);
    return () => window.removeEventListener(AUTH_SESSION_CLEARED_EVENT, handleSessionCleared);
  }, []);

  useEffect(() => {
    if (!isAuthChecking && currentUserId) {
      void refreshCurrentView();
    }
  }, [currentUserId, isAuthChecking]);


  useEffect(() => {
    if (selectedGameId !== null) {
      window.localStorage.setItem(SELECTED_GAME_STORAGE_KEY, String(selectedGameId));
    } else {
      window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
    }
  }, [selectedGameId]);

  useEffect(() => {
    if (!currentUserId || !selectedGameId) {
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
  }, [currentUserId, loadGameState, selectedGameId]);

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
        type: "leave_game",
      });
      const url = `${API_BASE_URL}/games/${selectedGameId}/actions`;
      const token = getAuthToken();
      void fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body,
        keepalive: true,
      });
    };

    window.addEventListener("pagehide", leaveLobby);
    return () => window.removeEventListener("pagehide", leaveLobby);
  }, [currentUserId, gameState, selectedGameId]);

  async function handleWelcomeSubmit(event: React.FormEvent) {
    event.preventDefault();
    const normalizedLogin = authLogin.trim();
    const normalizedName = authName.trim();
    if (!normalizedLogin || !authPassword) {
      setErrorMessage("Введите логин и пароль.");
      return;
    }
    if (authMode === "register" && !normalizedName) {
      setErrorMessage("Введите имя для регистрации.");
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const response =
        authMode === "login"
          ? await login({ login: normalizedLogin, password: authPassword })
          : await register({
              login: normalizedLogin,
              name: normalizedName,
              password: authPassword,
            });

      saveAuthSession(response);
      setAuthSession(response);
      setAuthPassword("");
      if (authMode === "register") {
        setAuthName("");
      }
      await loadGames();
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
      const response = await createGame({ title });
      setNewGameTitle("");
      setIsCreatingGame(false);
      setSelectedGameId(response.game.id);
      setGameState(response.state);
      await loadGames();
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
    await loadGameState(gameId);
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
        type,
        payload,
      });
      if (response.state) {
        setGameState(response.state);
      } else {
        await loadGameState(selectedGameId);
      }
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleManualRefresh() {
    await refreshCurrentView();
  }

  async function openProfile() {
    if (!currentUserId) {
      return;
    }
    setIsProfileOpen(true);
    setIsLoading(true);
    setErrorMessage(null);
    try {
      const nextProfile = await getMyProfile();
      setProfile(nextProfile);
      setProfileName(nextProfile.name);
      setProfileAvatarUrl(nextProfile.avatar_url ?? "");
    } catch (error) {
      showError(error);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleProfileSubmit(event: React.FormEvent) {
    event.preventDefault();
    const name = profileName.trim();
    const avatarUrl = profileAvatarUrl.trim();
    if (!name) {
      setErrorMessage("Введите имя.");
      setSuccessMessage(null);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    setSuccessMessage(null);
    try {
      const user = await updateMyProfile({ name, avatar_url: avatarUrl });
      saveAuthUser(user);
      setAuthSession((session) => (session ? { ...session, user } : null));
      const nextProfile = await getMyProfile();
      setProfile(nextProfile);
      setProfileName(nextProfile.name);
      setProfileAvatarUrl(nextProfile.avatar_url ?? "");
      if (selectedGameId) {
        await loadGameState(selectedGameId);
      }
      setSuccessMessage("Профиль обновлен.");
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handlePasswordSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!currentPassword || !newPassword) {
      setErrorMessage("Введите текущий и новый пароль.");
      setSuccessMessage(null);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    setSuccessMessage(null);
    try {
      await changePassword({ current_password: currentPassword, new_password: newPassword });
      setCurrentPassword("");
      setNewPassword("");
      setSuccessMessage("Пароль обновлен.");
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLeaveGame() {
    if (!selectedGameId || !currentUserId) {
      setErrorMessage("Игра или игрок не выбраны.");
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      await sendGameAction(selectedGameId, {
        type: "leave_game",
      });
      setSelectedGameId(null);
      setGameState(null);
      await loadGames();
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleBackToGames() {
    setSelectedGameId(null);
    setGameState(null);
    void refreshGameList();
  }

  function handleLogout() {
    clearAuthSession(false);
    setAuthSession(null);
    setSelectedGameId(null);
    setGameState(null);
    setIsProfileOpen(false);
    setProfile(null);
    setGames([]);
    setGameCards([]);
    setAuthPassword("");
    setCurrentPassword("");
    setNewPassword("");
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
          <div className="auth-tabs" role="tablist" aria-label="Авторизация">
            <button
              type="button"
              className={authMode === "login" ? "auth-tab active" : "auth-tab"}
              onClick={() => {
                setAuthMode("login");
                setErrorMessage(null);
              }}
            >
              Вход
            </button>
            <button
              type="button"
              className={authMode === "register" ? "auth-tab active" : "auth-tab"}
              onClick={() => {
                setAuthMode("register");
                setErrorMessage(null);
              }}
            >
              Регистрация
            </button>
          </div>
          <form className="welcome-form auth-form" onSubmit={handleWelcomeSubmit}>
            <label htmlFor="auth-login">Логин</label>
            <input
              id="auth-login"
              value={authLogin}
              onChange={(event) => setAuthLogin(event.target.value)}
              placeholder="Например, alice"
              autoComplete="username"
            />
            {authMode === "register" ? (
              <>
                <label htmlFor="auth-name">Твое имя</label>
                <input
                  id="auth-name"
                  value={authName}
                  onChange={(event) => setAuthName(event.target.value)}
                  placeholder="Например, Alice"
                  autoComplete="name"
                />
              </>
            ) : null}
            <label htmlFor="auth-password">Пароль</label>
            <input
              id="auth-password"
              value={authPassword}
              onChange={(event) => setAuthPassword(event.target.value)}
              placeholder="Минимум 8 символов"
              type="password"
              autoComplete={authMode === "login" ? "current-password" : "new-password"}
            />
            <button type="submit" className="primary-action" disabled={isSubmitting || isAuthChecking}>
              {authMode === "login" ? "Войти" : "Зарегистрироваться"}
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
        <button className="ghost-button" onClick={handleBackToGames}>
          Игры
        </button>
        <div className="brand-lockup">
          <span>Board of Directors</span>
          <small>тайное заседание</small>
        </div>
        <div className="player-chip">
          <button className="profile-chip-button" onClick={() => void openProfile()}>
            <UserAvatar name={currentUser.name} avatarUrl={currentUser.avatar_url} size="small" />
            <span>{currentUser.name}</span>
          </button>
          <button className="mini-button" onClick={handleLogout}>
            Сменить
          </button>
        </div>
      </header>

      <Toast message={errorMessage} onClose={() => setErrorMessage(null)} />
      <Toast message={successMessage} tone="success" onClose={() => setSuccessMessage(null)} />

      {isProfileOpen ? (
        <ProfileDialog
          profile={profile}
          currentUser={currentUser}
          profileName={profileName}
          profileAvatarUrl={profileAvatarUrl}
          currentPassword={currentPassword}
          newPassword={newPassword}
          isLoading={isLoading}
          isSubmitting={isSubmitting}
          onProfileNameChange={setProfileName}
          onProfileAvatarUrlChange={setProfileAvatarUrl}
          onCurrentPasswordChange={setCurrentPassword}
          onNewPasswordChange={setNewPassword}
          onSubmitProfile={handleProfileSubmit}
          onSubmitPassword={handlePasswordSubmit}
          onClose={() => setIsProfileOpen(false)}
        />
      ) : null}

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
            {filteredGameCards.map(({ game }) => (
              <article className="room-card" key={game.id}>
                <div>
                  <span className={`status-pill status-${game.status ?? "unknown"}`}>
                    {statusLabel(game.status)}
                  </span>
                  <h2>{game.title}</h2>
                </div>
                <div className="room-meta">
                  <span>{game.player_count ?? "?"} игроков</span>
                  <span>{game.current_round ? `Раунд ${game.current_round}` : "Перед стартом"}</span>
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
          onBack={handleBackToGames}
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
          majorVoteOptions={majorVoteOptions}
          decisionTypes={decisionTypes}
          moleTargets={moleTargets}
          moleSabotage={moleSabotage}
          moleVictoryPoints={gameState.mole_victory_points}
          playersVictoryPoints={gameState.players_victory_points}
          currentVotes={currentVotes}
          hasVoted={hasVoted}
          myCurrentVote={gameState.my_current_vote ?? null}
          canVote={canVote}
          canSelectMoleObjectives={canSelectMoleObjectives}
          canSubmitGovernanceProposal={canSubmitGovernanceProposal}
          canSkipGovernanceProposal={canSkipGovernanceProposal}
          canSendChatMessage={canSendChatMessage}
          isSubmitting={isSubmitting}
          onSelectMoleObjectives={(payload) => void handleAction("select_mole_objectives", payload)}
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
          canLeave={canLeave}
          canStart={canStart}
          canKick={canKick}
          canBan={canBan}
          hasMe={hasMe}
          chatMessages={chatMessages}
          canSendChatMessage={canSendChatMessage}
          isLoading={isLoading}
          isSubmitting={isSubmitting}
          onJoin={() => void handleAction("join_game")}
          onLeave={() => void handleLeaveGame()}
          onStart={() => void handleAction("start_game")}
          onKick={(userId) => void handleAction("kick_player", { user_id: userId })}
          onBan={(userId) => void handleAction("ban_player", { user_id: userId })}
          onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
          onRefresh={handleManualRefresh}
        />
      )}
    </main>
  );
}

function UserAvatar(props: { name: string; avatarUrl?: string; size?: "small" | "medium" | "large" }) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const avatarUrl = props.avatarUrl?.trim();
  const shouldShowImage = Boolean(avatarUrl && failedUrl !== avatarUrl);
  const initial = Array.from(props.name.trim())[0]?.toUpperCase() ?? "?";

  return (
    <span className={`user-avatar avatar-${props.size ?? "medium"}`} aria-hidden="true">
      {shouldShowImage ? (
        <img src={avatarUrl} alt="" onError={() => setFailedUrl(avatarUrl ?? null)} />
      ) : (
        <span>{initial}</span>
      )}
    </span>
  );
}

function formatWinRate(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function StatTile(props: { label: string; games: number; wins: number; losses: number; winrate: number }) {
  return (
    <div className="profile-stat">
      <span>{props.label}</span>
      <strong>{props.wins} / {props.games}</strong>
      <small>{props.losses} поражений · {formatWinRate(props.winrate)}</small>
    </div>
  );
}

function ProfileDialog(props: {
  profile: Profile | null;
  currentUser: AuthUser;
  profileName: string;
  profileAvatarUrl: string;
  currentPassword: string;
  newPassword: string;
  isLoading: boolean;
  isSubmitting: boolean;
  onProfileNameChange: (value: string) => void;
  onProfileAvatarUrlChange: (value: string) => void;
  onCurrentPasswordChange: (value: string) => void;
  onNewPasswordChange: (value: string) => void;
  onSubmitProfile: (event: React.FormEvent) => void;
  onSubmitPassword: (event: React.FormEvent) => void;
  onClose: () => void;
}) {
  const shownName = props.profileName || props.currentUser.name;
  const shownAvatar = props.profileAvatarUrl || props.currentUser.avatar_url;
  const stats = props.profile?.stats;

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="profile-dialog" role="dialog" aria-modal="true" aria-labelledby="profile-title">
        <div className="profile-dialog-header">
          <div className="profile-title">
            <UserAvatar name={shownName} avatarUrl={shownAvatar} size="large" />
            <div>
              <p className="eyebrow">профиль</p>
              <h2 id="profile-title">{shownName}</h2>
              <span>@{props.currentUser.login}</span>
            </div>
          </div>
          <button className="mini-button" onClick={props.onClose}>
            Закрыть
          </button>
        </div>

        {props.isLoading && !props.profile ? (
          <p className="quiet-text">Загружаем профиль...</p>
        ) : (
          <>
            <div className="profile-stats">
              <StatTile
                label="Всего"
                games={stats?.total.games ?? 0}
                wins={stats?.total.wins ?? 0}
                losses={stats?.total.losses ?? 0}
                winrate={stats?.total.winrate ?? 0}
              />
              <StatTile
                label="Крот"
                games={stats?.mole.games ?? 0}
                wins={stats?.mole.wins ?? 0}
                losses={stats?.mole.losses ?? 0}
                winrate={stats?.mole.winrate ?? 0}
              />
              <StatTile
                label="Директор"
                games={stats?.director.games ?? 0}
                wins={stats?.director.wins ?? 0}
                losses={stats?.director.losses ?? 0}
                winrate={stats?.director.winrate ?? 0}
              />
            </div>

            <form className="profile-form" onSubmit={props.onSubmitProfile}>
              <h3>Внешний вид</h3>
              <label>
                Имя
                <input
                  value={props.profileName}
                  onChange={(event) => props.onProfileNameChange(event.target.value)}
                  maxLength={64}
                  autoComplete="name"
                />
              </label>
              <label>
                URL аватарки
                <input
                  value={props.profileAvatarUrl}
                  onChange={(event) => props.onProfileAvatarUrlChange(event.target.value)}
                  placeholder="https://example.com/avatar.png"
                  inputMode="url"
                />
              </label>
              <button className="primary-action" type="submit" disabled={props.isSubmitting}>
                Сохранить профиль
              </button>
            </form>

            <form className="profile-form" onSubmit={props.onSubmitPassword}>
              <h3>Пароль</h3>
              <label>
                Текущий пароль
                <input
                  value={props.currentPassword}
                  onChange={(event) => props.onCurrentPasswordChange(event.target.value)}
                  type="password"
                  autoComplete="current-password"
                />
              </label>
              <label>
                Новый пароль
                <input
                  value={props.newPassword}
                  onChange={(event) => props.onNewPasswordChange(event.target.value)}
                  type="password"
                  autoComplete="new-password"
                  placeholder="Минимум 8 символов"
                />
              </label>
              <button className="secondary-action" type="submit" disabled={props.isSubmitting}>
                Сменить пароль
              </button>
            </form>
          </>
        )}
      </section>
    </div>
  );
}

function GameLobbyScreen(props: {
  state: PublicGameState | null;
  currentUserId: number;
  canJoin: boolean;
  canLeave: boolean;
  canStart: boolean;
  canKick: boolean;
  canBan: boolean;
  hasMe: boolean;
  chatMessages: PublicChatMessage[];
  canSendChatMessage: boolean;
  isLoading: boolean;
  isSubmitting: boolean;
  onJoin: () => void;
  onLeave: () => void;
  onStart: () => void;
  onKick: (userId: number) => void;
  onBan: (userId: number) => void;
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
          {props.canLeave ? (
            <button className="secondary-action" onClick={props.onLeave} disabled={props.isSubmitting}>
              Выйти
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
            canBan={props.canBan && player.user_id !== props.currentUserId}
            onKick={() => props.onKick(player.user_id)}
            onBan={() => props.onBan(player.user_id)}
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
  majorVoteOptions: string[];
  decisionTypes: Record<string, DecisionType>;
  moleTargets: string[];
  moleSabotage: string;
  moleVictoryPoints?: number;
  playersVictoryPoints?: number;
  currentVotes: PublicVoteState[];
  hasVoted: boolean;
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  canSelectMoleObjectives: boolean;
  canSubmitGovernanceProposal: boolean;
  canSkipGovernanceProposal: boolean;
  canSendChatMessage: boolean;
  isSubmitting: boolean;
  onSelectMoleObjectives: (payload: Record<string, unknown>) => void;
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
  const displayedMajorOptions = props.majorVoteOptions.length ? props.majorVoteOptions : props.availableDecisions;

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
            <div className="identity-name">
              <UserAvatar name={props.me?.name ?? "Наблюдатель"} avatarUrl={props.me?.avatar_url} size="medium" />
              <h2>{props.me?.name ?? "Наблюдатель"}</h2>
            </div>
            <div className="identity-meta">
              <span>{formatShare(props.me?.share_bps)} доля</span>
              <span>{formatShare(props.me?.authority_bps)} полномочия</span>
              <span>{roleLabel(props.me?.role)}</span>
              {props.me?.is_ceo ? <strong>CEO</strong> : null}
            </div>
          </section>

          {props.me?.role === "mole" ? (
            <section className="secret-card">
              <p className="eyebrow">Твои цели</p>
              <div className="score-row">
                <span>Крот: {props.moleVictoryPoints ?? 0}/3</span>
                <span>Совет: {props.playersVictoryPoints ?? 0}/3</span>
              </div>
              <h3>Подкопы</h3>
              <DecisionList values={props.moleTargets} emptyText="Цели еще не выбраны." />
              {props.moleSabotage ? (
                <div className="sabotage-secret">
                  <span>Диверсия</span>
                  <strong>{decisionLabel(props.moleSabotage)}</strong>
                </div>
              ) : null}
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
                  <div className="director-identity">
                    <UserAvatar name={player.name} avatarUrl={player.avatar_url} size="small" />
                    <div>
                      <strong>
                        {player.name}
                        {isWaitingForPlayer(player.user_id) ? (
                          <span className="pending-vote" aria-label="ожидаем голос">
                            ⌛
                          </span>
                        ) : null}
                      </strong>
                      <span>
                        Доля {formatShare(player.share_bps)} · Полномочия {formatShare(player.authority_bps)}
                      </span>
                    </div>
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

          {props.phase === "mole_objective_selection" ? (
            <MoleObjectiveSelectionPhase
              isMole={props.me?.role === "mole"}
              canSelect={props.canSelectMoleObjectives}
              isSubmitting={props.isSubmitting}
              onSubmit={props.onSelectMoleObjectives}
            />
          ) : props.phase === "governance_proposal" ? (
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
              currentVotes={props.currentVotes}
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
                {props.hasVoted ? <span className="wait-pill">Выбор сохранён, можно изменить</span> : null}
              </div>

              <div className="decision-grid">
                {displayedMajorOptions.map((decision) => {
                  const isMoleTarget = props.me?.role === "mole" && props.moleTargets.includes(decision);
                  const isMoleSabotage = props.me?.role === "mole" && props.moleSabotage === decision;
                  const isSelected = props.myCurrentVote?.decision === decision;
                  const type = decisionType(decision, props.decisionTypes);
                  return (
                    <button
                      type="button"
                      className={["decision-card", "decision-card-button", type, isMoleTarget ? "mole-target" : "", isMoleSabotage ? "mole-sabotage" : "", isSelected ? "selected-vote" : ""]
                        .filter(Boolean)
                        .join(" ")}
                      key={decision}
                      onClick={() => props.onVote(decision)}
                      disabled={!props.canVote || props.isSubmitting}
                    >
                      <span>{isMoleSabotage ? "Диверсия" : isMoleTarget ? "Подкоп" : "Решение"}</span>
                      <strong>{decisionTitle(decision)}</strong>
                      <div className="decision-meta">
                        <small className="decision-letter">{decision}</small>
                        <DecisionTypeTag type={type} />
                      </div>
                    </button>
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
                  disabled={!canAbstain || props.isSubmitting}
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

function MoleObjectiveSelectionPhase(props: {
  isMole: boolean;
  canSelect: boolean;
  isSubmitting: boolean;
  onSubmit: (payload: Record<string, unknown>) => void;
}) {
  const [targets, setTargets] = useState<string[]>([]);
  const [sabotage, setSabotage] = useState("");
  const selectedTargets = new Set(targets);
  const canSubmit = props.canSelect && targets.length === 3 && Boolean(sabotage) && !selectedTargets.has(sabotage);

  function toggleTarget(decision: string) {
    setTargets((current) => {
      if (current.includes(decision)) {
        return current.filter((item) => item !== decision);
      }
      if (current.length >= 3 || sabotage === decision) {
        return current;
      }
      return [...current, decision].sort();
    });
  }

  function chooseSabotage(decision: string) {
    setSabotage(decision);
    setTargets((current) => current.filter((item) => item !== decision));
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    props.onSubmit({ targets, sabotage });
  }

  if (!props.isMole) {
    return (
      <section className="objective-selection waiting-selection">
        <p className="eyebrow">подготовка</p>
        <h2>Крот выбирает цели</h2>
        <p className="quiet-text">Первое голосование начнется, когда тайный Крот выберет три Подкопа и одну Диверсию.</p>
      </section>
    );
  }

  return (
    <form className="objective-selection" onSubmit={submit}>
      <div className="section-heading compact-heading">
        <div>
          <p className="eyebrow">секретный выбор</p>
          <h2>Выбери Подкопы и Диверсию</h2>
        </div>
        <span className="wait-pill">{targets.length}/3 Подкопа</span>
      </div>
      <div className="objective-grid">
        {DECISION_OPTIONS.map((decision) => {
          const isTarget = selectedTargets.has(decision);
          const isSabotage = sabotage === decision;
          const type = decisionType(decision);
          return (
            <article
              className={["objective-card", type, isTarget ? "is-target" : "", isSabotage ? "is-sabotage" : ""]
                .filter(Boolean)
                .join(" ")}
              key={decision}
            >
              <span>{isSabotage ? "Диверсия" : isTarget ? "Подкоп" : "Решение"}</span>
              <strong>{decisionTitle(decision)}</strong>
              <div className="decision-meta">
                <small>{decision}</small>
                <DecisionTypeTag type={type} />
              </div>
              <div className="objective-actions">
                <button type="button" className="secondary-action" onClick={() => toggleTarget(decision)} disabled={props.isSubmitting || (!isTarget && (targets.length >= 3 || isSabotage))}>
                  Подкоп
                </button>
                <button type="button" className="sabotage-pick-action" onClick={() => chooseSabotage(decision)} disabled={props.isSubmitting}>
                  Диверсия
                </button>
              </div>
            </article>
          );
        })}
      </div>
      <button className="primary-action" type="submit" disabled={!canSubmit || props.isSubmitting}>
        Подтвердить цели
      </button>
    </form>
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
  const [plusUserId, setPlusUserId] = useState<number | null>(null);
  const [minusUserId, setMinusUserId] = useState<number | null>(null);

  const mySubmission = props.submissions.find((submission) => submission.user_id === props.currentUserId);
  const canAct = props.canSubmit || props.canSkip;
  const currentPlayer = props.players.find((player) => player.user_id === props.currentUserId);
  const canSubmitForm = props.canSubmit && (Boolean(plusUserId) || Boolean(minusUserId)) && plusUserId !== minusUserId;

  function togglePlus(userId: number) {
    setPlusUserId((current) => (current === userId ? null : userId));
    setMinusUserId((current) => (current === userId ? null : current));
  }

  function toggleMinus(userId: number) {
    setMinusUserId((current) => (current === userId ? null : userId));
    setPlusUserId((current) => (current === userId ? null : current));
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmitForm) {
      return;
    }
    if (plusUserId && minusUserId) {
      props.onSubmit({
        proposal_type: "share_transfer",
        from_user_id: minusUserId,
        to_user_id: plusUserId,
      });
      return;
    }
    if (plusUserId) {
      props.onSubmit({
        proposal_type: "treasury_grant",
        target_user_id: plusUserId,
      });
      return;
    }
    if (minusUserId) {
      props.onSubmit({
        proposal_type: "treasury_buyback",
        target_user_id: minusUserId,
      });
    }
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
          <div className="governance-proposal-summary">
            <span>Сила предложения</span>
            <strong>{formatShare(currentPlayer?.authority_bps)} полномочия</strong>
          </div>

          <div className="governance-pick-grid">
            {props.players.map((player) => (
              <article
                className={[
                  "governance-player-card",
                  plusUserId === player.user_id ? "plus-selected" : "",
                  minusUserId === player.user_id ? "minus-selected" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                key={player.user_id}
              >
                <div className="governance-player-main">
                  <UserAvatar name={player.name} avatarUrl={player.avatar_url} size="small" />
                  <div>
                    <strong>{player.name}</strong>
                    <span>{formatShare(player.share_bps)} · {formatShare(player.authority_bps)}</span>
                  </div>
                </div>
                <div className="governance-icon-actions">
                  <button
                    type="button"
                    className="icon-action plus-action"
                    onClick={() => togglePlus(player.user_id)}
                    disabled={!props.canSubmit || props.isSubmitting}
                    aria-label={`Дать долю: ${player.name}`}
                    title="Дать долю"
                  >
                    +
                  </button>
                  <button
                    type="button"
                    className="icon-action minus-action"
                    onClick={() => toggleMinus(player.user_id)}
                    disabled={!props.canSubmit || props.isSubmitting}
                    aria-label={`Оштрафовать: ${player.name}`}
                    title="Оштрафовать"
                  >
                    −
                  </button>
                </div>
              </article>
            ))}
          </div>

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
  currentVotes: PublicVoteState[];
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
            currentVotes={props.currentVotes}
            selected={props.myCurrentVote?.proposal_id === proposal.id}
            disabled={!props.canVote || props.hasVoted || props.isSubmitting}
            onVote={() => props.onVote(proposal.id)}
          />
        ))}
      </div>

      <GovernanceLiveVoteMath votes={props.currentVotes} players={props.players} />

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
  currentVotes: PublicVoteState[];
  selected: boolean;
  disabled: boolean;
  onVote: () => void;
}) {
  const proposer = props.players.find((player) => player.user_id === props.proposal.proposer_user_id);
  const proposerName = proposer?.name ?? playerName(props.players, props.proposal.proposer_user_id);
  const authorIds = props.proposal.author_user_ids?.length ? props.proposal.author_user_ids : [props.proposal.proposer_user_id];
  const proposalVotes = props.currentVotes.filter((vote) => vote.has_voted && vote.proposal_id === props.proposal.id);

  return (
    <article className={props.selected ? "proposal-card selected-vote" : "proposal-card"}>
      <span>Предложение #{props.proposal.id}</span>
      <strong>{describeGovernanceProposal(props.proposal, props.players)}</strong>
      <small>Сила: {formatShare(props.proposal.share_bps)}</small>
      <div className="proposal-authors">
        {authorIds.map((authorId) => {
          const author = props.players.find((player) => player.user_id === authorId);
          const name = author?.name ?? playerName(props.players, authorId);
          return (
            <span className="proposal-author" key={authorId}>
              <UserAvatar name={name} avatarUrl={author?.avatar_url ?? proposer?.avatar_url} size="small" />
              {name}
            </span>
          );
        })}
      </div>
      {proposalVotes.length ? (
        <div className="vote-math-list">
          {proposalVotes.map((vote) => (
            <VoteMathLine key={vote.user_id} vote={vote} players={props.players} />
          ))}
        </div>
      ) : null}
      <button className="primary-action" onClick={props.onVote} disabled={props.disabled}>
        Голосовать
      </button>
    </article>
  );
}

function GovernanceLiveVoteMath(props: { votes: PublicVoteState[]; players: PublicPlayerState[] }) {
  const abstainers = props.votes.filter((vote) => vote.has_voted && vote.abstain);
  if (!abstainers.length) {
    return null;
  }
  return (
    <div className="vote-math-panel">
      <span>Воздержались</span>
      <div className="vote-math-list">
        {abstainers.map((vote) => (
          <VoteMathLine key={vote.user_id} vote={vote} players={props.players} />
        ))}
      </div>
    </div>
  );
}

function VoteMathLine(props: { vote: PublicVoteState; players: PublicPlayerState[] }) {
  const player = props.players.find((item) => item.user_id === props.vote.user_id);
  const name = player?.name ?? playerName(props.players, props.vote.user_id);
  return (
    <small className="vote-math-line">
      {name}: {formatShare(props.vote.share_bps)} + {formatShare(props.vote.authority_bps)} = {formatShare(props.vote.voting_power_bps)}
    </small>
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
          <div>
            <span>Раунд {report.round}</span>
            <strong>{governanceReportText(report, props.players)}</strong>
          </div>
          {report.votes?.length ? (
            <div className="governance-report-votes">
              {report.votes.map((vote) => (
                <div className="governance-report-vote" key={`${report.round}-${vote.abstain ? "abstain" : vote.proposal_id}`}>
                  <span>{vote.abstain ? "Воздержались" : `Предложение #${vote.proposal_id}`}</span>
                  <strong>{formatShare(vote.voting_power_bps)}</strong>
                  <div className="vote-math-list">
                    {(vote.voters ?? []).map((voter) => (
                      <small className="vote-math-line" key={voter.user_id}>
                        {voter.name}: {formatShare(voter.share_bps)} + {formatShare(voter.authority_bps)} = {formatShare(voter.voting_power_bps)}
                      </small>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          ) : null}
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
              <div className="chat-message-head">
                <span className="chat-author">
                  <UserAvatar name={message.user_name} avatarUrl={message.avatar_url} size="small" />
                  <strong>{isMine ? "Ты" : message.user_name}</strong>
                </span>
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
  canBan: boolean;
  isSubmitting: boolean;
  onKick: () => void;
  onBan: () => void;
}) {
  return (
    <article className={props.player.user_id === props.currentUserId ? "player-card is-current" : "player-card"}>
      <div className="player-card-heading">
        <UserAvatar name={props.player.name} avatarUrl={props.player.avatar_url} size="medium" />
        <div>
          <h2>{props.player.name}</h2>
          <p>Доля {formatShare(props.player.share_bps)} · Полномочия {formatShare(props.player.authority_bps)}</p>
        </div>
      </div>
      <div className="badge-row">
        {props.player.is_host ? <span className="badge">Host</span> : null}
        {props.player.is_ceo ? <span className="badge accent">CEO</span> : null}
        {props.player.user_id === props.currentUserId ? <span className="badge current">Вы</span> : null}
      </div>
      {props.canKick || props.canBan ? (
        <div className="player-actions">
          {props.canKick ? (
            <button className="secondary-action" onClick={props.onKick} disabled={props.isSubmitting}>
              Кикнуть
            </button>
          ) : null}
          {props.canBan ? (
            <button className="kick-button" onClick={props.onBan} disabled={props.isSubmitting}>
              Забанить
            </button>
          ) : null}
        </div>
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
            <div>
              <span>{vote.abstain ? "Воздержались" : decisionLabel(vote.decision)}</span>
              <small>{(vote.voters ?? []).map((voter) => voter.name).join(", ") || formatVotesCount(vote.voter_count)}</small>
            </div>
            <strong>{formatShare(vote.share_bps)}</strong>
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

function DecisionTypeTag({ type }: { type: DecisionType }) {
  const isEmpowerment = type === "empowerment";
  return (
    <span
      className={isEmpowerment ? "decision-type-tag empowerment" : "decision-type-tag growth"}
      title={isEmpowerment ? "Победители получают +1% к Полномочиям" : "Победители получают +1% к доле"}
    >
      +1%
    </span>
  );
}

function Toast({ message, tone = "error", onClose }: { message: string | null; tone?: "error" | "success"; onClose: () => void }) {
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
