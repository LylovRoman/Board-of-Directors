import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  changePassword,
  createGame,
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
import type {
  ActionType,
  AuthUser,
  DecisionType,
  Game,
  GamePhase,
  GameStatus,
  GovernanceProposalType,
  MemorandumType,
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
const SOUND_STORAGE_KEY = "board-of-directors-sound-enabled";
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
type LiveStatus = "idle" | "connecting" | "connected" | "reconnecting" | "fallback";
type LobbySort = "newest" | "players" | "round";

function readStoredNumber(key: string): number | null {
  const raw = window.localStorage.getItem(key);
  if (!raw) {
    return null;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function readStoredBoolean(key: string): boolean {
  return window.localStorage.getItem(key) === "true";
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

function formatAccuracy(bps?: number): string {
  return formatShare(bps);
}

function liveStatusLabel(status: LiveStatus): string {
  switch (status) {
    case "connected":
      return "live";
    case "connecting":
      return "подключение";
    case "reconnecting":
      return "переподключение";
    case "fallback":
      return "обновление";
    default:
      return "offline";
  }
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

function finalDecisionClass(decision?: string, summary?: PublicGameState["final_summary"]): string {
  if (!decision || !summary) {
    return "final-decision-clean";
  }
  if (decision === summary.mole_sabotage) {
    return "final-decision-sabotage";
  }
  if (summary.mole_targets.includes(decision)) {
    return "final-decision-podkop";
  }
  return "final-decision-clean";
}

function memorandumTitle(type?: MemorandumType): string {
  return type === "risk" ? "Учитываю риски" : "Вижу возможности";
}

function memorandumRule(type?: MemorandumType): string {
  return type === "risk"
    ? "В этой тройке по крайней мере одно решение является целью крота."
    : "В этой тройке по крайней мере одно решение не является целью крота.";
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
      return `${playerName(players, proposal.from_user_id)} передает долю игроку ${playerName(
        players,
        proposal.to_user_id,
      )}`;
    case "treasury_grant":
      return `Выдать долю из резерва игроку ${playerName(players, proposal.target_user_id)}`;
    case "treasury_buyback":
      return `Оштрафовать ${playerName(players, proposal.target_user_id)} на долю в пользу резерва`;
    case "appoint_ceo":
      return `Назначить CEO: ${playerName(players, proposal.target_user_id)}`;
    default:
      return "Корпоративный манёвр";
  }
}

function governanceVoteTitle(
  vote: { abstain?: boolean; proposal_title?: string; proposal?: PublicGovernanceProposal; proposal_id?: number },
  players: PublicPlayerState[],
): string {
  if (vote.abstain) {
    return "Воздержались";
  }
  if (vote.proposal_title) {
    return vote.proposal_title;
  }
  if (vote.proposal) {
    return describeGovernanceProposal(vote.proposal, players);
  }
  return "Корпоративный маневр";
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

function playUiSound(kind: "vote" | "phase" | "finish", enabled: boolean) {
  if (!enabled) {
    return;
  }
  const AudioContextConstructor =
    window.AudioContext || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioContextConstructor) {
    return;
  }
  const context = new AudioContextConstructor();
  const oscillator = context.createOscillator();
  const gain = context.createGain();
  oscillator.frequency.value = kind === "finish" ? 420 : kind === "phase" ? 320 : 240;
  oscillator.type = "sine";
  gain.gain.setValueAtTime(0.0001, context.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.08, context.currentTime + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.0001, context.currentTime + 0.18);
  oscillator.connect(gain);
  gain.connect(context.destination);
  oscillator.start();
  oscillator.stop(context.currentTime + 0.2);
  window.setTimeout(() => void context.close(), 260);
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
  const [lobbySort, setLobbySort] = useState<LobbySort>("newest");
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
  const canSubmitGovernanceProposal = availableActions.includes("submit_governance_proposal");
  const canSkipGovernanceProposal = availableActions.includes("skip_governance_proposal");
  const canJoin = availableActions.includes("join_game");
  const canLeave = availableActions.includes("leave_game");
  const canStart = availableActions.includes("start_game");
  const canKick = availableActions.includes("kick_player");
  const canBan = availableActions.includes("ban_player");
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
    window.localStorage.setItem(SOUND_STORAGE_KEY, String(soundEnabled));
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
                playUiSound("phase", soundEnabled);
              }
              if (previous && !previous.is_finished && data.state?.is_finished) {
                playUiSound("finish", soundEnabled);
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
            void loadGames();
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
  }, [currentUserId, loadGames, selectedGameId]);

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
      void loadGames();
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [liveStatus, loadGames, selectedGameId]);

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
        if (type === "vote") {
          playUiSound("vote", soundEnabled);
        }
        if (gameState && gameState.phase !== response.state.phase) {
          playUiSound("phase", soundEnabled);
        }
        if (gameState && !gameState.is_finished && response.state.is_finished) {
          playUiSound("finish", soundEnabled);
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
    if (!targetUserId) {
      return;
    }
    setProfileUserId(targetUserId);
    setIsProfileOpen(true);
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
        <div className="topbar-tools">
          <span className={`live-pill live-${liveStatus}`}>{liveStatusLabel(liveStatus)}</span>
          <button className="mini-button" onClick={() => setIsRulesOpen(true)}>Правила</button>
          <button className="mini-button" onClick={() => setIsTutorialOpen(true)}>Обучение</button>
          <button className={soundEnabled ? "mini-button active" : "mini-button"} onClick={() => setSoundEnabled((value) => !value)}>
            {soundEnabled ? "Звук: вкл" : "Звук: выкл"}
          </button>
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
              <h1>Выбери заседание</h1>
            </div>
            <div className="toolbar-actions">
              <button className="primary-action" onClick={() => setIsCreatingGame((value) => !value)}>
                Создать новую игру
              </button>
            </div>
          </div>

          <div className="lobby-summary-grid">
            <LobbyStat label="Ожидают" value={lobbyStats.waiting} />
            <LobbyStat label="Идут" value={lobbyStats.active} />
            <LobbyStat label="Завершены" value={lobbyStats.finished} />
            <LobbyStat label="Мои" value={lobbyStats.mine} />
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
          </div>

          <div className="game-card-grid">
            {filteredGameCards.map(({ game }) => (
              <article className={game.is_member ? "room-card is-member" : "room-card"} key={game.id}>
                <div>
                  <span className={`status-pill status-${game.status ?? "unknown"}`}>
                    {statusLabel(game.status)}
                  </span>
                  <h2>{game.title}</h2>
                  <p className="room-phase">{game.status === "finished" ? winnerLabel(game.winner) : phaseLabel(game.phase)}</p>
                </div>
                <div className="room-players">
                  {(game.players ?? []).slice(0, 6).map((player) => (
                    <UserAvatar key={player.user_id} name={player.name} avatarUrl={player.avatar_url} size="small" />
                  ))}
                  {(game.player_count ?? 0) > 6 ? <span className="avatar-overflow">+{(game.player_count ?? 0) - 6}</span> : null}
                </div>
                <div className="room-meta">
                  <span>{game.player_count ?? "?"} игроков</span>
                  <span>{game.current_round ? `Раунд ${game.current_round}` : "Перед стартом"}</span>
                  {game.is_member ? <span>Вы внутри</span> : null}
                </div>
                <button className="primary-action" onClick={() => void openGame(game.id)}>
                  {game.status === "finished" ? "Открыть реплей" : "Войти"}
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
          canSubmitGovernanceProposal={canSubmitGovernanceProposal}
          canSkipGovernanceProposal={canSkipGovernanceProposal}
          canSendChatMessage={canSendChatMessage}
          isSubmitting={isSubmitting}
          onSelectMoleObjectives={(payload) => void handleAction("select_mole_objectives", payload)}
          onChooseMemorandum={(type) => void handleAction("choose_memorandum", { type })}
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
          onReactChatMessage={(messageId, emoji) => handleChatReaction(messageId, emoji)}
          onOpenProfile={(userId) => void openProfile(userId)}
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

function LobbyStat(props: { label: string; value: number }) {
  return (
    <div className="lobby-stat">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
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
  profilePosition: string;
  currentPassword: string;
  newPassword: string;
  isLoading: boolean;
  isSubmitting: boolean;
  canEdit: boolean;
  canRespect: boolean;
  onProfileNameChange: (value: string) => void;
  onProfileAvatarUrlChange: (value: string) => void;
  onProfilePositionChange: (value: string) => void;
  onCurrentPasswordChange: (value: string) => void;
  onNewPasswordChange: (value: string) => void;
  onSubmitProfile: (event: React.FormEvent) => void;
  onSubmitPassword: (event: React.FormEvent) => void;
  onRespect: () => void;
  onClose: () => void;
}) {
  const shownName = props.profileName || props.currentUser.name;
  const shownAvatar = props.profileAvatarUrl || props.currentUser.avatar_url;
  const shownPosition = props.profilePosition || props.currentUser.company_position;
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
              {shownPosition ? <small>{shownPosition}</small> : null}
              {props.profile?.login || props.canEdit ? <span>@{props.profile?.login || props.currentUser.login}</span> : null}
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
              <div className="profile-stat">
                <span>Respect</span>
                <strong>{props.profile?.respect_count ?? 0}</strong>
                <small>{props.profile?.respected_by_me ? "Уже выражен" : "Уважение"}</small>
              </div>
            </div>

            {!props.canEdit ? (
              <button className="primary-action" type="button" onClick={props.onRespect} disabled={!props.canRespect || props.isSubmitting}>
                + respect
              </button>
            ) : null}

            {props.canEdit ? (
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
                Должность в компании
                <input
                  value={props.profilePosition}
                  onChange={(event) => props.onProfilePositionChange(event.target.value)}
                  maxLength={64}
                  placeholder="Например, финансовый директор"
                  autoComplete="organization-title"
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
            ) : null}

            {props.canEdit ? (
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
            ) : null}
          </>
        )}
      </section>
    </div>
  );
}

function RulesDialog(props: { onClose: () => void }) {
  const tabs = [
    { id: "goal", label: "Цель", text: "Совет побеждает, если принимает три чистых решения. Крот побеждает, если через голосования проводит свои цели; диверсия стоит два очка." },
    { id: "roles", label: "Роли", text: "Директора видят личный меморандум с подсказками. Крот знает свои цели и пытается сделать их похожими на выгодные решения." },
    { id: "major", label: "Major vote", text: "В major vote каждый выбирает одну карточку. Голос можно менять до закрытия раунда. Принятое решение награждает поддержавших игроков." },
    { id: "governance", label: "Governance", text: "После принятого major решения игроки предлагают маневры с долями и полномочиями, затем голосуют за предложения. В этой фазе голос тоже можно менять до финального подсчета." },
    { id: "win", label: "Победа", text: "После финала раскрываются Крот, цели, диверсия, победители и точность major-голосов каждого игрока." },
    { id: "faq", label: "FAQ", text: "CEO получает бонус к полномочиям и не может воздерживаться в governance. Governance-голоса публичны, major-голоса раскрываются в отчетах раунда." },
  ];
  const [activeTab, setActiveTab] = useState(tabs[0].id);
  const active = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="rules-dialog" role="dialog" aria-modal="true" aria-labelledby="rules-title">
        <div className="profile-dialog-header">
          <div>
            <p className="eyebrow">справочник</p>
            <h2 id="rules-title">Правила игры</h2>
          </div>
          <button className="mini-button" onClick={props.onClose}>Закрыть</button>
        </div>
        <div className="rule-tabs" role="tablist">
          {tabs.map((tab) => (
            <button key={tab.id} className={tab.id === activeTab ? "mini-button active" : "mini-button"} onClick={() => setActiveTab(tab.id)}>
              {tab.label}
            </button>
          ))}
        </div>
        <div className="rule-body">
          <h3>{active.label}</h3>
          <p>{active.text}</p>
        </div>
      </section>
    </div>
  );
}

function TutorialDialog(props: { onClose: () => void }) {
  const steps = [
    { title: "1. Получи роль", text: "Директор ищет безопасные решения, Крот собирает свои цели. Роль Крота скрыта до финала." },
    { title: "2. Прочитай меморандум", text: "Меморандум не говорит истину напрямую, но помогает оценить набор решений перед первым голосованием." },
    { title: "3. Голосуй и меняй выбор", text: "В major vote и governance можно изменить голос, пока все игроки еще не закрыли раунд." },
    { title: "4. Используй governance", text: "Предложения меняют доли, полномочия и CEO. Сила голоса равна доле плюс полномочия." },
    { title: "5. Читай финал", text: "После игры смотри победителей, ошибки, точность и реплей, чтобы понять, где Совет свернул не туда." },
  ];
  const [index, setIndex] = useState(0);
  const step = steps[index];
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="tutorial-dialog" role="dialog" aria-modal="true" aria-labelledby="tutorial-title">
        <div className="profile-dialog-header">
          <div>
            <p className="eyebrow">обучение</p>
            <h2 id="tutorial-title">Быстрый ввод в партию</h2>
          </div>
          <button className="mini-button" onClick={props.onClose}>Закрыть</button>
        </div>
        <div className="tutorial-card">
          <span>{index + 1} / {steps.length}</span>
          <h3>{step.title}</h3>
          <p>{step.text}</p>
        </div>
        <div className="tutorial-track">
          {steps.map((item, itemIndex) => (
            <button key={item.title} className={itemIndex === index ? "tutorial-dot active" : "tutorial-dot"} onClick={() => setIndex(itemIndex)} aria-label={item.title} />
          ))}
        </div>
        <div className="toolbar-actions centered-actions">
          <button className="secondary-action" onClick={() => setIndex((value) => Math.max(0, value - 1))} disabled={index === 0}>Назад</button>
          <button className="primary-action" onClick={() => setIndex((value) => Math.min(steps.length - 1, value + 1))} disabled={index === steps.length - 1}>Дальше</button>
        </div>
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
  onReactChatMessage: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
  onRefresh: () => Promise<void>;
}) {
  const state = props.state;

  return (
    <section className="game-stage">
      <div className="section-heading">
        <div>
          <p className="eyebrow">комната</p>
          <h1>{state?.title ?? "Загрузка комнаты"}</h1>
          {state?.company_name ? <p className="quiet-text">{state.company_name}: {state.company_situation}</p> : null}
        </div>
        <div className="toolbar-actions">
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
            onOpenProfile={() => props.onOpenProfile(player.user_id)}
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
        onReact={props.onReactChatMessage}
        onOpenProfile={props.onOpenProfile}
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
  memorandum: PublicGameState["memorandum"] | null;
  memorandumPreference?: MemorandumType;
  moleVictoryPoints?: number;
  playersVictoryPoints?: number;
  currentVotes: PublicVoteState[];
  hasVoted: boolean;
  myCurrentVote: PublicOwnVoteState | null;
  canVote: boolean;
  canSelectMoleObjectives: boolean;
  canChooseMemorandum: boolean;
  canSubmitGovernanceProposal: boolean;
  canSkipGovernanceProposal: boolean;
  canSendChatMessage: boolean;
  isSubmitting: boolean;
  onSelectMoleObjectives: (payload: Record<string, unknown>) => void;
  onChooseMemorandum: (type: MemorandumType) => void;
  onVote: (decision: string) => void;
  onVoteProposal: (proposalId: number) => void;
  onAbstain: () => void;
  onSubmitGovernanceProposal: (payload: Record<string, unknown>) => void;
  onSkipGovernanceProposal: () => void;
  onSendChatMessage: (message: string) => Promise<void>;
  onReactChatMessage: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
  onRefresh: () => Promise<void>;
  isLoading: boolean;
  currentUserId: number;
}) {
  const [selectedReport, setSelectedReport] = useState<PublicRoundReport | null>(null);
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");
  const directorRowRefs = useRef(new Map<number, HTMLDivElement>());
  const previousDirectorRects = useRef(new Map<number, DOMRect>());
  const sortedPlayers = useMemo(
    () =>
      [...props.players].sort((left, right) => {
        if (right.share_bps !== left.share_bps) {
          return right.share_bps - left.share_bps;
        }
        const byName = left.name.localeCompare(right.name);
        return byName !== 0 ? byName : left.user_id - right.user_id;
      }),
    [props.players],
  );
  const playerSortSignature = sortedPlayers.map((player) => `${player.user_id}:${player.share_bps}`).join("|");
  useLayoutEffect(() => {
    const nextRects = new Map<number, DOMRect>();
    directorRowRefs.current.forEach((node, userId) => {
      nextRects.set(userId, node.getBoundingClientRect());
    });
    nextRects.forEach((nextRect, userId) => {
      const previousRect = previousDirectorRects.current.get(userId);
      const node = directorRowRefs.current.get(userId);
      if (!previousRect || !node) {
        return;
      }
      const deltaY = previousRect.top - nextRect.top;
      if (Math.abs(deltaY) > 1) {
        node.animate(
          [{ transform: `translateY(${deltaY}px)` }, { transform: "translateY(0)" }],
          { duration: 240, easing: "cubic-bezier(0.22, 1, 0.36, 1)" },
        );
      }
    });
    previousDirectorRects.current = nextRects;
  }, [playerSortSignature]);
  const isWaitingForPlayer = (userId: number) => {
    if (props.phase === "governance_proposal") {
      return !props.governanceSubmissions.some((item) => item.user_id === userId && item.status);
    }
    return !props.currentVotes.some((item) => item.user_id === userId && item.has_voted);
  };
  const displayedMajorOptions = props.majorVoteOptions.length ? props.majorVoteOptions : props.availableDecisions;
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  const majorUnlockMs = props.state.major_vote_unlocked_at ? new Date(props.state.major_vote_unlocked_at).getTime() : 0;
  const majorVoteLocked = props.phase === "major_voting" && majorUnlockMs > nowMs;
  const majorVoteSecondsLeft = Math.max(0, Math.ceil((majorUnlockMs - nowMs) / 1000));
  const canSubmitMajorVote = props.canVote && !majorVoteLocked;

  return (
    <section className="game-stage">
      <div className="play-columns">
        <aside className="side-stack">

          <section className="directors-panel">
            <div className="director-list">
              <div className="director-row company-row">
                <div className="director-identity">
                  <UserAvatar name="Компания" size="small" />
                  <div>
                    <strong>{props.state.company_name || "Компания"}</strong>
                    <span>Казначейский резерв {formatShare(props.state.treasury_share_bps)}</span>
                  </div>
                </div>
              </div>
              {sortedPlayers.map((player) => (
                  <div
                      key={player.user_id}
                      ref={(node) => {
                        if (node) {
                          directorRowRefs.current.set(player.user_id, node);
                        } else {
                          directorRowRefs.current.delete(player.user_id);
                        }
                      }}
                      className={player.user_id === props.currentUserId ? "director-row is-current" : "director-row"}
                  >
                    <button className="director-identity profile-link" type="button" onClick={() => props.onOpenProfile(player.user_id)}>
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
                        {player.company_position ? <span>{player.company_position}</span> : null}
                        <span>
                        Доля {formatShare(player.share_bps)}
                      </span>
                        <span>
                        Полномочия {formatShare(player.authority_bps)}
                        </span>
                      </div>
                    </button>
                    <div className="badge-row">
                      {player.is_ceo ? <span className="badge accent">CEO</span> : null}
                    </div>
                  </div>
              ))}
            </div>
          </section>

          {props.me?.role === "mole" ? (
            <section className="secret-card">
              <p className="eyebrow">Подкопы</p>
              <DecisionList values={props.moleTargets} emptyText="Цели еще не выбраны." />
              {props.moleSabotage ? (
                <>
              <p className="eyebrow">Диверсия</p>
                <div className="sabotage-secret">
                  <strong>{decisionLabel(props.moleSabotage)}</strong>
                </div>
                </>
              ) : null}
              <p className="eyebrow">Счёт</p>
              <div className="score-row">
                <span>Крот: {props.moleVictoryPoints ?? 0}/3</span>
                <span>Совет: {props.playersVictoryPoints ?? 0}/3</span>
              </div>
            </section>
          ) : (
            <section className="secret-card">
              <p className="eyebrow">Меморандум</p>
              {props.memorandum ? (
                <>
                  <h3>{memorandumTitle(props.memorandum.type)}</h3>
                  <p className="quiet-text">{memorandumRule(props.memorandum.type)}</p>
                  <DecisionList values={props.memorandum.decisions} emptyText="Меморандум еще не получен." />
                </>
              ) : (
                <p className="quiet-text">
                  {props.memorandumPreference
                    ? `Выбран профиль: ${memorandumTitle(props.memorandumPreference)}.`
                    : "Выбери тип меморандума, пока крот формирует цели."}
                </p>
              )}
            </section>
          )}

        </aside>

        <div className="main-stack">

          {props.phase === "mole_objective_selection" ? (
            <MoleObjectiveSelectionPhase
              isMole={props.me?.role === "mole"}
              canSelect={props.canSelectMoleObjectives}
              canChooseMemorandum={props.canChooseMemorandum}
              memorandumPreference={props.memorandumPreference}
              isSubmitting={props.isSubmitting}
              onSubmit={props.onSelectMoleObjectives}
              onChooseMemorandum={props.onChooseMemorandum}
            />
          ) : props.phase === "governance_proposal" ? (
            <GovernanceProposalPhase
              players={props.players}
              submissions={props.governanceSubmissions}
              treasuryShareBPS={props.state.treasury_share_bps}
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
                </div>
                {majorVoteLocked ? <span className="wait-pill">Обсуждение: {majorVoteSecondsLeft}с</span> : props.hasVoted ? <span className="wait-pill">Выбор сохранён, можно изменить</span> : null}
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
                      disabled={!canSubmitMajorVote || props.isSubmitting}
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
            </section>
          )}

          <ChatPanel
              messages={props.chatMessages}
              currentUserId={props.currentUserId}
              canSend={props.canSendChatMessage}
              isSubmitting={props.isSubmitting}
              onSend={props.onSendChatMessage}
              onReact={props.onReactChatMessage}
              onOpenProfile={props.onOpenProfile}
          />
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
  onReactChatMessage: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
  onRefresh: () => Promise<void>;
  onBack: () => void;
  isLoading: boolean;
}) {
  const [replayOpen, setReplayOpen] = useState(false);
  const playerWon =
    props.state.winner === "mole" ? props.me?.role === "mole" : props.state.winner === "players" && props.me?.role !== "mole";
  const summary = props.state.final_summary;
  const playerStats = [...(summary?.player_stats ?? [])].sort((left, right) => {
    if (left.mistakes !== right.mistakes) {
      return left.mistakes - right.mistakes;
    }
    return right.accuracy_bps - left.accuracy_bps;
  });
  const winners = playerStats.filter((stat) => stat.won);
  const leastMistakes = new Set(summary?.least_mistake_user_ids ?? []);
  const acceptedReports = props.roundReports.filter((report) => report.outcome === "accepted");
  const riskyReports = props.roundReports.filter((report) => report.outcome !== "accepted");

  if (replayOpen) {
    return <ReplayPanel state={props.state} steps={props.state.replay_steps ?? []} onBack={() => setReplayOpen(false)} />;
  }

  return (
    <section className="finish-screen">
      <p className="eyebrow">финал</p>
      <h1>{winnerLabel(props.state.winner)}</h1>
      {props.state.company_name ? <p className="quiet-text">{props.state.company_name}: {props.state.company_situation}</p> : null}
      {props.me?.role ? (
        <p className="personal-result">
          {roleLabel(props.me.role)}: {playerWon ? "Ты победил" : "Ты проиграл"}
        </p>
      ) : null}
      <div className="final-score-row">
        <span>Крот {summary?.mole_points ?? props.state.mole_victory_points ?? 0}/3</span>
        <span>Совет {summary?.players_points ?? props.state.players_victory_points ?? 0}/3</span>
      </div>
      <div className="final-grid">
        <section className="final-panel">
          <p className="eyebrow">победители</p>
          <div className="winner-list">
            {winners.map((winner) => (
              <span key={winner.user_id}>{winner.name} · {roleLabel(winner.role)}</span>
            ))}
            {!winners.length ? <span>{winnerLabel(props.state.winner)}</span> : null}
          </div>
        </section>
        <section className="final-panel">
          <p className="eyebrow">раскрытие</p>
          <p>Крот: {playerStats.find((stat) => stat.user_id === summary?.mole_user_id)?.name ?? "неизвестно"}</p>
          <p style={{ whiteSpace: 'pre-line' }}>
            Цели: {(summary?.mole_targets ?? []).map(decisionLabel).join("\n") || "нет данных"}
          </p>
          <p>Диверсия: {summary?.mole_sabotage ? decisionLabel(summary.mole_sabotage) : "нет данных"}</p>
        </section>
        <section className="final-panel final-table-panel">
          <p className="eyebrow">точность голосов</p>
          <div className="final-stats-table">
            {playerStats.map((stat) => (
              <div className={leastMistakes.has(stat.user_id) ? "final-stat-row best" : "final-stat-row"} key={stat.user_id}>
                <span>{stat.name}</span>
                <span>{stat.mistakes} ошибок</span>
                <strong>{formatAccuracy(stat.accuracy_bps)}</strong>
              </div>
            ))}
          </div>
        </section>
        <section className="final-panel">
          <p className="eyebrow">ключевые решения</p>
          <div className="final-decision-list">
            {acceptedReports.slice(-5).map((report, index) => (
              <span className={finalDecisionClass(report.decision, summary)} key={`accepted-${report.round}`}>Решение {index + 1}: {report.decision ? decisionLabel(report.decision) : "принято"}</span>
            ))}
            {riskyReports.slice(-2).map((report, index) => (
              <span key={`risky-${report.round}`}>Спорное решение {index + 1}: {report.reason ?? "ничья"}</span>
            ))}
          </div>
        </section>
      </div>
      <ChatPanel
        messages={props.chatMessages}
        currentUserId={props.currentUserId}
        canSend={props.canSendChatMessage}
        isSubmitting={props.isSubmitting}
        onSend={props.onSendChatMessage}
        onReact={props.onReactChatMessage}
        onOpenProfile={props.onOpenProfile}
      />
      <div className="toolbar-actions centered-actions">
        <button className="primary-action" onClick={() => setReplayOpen(true)}>
          Смотреть реплей
        </button>
        <button className="secondary-action" onClick={() => void props.onRefresh()} disabled={props.isLoading}>
          Обновить
        </button>
        <button className="secondary-action" onClick={props.onBack}>
          К списку игр
        </button>
      </div>
    </section>
  );
}

function ReplayPanel(props: { state: PublicGameState; steps: NonNullable<PublicGameState["replay_steps"]>; onBack: () => void }) {
  const steps = props.steps.length
    ? props.steps
    : [{ id: "final", kind: "final", title: "Финал", summary: winnerLabel(props.state.winner), winner: props.state.winner }];
  const [index, setIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState(1200);
  const step = steps[Math.min(index, steps.length - 1)];

  useEffect(() => {
    if (!isPlaying) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      setIndex((value) => {
        if (value >= steps.length - 1) {
          setIsPlaying(false);
          return value;
        }
        return value + 1;
      });
    }, speed);
    return () => window.clearInterval(intervalId);
  }, [isPlaying, speed, steps.length]);

  return (
    <section className="replay-screen">
      <div className="section-heading">
        <div>
          <p className="eyebrow">реплей</p>
          <h1>{props.state.title}</h1>
          {props.state.company_name ? <p className="quiet-text">{props.state.company_name}</p> : null}
        </div>
        <div className="toolbar-actions">
          <button className="secondary-action" onClick={props.onBack}>К финалу</button>
        </div>
      </div>
      <div className="replay-layout">
        <aside className="replay-timeline">
          {steps.map((item, itemIndex) => (
            <button key={item.id} className={itemIndex === index ? "replay-step active" : "replay-step"} onClick={() => setIndex(itemIndex)}>
              <span>{itemIndex + 1}</span>
              <strong>{item.title}</strong>
            </button>
          ))}
        </aside>
        <section className="replay-detail">
          <p className="eyebrow">{step.kind}</p>
          <h2>{step.title}</h2>
          <p>{step.summary}</p>
          {step.decision ? <p>Решение: {decisionLabel(step.decision)}</p> : null}
          {step.winner ? <p>Победитель: {winnerLabel(step.winner)}</p> : null}
          {step.votes?.length ? (
            <div className="replay-votes">
              {step.votes.map((vote) => (
                <div className="round-report-row" key={`${step.id}-${vote.label}`}>
                  <div>
                    <span>{vote.label}</span>
                    <small>{vote.voters.join(", ") || "без голосов"}</small>
                  </div>
                  <strong>{formatShare(vote.voting_power_bps ?? vote.share_bps)}</strong>
                </div>
              ))}
            </div>
          ) : null}
        </section>
      </div>
      <div className="replay-controls">
        <button className="secondary-action" onClick={() => setIndex((value) => Math.max(0, value - 1))} disabled={index === 0}>Назад</button>
        <button className="primary-action" onClick={() => setIsPlaying((value) => !value)}>{isPlaying ? "Пауза" : "Play"}</button>
        <button className="secondary-action" onClick={() => setIndex((value) => Math.min(steps.length - 1, value + 1))} disabled={index === steps.length - 1}>Вперед</button>
        <select value={speed} onChange={(event) => setSpeed(Number(event.target.value))} aria-label="Скорость реплея">
          <option value={1800}>0.75x</option>
          <option value={1200}>1x</option>
          <option value={700}>1.5x</option>
        </select>
      </div>
    </section>
  );
}

