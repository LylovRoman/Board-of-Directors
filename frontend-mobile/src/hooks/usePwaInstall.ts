import { useCallback, useEffect, useState } from "react";

type InstallOutcome = "accepted" | "dismissed" | "unavailable";

interface BeforeInstallPromptEvent extends Event {
  readonly userChoice: Promise<{ outcome: "accepted" | "dismissed"; platform: string }>;
  prompt: () => Promise<void>;
}

interface NavigatorWithStandalone extends Navigator {
  standalone?: boolean;
}

function isStandaloneMode() {
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    Boolean((window.navigator as NavigatorWithStandalone).standalone)
  );
}

function isIosDevice() {
  const userAgent = window.navigator.userAgent.toLowerCase();
  const platform = window.navigator.platform;
  return /iphone|ipad|ipod/.test(userAgent) || (platform === "MacIntel" && window.navigator.maxTouchPoints > 1);
}

export function usePwaInstall() {
  const [promptEvent, setPromptEvent] = useState<BeforeInstallPromptEvent | null>(null);
  const [isStandalone, setIsStandalone] = useState(isStandaloneMode);
  const [isIos] = useState(isIosDevice);

  useEffect(() => {
    const handleBeforeInstallPrompt = (event: Event) => {
      event.preventDefault();
      setPromptEvent(event as BeforeInstallPromptEvent);
    };
    const handleInstalled = () => {
      setPromptEvent(null);
      setIsStandalone(true);
    };
    const handleDisplayModeChange = () => setIsStandalone(isStandaloneMode());

    window.addEventListener("beforeinstallprompt", handleBeforeInstallPrompt);
    window.addEventListener("appinstalled", handleInstalled);

    const standaloneMedia = window.matchMedia("(display-mode: standalone)");
    standaloneMedia.addEventListener("change", handleDisplayModeChange);

    return () => {
      window.removeEventListener("beforeinstallprompt", handleBeforeInstallPrompt);
      window.removeEventListener("appinstalled", handleInstalled);
      standaloneMedia.removeEventListener("change", handleDisplayModeChange);
    };
  }, []);

  const install = useCallback(async (): Promise<InstallOutcome> => {
    if (!promptEvent) {
      return "unavailable";
    }

    await promptEvent.prompt();
    const choice = await promptEvent.userChoice;
    setPromptEvent(null);
    return choice.outcome;
  }, [promptEvent]);

  return {
    canInstall: Boolean(promptEvent) && !isStandalone,
    isStandalone,
    install,
    shouldShowManualIosInstall: isIos && !isStandalone && !promptEvent,
  };
}
