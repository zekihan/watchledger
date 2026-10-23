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
type BaseStatus = "planned" | "watching" | "completed" | "hold" | "dropped";
interface WatchDecision {
  watched?: boolean;
  watched_date?: string | null;
  watched_time?: string | null;
  explicit_unwatch?: boolean;
}
interface Candidate extends WatchDecision {
  id: string;
  season: number;
  number: number;
  retired: boolean;
}
interface ReviewData {
  source: {
    title: string;
    episodes: SourceEpisode[];
    issues: string[];
    base?: BaseStatus;
    rating?: number | null;
    movie_watched?: boolean;
    movie_date?: { day?: string; timestamp?: string };
  };
  current?:
    | (WatchDecision & { base: BaseStatus; rating: number | null; status_name?: string | null })
    | null;
  catalog_cached: boolean;
  seasons: number[];
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
  const [season, setSeason] = useState("");
  const load = useMutation({
    mutationFn: (selectedSeason?: number) =>
      api<{ stale: boolean; warning?: string }>(
        `/import-reviews/${id}/catalog`,
        "POST",
        selectedSeason === undefined ? {} : { season: selectedSeason },
      ),
    onSuccess: async () => {
      setConfirmed(false);
      await client.invalidateQueries({ queryKey: ["import-candidates", id] });
    },
  });
  const known = Object.fromEntries(data.links.map((link) => [link.source_key, link.episode_id]));
  const selected = data.source.episodes.map((source) => links[source.key] ?? "");
  const complete =
    selected.every((target, index) =>
      data.episodes.some(
        (candidate) =>
          candidate.id === target &&
          (!candidate.retired || known[data.source.episodes[index]?.key ?? ""] === target),
      ),
    ) && new Set(selected).size === selected.length;
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

      {/^tmdb:(movie|tv):[0-9]+$/.test(data.target) && (
        <a href={`#detail/${data.target.split(":").slice(1).join("/")}`}>{t("openSourceTitle")}</a>
      )}
      <div className="settings-grid">
        <div>
          <strong>{t("incomingSource")}</strong>
          {data.source.movie_watched && (
            <p>
              {t("sourceWatchDate")}:{" "}
              {data.source.movie_date?.day ?? data.source.movie_date?.timestamp ?? t("unknown")}
            </p>
          )}
          <p>
            {t("status")}: {data.source.base ? t(data.source.base) : "—"} · {t("rating")}:{" "}
            {data.source.rating ?? t("noRating")}
          </p>
        </div>
        <div>
          <strong>{t("existingApp")}</strong>
          <p>
            {data.current
              ? `${t("status")}: ${data.current.status_name ?? t(data.current.base)} · ${t("rating")}: ${String(data.current.rating ?? t("noRating"))}`
              : t("notInLibrary")}
          </p>
          {data.current && data.source.episodes.length === 0 && (
            <WatchDecisionLabel value={data.current} t={t} />
          )}
        </div>
      </div>

      {/^tmdb:(movie|tv):[0-9]+$/.test(data.target) && (
        <div className="panel">
          <p>{t("loadMetadataHint")}</p>
          <button
            disabled={load.isPending || mutation.isPending || mutation.isSuccess}
            onClick={() => {
              setConfirmed(false);
              load.mutate(undefined);
            }}
          >
            {t("loadImportMetadata")}
          </button>
          {data.seasons.length > 0 && (
            <>
              <label>
                {t("targetSeason")}
                <select
                  value={season}
                  disabled={load.isPending || mutation.isPending || mutation.isSuccess}
                  onChange={(event) => {
                    setSeason(event.target.value);
                  }}
                >
                  <option value="">{t("select")}</option>
                  {data.seasons.map((number) => (
                    <option key={number} value={number}>
                      {number}
                    </option>
                  ))}
                </select>
              </label>
              <button
                disabled={
                  season === "" || load.isPending || mutation.isPending || mutation.isSuccess
                }
                onClick={() => {
                  setConfirmed(false);
                  load.mutate(Number(season));
                }}
              >
                {t("loadImportSeason")}
              </button>
            </>
          )}
          {load.isPending && <p role="status">{t("loading")}</p>}
          {load.error && <p role="alert">{load.error.message}</p>}
          {load.isSuccess && (
            <p role="status">{load.data.stale ? load.data.warning : t("catalogLoadedNoHistory")}</p>
          )}
        </div>
      )}
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
                      <WatchDecisionLabel
                        value={data.episodes.find((episode) => episode.id === links[source.key])}
                        t={t}
                      />
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
            !data.catalog_cached ||
            load.isPending ||
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

function WatchDecisionLabel({ value, t }: { value: WatchDecision | undefined; t: T }) {
  if (!value) return null;
  if (value.explicit_unwatch) return <small>{t("explicitUnwatch")}</small>;
  if (value.watched)
    return (
      <small>
        {t("existingWatch")}: {value.watched_date ?? value.watched_time ?? t("unknown")}
      </small>
    );
  return <small>{t("noWatchRecord")}</small>;
}
