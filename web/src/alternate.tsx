import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";

interface Alternate {
  structure: string;
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
      };
    }[];
  }[];
}
export function AlternateView({ id, t }: { id: string; t: T }) {
  const q = useQuery({
    queryKey: ["alternate", id],
    queryFn: () => api<Alternate>(`/library/${encodeURIComponent(id)}/alternate`),
  });
  return (
    <section className="panel">
      <h2>{t("alternateViews")}</h2>
      <p>{t("alternateHistoryNotice")}</p>
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
              {title.base ?? t("notInLibrary")} · {t("rating")}: {title.rating ?? t("unknown")}
            </p>
            {title.incomplete && <p>{t("alternateIncomplete")}</p>}
            <ul>
              {title.episodes.map((ep) => (
                <li key={ep.id}>
                  <strong>
                    {ep.season}:{ep.number} {ep.name}
                  </strong>
                  {" · "}
                  <span>
                    {ep.state.direct
                      ? t("recordedWatch")
                      : ep.state.derived
                        ? t("derivedWatch")
                        : t("unwatched")}
                  </span>
                  {ep.retired && <span> · {t("retiredEpisode")}</span>}
                  {ep.state.blocked && <span> · {t("mappingNeedsReview")}</span>}
                  {ep.state.sources.length > 0 && (
                    <small>
                      {" "}
                      ({t("connectedEpisodes")}: {ep.state.sources.join(", ")})
                    </small>
                  )}
                </li>
              ))}
            </ul>
          </section>
        ))}
      <a href="#mappings">{t("reviewMapping")}</a>
    </section>
  );
}
