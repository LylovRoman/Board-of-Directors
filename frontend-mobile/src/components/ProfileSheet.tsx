import type { FormEvent } from "react";
import { Avatar } from "./Avatar";
import { bpsToPercent } from "../gameText";
import type { AuthUser, Profile } from "../types";

interface ProfileSheetContentProps {
  currentUser: AuthUser;
  profile: Profile | null;
  profileUserId: number | null;
  isLoading: boolean;
  isSubmitting: boolean;
  name: string;
  avatarUrl: string;
  position: string;
  currentPassword: string;
  newPassword: string;
  onNameChange: (value: string) => void;
  onAvatarUrlChange: (value: string) => void;
  onPositionChange: (value: string) => void;
  onCurrentPasswordChange: (value: string) => void;
  onNewPasswordChange: (value: string) => void;
  onSubmitProfile: (event: FormEvent<HTMLFormElement>) => void;
  onSubmitPassword: (event: FormEvent<HTMLFormElement>) => void;
  onRespect: () => void;
}

export function ProfileSheetContent(props: ProfileSheetContentProps) {
  const canEdit = props.profileUserId === props.currentUser.id;
  const profileName = props.name || props.profile?.name || props.currentUser.name;
  const avatarUrl = props.avatarUrl || props.profile?.avatar_url || props.currentUser.avatar_url;
  const position = props.position || props.profile?.company_position || props.currentUser.company_position || "Директор";
  const stats = props.profile?.stats;

  return (
    <div className="profile-sheet-content">
      <section className="profile-hero">
        <Avatar name={profileName} avatarUrl={avatarUrl} size="lg" />
        <div>
          <h3>{profileName}</h3>
          <span>{position}</span>
          {props.profile?.login ? <small>@{props.profile.login}</small> : null}
        </div>
      </section>

      {props.isLoading && !props.profile ? <div className="empty-inline">Загружаю профиль...</div> : null}

      <section className="stat-grid">
        <div>
          <small>Игры</small>
          <strong>{stats?.total.games ?? 0}</strong>
        </div>
        <div>
          <small>Победы</small>
          <strong>{stats?.total.wins ?? 0}</strong>
        </div>
        <div>
          <small>Winrate</small>
          <strong>{Math.round((stats?.total.winrate ?? 0) * 100)}%</strong>
        </div>
        <div>
          <small>Respect</small>
          <strong>{props.profile?.respect_count ?? 0}</strong>
        </div>
      </section>

      <section className="role-stats">
        <div>
          <span>Крот</span>
          <strong>{stats?.mole.wins ?? 0}/{stats?.mole.games ?? 0}</strong>
        </div>
        <div>
          <span>Директор</span>
          <strong>{stats?.director.wins ?? 0}/{stats?.director.games ?? 0}</strong>
        </div>
        <div>
          <span>Общий winrate</span>
          <strong>{bpsToPercent((stats?.total.winrate ?? 0) * 10000)}</strong>
        </div>
      </section>

      {!canEdit && props.profileUserId ? (
        <button
          className="primary-action wide-action"
          type="button"
          disabled={props.isSubmitting || props.profile?.respected_by_me}
          onClick={props.onRespect}
        >
          {props.profile?.respected_by_me ? "Respect уже выражен" : "+ respect"}
        </button>
      ) : null}

      {canEdit ? (
        <>
          <form className="stack-form" onSubmit={props.onSubmitProfile}>
            <label>
              <span>Имя</span>
              <input value={props.name} onChange={(event) => props.onNameChange(event.target.value)} />
            </label>
            <label>
              <span>Должность</span>
              <input value={props.position} onChange={(event) => props.onPositionChange(event.target.value)} />
            </label>
            <label>
              <span>Аватар URL</span>
              <input value={props.avatarUrl} onChange={(event) => props.onAvatarUrlChange(event.target.value)} />
            </label>
            <button className="primary-action wide-action" type="submit" disabled={props.isSubmitting}>
              Сохранить профиль
            </button>
          </form>
          <form className="stack-form" onSubmit={props.onSubmitPassword}>
            <label>
              <span>Текущий пароль</span>
              <input
                type="password"
                value={props.currentPassword}
                onChange={(event) => props.onCurrentPasswordChange(event.target.value)}
              />
            </label>
            <label>
              <span>Новый пароль</span>
              <input
                type="password"
                value={props.newPassword}
                onChange={(event) => props.onNewPasswordChange(event.target.value)}
              />
            </label>
            <button className="secondary-action wide-action" type="submit" disabled={props.isSubmitting}>
              Сменить пароль
            </button>
          </form>
        </>
      ) : null}
    </div>
  );
}
