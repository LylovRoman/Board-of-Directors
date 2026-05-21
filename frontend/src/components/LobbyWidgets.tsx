import type { Game, LeaderboardEntry, LeaderboardSort } from "../types";
import { formatAccuracy, formatWinRate, phaseLabel, statusLabel, winnerLabel } from "../gameText";
import { UserAvatar } from "./UserAvatar";

export function LobbyStat(props: { label: string; value: number; active?: boolean; onClick: () => void }) {
  return (
      <button type="button" className={props.active ? "lobby-stat active" : "lobby-stat"} onClick={props.onClick}>
        <span>{props.label}</span>
        <strong>{props.value}</strong>
      </button>
  );
}

export function RoomTable(props: { games: Game[]; onOpen: (gameId: number) => void }) {
  return (
      <div className="room-table-wrap">
        <table className="room-table">
          <thead>
          <tr>
            <th>Статус</th>
            <th>Комната</th>
            <th>Игроки</th>
            <th>Host</th>
            <th>Фаза</th>
            <th></th>
          </tr>
          </thead>
          <tbody>
          {props.games.map((game) => {
            const players = game.players ?? [];
            const host = players.find((player) => player.is_host);
            const playerCount = game.player_count ?? players.length;
            const playerLimit = game.status === "lobby" ? 8 : (game.started_player_count || playerCount || 8);
            return (
                <tr key={game.id} className={game.is_member ? "is-member" : ""}>
                  <td><span className={`status-pill status-${game.status ?? "unknown"}`}>{statusLabel(game.status)}</span></td>
                  <td>
                    <strong>{game.title}</strong>
                  </td>
                  <td>
                    <div className="room-players compact">
                      {players.slice(0, 5).map((player) => (
                          <UserAvatar key={player.user_id} name={player.name} avatarUrl={player.avatar_url} size="small" />
                      ))}
                      <span className="player-counter">{playerCount}/{playerLimit}</span>
                    </div>
                  </td>
                  <td>{host?.name ?? "—"}</td>
                  <td>{game.status === "finished" ? winnerLabel(game.winner) : game.current_round ? `Раунд ${game.current_round}` : phaseLabel(game.phase)}</td>
                  <td>
                    <button className="secondary-action" type="button" onClick={() => props.onOpen(game.id)}>
                      {game.status === "finished" ? "Реплей" : "Войти"}
                    </button>
                  </td>
                </tr>
            );
          })}
          </tbody>
        </table>
      </div>
  );
}

export function LeaderboardTable(props: {
  entries: LeaderboardEntry[];
  sort: LeaderboardSort;
  hidden: boolean;
  onSortChange: (sort: LeaderboardSort) => void;
  onToggleHidden: () => void;
  onOpenProfile: (userId: number) => void;
}) {
  return (
      <section className="leaderboard-panel">
        {props.hidden ? null : props.entries.length ? (
            <div className="leaderboard-table-wrap">
              <table className="leaderboard-table">
                <thead>
                <tr>
                  <th>Место</th>
                  <th>Игрок</th>
                  <th>Ранг</th>
                  <th>Игры</th>
                  <th>Победы</th>
                  <th>Winrate</th>
                  <th>Respect</th>
                  <th>Точность</th>
                  <th>XP</th>
                </tr>
                </thead>
                <tbody>
                {props.entries.slice(0, 8).map((entry) => (
                    <tr key={entry.user.id}>
                      <td>#{entry.rank}</td>
                      <td>
                        <button className="leaderboard-player profile-link" type="button" onClick={() => props.onOpenProfile(entry.user.id)}>
                          <UserAvatar name={entry.user.name} avatarUrl={entry.user.avatar_url} size="small" />
                          <span>
                        <strong>{entry.user.name}</strong>
                            {entry.user.company_position ? <small>{entry.user.company_position}</small> : null}
                      </span>
                        </button>
                      </td>
                      <td>{entry.rank_title || entry.user.rank_title}</td>
                      <td>{entry.games}</td>
                      <td>{entry.wins}</td>
                      <td>{formatWinRate(entry.winrate)}</td>
                      <td>{entry.respect_delta}</td>
                      <td>{formatAccuracy(entry.accuracy_bps)}</td>
                      <td>{entry.xp}</td>
                    </tr>
                ))}
                </tbody>
              </table>
            </div>
        ) : (
            <div className="leaderboard-empty">
              <strong>Рейтинг пока пуст</strong>
              <span>В таблицу попадают игроки с тремя завершенными партиями за неделю.</span>
            </div>
        )}
      </section>
  );
}
