import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  changePassword,
  createGame,
  getGameState,
  getLeaderboard,
  getMe,
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
import type { MusicName, SfxName } from "./audio";
import { BottomSheet } from "./components/BottomSheet";
import { ChatSheetContent } from "./components/ChatSheet";
import { ProfileSheetContent } from "./components/ProfileSheet";
import { ReplaySheetContent } from "./components/ReplaySheet";
import { RulesSheetContent } from "./components/RulesSheet";
import { Toast } from "./components/Toast";
import { getErrorMessage, normalizeList } from "./gameText";
import { usePwaInstall } from "./hooks/usePwaInstall";
import { AuthScreen } from "./screens/AuthScreen";
import { GameScreen } from "./screens/GameScreen";
import { LeaderboardSheetContent, LobbyScreen } from "./screens/LobbyScreen";
import type {
  ActionType,
  AuthUser,
  Game,
  GameActionResponse,
  LeaderboardEntry,
  Profile,
  PublicGameState,
} from "./types";
import "./styles.css";

const MOBILE_SELECTED_GAME_KEY = "board-of-directors-mobile-selected-game-id";
const MOBILE_SOUND_KEY = "board-of-directors-mobile-sound-enabled";

type AuthMode = "login" | "register";
type LiveStatus = "offline" | "connecting" | "live" | "fallback";

const ACTION_SFX: Partial<Record<ActionType, SfxName>> = {
  join_game: "join",
  leave_game: "close",
  kick_player: "success",
  ban_player: "success",
  add_bot: "success",
  send_chat_message: "chat-send",
  react_chat_message: "reaction",
  update_game_settings: "success",
  start_game: "start",
  choose_memorandum: "success",
  select_mole_objectives: "success",
  place_compliance_watch: "success",
  break_case: "success",
  vote: "vote",
  submit_governance_proposal: "success",
  skip_governance_proposal: "close",
};

