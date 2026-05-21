import type React from "react";
import type { AuthUser, Profile } from "../types";
import { formatAccuracy, formatWinRate } from "../gameText";
import { LoadingBlock } from "./LoadingBlock";
import { UserAvatar } from "./UserAvatar";

export function StatTile(props: {
  label: string;
  games: number;
  wins: number;
  losses: number;
  winrate: number;
}) {
  return (
      <div className="profile-stat">
        <span>{props.label}</span>
        <strong>
          {formatWinRate(props.winrate)}
        </strong>
        <small>{props.wins} побед · {props.losses} поражений</small>
      </div>
  );
}

export function ProfileDialog(props: {
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
  const shownPosition = props.canEdit
      ? (props.profilePosition || props.currentUser.company_position || "Директор")
      : (props.profile?.company_position || "Директор");
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
              <LoadingBlock label="Загружаю профиль" />
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
                    <span>Точность</span>
                    <strong>{formatAccuracy(stats?.total.accuracy_bps ?? 0)}</strong>
                    <small>Точность в основном голосовании</small>
                  </div>
                  <div className="profile-stat">
                    <span>Уважение</span>
                    <strong>{props.profile?.respect_count ? "+" + props.profile?.respect_count : 0}</strong>
                    <small>{props.profile?.respected_by_me ? "Уже выражен" : "Получено от других игроков"}</small>
                  </div>
                  <div className="profile-stat">
                    <span>Ранг</span>
                    <strong>{props.profile?.rank_title ?? "Стажер совета"}</strong>
                    <small>{props.profile?.xp ?? 0} XP</small>
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
