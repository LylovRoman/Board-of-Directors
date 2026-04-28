import DevApp from "./DevApp";
import PlayerApp from "./PlayerApp";

export default function App() {
  const path = window.location.pathname;

  if (path.startsWith("/dev")) {
    return <DevApp />;
  }

  return <PlayerApp />;
}
