import { useAlternateTracking } from "./alternate-tracking";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";

interface Alternate {
  structure: string;
  attribution_notices?: string[];
  override?: string | null;
  incomplete: boolean;
  titles: {
    id: string;
    title: string;
    kind: string;
    base: string | null;
    rating: number | null;
    incomplete: boolean;
    watched: number;
    total: number;
    episodes: {
      id: string;
      season: number;
      number: number;
      name: string;
      retired: boolean;
      state: {
        direct: boolean;
        derived: boolean;
        blocked: boolean;
        sources: string[];
        mapping_id?: string;
        any_source?: boolean;
      };
    }[];
  }[];
}
export function AlternateView({ id, t, timezone }: { id: string; t: T; timezone?: string }) {
  const tracking = useAlternateTracking(t, timezone);
  const client = useQueryClient();
  const structure = useMutation({
    mutationFn: (view: string) =>
      api(`/library/${encodeURIComponent(id)}/structure`, "PUT", { view }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["alternate", id] });
      await client.invalidateQueries({ queryKey: ["library"] });
      await client.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
  const q = useQuery({
    queryKey: ["alternate", id],
    queryFn: () => api<Alternate>(`/library/${encodeURIComponent(id)}/alternate`),
  });
  return (
    <details className="panel alternate-disclosure">
      <summary>
        <span>{t("alternateViews")}</span>
        {q.data?.incomplete && <small>{t("mappingNeedsReview")}</small>}
      </summary>
      <p>{t("alternateHistoryNotice")}</p>
      {q.data && (
        <label>
          {t("seriesStructure")}
          <select
            disabled={structure.isPending}
            value={q.data.override ?? "default"}
            onChange={(e) => structure.mutate(e.target.value)}
          >
            <option value="default">{t("defaultStructure")}</option>
            <option value="native">{t("native")}</option>
            <option value="show">{t("showStructure")}</option>
          </select>
        </label>
      )}
      {structure.error && <p role="alert">{structure.error.message}</p>}
      {q.isPending && <p role="status">{t("loading")}</p>}
      {q.error && (
        <p role="alert">
          {q.error.message}
          <button
            onClick={() => {
              void q.refetch();
            }}
          >
            {t("retry")}
          </button>
        </p>
      )}
      {q.data?.incomplete && <p className="notice">{t("alternateIncomplete")}</p>}
      {q.data && tracking.controls}
      {[...(q.data?.titles ?? [])]
        .sort((a, b) =>
          q.data?.structure === "native"
            ? Number(b.kind === "anime") - Number(a.kind === "anime")
            : Number(b.kind === "tv") - Number(a.kind === "tv"),
        )
        .map((title) => (
          <section key={title.id}>
            <h3>
              {title.title} · {title.watched}/{title.total}
            </h3>
            <p>
              {title.base ?? t("notInLibrary")} · {t("rating")}: {title.rating ?? t("noRating")}
            </p>
            {title.incomplete && <p>{t("alternateIncomplete")}</p>}
            <a href={`#detail/${title.id.split(":").slice(1).join("/")}`}>{t("details")}</a>
            <div className="toolbar">
              {[true, false].map((watched) => (
                <button
                  key={String(watched)}
                  disabled={
                    tracking.pending ||
                    title.episodes.length === 0 ||
                    (title.base === null &&
                      ((title.kind !== "tv" && title.kind !== "movie") || title.incomplete))
                  }
                  onClick={() =>
                    tracking.run({
                      catalog_id: title.id,
                      whole: true,
                      watched,
                      view_anchor: title.base === null ? id : undefined,
                    })
                  }
                >
                  {t(title.kind === "movie" ? "movie" : "wholeScope")} ·{" "}
                  {watched ? t("watched") : t("unwatched")}
                </button>
              ))}
            </div>
            <div className="toolbar">
              {title.kind !== "movie" &&
                [...new Set(title.episodes.map((ep) => ep.season))].map((season) => (
                  <span key={season}>
                    {[true, false].map((watched) => (
                      <button
                        key={String(watched)}
                        disabled={
                          tracking.pending ||
                          (title.base === null &&
                            ((title.kind !== "tv" && title.kind !== "movie") || title.incomplete))
                        }
                        onClick={() =>
                          tracking.run({
                            catalog_id: title.id,
                            season,
                            watched,
                            view_anchor: title.base === null ? id : undefined,
                          })
                        }
                      >
                        {t("seasonScope")} {season} · {watched ? t("watched") : t("unwatched")}
                      </button>
                    ))}
                  </span>
                ))}
            </div>
            <ul>
              {title.episodes.map((ep) => (
                <li key={ep.id}>
                  <strong>
                    {title.kind === "movie" ? title.title : `${ep.season}:${ep.number} ${ep.name}`}
                  </strong>
                  {" · "}
                  <span>
                    {ep.state.direct
                      ? t("recordedWatch")
                      : ep.state.derived
                        ? t("derivedWatch")
                        : t("unwatched")}
                  </span>
                  {(title.base !== null ||
                    ep.state.direct ||
                    ((title.kind === "tv" || title.kind === "movie") &&
                      !ep.state.blocked &&
                      ep.state.sources.length > 0 &&
                      ep.state.sources.every(
                        (source) =>
                          (ep.state.any_source && source.startsWith("tmdb:movie:")) ||
                          q.data?.titles.some(
                            (native) =>
                              native.kind === "anime" &&
                              native.base !== null &&
                              native.episodes.some((e) => e.id === source),
                          ),
                      ))) &&
                    (!ep.retired || ep.state.direct) && (
                      <button
                        disabled={tracking.pending}
                        onClick={() =>
                          tracking.run({
                            catalog_id: title.id,
                            ...(title.kind === "movie" ? { whole: true } : { episode_id: ep.id }),
                            watched: !(ep.state.direct || ep.state.derived),
                            view_anchor: title.base === null ? id : undefined,
                          })
                        }
                      >
                        {ep.state.direct || ep.state.derived ? t("unwatched") : t("watched")}
                      </button>
                    )}
                  {ep.retired && <span> · {t("retiredEpisode")}</span>}
                  {ep.state.blocked && <span> · {t("mappingNeedsReview")}</span>}
                  {ep.state.sources.length > 0 && (
                    <small>
                      {" "}
                      ({t("connectedEpisodes")}:{" "}
                      {ep.state.sources
                        .map((id) => {
                          const title = q.data?.titles.find((title) =>
                            title.episodes.some((e) => e.id === id),
                          );
                          const e = title?.episodes.find((e) => e.id === id);
                          return title && e
                            ? title.kind === "movie"
                              ? title.title
                              : `${title.title} · ${e.season}:${e.number}${e.name ? ` · ${e.name}` : ""}`
                            : id;
                        })
                        .join(", ")}
                      )
                    </small>
                  )}
                </li>
              ))}
            </ul>
          </section>
        ))}
      {q.data?.attribution_notices?.map((notice, i) => (
        <p className="muted" key={i}>
          {notice}
        </p>
      ))}
      <a href="#mappings">{t("reviewMapping")}</a>
    </details>
  );
}
