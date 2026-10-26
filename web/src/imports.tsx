import { PageControls } from "./page-controls";
import { importMessage } from "./import-messages";
import { QueryFeedback } from "./query-feedback";
import { ImportReview } from "./import-review";
import { useRef, useState } from "react";
import { usePreviewRequest } from "./use-preview-request";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, downloadExport } from "./api";
import type { T } from "./App";

interface EpisodeMatch {
  source_key: string;
  episode_id: string;
  season: number;
  number: number;
  title: string;
}
interface Preview {
  id: string;
  additions: number;
  conflicts: number;
  unresolved: number;
  warnings: string[];
  preparation_total?: number;
  pending_catalog?: number;
  episode_matches?: number;
  episode_exceptions?: number;
  episode_match_digest?: string;
  items: {
    key: string;
    title: string;
    action: string;
    issues: string[];
    warnings: string[];
    matches?: EpisodeMatch[];
    episode_exceptions?: number;
  }[];
}
interface ImportRecord {
  id: string;
  kind: string;
  state: string;
}
interface Review {
  id: string;
  import_id: string;
  reason: string;
  item: { source?: { title?: string; name?: string }; reasons?: string[] };
}
export function SimklPanel({ t, backed }: { t: T; backed: boolean }) {
  const client = useQueryClient();
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  const [previewPage, setPreviewPage] = useState(0);
  const [matchPage, setMatchPage] = useState(0);
  const [reviewPage, setReviewPage] = useState(0);
  const [activeReview, setActiveReview] = useState<string | null>(null);
  const beginPreview = usePreviewRequest();
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<{ next: number; total: number } | null>(null);
  const [resumeID, setResumeID] = useState<string | null>(null);
  const [confirmMatches, setConfirmMatches] = useState(false);
  const [error, setError] = useState("");
  const imports = useQuery({
    queryKey: ["imports"],
    queryFn: () => api<ImportRecord[]>("/imports"),
  });
  const reviews = useQuery({
    queryKey: ["import-reviews"],
    queryFn: () => api<Review[]>("/import-reviews"),
  });
  const visibleReviewPage = Math.min(
    reviewPage,
    Math.max(0, Math.ceil((reviews.data?.length ?? 0) / 50) - 1),
  );
  const apply = useMutation({
    mutationFn: async (selected: Preview) => {
      if (!backed) await downloadExport();
      return api<{ reviews: number; status: string }>(`/simkl/${selected.id}/apply`, "POST", {
        confirm_episode_matches: confirmMatches,
        episode_match_digest: confirmMatches ? selected.episode_match_digest : undefined,
      });
    },
    onSuccess: async (result) => {
      setPreview(null);
      setResumeID(null);
      setProgress(null);
      await client.invalidateQueries();
      if (result.reviews > 0) reviewHeading.current?.focus();
    },
  });
  async function prepare(initial: Preview, request: ReturnType<typeof beginPreview>, offset = 0) {
    setPreview(initial);
    setConfirmMatches(false);
    setMatchPage(0);
    if (!initial.preparation_total) return;
    setResumeID(initial.id);
    let next = offset;
    const total = initial.preparation_total;
    setProgress({ next, total });
    while (next < total && request.current()) {
      const result = await api<{ next: number; total: number; paused: boolean; preview?: Preview }>(
        `/simkl/${initial.id}/prepare`,
        "POST",
        { offset: next },
        request.signal,
      );
      if (!request.current()) return;
      setProgress({ next: result.next, total: result.total });
      if (result.preview) setPreview(result.preview);
      if (result.paused) {
        setError(t("importPreparationPaused"));
        return;
      }
      if (result.next <= next && result.next < result.total) throw new Error(t("error"));
      next = result.next;
    }
    if (request.current()) {
      setProgress(null);
      setResumeID(null);
    }
  }
  async function resume(id: string) {
    const request = beginPreview();
    setBusy(true);
    setError("");
    setPreview(null);
    apply.reset();
    try {
      const saved = await api<Preview>(`/simkl/${id}/preview`, "POST", {}, request.signal);
      if (request.current()) await prepare(saved, request);
    } catch (e) {
      if (request.current()) setError(e instanceof Error ? e.message : t("error"));
    } finally {
      if (request.current()) setBusy(false);
    }
  }
  async function read(file: File | undefined) {
    const request = beginPreview();
    setPreview(null);
    setPreviewPage(0);
    setProgress(null);
    setResumeID(null);
    setConfirmMatches(false);
    setBusy(false);
    setError("");
    apply.reset();
    if (!file) return;
    if (file.size > 32 * 1024 * 1024) {
      setError(t("importSize"));
      return;
    }
    setBusy(true);
    try {
      const value: unknown = JSON.parse(await file.text());
      if (!request.current()) return;
      const result = await api<Preview>("/simkl/preview", "POST", value, request.signal);
      if (request.current()) {
        await client.invalidateQueries({ queryKey: ["imports"] });
        if (request.current()) await prepare(result, request);
      }
    } catch (e) {
      if (request.current()) setError(e instanceof Error ? e.message : t("error"));
    } finally {
      if (request.current()) setBusy(false);
    }
  }
  return (
    <section aria-label={t("import")}>
      <h3>{t("import")}</h3>
      <p className="notice">{t("simklLimits")}</p>
      <label>
        {t("simklFile")}
        <input
          type="file"
          accept="application/json,.json"
          disabled={apply.isPending}
          onChange={(e) => {
            void read(e.target.files?.[0]);
          }}
        />
      </label>
      {busy && (
        <div role="status">
          <p>
            {progress
              ? `${t("importPreparing")}: ${progress.next} / ${progress.total}`
              : t("loading")}
          </p>
          {progress && (
            <progress
              aria-label={t("importPreparing")}
              value={progress.next}
              max={progress.total}
            />
          )}
          <p>{t("importPreparationSafe")}</p>
          <button
            onClick={() => {
              beginPreview();
              setBusy(false);
              setPreview(null);
            }}
          >
            {t("cancel")}
          </button>
        </div>
      )}
      {!busy && resumeID && (
        <button
          onClick={() => {
            void resume(resumeID);
          }}
        >
          {t("importRetryPreparation")}
        </button>
      )}
      {[error, apply.error?.message].filter(Boolean).map((message, i) => (
        <p role="alert" key={i}>
          {message}
        </p>
      ))}
      {preview && !busy && (
        <div className="preview">
          <h4>{t("previewReady")}</h4>
          <p>
            {t("additions")}: {preview.additions} · {t("conflicts")}: {preview.conflicts} ·{" "}
            {t("unresolved")}: {preview.unresolved}
          </p>
          <ul>
            {preview.warnings.map((warning) => (
              <li key={warning}>{importMessage(warning, t)}</li>
            ))}
          </ul>
          <details>
            <summary>
              {t("sourceItems")} ({preview.items.length})
            </summary>
            <ul>
              {preview.items.slice(previewPage * 50, (previewPage + 1) * 50).map((item, index) => (
                <li key={`${item.key}:${index}`}>
                  {item.title || item.key} · {importMessage(item.action, t)}
                  <ul>
                    {[...item.issues, ...item.warnings].map((issue, i) => (
                      <li key={i}>{importMessage(issue, t)}</li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
            <PageControls
              page={previewPage}
              count={preview.items.length}
              size={50}
              label={t("sourceItems")}
              change={setPreviewPage}
              t={t}
            />
          </details>
          {preview.items.length > 50 && <p>{t("importAllPages")}</p>}
          {!!preview.pending_catalog && (
            <p className="notice">
              {t("importUnavailableTitles")}: {preview.pending_catalog}
            </p>
          )}
          {!!preview.episode_matches && (
            <div className="panel">
              <h4>
                {t("importProposedEpisodes")}: {preview.episode_matches}
              </h4>
              <p>{t("importEpisodeProposalHint")}</p>
              <details>
                <summary>{t("importInspectMatches")}</summary>
                <ul>
                  {preview.items
                    .filter((item) => item.matches?.length)
                    .slice(matchPage * 50, (matchPage + 1) * 50)
                    .map((item) => (
                      <li key={item.key}>
                        <details>
                          <summary>
                            {item.title} · {item.matches?.length}
                          </summary>
                          <ul>
                            {item.matches?.map((match) => (
                              <li key={match.source_key}>
                                S{match.season} E{match.number} → {match.title || match.episode_id}
                              </li>
                            ))}
                          </ul>
                        </details>
                      </li>
                    ))}
                </ul>
                <PageControls
                  page={previewPage}
                  count={preview.items.filter((item) => item.matches?.length).length}
                  size={50}
                  label={t("importInspectMatches")}
                  change={setPreviewPage}
                  t={t}
                />
              </details>
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={confirmMatches}
                  disabled={apply.isPending}
                  onChange={(event) => setConfirmMatches(event.target.checked)}
                />
                {t("importConfirmEpisodes")}
              </label>
              {!confirmMatches && <p>{t("importEpisodesNotConfirmed")}</p>}
            </div>
          )}
          {!backed && <p>{t("importAutomaticBackup")}</p>}
          <button
            disabled={apply.isPending}
            onClick={() => {
              apply.mutate(preview);
            }}
          >
            {backed ? t("apply") : t("importBackupAndApply")}
          </button>
        </div>
      )}
      {apply.isSuccess && <p role="status">{t("simklApplied")}</p>}
      <h4>{t("sourceArchives")}</h4>
      <QueryFeedback query={imports} t={t} />
      {imports.isSuccess && !imports.data.some((row) => row.kind === "simkl") && (
        <p className="muted">{t("empty")}</p>
      )}
      <ul>
        {imports.data
          ?.filter((row) => row.kind === "simkl")
          .map((row) => (
            <li key={row.id}>
              <a href={`/api/imports/${row.id}/archive`} download>
                {t("export")} · {row.id}
              </a>{" "}
              · {importMessage(row.state, t)}
              {(row.state === "preview" || row.state === "applied") && (
                <button
                  disabled={busy || apply.isPending}
                  onClick={() => {
                    void resume(row.id);
                  }}
                >
                  {t("importContinue")}
                </button>
              )}
            </li>
          ))}
      </ul>
      <h4 ref={reviewHeading} tabIndex={-1}>
        {t("unresolved")} ({reviews.data?.length ?? 0})
      </h4>
      <QueryFeedback query={reviews} t={t} />
      {reviews.isSuccess && reviews.data.length === 0 && <p className="muted">{t("empty")}</p>}
      <p>{t("reviewInstructions")}</p>
      {activeReview && (
        <ImportReview
          key={activeReview}
          id={activeReview}
          t={t}
          backed={backed}
          close={() => {
            setActiveReview(null);
          }}
        />
      )}
      <ul>
        {reviews.data?.slice(visibleReviewPage * 50, (visibleReviewPage + 1) * 50).map((row) => (
          <li key={row.id}>
            {row.item.source?.title ?? row.item.source?.name ?? importMessage(row.reason, t)}
            {row.reason === "simkl_source_review" && (
              <button
                onClick={() => {
                  setActiveReview(row.id);
                }}
              >
                {t("resolveImport")}
              </button>
            )}
            <ul>
              {row.item.reasons?.map((reason, i) => (
                <li key={i}>{importMessage(reason, t)}</li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
      <PageControls
        page={visibleReviewPage}
        count={reviews.data?.length ?? 0}
        size={50}
        label={t("unresolved")}
        change={setReviewPage}
        t={t}
      />
    </section>
  );
}
