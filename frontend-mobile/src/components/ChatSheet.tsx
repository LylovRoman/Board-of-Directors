import { Send, SmilePlus } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { Avatar } from "./Avatar";
import { ALLOWED_REACTIONS, formatTime } from "../gameText";
import type { PublicChatMessage, PublicPlayerState } from "../types";

interface ChatSheetContentProps {
  messages: PublicChatMessage[];
  players: PublicPlayerState[];
  currentUserId: number;
  canSend: boolean;
  isSubmitting: boolean;
  onSend: (message: string) => Promise<void>;
  onReact: (messageId: number, emoji: string) => Promise<void>;
  onOpenProfile: (userId: number) => void;
}

const DECISION_OPTIONS = ["A", "B", "C", "D", "E", "F", "G", "H"];

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function effectivePlayerPositionLabel(player: PublicPlayerState): string {
  return player.is_ceo ? "CEO" : (player.company_position ?? "").trim();
}

function stripKnownPositionSuffixes(text: string, players: PublicPlayerState[]): string {
  return players.reduce((current, player) => {
    const position = effectivePlayerPositionLabel(player);
    const name = player.name?.trim();
    if (!name || !position) {
      return current;
    }
    const pattern = new RegExp(`${escapeRegExp(name)},\\s*${escapeRegExp(position)}(?=\\)|,|\\.|;|$)`, "g");
    return current.replace(pattern, name);
  }, text);
}

function majorVoteDecisionFromDetails(details?: string[] | null): string | null {
  let winner: { decision: string; share: number } | null = null;
  for (const detail of details ?? []) {
    const match = detail.match(/^([A-H])\s+[—-].*?:\s+(\d+(?:[.,]\d+)?)%/);
    if (!match) {
      continue;
    }
    const share = Number(match[2].replace(",", "."));
    if (!Number.isFinite(share)) {
      continue;
    }
    if (!winner || share > winner.share) {
      winner = { decision: match[1], share };
    }
  }
  return winner?.decision ?? null;
}

function systemChatTitle(message: PublicChatMessage): string {
  const title = message.title?.trim() || "";
  switch (message.system_event_type) {
    case "company_briefing":
      return title && !title.startsWith("Компания:") ? title : "Брифинг компании";
    case "sabotage_accepted":
      return title && !title.startsWith("Тревожный сигнал:") ? title : "Тревожный сигнал";
    case "mole_revealed":
      return title && !title.startsWith("Крот раскрыт:") ? title : "Крот раскрыт";
    case "major_vote_accepted":
    case "major_vote_rejected": {
      const value = title.startsWith("Итоги major vote:") ? title.slice("Итоги major vote:".length).trim() : "";
      if (DECISION_OPTIONS.includes(value) || value === "не принято") {
        return `Итоги major vote: ${value}`;
      }
      const decision = majorVoteDecisionFromDetails(message.details);
      if (decision) {
        return `Итоги major vote: ${decision}`;
      }
      return message.system_event_type === "major_vote_rejected" ? "Итоги major vote: не принято" : "Итоги major vote: принято";
    }
    default:
      return title || "Системное сообщение";
  }
}