function readStoredGameId(): number | null {
  const raw = window.localStorage.getItem(MOBILE_SELECTED_GAME_KEY);
  const parsed = raw ? Number(raw) : Number.NaN;
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function readStoredBoolean(key: string): boolean {
  return window.localStorage.getItem(key) === "true";
}

function liveStatusLabel(status: LiveStatus): string {
  switch (status) {
    case "live":
      return "live";
    case "connecting":
      return "подключение";
    case "fallback":
      return "обновление";
    default:
      return "offline";
  }
}

export default function App() {
  const storedSession = readStoredAuthSession();
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(storedSession?.user ?? null);
  const [isAuthChecking, setIsAuthChecking] = useState(() => Boolean(storedSession?.token));
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [authLogin, setAuthLogin] = useState("");
  const [authPassword, setAuthPassword] = useState("");
  const [authName, setAuthName] = useState("");
  const [authAvatarUrl, setAuthAvatarUrl] = useState("");

  const [games, setGames] = useState<Game[]>([]);
  const [leaderboardEntries, setLeaderboardEntries] = useState<LeaderboardEntry[]>([]);
  const [selectedGameId, setSelectedGameId] = useState<number | null>(() => readStoredGameId());
  const [gameState, setGameState] = useState<PublicGameState | null>(null);
  const [soundEnabled, setSoundEnabled] = useState(() => readStoredBoolean(MOBILE_SOUND_KEY));
  const [createTitle, setCreateTitle] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const [profileOpen, setProfileOpen] = useState(false);
  const [profileUserId, setProfileUserId] = useState<number | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [profileName, setProfileName] = useState("");
  const [profileAvatarUrl, setProfileAvatarUrl] = useState("");
  const [profilePosition, setProfilePosition] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");

  const [chatOpen, setChatOpen] = useState(false);
  const [rulesOpen, setRulesOpen] = useState(false);
  const [leaderboardOpen, setLeaderboardOpen] = useState(false);
  const [replayOpen, setReplayOpen] = useState(false);

  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [liveStatus, setLiveStatus] = useState<LiveStatus>("offline");
  const [installHelpOpen, setInstallHelpOpen] = useState(false);
  const { canInstall, install, shouldShowManualIosInstall } = usePwaInstall();

  const currentUserId = currentUser?.id ?? 0;
  const showInstallAction = canInstall || shouldShowManualIosInstall;
  const activeMusic = useMemo<MusicName | null>(() => {
    if (!currentUser) {
      return null;
    }
    if (selectedGameId && gameState?.is_finished) {
      return "finale";
    }
    if (selectedGameId) {
      return "meeting";
    }
    return "lobby";
  }, [currentUser, gameState?.is_finished, selectedGameId]);

  const handleInstallClick = useCallback(async () => {
    if (canInstall) {
      const outcome = await install();
      if (outcome === "accepted") {
        setSuccessMessage("Приложение устанавливается.");
      }
      return;
    }

    setInstallHelpOpen(true);
  }, [canInstall, install]);

  const showError = useCallback((error: unknown) => {
    setErrorMessage(getErrorMessage(error));
  }, []);

  const loadGames = useCallback(async () => {
    const nextGames = await listGames();
    nextGames.sort((left, right) => {
      if ((left.status === "started") !== (right.status === "started")) {
        return left.status === "started" ? -1 : 1;
      }
      return new Date(right.created_at).getTime() - new Date(left.created_at).getTime();
    });
    setGames(nextGames);
    return nextGames;
  }, []);

  const loadLeaderboard = useCallback(async () => {
    const response = await getLeaderboard("week");
    setLeaderboardEntries(response.entries ?? []);
    return response.entries ?? [];
  }, []);

  const loadHome = useCallback(async () => {
    await Promise.all([loadGames(), loadLeaderboard()]);
  }, [loadGames, loadLeaderboard]);

  const loadGame = useCallback(async (gameId: number) => {
    const state = await getGameState(gameId);
    setGameState(state);
    return state;
  }, []);

  const refreshCurrentView = useCallback(async () => {
    if (!currentUser) {
      return;
    }
    setIsLoading(true);
    try {
      await loadHome();
      if (selectedGameId) {
        await loadGame(selectedGameId);
      }
    } catch (error) {
      showError(error);
    } finally {
      setIsLoading(false);
    }
  }, [currentUser, loadGame, loadHome, selectedGameId, showError]);

  useEffect(() => {
    async function validateSession() {
      if (!getAuthToken()) {
        setIsAuthChecking(false);
        return;
      }
      try {
        const user = await getMe();
        saveAuthUser(user);
        setCurrentUser(user);
      } catch {
        clearAuthSession(false);
        setCurrentUser(null);
        setSelectedGameId(null);
        setGameState(null);
      } finally {
        setIsAuthChecking(false);
      }
    }

    void validateSession();
  }, []);

  useEffect(() => {
    const handleCleared = () => {
      setCurrentUser(null);
      setSelectedGameId(null);
      setGameState(null);
      setGames([]);
      setLeaderboardEntries([]);
      window.localStorage.removeItem(MOBILE_SELECTED_GAME_KEY);
    };

    window.addEventListener(AUTH_SESSION_CLEARED_EVENT, handleCleared);
    return () => window.removeEventListener(AUTH_SESSION_CLEARED_EVENT, handleCleared);
  }, []);

  useEffect(() => {
    if (!currentUser) {
      return;
    }
    void refreshCurrentView();
  }, [currentUser, refreshCurrentView]);

  useEffect(() => {
    if (!currentUser || !selectedGameId) {
      return;
    }
    window.localStorage.setItem(MOBILE_SELECTED_GAME_KEY, String(selectedGameId));
    void loadGame(selectedGameId).catch(showError);
  }, [currentUser, loadGame, selectedGameId, showError]);

  useEffect(() => {
    preloadAudio();
    return () => stopMusic();
  }, []);

  useEffect(() => {
    window.localStorage.setItem(MOBILE_SOUND_KEY, String(soundEnabled));
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
    if (!currentUser) {
      setLiveStatus("offline");
      return undefined;
    }
    const token = getAuthToken();
    if (!token) {
      setLiveStatus("offline");
      return undefined;
    }

    let socket: WebSocket | null = null;
    let reconnectId: number | null = null;
    let attempts = 0;
    let stopped = false;

    const connect = () => {
      if (stopped) {
        return;
      }
      setLiveStatus("connecting");
      socket = new WebSocket(`${WS_BASE_URL}/games/ws?token=${encodeURIComponent(token)}`);
      socket.onopen = () => {
        attempts = 0;
        setLiveStatus("live");
      };
      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as { type?: string; games?: Game[] };
          if (data.type === "games" && Array.isArray(data.games)) {
            setGames(data.games);
          }
        } catch {
          // Ignore malformed live payloads; fallback polling still protects the UI.
        }
      };
      socket.onclose = () => {
        if (stopped) {
          return;
        }
        attempts += 1;
        if (attempts > 5) {
          setLiveStatus("fallback");
          void loadHome().catch(showError);
          return;
        }
        reconnectId = window.setTimeout(connect, Math.min(8000, 900 * attempts));
      };
      socket.onerror = () => socket?.close();
    };

    connect();
    return () => {
      stopped = true;
      if (reconnectId) {
        window.clearTimeout(reconnectId);
      }
      socket?.close();
    };
  }, [currentUser, loadHome, showError]);

  useEffect(() => {
    if (!currentUser || !selectedGameId) {
      return undefined;
    }
    const token = getAuthToken();
    if (!token) {
      return undefined;
    }

    let socket: WebSocket | null = null;
    let reconnectId: number | null = null;
    let attempts = 0;
    let stopped = false;

    const connect = () => {
      if (stopped) {
        return;
      }
      socket = new WebSocket(`${WS_BASE_URL}/games/${selectedGameId}/ws?token=${encodeURIComponent(token)}`);
      socket.onopen = () => {
        attempts = 0;
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
          // Keep the current state; manual and fallback refresh are available.
        }
      };
      socket.onclose = () => {
        if (stopped) {
          return;
        }
        attempts += 1;
        if (attempts > 5) {
          void loadGame(selectedGameId).catch(showError);
          return;
        }
        reconnectId = window.setTimeout(connect, Math.min(8000, 900 * attempts));
      };
      socket.onerror = () => socket?.close();
    };

    connect();
    return () => {
      stopped = true;
      if (reconnectId) {
        window.clearTimeout(reconnectId);
      }
      socket?.close();
    };
  }, [currentUser, currentUserId, loadGame, selectedGameId, showError, soundEnabled]);

  useEffect(() => {
    if (liveStatus !== "fallback" || !currentUser) {
      return undefined;
    }
    const id = window.setInterval(() => {
      void refreshCurrentView();
    }, 5000);
    return () => window.clearInterval(id);
  }, [currentUser, liveStatus, refreshCurrentView]);

  async function handleAuthSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const loginValue = authLogin.trim();
    const passwordValue = authPassword;
    const nameValue = authName.trim();
    const avatarValue = authAvatarUrl.trim();
    if (!loginValue || !passwordValue || (authMode === "register" && !nameValue)) {
      setErrorMessage("Заполните обязательные поля.");
      return;
    }
    setIsSubmitting(true);
    try {
      const response =
        authMode === "register"
          ? await register({ login: loginValue, password: passwordValue, name: nameValue, avatar_url: avatarValue || undefined })
          : await login({ login: loginValue, password: passwordValue });
      saveAuthSession(response);
      setCurrentUser(response.user);
      setAuthPassword("");
      setAuthName("");
      setAuthAvatarUrl("");
      setSuccessMessage("Сессия открыта.");
      await loadHome();
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateGame(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const title = createTitle.trim() || "Совет директоров";
    setIsSubmitting(true);
    try {
      const response = await createGame({ title });
      setCreateTitle("");
      setIsCreating(false);
      setSelectedGameId(response.game.id);
      setGameState(response.state);
      window.localStorage.setItem(MOBILE_SELECTED_GAME_KEY, String(response.game.id));
      await loadHome();
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function openGame(gameId: number) {
    setSelectedGameId(gameId);
    window.localStorage.setItem(MOBILE_SELECTED_GAME_KEY, String(gameId));
    setIsLoading(true);
    try {
      await loadGame(gameId);
    } catch (error) {
      showError(error);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleAction(type: ActionType, payload?: Record<string, unknown>) {
    if (!selectedGameId) {
      setErrorMessage("Игра не выбрана.");
      return;
    }
    setIsSubmitting(true);
    try {
      const response = (await sendGameAction(selectedGameId, { type, payload })) as GameActionResponse & {
        game_deleted?: boolean;
      };
      if (response.game_deleted) {
        setSelectedGameId(null);
        setGameState(null);
        window.localStorage.removeItem(MOBILE_SELECTED_GAME_KEY);
      } else if (response.state) {
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
        await loadGame(selectedGameId);
      }
      await loadHome();
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleBackToLobby() {
    setSelectedGameId(null);
    setGameState(null);
    setChatOpen(false);
    setReplayOpen(false);
    window.localStorage.removeItem(MOBILE_SELECTED_GAME_KEY);
    void loadHome().catch(showError);
  }

  async function openProfile(userId?: number) {
    const targetId = userId ?? currentUserId;
    if (!targetId || targetId < 0) {
      return;
    }
    setProfileOpen(true);
    setProfileUserId(targetId);
    setProfile(null);
    setProfileName("");
    setProfileAvatarUrl("");
    setProfilePosition("");
    setIsLoading(true);
    try {
      const nextProfile = targetId === currentUserId ? await getMyProfile() : await getUserProfile(targetId);
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

  async function handleProfileSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    try {
      const user = await updateMyProfile({
        name: profileName.trim(),
        avatar_url: profileAvatarUrl.trim(),
        company_position: profilePosition.trim(),
      });
      saveAuthUser(user);
      setCurrentUser(user);
      setSuccessMessage("Профиль обновлен.");
      await openProfile(user.id);
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handlePasswordSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    try {
      await changePassword({ current_password: currentPassword, new_password: newPassword });
      setCurrentPassword("");
      setNewPassword("");
      setSuccessMessage("Пароль изменен.");
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleRespectProfile() {
    if (!profileUserId || profileUserId === currentUserId) {
      return;
    }
    setIsSubmitting(true);
    try {
      const nextProfile = await respectUser(profileUserId);
      setProfile(nextProfile);
      setSuccessMessage("Respect выражен.");
    } catch (error) {
      showError(error);
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleLogout() {
    clearAuthSession();
    setCurrentUser(null);
    setSelectedGameId(null);
    setGameState(null);
    setProfileOpen(false);
    setChatOpen(false);
    setReplayOpen(false);
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

  const sortedGames = useMemo(() => games, [games]);
  const currentLiveLabel = liveStatusLabel(liveStatus);
  const installHelpSheet = (
    <BottomSheet title="Установить приложение" eyebrow="PWA" open={installHelpOpen} onClose={() => setInstallHelpOpen(false)}>
      <div className="install-help">
        <strong>iPhone / iPad</strong>
        <p>Откройте меню «Поделиться» и выберите «На экран Домой».</p>
        <span>После этого Board of Directors появится отдельной иконкой.</span>
      </div>
    </BottomSheet>
  );

  if (isAuthChecking) {
    return (
      <main className="mobile-shell loading-shell">
        <div className="loading-card">
          <span className="loader" />
          <strong>Проверяю сессию</strong>
        </div>
      </main>
    );
  }

  if (!currentUser) {
    return (
      <>
        <AuthScreen
          mode={authMode}
          login={authLogin}
          password={authPassword}
          name={authName}
          avatarUrl={authAvatarUrl}
          isSubmitting={isSubmitting}
          onModeChange={setAuthMode}
          onLoginChange={setAuthLogin}
          onPasswordChange={setAuthPassword}
          onNameChange={setAuthName}
          onAvatarUrlChange={setAuthAvatarUrl}
          onSubmit={handleAuthSubmit}
          showInstallAction={showInstallAction}
          onInstallClick={handleInstallClick}
        />
        {installHelpSheet}
        <Toast message={errorMessage} onClose={() => setErrorMessage(null)} />
        <Toast message={successMessage} tone="success" onClose={() => setSuccessMessage(null)} />
      </>
    );
  }

  return (
    <>
      {selectedGameId && gameState ? (
        <GameScreen
          state={gameState}
          currentUserId={currentUserId}
          liveStatus={currentLiveLabel}
          isSubmitting={isSubmitting}
          soundEnabled={soundEnabled}
          onAction={(type, payload) => void handleAction(type, payload)}
          onBack={handleBackToLobby}
          onRefresh={() => void refreshCurrentView()}
          onToggleSound={handleSoundToggle}
          onOpenChat={() => setChatOpen(true)}
          onOpenRules={() => setRulesOpen(true)}
          onOpenReplay={() => setReplayOpen(true)}
          onOpenProfile={(userId) => void openProfile(userId)}
        />
      ) : (
        <LobbyScreen
          currentUser={currentUser}
          games={sortedGames}
          leaderboardEntries={leaderboardEntries}
          liveStatus={currentLiveLabel}
          createTitle={createTitle}
          isCreating={isCreating}
          isLoading={isLoading || isSubmitting}
          soundEnabled={soundEnabled}
          onCreateTitleChange={setCreateTitle}
          onCreateToggle={setIsCreating}
          onCreateGame={handleCreateGame}
          onOpenGame={(gameId) => void openGame(gameId)}
          onOpenProfile={(userId) => void openProfile(userId)}
          onOpenLeaderboard={() => setLeaderboardOpen(true)}
          onOpenRules={() => setRulesOpen(true)}
          onRefresh={() => void refreshCurrentView()}
          onToggleSound={handleSoundToggle}
          onLogout={handleLogout}
          showInstallAction={showInstallAction}
          onInstallClick={handleInstallClick}
        />
      )}

      <BottomSheet title="Чат партии" eyebrow={gameState?.company_name} open={chatOpen && Boolean(gameState)} onClose={() => setChatOpen(false)}>
        {gameState ? (
          <ChatSheetContent
            messages={normalizeList(gameState.chat_messages)}
            players={normalizeList(gameState.players)}
            currentUserId={currentUserId}
            canSend={(gameState.available_actions ?? []).includes("send_chat_message")}
            isSubmitting={isSubmitting}
            onSend={(message) => handleAction("send_chat_message", { message })}
            onReact={(messageId, emoji) => handleAction("react_chat_message", { message_id: messageId, emoji })}
            onOpenProfile={(userId) => void openProfile(userId)}
          />
        ) : null}
      </BottomSheet>

      <BottomSheet title="Профиль" open={profileOpen} onClose={() => setProfileOpen(false)}>
        <ProfileSheetContent
          currentUser={currentUser}
          profile={profile}
          profileUserId={profileUserId}
          isLoading={isLoading}
          isSubmitting={isSubmitting}
          name={profileName}
          avatarUrl={profileAvatarUrl}
          position={profilePosition}
          currentPassword={currentPassword}
          newPassword={newPassword}
          onNameChange={setProfileName}
          onAvatarUrlChange={setProfileAvatarUrl}
          onPositionChange={setProfilePosition}
          onCurrentPasswordChange={setCurrentPassword}
          onNewPasswordChange={setNewPassword}
          onSubmitProfile={handleProfileSubmit}
          onSubmitPassword={handlePasswordSubmit}
          onRespect={handleRespectProfile}
        />
      </BottomSheet>

      <BottomSheet title="Рейтинг директоров" eyebrow="неделя" open={leaderboardOpen} onClose={() => setLeaderboardOpen(false)}>
        <LeaderboardSheetContent entries={leaderboardEntries} onOpenProfile={(userId) => void openProfile(userId)} />
      </BottomSheet>

      <BottomSheet title="Правила" eyebrow="коротко" open={rulesOpen} onClose={() => setRulesOpen(false)}>
        <RulesSheetContent />
      </BottomSheet>

      <BottomSheet title="Replay" eyebrow={gameState?.company_name} open={replayOpen && Boolean(gameState)} onClose={() => setReplayOpen(false)}>
        {gameState ? <ReplaySheetContent state={gameState} steps={normalizeList(gameState.replay_steps)} /> : null}
      </BottomSheet>

      {installHelpSheet}
      <Toast message={errorMessage} onClose={() => setErrorMessage(null)} />
      <Toast message={successMessage} tone="success" onClose={() => setSuccessMessage(null)} />
    </>
  );
}
