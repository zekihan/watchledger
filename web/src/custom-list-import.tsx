import { useState } from "react";
import { usePreviewRequest } from "./use-preview-request";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";
interface ListPreview {
  api_fetched_at?: string;
  id: string;
  source: { id: string; name: string; warnings: string[] };
  members: {
    key: string;
    title: string;
    catalog_id: string;
    reason?: string;
    source_url?: string;
  }[];
  action: string;
  unresolved: number;
  existing_list_id: string;
}
export function CustomListImport({ t, backed }: { t: T; backed: boolean }) {
  const client = useQueryClient();
  const beginPreview = usePreviewRequest();
  const [apiClientID, setAPIClientID] = useState(""),
    [apiToken, setAPIToken] = useState(""),
    [apiListID, setAPIListID] = useState("");
  const [preview, setPreview] = useState<ListPreview | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [page, setPage] = useState(0);
  const apply = useMutation({
    mutationFn: (id: string) =>
      api<{ status: string; list_id?: string }>(`/simkl/custom-lists/${id}/apply`, "POST", {}),
    onSuccess: async () => {
      setPreview(null);
      await client.invalidateQueries();
    },
  });
  async function read(files: FileList | null) {
    const request = beginPreview();
    setPreview(null);
    setError("");
    setBusy(false);
    setPage(0);
    apply.reset();
    if (!files?.length) return;
    const selected = Array.from(files);
    if (selected.reduce((total, file) => total + file.size, 0) > 32 * 1024 * 1024) {
      setError(t("importSize"));
      return;
    }
    setBusy(true);
    try {
      const pages: unknown[] = [];
      for (const file of selected) {
        const value: unknown = JSON.parse(await file.text());
        if (!request.current()) return;
        if (Array.isArray(value)) pages.push(...(value as unknown[]));
        else pages.push(value);
      }
      const result = await api<ListPreview>(
        "/simkl/custom-lists/preview",
        "POST",
        pages.length === 1 ? pages[0] : pages,
        request.signal,
      );
      if (request.current()) {
        setPreview(result);
        await client.invalidateQueries({ queryKey: ["imports"] });
      }
    } catch (e) {
      if (request.current()) setError(e instanceof Error ? e.message : t("error"));
    } finally {
      if (request.current()) setBusy(false);
    }
  }
  async function fetchSource() {
    const request = beginPreview();
    setPreview(null);
    setError("");
    setBusy(true);
    setPage(0);
    apply.reset();
    const body = {
      client_id: apiClientID.trim(),
      access_token: apiToken.trim(),
      list_id: apiListID.trim(),
    };
    setAPIToken("");
    try {
      const pending = api<ListPreview>("/simkl/custom-lists/fetch", "POST", body, request.signal);
      body.access_token = "";
      const result = await pending;
      if (request.current()) {
        setPreview(result);
        await client.invalidateQueries({ queryKey: ["imports"] });
      }
    } catch (e) {
      if (request.current()) setError(e instanceof Error ? e.message : t("error"));
    } finally {
      body.access_token = "";
      if (request.current()) {
        setBusy(false);
      }
    }
  }
  return (
    <section className="panel" aria-label={t("customListImport")}>
      <h3>{t("customListImport")}</h3>
      <p>{t("customListImportHint")}</p>
      <p className="notice">{t("customListAtomic")}</p>
      <details>
        <summary>{t("simklReadAPI")}</summary>
        <p>{t("simklReadAPIHint")}</p>
        <p>
          <a href="https://api.simkl.org/guides/custom-lists" target="_blank" rel="noreferrer">
            SIMKL API
          </a>{" "}
          ·{" "}
          <a href="https://api.simkl.org/api-rules" target="_blank" rel="noreferrer">
            {t("sourceRules")}
          </a>
        </p>
        <fieldset disabled={busy || apply.isPending}>
          <label>
            {t("simklClientID")}
            <input
              autoComplete="off"
              value={apiClientID}
              onChange={(e) => setAPIClientID(e.target.value)}
            />
          </label>
          <label>
            {t("simklAccessToken")}
            <input
              type="password"
              autoComplete="off"
              data-bwignore="true"
              value={apiToken}
              onChange={(e) => setAPIToken(e.target.value)}
            />
          </label>
          <label>
            {t("simklListID")}
            <input
              inputMode="numeric"
              value={apiListID}
              onChange={(e) => setAPIListID(e.target.value)}
            />
          </label>
          <button
            disabled={!apiClientID.trim() || !apiToken.trim() || !/^\d+$/.test(apiListID)}
            onClick={() => {
              void fetchSource();
            }}
          >
            {t("fetchListPreview")}
          </button>
        </fieldset>
      </details>
      <label>
        {t("customListFiles")}
        <input
          type="file"
          multiple
          accept="application/json,.json"
          disabled={apply.isPending}
          onChange={(e) => {
            void read(e.target.files);
          }}
        />
      </label>
      {busy && <p role="status">{t("loading")}</p>}
      {[error, apply.error?.message].filter(Boolean).map((message, i) => (
        <p role="alert" key={i}>
          {message}
        </p>
      ))}
      {preview && !busy && (
        <div className="preview">
          <h4>{preview.source.name}</h4>
          <p>
            <img
              src="https://us.simkl.in/img_favicon/v2/favicon-192x192.png"
              alt="Simkl"
              width="18"
              height="18"
            />{" "}
            SIMKL
          </p>
          {preview.api_fetched_at && (
            <p>
              {t("freshness")}: {new Date(preview.api_fetched_at).toLocaleString()}
            </p>
          )}
          <p>
            {preview.members.length} {t("title")} · {t("unresolved")}: {preview.unresolved}
          </p>
          {preview.source.warnings.length > 0 && <p>{t("manualSnapshot")}</p>}
          {preview.existing_list_id && <p>{t("keepExistingList")}</p>}
          {preview.action === "preserve_existing_app_deletion" && <p>{t("keepDeletedList")}</p>}
          <ol>
            {preview.members.slice(page * 50, page * 50 + 50).map((member) => (
              <li key={member.key}>
                {member.title} · {member.catalog_id || member.reason}
                {member.source_url && (
                  <a href={member.source_url} target="_blank" rel="noreferrer">
                    {" "}
                    · SIMKL ↗
                  </a>
                )}
              </li>
            ))}
          </ol>
          {preview.members.length > 50 && (
            <div className="toolbar">
              <button
                disabled={page === 0}
                onClick={() => {
                  setPage(page - 1);
                }}
              >
                {t("previousPage")}
              </button>
              <span>
                {page + 1} / {Math.ceil(preview.members.length / 50)}
              </span>
              <button
                disabled={(page + 1) * 50 >= preview.members.length}
                onClick={() => {
                  setPage(page + 1);
                }}
              >
                {t("nextPage")}
              </button>
            </div>
          )}
          <button
            disabled={!backed || apply.isPending}
            onClick={() => {
              apply.mutate(preview.id);
            }}
          >
            {preview.action === "create_manual_snapshot"
              ? t("importManualSnapshot")
              : t("archiveForReview")}
          </button>
        </div>
      )}
      {apply.isSuccess && (
        <p role="status">
          {apply.data.status === "created" ? t("customListCreated") : t("customListRetained")}
        </p>
      )}
    </section>
  );
}
