import type { PublicChatMessage, PublicGameState } from "../types";
import { ChatPanel } from "../components/ChatPanel";
import { PlayerCard } from "../components/PlayerCard";

export function GameLobbyScreen(props: {
  state: PublicGameState | null;
  currentUserId: number;
  canJoin: boolean;
  canLeave: boolean;
  canStart: boolean;
  canKick: boolean;
  canBan: boolean;
  canAddBot: boolean;
  hasMe: boolean;
  chatMessages: PublicChatMessage[];
  canSendChatMessage: boolean;
  isLoading: boolean;
  isSubmitting: boolean;
  onJoin: () => void;
  onLeave: () => void;
  onStart: () => void;
  onAddBot: () => void;
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
        <div>
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
            {props.canStart ? (
                <button className="primary-action" onClick={props.onStart} disabled={props.isSubmitting}>
                  Начать игру
                </button>
            ) : null}
            {props.canAddBot ? (
                <button className="secondary-action" onClick={props.onAddBot} disabled={props.isSubmitting}>
                  Добавить бота
                </button>
            ) : null}
            {props.canLeave ? (
                <button className="secondary-action" onClick={props.onLeave} disabled={props.isSubmitting}>
                  Выйти
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
            players={state?.players ?? []}
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