function MoleObjectiveSelectionPhase(props: {
  isMole: boolean;
  canSelect: boolean;
  canChooseMemorandum: boolean;
  memorandumPreference?: MemorandumType;
  isSubmitting: boolean;
  onSubmit: (payload: Record<string, unknown>) => void;
  onChooseMemorandum: (type: MemorandumType) => void;
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
        {props.memorandumPreference ? (
          <div className="memorandum-choice selected">
            <strong>{memorandumTitle(props.memorandumPreference)}</strong>
            <span>{memorandumRule(props.memorandumPreference)}</span>
          </div>
        ) : (
          <div className="memorandum-choice-grid">
            <button
              type="button"
              className="memorandum-choice"
              disabled={!props.canChooseMemorandum || props.isSubmitting}
              onClick={() => props.onChooseMemorandum("opportunity")}
            >
              <strong>Принимая решения, я часто вижу возможности</strong>
              <span>{memorandumRule("opportunity")}</span>
            </button>
            <button
              type="button"
              className="memorandum-choice"
              disabled={!props.canChooseMemorandum || props.isSubmitting}
              onClick={() => props.onChooseMemorandum("risk")}
            >
              <strong>Принимая решения, я часто учитываю риски</strong>
              <span>{memorandumRule("risk")}</span>
            </button>
          </div>
        )}
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
  treasuryShareBPS: number;
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
  const proposalStrengthBPS = currentPlayer?.authority_bps ?? 0;
  const effectiveShareBPS = useMemo(() => {
    if (!proposalStrengthBPS || plusUserId === minusUserId) {
      return 0;
    }
    if (plusUserId && minusUserId) {
      const from = props.players.find((player) => player.user_id === minusUserId);
      return Math.min(proposalStrengthBPS, Math.max(0, (from?.share_bps ?? 0) - 500));
    }
    if (plusUserId) {
      return Math.min(proposalStrengthBPS, Math.max(0, props.treasuryShareBPS));
    }
    if (minusUserId) {
      const target = props.players.find((player) => player.user_id === minusUserId);
      return Math.min(proposalStrengthBPS, Math.max(0, (target?.share_bps ?? 0) - 500));
    }
    return 0;
  }, [minusUserId, plusUserId, proposalStrengthBPS, props.players, props.treasuryShareBPS]);
  const isPartialProposal = effectiveShareBPS > 0 && effectiveShareBPS < proposalStrengthBPS;
  const canSubmitForm = props.canSubmit && (Boolean(plusUserId) || Boolean(minusUserId)) && plusUserId !== minusUserId && effectiveShareBPS > 0;

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
            {plusUserId || minusUserId ? (
              <span>
                Применится: <strong>{formatShare(effectiveShareBPS)}</strong>
              </span>
            ) : null}
            {isPartialProposal ? (
              <small className="governance-warning">Будет передана не вся доля: останется минимум 5% у игрока или 0% в резерве.</small>
            ) : null}
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
        </div>
        {props.hasVoted ? <span className="wait-pill">Выбор сохранен, можно изменить</span> : null}
      </div>

      <div className="proposal-grid">
        {props.proposals.map((proposal) => (
          <GovernanceProposalCard
            key={proposal.id}
            proposal={proposal}
            players={props.players}
            currentVotes={props.currentVotes}
            selected={props.myCurrentVote?.proposal_id === proposal.id}
            disabled={!props.canVote || props.isSubmitting}
            onVote={() => props.onVote(proposal.id)}
          />
        ))}
      </div>

      {props.isCEO ? null : (
        <button
          className={props.myCurrentVote?.abstain ? "secondary-action abstain-button selected-abstain" : "secondary-action abstain-button"}
          onClick={props.onAbstain}
          disabled={!canAbstain || props.isSubmitting}
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
    <article
      className={["proposal-card", "proposal-card-button", props.selected ? "selected-vote" : "", props.disabled ? "is-disabled" : ""]
        .filter(Boolean)
        .join(" ")}
      role="button"
      tabIndex={props.disabled ? -1 : 0}
      onClick={() => {
        if (!props.disabled) {
          props.onVote();
        }
      }}
      onKeyDown={(event) => {
        if (!props.disabled && (event.key === "Enter" || event.key === " ")) {
          event.preventDefault();
          props.onVote();
        }
      }}
    >
      <strong>{describeGovernanceProposal(props.proposal, props.players)}</strong>
      <small>Сила: {formatShare(props.proposal.share_bps)}</small>
      <div className="proposal-authors">
        {authorIds.map((authorId) => {
          const author = props.players.find((player) => player.user_id === authorId);
          const name = author?.name ?? playerName(props.players, authorId);
          return (
            <span className="proposal-author" key={authorId}>
              <UserAvatar name={name} avatarUrl={author?.avatar_url} size="small" />
              {name}
            </span>
          );
        })}
      </div>
    </article>
  );
}

function ChatPanel(props: {
  messages: PublicChatMessage[];
  currentUserId: number;
  canSend: boolean;
  isSubmitting: boolean;
  onSend: (message: string) => Promise<void>;
  onReact: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
}) {
  const [draft, setDraft] = useState("");
  const [historyMode, setHistoryMode] = useState(false);
  const [emojiOpen, setEmojiOpen] = useState(false);
  const [expandedSystemIds, setExpandedSystemIds] = useState<Record<number, boolean>>({});
  const inputRef = useRef<HTMLInputElement | null>(null);
  const messagesRef = useRef<HTMLDivElement | null>(null);
  const emojiChoices = ["👍", "🤝", "💼", "📈", "⚠️", "🕵️", "✅", "🔥"];

  function scrollToBottom() {
    window.requestAnimationFrame(() => {
      if (messagesRef.current) {
        messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
      }
    });
  }

  const visibleMessages = useMemo(() => {
    if (!historyMode) {
      return props.messages;
    }
    const historyTypes = new Set([
      "major_vote_accepted",
      "major_vote_rejected",
      "governance_accepted",
      "governance_rejected",
      "sabotage_accepted",
      "mole_revealed",
    ]);
    return props.messages.filter((message) => message.kind === "system" && historyTypes.has(message.system_event_type ?? ""));
  }, [historyMode, props.messages]);

  const groupedMessages = useMemo(() => {
    type ChatGroup = { id: string; messages: PublicChatMessage[]; isSystem: boolean };
    const groups: ChatGroup[] = [];
    for (const message of visibleMessages) {
      const isSystem = message.kind === "system" || message.user_id === 0;
      const lastGroup = groups[groups.length - 1];
      const lastMessage = lastGroup?.messages[lastGroup.messages.length - 1];
      const canMerge =
        !isSystem &&
        lastGroup &&
        !lastGroup.isSystem &&
        lastMessage?.user_id === message.user_id &&
        Math.abs(new Date(message.created_at).getTime() - new Date(lastMessage.created_at).getTime()) <= 180000;
      if (canMerge && lastGroup) {
        lastGroup.messages.push(message);
      } else {
        groups.push({ id: `${message.id}-${message.created_at}`, messages: [message], isSystem });
      }
    }
    return groups;
  }, [visibleMessages]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const message = draft.trim();
    if (!message || !props.canSend || props.isSubmitting) {
      return;
    }
    await props.onSend(message);
    setDraft("");
    setEmojiOpen(false);
    window.requestAnimationFrame(() => inputRef.current?.focus());
    scrollToBottom();
  }

  function insertEmoji(emoji: string) {
    const input = inputRef.current;
    const start = input?.selectionStart ?? draft.length;
    const end = input?.selectionEnd ?? draft.length;
    const nextDraft = `${draft.slice(0, start)}${emoji}${draft.slice(end)}`.slice(0, 500);
    setDraft(nextDraft);
    window.requestAnimationFrame(() => {
      inputRef.current?.focus();
      const cursor = Math.min(start + emoji.length, nextDraft.length);
      inputRef.current?.setSelectionRange(cursor, cursor);
    });
  }

  return (
    <section className="chat-panel">
      <div className="chat-heading">
        <div>
          <p className="eyebrow">чат</p>
        </div>
        <div className="chat-heading-actions">
          <button
            type="button"
            className={historyMode ? "mini-button active" : "mini-button"}
            onClick={() => setHistoryMode((value) => !value)}
          >
            История
          </button>
          <span>{visibleMessages.length}</span>
        </div>
      </div>

      <div className="chat-messages" ref={messagesRef}>
        {groupedMessages.map((group) => {
          const firstMessage = group.messages[0];
          const isMine = firstMessage.user_id === props.currentUserId;
          if (group.isSystem) {
            return (
              <SystemChatMessage
                key={group.id}
                message={firstMessage}
                expanded={expandedSystemIds[firstMessage.id] ?? false}
                onToggle={() => setExpandedSystemIds((current) => ({ ...current, [firstMessage.id]: !current[firstMessage.id] }))}
              />
            );
          }
          return (
            <article className={isMine ? "chat-message is-mine" : "chat-message"} key={group.id}>
              <div className="chat-message-head">
                <button className="chat-author profile-link" type="button" onClick={() => props.onOpenProfile(firstMessage.user_id)}>
                  <UserAvatar name={firstMessage.user_name} avatarUrl={firstMessage.avatar_url} size="small" />
                  <span className="chat-author-text">
                    <strong>{firstMessage.user_name}</strong>
                    {firstMessage.company_position ? <small> · {firstMessage.company_position}</small> : null}
                  </span>
                </button>
                <small>{formatChatTime(firstMessage.created_at)}</small>
              </div>
              {group.messages.map((message) => (
                <div className={message.kind === "official" ? "chat-message-line official-line" : "chat-message-line"} key={`${message.id}-${message.created_at}`}>
                  <p>{message.message}</p>
                  <ChatReactions message={message} choices={emojiChoices} disabled={props.isSubmitting} onReact={props.onReact} />
                </div>
              ))}
            </article>
          );
        })}
        {!visibleMessages.length ? <p className="quiet-text">{historyMode ? "В истории пока нет системных итогов." : "В переговорной пока тихо."}</p> : null}
      </div>

      {historyMode ? null : (
      <form className="chat-form" onSubmit={submit}>
        <input
          ref={inputRef}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder={props.canSend ? "Сообщение совету" : "Чат доступен участникам комнаты"}
          maxLength={500}
          disabled={!props.canSend || props.isSubmitting}
          onFocus={scrollToBottom}
        />
        <div className="emoji-picker-wrap">
          <button
            className="emoji-toggle"
            type="button"
            onClick={() => setEmojiOpen((value) => !value)}
            disabled={!props.canSend || props.isSubmitting}
            aria-label="Emoji"
            title="Emoji"
          >
            🤠
          </button>
          {emojiOpen ? (
            <div className="emoji-picker">
              {emojiChoices.map((emoji) => (
                <button key={emoji} type="button" onClick={() => insertEmoji(emoji)}>
                  {emoji}
                </button>
              ))}
            </div>
          ) : null}
        </div>
        <button className="primary-action" type="submit" disabled={!draft.trim() || !props.canSend || props.isSubmitting}>
          Отправить
        </button>
      </form>
      )}
    </section>
  );
}

function ChatReactions(props: {
  message: PublicChatMessage;
  choices: string[];
  disabled: boolean;
  onReact: (messageId: number, emoji: string) => Promise<void>;
}) {
  const reactions = props.message.reactions ?? [];
  return (
    <div className="chat-reactions">
      {props.choices.map((emoji) => {
        const reaction = reactions.find((item) => item.emoji === emoji);
        const count = reaction?.count ?? 0;
        return (
          <button
            key={emoji}
            type="button"
            className={reaction?.reacted_by_me ? "reaction-button active" : "reaction-button"}
            disabled={props.disabled}
            onClick={() => props.onReact(props.message.id, emoji)}
            title={emoji}
          >
            <span>{emoji}</span>
            {count > 0 ? <strong>{count}</strong> : null}
          </button>
        );
      })}
    </div>
  );
}

function SystemChatMessage(props: { message: PublicChatMessage; expanded: boolean; onToggle: () => void }) {
  const details = props.message.details ?? [];
  const hasDetails = details.length > 0;
  const title = props.message.title || "Системное сообщение";
  const summary = props.message.summary || props.message.message;
  return (
    <article className={["chat-message", "system-message", props.message.tone ? `tone-${props.message.tone}` : ""].filter(Boolean).join(" ")}>
      <div className="chat-message-head">
        <span className="chat-author">
          <strong>{title}</strong>
        </span>
        <small>{formatChatTime(props.message.created_at)}</small>
      </div>
      <p>{summary}</p>
      {hasDetails ? (
        <>
          {props.expanded ? (
            <div className="system-details">
              {details.map((detail, index) => (
                <span key={`${props.message.id}-${index}`}>{detail}</span>
              ))}
            </div>
          ) : null}
          <button type="button" className="mini-button system-toggle" onClick={props.onToggle}>
            {props.expanded ? "Скрыть" : "Показать полностью"}
          </button>
        </>
      ) : null}
    </article>
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
  onOpenProfile: () => void;
}) {
  return (
    <article className={props.player.user_id === props.currentUserId ? "player-card is-current" : "player-card"}>
      <button className="player-card-heading profile-link" type="button" onClick={props.onOpenProfile}>
        <UserAvatar name={props.player.name} avatarUrl={props.player.avatar_url} size="medium" />
        <div>
          <h2>{props.player.name}</h2>
          {props.player.company_position ? <small>{props.player.company_position}</small> : null}
          <p>Доля {formatShare(props.player.share_bps)} · Полномочия {formatShare(props.player.authority_bps)}</p>
        </div>
      </button>
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
      title={isEmpowerment ? "Победители получают +1% к полномочиям" : "Победители получают +1% к доле"}
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
