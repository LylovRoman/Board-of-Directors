import { useMemo, useRef, useState } from "react";
import type { PublicChatMessage, PublicPlayerState } from "../types";
import { formatChatTime, stripKnownPositionSuffixes, systemChatTitle } from "../gameText";
import { UserAvatar } from "./UserAvatar";

export function ChatPanel(props: {
  messages: PublicChatMessage[];
  players: PublicPlayerState[];
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
  const [openReactionMessageId, setOpenReactionMessageId] = useState<number | null>(null);
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
      "mole_exposed_by_compliance",
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
    setOpenReactionMessageId(null);
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
                      players={props.players}
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
                        <div className="chat-line-main">
                          <p>{message.message}</p>
                          <button
                              type="button"
                              className={openReactionMessageId === message.id ? "reaction-toggle active" : "reaction-toggle"}
                              disabled={props.isSubmitting}
                              onClick={() => setOpenReactionMessageId((current) => (current === message.id ? null : message.id))}
                              aria-label="Добавить реакцию"
                              title="Добавить реакцию"
                          >
                            🤠
                          </button>
                        </div>
                        <ChatReactions
                            message={message}
                            choices={emojiChoices}
                            pickerOpen={openReactionMessageId === message.id}
                            disabled={props.isSubmitting}
                            onReact={async (messageId, emoji) => {
                              await props.onReact(messageId, emoji);
                              setOpenReactionMessageId(null);
                            }}
                        />
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

export function ChatReactions(props: {
  message: PublicChatMessage;
  choices: string[];
  pickerOpen: boolean;
  disabled: boolean;
  onReact: (messageId: number, emoji: string) => Promise<void>;
}) {
  const reactions = props.message.reactions ?? [];
  const visibleReactions = reactions.filter((reaction) => reaction.count > 0);
  if (!visibleReactions.length && !props.pickerOpen) {
    return null;
  }
  return (
      <div className="chat-reactions">
        {visibleReactions.length ? (
            <div className="reaction-summary">
              {visibleReactions.map((reaction) => (
                  <button
                      key={reaction.emoji}
                      type="button"
                      className={reaction.reacted_by_me ? "reaction-button active" : "reaction-button"}
                      disabled={props.disabled}
                      onClick={() => props.onReact(props.message.id, reaction.emoji)}
                      title={reaction.emoji}
                  >
                    <span>{reaction.emoji}</span>
                    <strong>{reaction.count}</strong>
                  </button>
              ))}
            </div>
        ) : null}
        {props.pickerOpen ? (
            <div className="reaction-picker">
              {props.choices.map((emoji) => {
                const reaction = reactions.find((item) => item.emoji === emoji);
                return (
                    <button
                        key={emoji}
                        type="button"
                        className={reaction?.reacted_by_me ? "reaction-choice active" : "reaction-choice"}
                        disabled={props.disabled}
                        onClick={() => props.onReact(props.message.id, emoji)}
                        title={emoji}
                    >
                      {emoji}
                    </button>
                );
              })}
            </div>
        ) : null}
      </div>
  );
}

export function SystemChatMessage(props: { message: PublicChatMessage; players: PublicPlayerState[]; expanded: boolean; onToggle: () => void }) {
  const details = (props.message.details ?? []).map((detail) => stripKnownPositionSuffixes(detail, props.players));
  const hasDetails = details.length > 0;
  const title = systemChatTitle(props.message);
  const summary = stripKnownPositionSuffixes(props.message.summary || props.message.message, props.players);
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
