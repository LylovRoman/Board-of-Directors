import { BarChart3, BookOpen, LogOut, Plus, RefreshCcw, UserRound, Users } from "lucide-react";
import type { FormEvent } from "react";
import { Avatar } from "../components/Avatar";
import { bpsToPercent, phaseLabel, statusLabel } from "../gameText";
import type { AuthUser, Game, LeaderboardEntry } from "../types";

interface LobbyScreenProps {
  currentUser: AuthUser;
  games: Game[];
  leaderboardEntries: LeaderboardEntry[];
  liveStatus: string;
  createTitle: string;
  isCreating: boolean;
  isLoading: boolean;
  onCreateTitleChange: (value: string) => void;
  onCreateToggle: (value: boolean) => void;
  onCreateGame: (event: FormEvent<HTMLFormElement>) => void;
  onOpenGame: (gameId: number) => void;
  onOpenProfile: (userId?: number) => void;
  onOpenLeaderboard: () => void;
  onOpenRules: () => void;
  onRefresh: () => void;
  onLogout: () => void;
}

export function LobbyScreen(props: LobbyScreenProps) {
  const activeGames = props.games.filter((game) => game.status !== "finished").length;
  const myGames = props.games.filter((game) => game.is_member && game.status !== "finished").length;

  return (
    <main className="mobile-shell lobby-shell">
      <header className="mobile-topbar">
        <button className="profile-chip" type="button" onClick={() => props.onOpenProfile()}>
          <Avatar name={props.currentUser.name} avatarUrl={props.currentUser.avatar_url} size="sm" />
          <span>
            <strong>{props.currentUser.name}</strong>
            <small>{props.currentUser.company_position || "Директор"}</small>
          </span>
        </button>
        <span className={`live-pill live-${props.liveStatus}`}>{props.liveStatus}</span>
      </header>

      <section className="dashboard-strip">
        <div>
          <small>Комнат</small>
          <strong>{props.games.length}</strong>
        </div>
        <div>
          <small>Активных</small>
          <strong>{activeGames}</strong>
        </div>
        <div>
          <small>Мои</small>
          <strong>{myGames}</strong>
        </div>
      </section>

      <section className="section-block">
        <div className="section-heading-row">
          <div>
            <p className="eyebrow">лобби</p>
            <h1>Комнаты</h1>
          </div>
          <button className="icon-button" type="button" aria-label="Обновить" onClick={props.onRefresh}>
            <RefreshCcw size={19} />
          </button>
        </div>

        {props.isCreating ? (
          <form className="create-game-form" onSubmit={props.onCreateGame}>
            <label>
              <span>Название комнаты</span>
              <input
                value={props.createTitle}
                onChange={(event) => props.onCreateTitleChange(event.target.value)}
                placeholder="Совет директоров"
              />
            </label>
            <div className="button-row">
              <button className="primary-action" type="submit" disabled={props.isLoading}>
                Создать
              </button>
              <button className="secondary-action" type="button" onClick={() => props.onCreateToggle(false)}>
                Отмена
              </button>
            </div>
          </form>
        ) : (
          <button className="create-room-button" type="button" onClick={() => props.onCreateToggle(true)}>
            <Plus size={19} />
            Создать комнату
          </button>
        )}

        <div className="game-list">
          {props.games.length ? (
            props.games.map((game) => <LobbyGameCard key={game.id} game={game} onOpen={() => props.onOpenGame(game.id)} />)
          ) : (
            <div className="empty-state">
              <strong>Комнат пока нет</strong>
              <span>Создай первую комнату и пригласи директоров.</span>
            </div>
          )}
        </div>
      </section>

      <section className="section-block leaderboard-preview">
        <div className="section-heading-row">
          <div>
            <p className="eyebrow">неделя</p>
            <h2>Рейтинг</h2>
          </div>
          <button className="text-button" type="button" onClick={props.onOpenLeaderboard}>
            Все
          </button>
        </div>
        {props.leaderboardEntries.slice(0, 3).map((entry) => (
          <button className="leader-row" type="button" key={entry.user.id} onClick={() => props.onOpenProfile(entry.user.id)}>
            <span>#{entry.rank}</span>
            <Avatar name={entry.user.name} avatarUrl={entry.user.avatar_url} size="sm" />
            <strong>{entry.user.name}</strong>
            <em>{entry.rating_points}</em>
          </button>
        ))}
        {!props.leaderboardEntries.length ? (
          <div className="empty-inline">Рейтинг появится после трех завершенных партий.</div>
        ) : null}
      </section>

      <nav className="bottom-nav" aria-label="Главная навигация">
        <button className="active" type="button">
          <Users size={19} />
          Игры
        </button>
        <button type="button" onClick={props.onOpenLeaderboard}>
          <BarChart3 size={19} />
          Рейтинг
        </button>
        <button type="button" onClick={props.onOpenRules}>
          <BookOpen size={19} />
          Правила
        </button>
        <button type="button" onClick={() => props.onOpenProfile()}>
          <UserRound size={19} />
          Профиль
        </button>
        <button type="button" onClick={props.onLogout}>
          <LogOut size={19} />
          Выход
        </button>
      </nav>
    </main>
  );
}

function LobbyGameCard({ game, onOpen }: { game: Game; onOpen: () => void }) {
  const players = game.players ?? [];
  const host = players.find((player) => player.is_host);

  return (
    <article className="game-card">
      <button className="game-card-main" type="button" onClick={onOpen}>
        <div className="game-card-head">
          <span className={`status-dot status-${game.status ?? "unknown"}`} />
          <div>
            <strong>{game.company_name || game.title}</strong>
            <small>{statusLabel(game.status)} · {phaseLabel(game.phase)}</small>
          </div>
        </div>
        {game.company_situation ? <p>{game.company_situation}</p> : null}
        <div className="game-card-meta">
          <span>{game.player_count ?? players.length}/8 игроков</span>
          <span>Раунд {game.current_round ?? 0}</span>
          {game.is_member ? <span>ты внутри</span> : null}
        </div>
        <div className="avatar-stack">
          {players.slice(0, 5).map((player) => (
            <Avatar key={player.user_id} name={player.name} avatarUrl={player.avatar_url} size="sm" />
          ))}
          {host ? <span className="host-label">Host: {host.name}</span> : null}
        </div>
      </button>
    </article>
  );
}

export function LeaderboardSheetContent(props: {
  entries: LeaderboardEntry[];
  onOpenProfile: (userId: number) => void;
}) {
  if (!props.entries.length) {
    return (
      <div className="empty-state">
        <strong>Рейтинг пока пуст</strong>
        <span>В таблицу попадают игроки с тремя завершенными партиями за неделю.</span>
      </div>
    );
  }

  return (
    <div className="leaderboard-list">
      {props.entries.map((entry) => (
        <button className="leader-row large" type="button" key={entry.user.id} onClick={() => props.onOpenProfile(entry.user.id)}>
          <span>#{entry.rank}</span>
          <Avatar name={entry.user.name} avatarUrl={entry.user.avatar_url} size="md" />
          <span>
            <strong>{entry.user.name}</strong>
            <small>{entry.games} игр · {Math.round(entry.winrate * 100)}% побед · {bpsToPercent(entry.user.stats?.total.winrate ? entry.user.stats.total.winrate * 10000 : 0)}</small>
          </span>
          <em>{entry.rating_points}</em>
        </button>
      ))}
    </div>
  );
}
