import { useEffect, useState } from "react";

type InstallEvent = Event & { prompt: () => Promise<{ outcome: string }> };
export function useInstall() {
  const [prompt, setPrompt] = useState<InstallEvent | null>(null);
  const [installed, setInstalled] = useState(
    () => window.matchMedia?.("(display-mode: standalone)").matches ?? false,
  );
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    const available = (event: Event) => {
      if (!("prompt" in event) || typeof event.prompt !== "function") return;
      event.preventDefault();
      setPrompt(event as InstallEvent);
      setFailed(false);
    };
    const complete = () => {
      setInstalled(true);
      setPrompt(null);
    };
    window.addEventListener("beforeinstallprompt", available);
    window.addEventListener("appinstalled", complete);
    return () => {
      window.removeEventListener("beforeinstallprompt", available);
      window.removeEventListener("appinstalled", complete);
    };
  }, []);
  const install = () => {
    if (!prompt || pending) return;
    setPending(true);
    setFailed(false);
    const current = prompt;
    setPrompt(null);
    return current
      .prompt()
      .catch(() => {
        setFailed(true);
      })
      .finally(() => {
        setPending(false);
      });
  };
  return { installed, available: !!prompt, pending, failed, install };
}
