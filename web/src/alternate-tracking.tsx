import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { ChoiceDialog } from "./tracking-ui";
import { useTrackingDialogFocus } from "./use-tracking-dialog-focus";
import { localDay } from "./dates";
import type { T } from "./App";
export interface AlternateScope {
  catalog_id: string;
  episode_id?: string;
  season?: number;
  whole?: boolean;
  watched: boolean;
  view_anchor?: string;
}
export function useAlternateTracking(t: T, timezone?: string) {
  const client = useQueryClient();
  const [mode, setMode] = useState("today"),
    [day, setDay] = useState(localDay(timezone));
  const [prompt, setPrompt] = useState<{
    title: string;
    yes: string;
    no: string;
    resolve: (v: boolean | null) => void;
  } | null>(null);
  const active = useRef(true),
    pending = useRef<((v: boolean | null) => void) | null>(null);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      pending.current?.(null);
      pending.current = null;
    };
  }, []);
  const ask = (title: string, yes: string, no: string) =>
    new Promise<boolean | null>((resolve) => {
      pending.current = resolve;
      setPrompt({ title, yes, no, resolve });
    });
  const mutation = useMutation({
    mutationFn: async (scope: AlternateScope) => {
      const { catalog_id, ...action } = scope;
      if (action.watched && mode === "chosen" && !day) throw new Error(t("chooseWatchDate"));
      const date = mode === "unknown" ? {} : { day: mode === "today" ? localDay(timezone) : day };
      const path = `/library/${encodeURIComponent(catalog_id)}/track`;
      const preview = await api<{
        earlier: unknown[];
        offer_return_watching: boolean;
        linked_plan?: string;
        linked_changes?: { title: string; episode_id: string }[] | null;
      }>(path, "POST", { ...action, date, preview: true });
      if (!active.current) return;
      let earlier = false,
        include_linked = false,
        return_watching = false;
      if (action.watched && action.episode_id && preview.earlier.length) {
        const choice = await ask(t("earlierPrompt"), t("earlier"), t("onlyThis"));
        if (choice === null || !active.current) return;
        earlier = choice;
      }
      if (!action.watched && preview.linked_changes?.length) {
        const choice = await ask(
          t("linkedUnwatchPrompt") +
            "\n" +
            preview.linked_changes.map((e) => `${e.title} · ${e.episode_id}`).join("\n"),
          t("unwatchLinked"),
          t("cancel"),
        );
        if (!choice || !active.current) return;
        include_linked = true;
      }
      if (preview.offer_return_watching) {
        const choice = await ask(t("linkedReturnPrompt"), t("yes"), t("keep"));
        if (choice === null || !active.current) return;
        return_watching = choice;
      }
      if (!active.current) return;
      return api(path, "POST", {
        ...action,
        date,
        earlier,
        include_linked,
        linked_plan: include_linked ? preview.linked_plan : undefined,
        return_watching,
      });
    },
    onSuccess: async () => {
      for (const key of [
        "alternate",
        "history",
        "effective",
        "library",
        "dashboard",
        "statistics",
        "watched-titles",
      ]) {
        await client.invalidateQueries({ queryKey: [key] });
      }
    },
  });
  const trackingFocus = useTrackingDialogFocus(mutation.isPending);
  const controls = (
    <>
      <label>
        {t("watchDate")}{" "}
        <select
          value={mode}
          disabled={mutation.isPending}
          onChange={(e) => setMode(e.target.value)}
        >
          <option value="today">{t("today")}</option>
          <option value="chosen">{t("chosen")}</option>
          <option value="unknown">{t("unknown")}</option>
        </select>
      </label>
      {mode === "chosen" && (
        <input
          type="date"
          aria-label={t("watchDate")}
          value={day}
          disabled={mutation.isPending}
          required
          onChange={(e) => setDay(e.target.value)}
        />
      )}
      {mutation.error && <p role="alert">{mutation.error.message}</p>}
      {prompt && (
        <ChoiceDialog
          title={prompt.title}
          yes={prompt.yes}
          no={prompt.no}
          submit={t("save")}
          cancel={t("cancel")}
          onClose={(value) => {
            trackingFocus.requestRestore();
            pending.current = null;
            prompt.resolve(value);
            setPrompt(null);
          }}
        />
      )}
    </>
  );
  return {
    controls,
    run: (scope: AlternateScope) => {
      trackingFocus.capture();
      mutation.mutate(scope);
    },
    pending: mutation.isPending,
  };
}
