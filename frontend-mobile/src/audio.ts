export type MusicName = "lobby" | "meeting" | "finale";

export type SfxName =
  | "click"
  | "open"
  | "close"
  | "vote"
  | "phase"
  | "finish"
  | "chat-send"
  | "chat-receive"
  | "reaction"
  | "success"
  | "error"
  | "join"
  | "start";

const MUSIC_PATHS: Record<MusicName, string> = {
  lobby: "/audio/music/lobby.mp3",
  meeting: "/audio/music/meeting.mp3",
  finale: "/audio/music/finale.mp3",
};

const SFX_PATHS: Record<SfxName, string> = {
  click: "/audio/sfx/click.mp3",
  open: "/audio/sfx/open.mp3",
  close: "/audio/sfx/close.mp3",
  vote: "/audio/sfx/vote.mp3",
  phase: "/audio/sfx/phase.mp3",
  finish: "/audio/sfx/finish.mp3",
  "chat-send": "/audio/sfx/chat-send.mp3",
  "chat-receive": "/audio/sfx/chat-receive.mp3",
  reaction: "/audio/sfx/reaction.mp3",
  success: "/audio/sfx/success.mp3",
  error: "/audio/sfx/error.mp3",
  join: "/audio/sfx/join.mp3",
  start: "/audio/sfx/start.mp3",
};

const MUSIC_VOLUME = 0.28;
const SFX_VOLUME = 0.72;

let currentMusic: HTMLAudioElement | null = null;
let currentMusicName: MusicName | null = null;
let didPreload = false;

function makeAudio(src: string): HTMLAudioElement {
  const audio = new Audio(src);
  audio.preload = "auto";
  return audio;
}

function playElement(audio: HTMLAudioElement): void {
  const playPromise = audio.play();
  if (playPromise) {
    void playPromise.catch(() => undefined);
  }
}

export function preloadAudio(): void {
  if (didPreload) {
    return;
  }
  didPreload = true;
  [...Object.values(MUSIC_PATHS), ...Object.values(SFX_PATHS)].forEach((src) => {
    makeAudio(src).load();
  });
}

export function playSfx(name: SfxName, enabled: boolean): void {
  if (!enabled) {
    return;
  }

  const audio = makeAudio(SFX_PATHS[name]);
  audio.volume = SFX_VOLUME;
  playElement(audio);
}

export function playMusic(name: MusicName, enabled: boolean): void {
  if (!enabled) {
    stopMusic();
    return;
  }

  if (currentMusic && currentMusicName === name) {
    if (currentMusic.paused) {
      playElement(currentMusic);
    }
    return;
  }

  stopMusic();

  currentMusicName = name;
  currentMusic = makeAudio(MUSIC_PATHS[name]);
  currentMusic.loop = true;
  currentMusic.volume = MUSIC_VOLUME;
  playElement(currentMusic);
}

export function stopMusic(): void {
  if (!currentMusic) {
    currentMusicName = null;
    return;
  }

  currentMusic.pause();
  currentMusic.currentTime = 0;
  currentMusic = null;
  currentMusicName = null;
}
