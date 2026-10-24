import { QueryFeedback } from "./query-feedback";
import { ImportReview } from "./import-review";
import { useState } from "react";
import { usePreviewRequest } from "./use-preview-request";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";

interface Preview {
  id: string;
  additions: number;
  conflicts: number;
  unresolved: number;
  warnings: string[];
  items: { key: string; title: string; action: string; issues: string[]; warnings: string[] }[];
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
  const [activeReview, setActiveReview] = useState<string | null>(null);
  const beginPreview = usePreviewRequest();
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const imports = useQuery({
    queryKey: ["imports"],
    queryFn: () => api<ImportRecord[]>("/imports"),
  });
  const reviews = useQuery({
    queryKey: ["import-reviews"],
    queryFn: () => api<Review[]>("/import-reviews"),
  });
  const apply = useMutation({
    mutationFn: (id: string) =>
      api<{ reviews: number; status: string }>(`/simkl/${id}/apply`, "POST", {}),
    onSuccess: async () => {
      setPreview(null);
      await client.invalidateQueries();
    },
  });
  async function read(file: File | undefined) {
    const request = beginPreview();
    setPreview(null);
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
      if (request.current()) setPreview(result);
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
      {busy && <p role="status">{t("loading")}</p>}
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
              <li key={warning}>{warning}</li>
            ))}
          </ul>
          <details>
            <summary>
              {t("sourceItems")} ({preview.items.length})
            </summary>
            <ul>
              {preview.items.map((item, index) => (
                <li key={`${item.key}:${index}`}>
                  {item.title || item.key} · {item.action}
                  <ul>
                    {[...item.issues, ...item.warnings].map((issue, i) => (
                      <li key={i}>{issue}</li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
          </details>
          <button
            disabled={!backed || apply.isPending}
            onClick={() => {
              apply.mutate(preview.id);
            }}
          >
            {t("apply")}
          </button>
        </div>
      )}
      {apply.isSuccess && <p role="status">{t("simklApplied")}</p>}
      <h4>{t("sourceArchives")}</h4>
      <QueryFeedback query={imports} t={t} />
      <ul>
        {imports.data
          ?.filter((row) => row.kind === "simkl")
          .map((row) => (
            <li key={row.id}>
              <a href={`/api/imports/${row.id}/archive`} download>
                {t("export")} · {row.id}
              </a>{" "}
              · {row.state}
            </li>
          ))}
      </ul>
      <h4>{t("unresolved")}</h4>
      <QueryFeedback query={reviews} t={t} />
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
        {reviews.data?.map((row) => (
          <li key={row.id}>
            {row.item.source?.title ?? row.item.source?.name ?? row.reason}
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
                <li key={i}>{reason}</li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    </section>
  );
}
