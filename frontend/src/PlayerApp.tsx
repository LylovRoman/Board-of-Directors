import { useCallback, useEffect, useMemo, useState } from "react";
import {
  changePassword,
  createGame,
  getLeaderboard,
  getMe,
  getGameState,
  getMyProfile,
  getUserProfile,
  listGames,
  login,
  register,
  respectUser,
  sendGameAction,
  updateMyProfile,
  WS_BASE_URL,
} from "./api";
import {
  AUTH_SESSION_CLEARED_EVENT,
  clearAuthSession,
  getAuthToken,
  readStoredAuthSession,
  saveAuthSession,
  saveAuthUser,
} from "./authSession";
import { playMusic, playSfx, preloadAudio, stopMusic } from "./audio";
import type { MusicName } from "./audio";
import type {
  ActionType,
  AuthUser,
  Game,
  GameStatus,
  LeaderboardEntry,
  LeaderboardSort,
  Profile,
  PublicGameState,
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
import { LoadingBlock } from "./components/LoadingBlock";
import { LobbyStat, LeaderboardTable, RoomTable } from "./components/LobbyWidgets";
import { ProfileDialog } from "./components/ProfileDialog";
import { RulesDialog } from "./components/RulesDialog";
import { Toast } from "./components/Toast";
import { TutorialDialog } from "./components/TutorialDialog";
import { UserAvatar } from "./components/UserAvatar";
import { FinishScreen } from "./screens/FinishScreen";
import { GameLobbyScreen } from "./screens/GameLobbyScreen";
import { StartedGameScreen } from "./screens/StartedGameScreen";
import {
  ACTION_SFX,
  DECISION_TYPE_FALLBACK,
  LEADERBOARD_HIDDEN_STORAGE_KEY,
  SELECTED_GAME_STORAGE_KEY,
  SOUND_STORAGE_KEY,
} from "./gameData";
import type { AuthMode, GameCard, LobbySort, LiveStatus } from "./gameData";
import { getErrorMessage, liveStatusLabel } from "./gameText";
import { readStoredBoolean, readStoredNumber } from "./storage";


export default function PlayerApp() {
  const [authSession, setAuthSession] = useState(() => readStoredAuthSession());
  const [isAuthChecking, setIsAuthChecking] = useState(() => Boolean(getAuthToken()));
  const [games, setGames] = useState<Game[]>([]);
  const [gameCards, setGameCards] = useState<GameCard[]>([]);
  const [leaderboardEntries, setLeaderboardEntries] = useState<LeaderboardEntry[]>([]);
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
  const [lobbyStatusFilter, setLobbyStatusFilter] = useState<GameStatus | "all">("lobby");
  const [onlyMyGames, setOnlyMyGames] = useState(false);
  const [lobbySort, setLobbySort] = useState<LobbySort>("newest");
  const [leaderboardSort, setLeaderboardSort] = useState<LeaderboardSort>("winrate");
  const [isLeaderboardHidden, setIsLeaderboardHidden] = useState(() => readStoredBoolean(LEADERBOARD_HIDDEN_STORAGE_KEY));
  const [isRulesOpen, setIsRulesOpen] = useState(false);
  const [isTutorialOpen, setIsTutorialOpen] = useState(false);
  const [soundEnabled, setSoundEnabled] = useState(() => readStoredBoolean(SOUND_STORAGE_KEY));
  const [liveStatus, setLiveStatus] = useState<LiveStatus>("idle");
  const [isCreatingGame, setIsCreatingGame] = useState(false);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [profileUserId, setProfileUserId] = useState<number | null>(null);
  const [profileName, setProfileName] = useState("");
  const [profileAvatarUrl, setProfileAvatarUrl] = useState("");
  const [profilePosition, setProfilePosition] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const currentUser: AuthUser | null = authSession?.user ?? null;
  const currentUserId = currentUser?.id ?? null;
  const activeMusic = useMemo<MusicName | null>(() => {
    if (!currentUserId) {
      return null;
    }
    if (selectedGameId && gameState?.is_finished) {
      return "finale";
    }
    if (selectedGameId) {
      return "meeting";
    }
    return "lobby";
  }, [currentUserId, gameState?.is_finished, selectedGameId]);
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
  const canChooseMemorandum = availableActions.includes("choose_memorandum");
  const canPlaceComplianceWatch = availableActions.includes("place_compliance_watch");
  const canSubmitGovernanceProposal = availableActions.includes("submit_governance_proposal");
  const canSkipGovernanceProposal = availableActions.includes("skip_governance_proposal");
  const canJoin = availableActions.includes("join_game");
  const canLeave = availableActions.includes("leave_game");
  const canStart = availableActions.includes("start_game");
  const canKick = availableActions.includes("kick_player");
  const canBan = availableActions.includes("ban_player");
  const canAddBot = availableActions.includes("add_bot");
  const canSendChatMessage = availableActions.includes("send_chat_message");
  const lobbyStats = useMemo(() => {
    const active = gameCards.filter(({ game }) => game.status === "started").length;
    const waiting = gameCards.filter(({ game }) => game.status === "lobby").length;
    const finished = gameCards.filter(({ game }) => game.status === "finished").length;
    const mine = gameCards.filter(({ game }) => Boolean(game.is_member || (currentUserId && game.player_user_ids?.includes(currentUserId)))).length;
    return { active, waiting, finished, mine };
  }, [currentUserId, gameCards]);

  const filteredGameCards = useMemo(() => {
    const normalizedFilter = lobbyFilter.trim().toLowerCase();
    const filtered = gameCards.filter(({ game }) => {
      const title = game.title.toLowerCase();
      const companyName = game.company_name?.toLowerCase() ?? "";
      const matchesText = !normalizedFilter || title.includes(normalizedFilter) || companyName.includes(normalizedFilter);
      const matchesStatus = lobbyStatusFilter === "all" || game.status === lobbyStatusFilter;
      const matchesOwner =
          !onlyMyGames ||
          Boolean(game.is_member || (currentUserId && game.player_user_ids?.includes(currentUserId)));
      return matchesText && matchesStatus && matchesOwner;
    });
    return [...filtered].sort((left, right) => {
      if (lobbySort === "players") {
        return (right.game.player_count ?? 0) - (left.game.player_count ?? 0);
      }
      if (lobbySort === "round") {
        return (right.game.current_round ?? 0) - (left.game.current_round ?? 0);
      }
      return new Date(right.game.created_at).getTime() - new Date(left.game.created_at).getTime();
    });
  }, [currentUserId, gameCards, lobbyFilter, lobbySort, lobbyStatusFilter, onlyMyGames]);

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

  const loadLeaderboard = useCallback(async () => {
    try {
      const response = await getLeaderboard("week", leaderboardSort);
      setLeaderboardEntries(response.entries ?? []);
      return response.entries ?? [];
    } catch (error) {
      showError(error);
      return [];
    }
  }, [leaderboardSort, showError]);

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
      await Promise.all([loadGames(), loadLeaderboard()]);
    } finally {
      setIsLoading(false);
    }
  }, [currentUserId, loadGames, loadLeaderboard]);

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
      setLeaderboardEntries([]);
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
    if (!isAuthChecking && currentUserId) {
      void loadLeaderboard();
    }
  }, [currentUserId, isAuthChecking, loadLeaderboard]);


  useEffect(() => {
    if (selectedGameId !== null) {
      window.localStorage.setItem(SELECTED_GAME_STORAGE_KEY, String(selectedGameId));
    } else {
      window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
    }
  }, [selectedGameId]);

  useEffect(() => {
    preloadAudio();
    return () => stopMusic();
  }, []);

  useEffect(() => {
    window.localStorage.setItem(SOUND_STORAGE_KEY, String(soundEnabled));
  }, [soundEnabled]);

  useEffect(() => {
    if (!soundEnabled || !activeMusic) {
      stopMusic();
      return;
    }
    playMusic(activeMusic, true);
  }, [activeMusic, soundEnabled]);

  useEffect(() => {
    if (!soundEnabled) {
      return undefined;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }
      if (target.closest("button:not(:disabled), [role='button']")) {
        playSfx("click", true);
      }
    };

    window.addEventListener("pointerdown", handlePointerDown, { capture: true });
    return () => window.removeEventListener("pointerdown", handlePointerDown, { capture: true });
  }, [soundEnabled]);

  useEffect(() => {
    if (!currentUserId || !selectedGameId) {
      setLiveStatus("idle");
      return undefined;
    }

    const token = getAuthToken();
    if (!token) {
      setLiveStatus("fallback");
      return undefined;
    }

    let socket: WebSocket | null = null;
    let reconnectId: number | null = null;
    let closed = false;
    let reconnectAttempts = 0;

    const connect = () => {
      setLiveStatus(reconnectAttempts === 0 ? "connecting" : "reconnecting");
      socket = new WebSocket(`${WS_BASE_URL}/games/${selectedGameId}/ws?token=${encodeURIComponent(token)}`);
      socket.onopen = () => {
        reconnectAttempts = 0;
        setLiveStatus("connected");
      };
      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as { type?: string; state?: PublicGameState };
          if (data.type === "state" && data.state) {
            setGameState((previous) => {
              if (previous && previous.phase !== data.state?.phase) {
                playSfx("phase", soundEnabled);
              }
              if (previous && !previous.is_finished && data.state?.is_finished) {
                playSfx("finish", soundEnabled);
              }
              const previousMessages = previous?.chat_messages ?? [];
              const nextMessages = data.state?.chat_messages ?? [];
              const latestMessage = nextMessages[nextMessages.length - 1];
              if (previous && nextMessages.length > previousMessages.length && latestMessage?.user_id !== currentUserId) {
                playSfx("chat-receive", soundEnabled);
              }
              return data.state ?? previous;
            });
          }
        } catch {
          // Ignore malformed live messages; the next state push will recover the view.
        }
      };
      socket.onclose = () => {
        if (!closed) {
          reconnectAttempts += 1;
          if (reconnectAttempts > 5) {
            setLiveStatus("fallback");
            void getGameState(selectedGameId)
                .then(setGameState)
                .catch(() => undefined);
            return;
          }
          reconnectId = window.setTimeout(connect, Math.min(8000, 900 * reconnectAttempts));
        }
      };
      socket.onerror = () => {
        socket?.close();
      };
    };

    connect();

    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") {
        void loadGameState(selectedGameId);
      }
    };
    document.addEventListener("visibilitychange", refreshWhenVisible);

    return () => {
      closed = true;
      setLiveStatus("idle");
      if (reconnectId !== null) {
        window.clearTimeout(reconnectId);
      }
      document.removeEventListener("visibilitychange", refreshWhenVisible);
      socket?.close();
    };
  }, [currentUserId, loadGameState, selectedGameId, soundEnabled]);

  useEffect(() => {
    if (!currentUserId || selectedGameId) {
      return undefined;
    }

    const token = getAuthToken();
    if (!token) {
      setLiveStatus("fallback");
      return undefined;
    }

    let socket: WebSocket | null = null;
    let reconnectId: number | null = null;
    let closed = false;
    let reconnectAttempts = 0;

    const connect = () => {
      setLiveStatus(reconnectAttempts === 0 ? "connecting" : "reconnecting");
      socket = new WebSocket(`${WS_BASE_URL}/games/ws?token=${encodeURIComponent(token)}`);
      socket.onopen = () => {
        reconnectAttempts = 0;
        setLiveStatus("connected");
      };
      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as { type?: string; games?: Game[] };
          if (data.type === "games" && Array.isArray(data.games)) {
            setGames(data.games);
            setGameCards(data.games.map((game) => ({ game })));
          }
        } catch {
          // Ignore malformed live messages; fallback polling will recover if needed.
        }
      };
      socket.onclose = () => {
        if (!closed) {
          reconnectAttempts += 1;
          if (reconnectAttempts > 5) {
            setLiveStatus("fallback");
            void Promise.all([loadGames(), loadLeaderboard()]);
            return;
          }
          reconnectId = window.setTimeout(connect, Math.min(8000, 900 * reconnectAttempts));
        }
      };
      socket.onerror = () => {
        socket?.close();
      };
    };

    connect();

    return () => {
      closed = true;
      if (reconnectId !== null) {
        window.clearTimeout(reconnectId);
      }
      socket?.close();
    };
  }, [currentUserId, loadGames, loadLeaderboard, selectedGameId]);

  useEffect(() => {
    if (!selectedGameId || liveStatus !== "fallback") {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void getGameState(selectedGameId)
          .then(setGameState)
          .catch(() => undefined);
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [liveStatus, loadGameState, selectedGameId]);

  useEffect(() => {
    if (selectedGameId || liveStatus !== "fallback") {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void Promise.all([loadGames(), loadLeaderboard()]);
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [liveStatus, loadGames, loadLeaderboard, selectedGameId]);

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
      await Promise.all([loadGames(), loadLeaderboard()]);
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
        const actionSound = ACTION_SFX[type];
        if (actionSound) {
          playSfx(actionSound, soundEnabled);
        }
        if (gameState && gameState.phase !== response.state.phase) {
          playSfx("phase", soundEnabled);
        }
        if (gameState && !gameState.is_finished && response.state.is_finished) {
          playSfx("finish", soundEnabled);
        }
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

  async function handleChatReaction(messageId: number, emoji: string) {
    await handleAction("react_chat_message", { message_id: messageId, emoji });
  }

  async function openProfile(userId?: number) {
    const targetUserId = userId ?? currentUserId;
    if (!targetUserId || targetUserId <= 0) {
      return;
    }
    setProfileUserId(targetUserId);
    setIsProfileOpen(true);
    setProfile(null);
    setProfileName("");
    setProfileAvatarUrl("");
    setProfilePosition("");
    setIsLoading(true);
    setErrorMessage(null);
    try {
      const nextProfile = targetUserId === currentUserId ? await getMyProfile() : await getUserProfile(targetUserId);
      setProfile(nextProfile);
      setProfileName(nextProfile.name);
      setProfileAvatarUrl(nextProfile.avatar_url ?? "");
      setProfilePosition(nextProfile.company_position ?? "");
    } catch (error) {
      showError(error);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleRespectProfile() {
    if (!profileUserId || profileUserId === currentUserId) {
      return;
    }
    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const nextProfile = await respectUser(profileUserId);
      setProfile(nextProfile);
      setSuccessMessage("+ respect");
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleProfileSubmit(event: React.FormEvent) {
    event.preventDefault();
    const name = profileName.trim();
    const avatarUrl = profileAvatarUrl.trim();
    const position = profilePosition.trim();
    if (!name) {
      setErrorMessage("Введите имя.");
      setSuccessMessage(null);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    setSuccessMessage(null);
    try {
      const user = await updateMyProfile({ name, avatar_url: avatarUrl, company_position: position });
      saveAuthUser(user);
      setAuthSession((session) => (session ? { ...session, user } : null));
      const nextProfile = await getMyProfile();
      setProfile(nextProfile);
      setProfileName(nextProfile.name);
      setProfileAvatarUrl(nextProfile.avatar_url ?? "");
      setProfilePosition(nextProfile.company_position ?? "");
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
    setLeaderboardEntries([]);
    setAuthPassword("");
    setCurrentPassword("");
    setNewPassword("");
    window.localStorage.removeItem(SELECTED_GAME_STORAGE_KEY);
  }

  function handleSoundToggle() {
    const nextEnabled = !soundEnabled;
    setSoundEnabled(nextEnabled);

    if (nextEnabled) {
      playSfx("click", true);
      if (activeMusic) {
        playMusic(activeMusic, true);
      }
      return;
    }

    stopMusic();
  }

  function applyLobbySummaryFilter(filter: GameStatus | "mine") {
    setLobbyFilter("");
    setLobbySort("newest");
    if (filter === "mine") {
      setOnlyMyGames((value) => !value);
      setLobbyStatusFilter("all");
      return;
    }
    setOnlyMyGames(false);
    setLobbyStatusFilter(filter);
  }

  function toggleLeaderboardVisibility() {
    setIsLeaderboardHidden((value) => {
      const next = !value;
      window.localStorage.setItem(LEADERBOARD_HIDDEN_STORAGE_KEY, String(next));
      return next;
    });
  }

  function showLeaderboardSort(sort: LeaderboardSort) {
    setLeaderboardSort(sort);
    setIsLeaderboardHidden(false);
    window.localStorage.setItem(LEADERBOARD_HIDDEN_STORAGE_KEY, "false");
  }

  if (isAuthChecking && !currentUser) {
    return (
        <main className="play-shell welcome-screen">
          <LoadingBlock label="Проверяю сессию" />
        </main>
    );
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
          <div className="brand-lockup" onClick={handleBackToGames}>
            <span>Board of Directors</span>
            <small>тайное заседание</small>
          </div>
          <div className="topbar-tools">
            <span className={`live-pill live-${liveStatus}`}>{liveStatusLabel(liveStatus)}</span>
            <button className="mini-button" onClick={() => setIsRulesOpen(true)}>Правила</button>
            <button className="mini-button" onClick={() => setIsTutorialOpen(true)}>Обучение</button>
            <button className={soundEnabled ? "mini-button active" : "mini-button"} onClick={handleSoundToggle}>
              {soundEnabled ? "Звук: вкл" : "Звук: выкл"}
            </button>
          </div>
          <div className="player-chip">
            <button className="profile-chip-button" onClick={() => void openProfile()}>
              <UserAvatar name={currentUser.name} avatarUrl={currentUser.avatar_url} size="small" />
              <span>{currentUser.name}</span>
            </button>
            <button className="mini-button" onClick={handleLogout}>
              Выйти
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
                profilePosition={profilePosition}
                currentPassword={currentPassword}
                newPassword={newPassword}
                isLoading={isLoading}
                isSubmitting={isSubmitting}
                canEdit={profileUserId === currentUserId}
                canRespect={Boolean(profileUserId && profileUserId !== currentUserId && !profile?.respected_by_me)}
                onRespect={() => void handleRespectProfile()}
                onProfileNameChange={setProfileName}
                onProfileAvatarUrlChange={setProfileAvatarUrl}
                onProfilePositionChange={setProfilePosition}
                onCurrentPasswordChange={setCurrentPassword}
                onNewPasswordChange={setNewPassword}
                onSubmitProfile={handleProfileSubmit}
                onSubmitPassword={handlePasswordSubmit}
                onClose={() => {
                  setIsProfileOpen(false);
                  setProfileUserId(null);
                  setProfile(null);
                }}
            />
        ) : null}
        {isRulesOpen ? <RulesDialog onClose={() => setIsRulesOpen(false)} /> : null}
        {isTutorialOpen ? <TutorialDialog onClose={() => setIsTutorialOpen(false)} /> : null}

        {!selectedGameId ? (
            <section className="lobby-browser">
              <div className="section-heading">
                <div>
                  <p className="eyebrow">лобби</p>
                </div>
                <div className="toolbar-actions">
                  <button className="primary-action" onClick={() => setIsCreatingGame((value) => !value)}>
                    Создать новую игру
                  </button>
                </div>
              </div>

              <div className="lobby-summary-grid">
                <LobbyStat label="Ожидают" value={lobbyStats.waiting} active={lobbyStatusFilter === "lobby" && !onlyMyGames} onClick={() => applyLobbySummaryFilter("lobby")} />
                <LobbyStat label="Идут" value={lobbyStats.active} active={lobbyStatusFilter === "started" && !onlyMyGames} onClick={() => applyLobbySummaryFilter("started")} />
                <LobbyStat label="Завершены" value={lobbyStats.finished} active={lobbyStatusFilter === "finished" && !onlyMyGames} onClick={() => applyLobbySummaryFilter("finished")} />
                <LobbyStat label="Мои" value={lobbyStats.mine} active={onlyMyGames} onClick={() => applyLobbySummaryFilter("mine")} />
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

              {false ? <div className="lobby-filters">
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
                <select
                    value={lobbySort}
                    onChange={(event) => setLobbySort(event.target.value as LobbySort)}
                    aria-label="Сортировка"
                >
                  <option value="newest">Сначала новые</option>
                  <option value="players">Больше игроков</option>
                  <option value="round">Поздний раунд</option>
                </select>
                <label className="checkbox filter-checkbox">
                  <input
                      type="checkbox"
                      checked={onlyMyGames}
                      onChange={(event) => setOnlyMyGames(event.target.checked)}
                  />
                  Только мои игры
                </label>
              </div> : null}

              <div className="room-table-shell">
                {isLoading && !gameCards.length ? (
                    <LoadingBlock label="Загружаю комнаты" />
                ) : filteredGameCards.length ? (
                    <RoomTable games={filteredGameCards.map(({ game }) => game)} onOpen={(gameId) => void openGame(gameId)} />
                ) : null}
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

              <div className="section-heading">
                <div>
                  <p className="eyebrow">Рейтинг недели</p>
                </div>
              </div>
              <div className="lobby-summary-grid leaderboard-switch" role="group" aria-label="Рейтинг недели">
                <>
                  {([
                    { value: "winrate", label: "Винрейт", hint: "победы" },
                    { value: "respect", label: "Респект", hint: "respect" },
                    { value: "accuracy", label: "Точность", hint: "major vote" },
                  ] as const).map((option) => (
                      <button
                          key={option.value}
                          type="button"
                          className={
                            leaderboardSort === option.value
                                ? "lobby-stat active"
                                : "lobby-stat"
                          }
                          onClick={() => setLeaderboardSort(option.value)}
                      >
                        <strong>{option.label}</strong>
                      </button>
                  ))}
                </>
              </div>
              <LeaderboardTable
                  entries={leaderboardEntries}
                  sort={leaderboardSort}
                  hidden={isLeaderboardHidden}
                  onSortChange={showLeaderboardSort}
                  onToggleHidden={toggleLeaderboardVisibility}
                  onOpenProfile={(userId) => void openProfile(userId)}
              />
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
                onReactChatMessage={(messageId, emoji) => handleChatReaction(messageId, emoji)}
                onOpenProfile={(userId) => void openProfile(userId)}
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
                memorandum={gameState.memorandum ?? null}
                memorandumPreference={gameState.memorandum_preference}
                moleVictoryPoints={gameState.mole_victory_points}
                playersVictoryPoints={gameState.players_victory_points}
                currentVotes={currentVotes}
                hasVoted={hasVoted}
                myCurrentVote={gameState.my_current_vote ?? null}
                canVote={canVote}
                canSelectMoleObjectives={canSelectMoleObjectives}
                canChooseMemorandum={canChooseMemorandum}
                canPlaceComplianceWatch={canPlaceComplianceWatch}
                canSubmitGovernanceProposal={canSubmitGovernanceProposal}
                canSkipGovernanceProposal={canSkipGovernanceProposal}
                canSendChatMessage={canSendChatMessage}
                isSubmitting={isSubmitting}
                onSelectMoleObjectives={(payload) => void handleAction("select_mole_objectives", payload)}
                onChooseMemorandum={(type) => void handleAction("choose_memorandum", { type })}
                onPlaceComplianceWatch={(targetUserId) => void handleAction("place_compliance_watch", { target_user_id: targetUserId })}
                onVote={(decision) => void handleAction("vote", { decision, abstain: false })}
                onVoteProposal={(proposalId) => void handleAction("vote", { proposal_id: proposalId, abstain: false })}
                onAbstain={() => void handleAction("vote", { abstain: true })}
                onSubmitGovernanceProposal={(payload) => void handleAction("submit_governance_proposal", payload)}
                onSkipGovernanceProposal={() => void handleAction("skip_governance_proposal")}
                onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
                onReactChatMessage={(messageId, emoji) => handleChatReaction(messageId, emoji)}
                onOpenProfile={(userId) => void openProfile(userId)}
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
                canAddBot={canAddBot}
                hasMe={hasMe}
                chatMessages={chatMessages}
                canSendChatMessage={canSendChatMessage}
                isLoading={isLoading}
                isSubmitting={isSubmitting}
                onJoin={() => void handleAction("join_game")}
                onLeave={() => void handleLeaveGame()}
                onStart={() => void handleAction("start_game")}
                onAddBot={() => void handleAction("add_bot")}
                onKick={(userId) => void handleAction("kick_player", { user_id: userId })}
                onBan={(userId) => void handleAction("ban_player", { user_id: userId })}
                onSendChatMessage={(message) => handleAction("send_chat_message", { message })}
                onReactChatMessage={(messageId, emoji) => handleChatReaction(messageId, emoji)}
                onOpenProfile={(userId) => void openProfile(userId)}
                onRefresh={handleManualRefresh}
            />
        )}
      </main>
  );
}
