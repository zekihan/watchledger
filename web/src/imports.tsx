import { ImportReview } from "./import-review";
import { useRef, useState } from "react";
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
  item: { source?: { title: string }; reasons?: string[] };
}
export function SimklPanel({ t, backed }: { t: T; backed: boolean }) {
  const client = useQueryClient();
  const [activeReview, setActiveReview] = useState<string | null>(null);
  const generation = useRef(0);
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
    const current = ++generation.current;
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
      const result = await api<Preview>("/simkl/preview", "POST", value);
      if (current === generation.current) setPreview(result);
    } catch (e) {
      if (current === generation.current) setError(e instanceof Error ? e.message : t("error"));
    } finally {
      if (current === generation.current) setBusy(false);
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
      {[error, apply.error?.message, imports.error?.message, reviews.error?.message]
        .filter(Boolean)
        .map((message, i) => (
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
            {row.item.source?.title ?? row.reason}
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