export function ChatSheetContent(props: ChatSheetContentProps) {
  const [draft, setDraft] = useState("");
  const [historyOnly, setHistoryOnly] = useState(false);
  const [emojiOpen, setEmojiOpen] = useState(false);
  const [expandedIds, setExpandedIds] = useState<Record<number, boolean>>({});
  const [pickerMessageId, setPickerMessageId] = useState<number | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const visibleMessages = useMemo(() => {
    if (!historyOnly) {
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
  }, [historyOnly, props.messages]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const message = draft.trim();
    if (!message || !props.canSend || props.isSubmitting) {
      return;
    }
    await props.onSend(message);
    setDraft("");
    setEmojiOpen(false);
    setPickerMessageId(null);
    window.requestAnimationFrame(() => inputRef.current?.focus());
  }

  function insertEmoji(emoji: string) {
    const start = inputRef.current?.selectionStart ?? draft.length;
    const end = inputRef.current?.selectionEnd ?? draft.length;
    const next = `${draft.slice(0, start)}${emoji}${draft.slice(end)}`.slice(0, 500);
    setDraft(next);
    window.requestAnimationFrame(() => {
      inputRef.current?.focus();
      const cursor = Math.min(start + emoji.length, next.length);
      inputRef.current?.setSelectionRange(cursor, cursor);
    });
  }

  return (
    <div className="chat-sheet-content">
      <div className="sheet-toolbar">
        <button className={historyOnly ? "mini-button active" : "mini-button"} type="button" onClick={() => setHistoryOnly((value) => !value)}>
          История
        </button>
        <span>{visibleMessages.length} сообщений</span>
      </div>

      <div className="mobile-chat-list">
        {visibleMessages.map((message) => {
          const isSystem = message.kind === "system" || message.user_id === 0;
          const isMine = message.user_id === props.currentUserId;
          const isExpanded = expandedIds[message.id] ?? false;

          if (isSystem) {
            const details = (message.details ?? []).map((detail) => stripKnownPositionSuffixes(detail, props.players));
            const summary = stripKnownPositionSuffixes(message.summary || message.message, props.players);
            return (
              <article className={`mobile-chat-message system tone-${message.tone || "neutral"}`} key={message.id}>
                <div className="system-title-row">
                  <strong>{systemChatTitle(message)}</strong>
                  <small>{formatTime(message.created_at)}</small>
                </div>
                <p>{summary}</p>
                {details.length ? (
                  <>
                    {isExpanded ? (
                      <ul>
                        {details.map((detail, index) => (
                          <li key={`${message.id}-${index}`}>{detail}</li>
                        ))}
                      </ul>
                    ) : null}
                    <button
                      className="text-button"
                      type="button"
                      onClick={() => setExpandedIds((current) => ({ ...current, [message.id]: !isExpanded }))}
                    >
                      {isExpanded ? "Свернуть" : "Детали"}
                    </button>
                  </>
                ) : null}
              </article>
            );
          }

          return (
            <article className={isMine ? "mobile-chat-message mine" : "mobile-chat-message"} key={message.id}>
              <header>
                <button className="chat-author-button" type="button" onClick={() => props.onOpenProfile(message.user_id)}>
                  <Avatar name={message.user_name} avatarUrl={message.avatar_url} size="sm" />
                  <span>
                    <strong>{message.user_name}</strong>
                    {message.company_position ? <small>{message.company_position}</small> : null}
                  </span>
                </button>
                <small>{formatTime(message.created_at)}</small>
              </header>
              <p>{message.message}</p>
              <div className="reaction-row">
                {message.reactions?.map((reaction) => (
                  <button
                    className={reaction.reacted_by_me ? "reaction-pill active" : "reaction-pill"}
                    type="button"
                    key={reaction.emoji}
                    disabled={props.isSubmitting}
                    onClick={() => props.onReact(message.id, reaction.emoji)}
                  >
                    {reaction.emoji} {reaction.count}
                  </button>
                ))}
                <button
                  className="reaction-add"
                  type="button"
                  disabled={props.isSubmitting}
                  onClick={() => setPickerMessageId((current) => (current === message.id ? null : message.id))}
                  aria-label="Добавить реакцию"
                >
                  <SmilePlus size={16} />
                </button>
              </div>
              {pickerMessageId === message.id ? (
                <div className="emoji-picker compact">
                  {ALLOWED_REACTIONS.map((emoji) => (
                    <button type="button" key={emoji} onClick={() => props.onReact(message.id, emoji)} disabled={props.isSubmitting}>
                      {emoji}
                    </button>
                  ))}
                </div>
              ) : null}
            </article>
          );
        })}
        {!visibleMessages.length ? <div className="empty-inline">Пока тихо. Самое время заявить позицию.</div> : null}
      </div>

      <form className="mobile-chat-form" onSubmit={submit}>
        {emojiOpen ? (
          <div className="emoji-picker">
            {ALLOWED_REACTIONS.map((emoji) => (
              <button type="button" key={emoji} onClick={() => insertEmoji(emoji)}>
                {emoji}
              </button>
            ))}
          </div>
        ) : null}
        <button className="icon-button" type="button" onClick={() => setEmojiOpen((value) => !value)} aria-label="Эмодзи">
          <SmilePlus size={19} />
        </button>
        <input
          ref={inputRef}
          value={draft}
          maxLength={500}
          onChange={(event) => setDraft(event.target.value)}
          placeholder={props.canSend ? "Сообщение или /me заявление" : "Чат доступен участникам"}
          disabled={!props.canSend || props.isSubmitting}
        />
        <button className="send-button" type="submit" disabled={!draft.trim() || !props.canSend || props.isSubmitting} aria-label="Отправить">
          <Send size={18} />
        </button>
      </form>
    </div>
  );
}
