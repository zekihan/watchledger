import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";

interface SourceEpisode {
  key: string;
  season: number;
  number: number;
  date: { day?: string; timestamp?: string };
}
interface Candidate {
  id: string;
  season: number;
  number: number;
  retired: boolean;
}
interface ReviewData {
  source: { title: string; episodes: SourceEpisode[]; issues: string[] };
  target: string;
  episodes: Candidate[];
  links: { source_key: string; episode_id: string }[];
}
export function ImportReview({
  id,
  t,
  backed,
  close,
}: {
  id: string;
  t: T;
  backed: boolean;
  close: () => void;
}) {
  const candidates = useQuery({
    queryKey: ["import-candidates", id],
    queryFn: () => api<ReviewData>(`/import-reviews/${id}/candidates`),
  });
  if (candidates.isPending) return <p role="status">{t("loading")}</p>;
  if (candidates.error)
    return (
      <div>
        <p role="alert">{candidates.error.message}</p>
        <button onClick={close}>{t("cancel")}</button>
      </div>
    );
  return (
    <ReviewEditor key={id} id={id} data={candidates.data} t={t} backed={backed} close={close} />
  );
}
function ReviewEditor({
  id,
  data,
  t,
  backed,
  close,
}: {
  id: string;
  data: ReviewData;
  t: T;
  backed: boolean;
  close: () => void;
}) {
  const client = useQueryClient();
  const [links, setLinks] = useState<Record<string, string>>(() =>
    Object.fromEntries(data.links.map((link) => [link.source_key, link.episode_id])),
  );
  const [confirmed, setConfirmed] = useState(false);
  const [page, setPage] = useState(0);
  const known = Object.fromEntries(data.links.map((link) => [link.source_key, link.episode_id]));
  const selected = data.source.episodes.map((source) => links[source.key] ?? "");
  const complete = selected.every(Boolean) && new Set(selected).size === selected.length;
  const pages = Math.max(1, Math.ceil(data.source.episodes.length / 20));
  const mutation = useMutation({
    mutationFn: () =>
      api<{ history_added: number; app_decisions_preserved: number }>(
        `/import-reviews/${id}/resolve`,
        "POST",
        {
          confirm: true,
          accept_existing: true,
          links: data.source.episodes.map((source) => ({
            source_key: source.key,
            episode_id: links[source.key],
          })),
        },
      ),
    onSuccess: async () => {
      await client.invalidateQueries();
    },
  });
  return (
    <section className="panel" aria-label={t("resolveImport")}>
      <h4>{data.source.title}</h4>
      <p>
        {t("mappingTarget")}: {data.target || t("unresolved")}
      </p>
      <p className="notice">{t("importMappingSafety")}</p>
      {data.source.issues.length > 0 && <p role="alert">{data.source.issues.join(", ")}</p>}
      {!backed && <p>{t("backupFirst")}</p>}
      {data.source.episodes.length > 0 && (
        <>
          <p>
            {selected.filter(Boolean).length} / {data.source.episodes.length} ·{" "}
            {t("linkedEpisodes")}
          </p>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t("sourceEpisode")}</th>
                  <th>{t("history")}</th>
                  <th>{t("targetEpisode")}</th>
                </tr>
              </thead>
              <tbody>
                {data.source.episodes.slice(page * 20, page * 20 + 20).map((source) => (
                  <tr key={source.key}>
                    <td>
                      S{source.season} E{source.number}
                      <small>{source.key}</small>
                    </td>
                    <td>{source.date.day ?? source.date.timestamp ?? t("unknown")}</td>
                    <td>
                      <select
                        aria-label={`${t("targetEpisode")} ${source.key}`}
                        disabled={
                          Boolean(known[source.key]) || mutation.isPending || mutation.isSuccess
                        }
                        value={links[source.key] ?? ""}
                        onChange={(event) => {
                          setConfirmed(false);
                          setLinks({ ...links, [source.key]: event.target.value });
                        }}
                      >
                        <option value="">{t("select")}</option>
                        {data.episodes
                          .filter((episode) => !episode.retired || known[source.key] === episode.id)
                          .map((episode) => (
                            <option key={episode.id} value={episode.id}>
                              S{episode.season} E{episode.number} · {episode.id}
                              {episode.retired ? ` · ${t("retiredEpisode")}` : ""}
                            </option>
                          ))}
                      </select>
                      {known[source.key] && <small>{t("confirmedIdentity")}</small>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="actions">
            <button
              disabled={page === 0}
              onClick={() => {
                setPage(page - 1);
              }}
            >
              {t("previousPage")}
            </button>
            <span>
              {page + 1} / {pages}
            </span>
            <button
              disabled={page + 1 >= pages}
              onClick={() => {
                setPage(page + 1);
              }}
            >
              {t("nextPage")}
            </button>
          </div>
        </>
      )}
      <label className="check-label">
        <input
          type="checkbox"
          checked={confirmed}
          disabled={mutation.isPending || mutation.isSuccess}
          onChange={(event) => {
            setConfirmed(event.target.checked);
          }}
        />
        {t("confirmImportMapping")}
      </label>
      {mutation.error && <p role="alert">{mutation.error.message}</p>}
      {mutation.isSuccess && <p role="status">{t("importResolved")}</p>}
      <div className="actions">
        <button
          disabled={
            !backed ||
            !complete ||
            !confirmed ||
            !data.target ||
            data.source.issues.length > 0 ||
            mutation.isPending ||
            mutation.isSuccess
          }
          onClick={() => {
            mutation.mutate();
          }}
        >
          {t("resolveImport")}
        </button>
        <button disabled={mutation.isPending} onClick={close}>
          {t("cancel")}
        </button>
      </div>
    </section>
  );
}
