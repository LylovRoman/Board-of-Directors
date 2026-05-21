import type { PublicPlayerState } from "../types";
import { formatShare } from "../gameText";
import { UserAvatar } from "./UserAvatar";

export function PlayerCard(props: {
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
        <button className="player-card-heading profile-link" type="button" onClick={props.player.is_bot ? undefined : props.onOpenProfile} disabled={props.player.is_bot}>
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
          {props.player.is_bot ? <span className="badge bot">Bot</span> : null}
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
