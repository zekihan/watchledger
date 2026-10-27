import { useAlternateTracking } from "./alternate-tracking";
import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { Dashboard, User } from "./api";
import type { T } from "./App";
import { displayDate } from "./dates";
import { episodeLink } from "./episode-links";
export function DashboardPage({ user, t }: { user: User; t: T }) {
  const q = useQuery({ queryKey: ["dashboard"], queryFn: () => api<Dashboard>("/dashboard") });
  const client = useQueryClient();
  const tracking = useAlternateTracking(t, user.Settings.timezone);
  const dragged = useRef<string | null>(null);
  const [showAll, setShowAll] = useState(false);
  const order = useMutation({
    mutationFn: (ids: string[]) => api("/up-next/order", "PUT", { ids }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["dashboard"] }),
  });
  function move(id: string, before: string) {
    if (order.isPending || id === before) return;
    const current = (q.data?.up_next ?? []).map((entry) => entry.catalog_id);
    if (!current.includes(id) || !current.includes(before)) return;
    const ids = current.filter((entry) => entry !== id);
    ids.splice(ids.indexOf(before), 0, id);
    order.mutate(ids);
  }
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <div role="alert">
        {q.error.message}
        <button
          onClick={() => {
            void q.refetch();
          }}
        >
          {t("retry")}
        </button>
      </div>
    );
  if (!q.data) return null;
  const d = q.data;
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">{t("makeTime")}</p>
          <h1>{t("home")}</h1>
          <p className="muted">{user.Name}</p>
        </div>
        <a className="button" href="#discover">
          {t("discover")} ↗
        </a>
      </div>
      <div className="section-heading">
        <h2>{t("upNext")}</h2>
        <span className="muted">{d.up_next.length}</span>
      </div>
      {!!d.up_next.length && <div className="tracking-bar">{tracking.controls}</div>}
      {!d.up_next.length ? (
        <p>{t("empty")}</p>
      ) : (
        <div className="next-grid">
          {(showAll ? d.up_next : d.up_next.slice(0, 10)).map((e, i) => (
            <div key={e.episode_id} className="next-item">
              <a
                className="next-card"
                draggable={!order.isPending}
                onDragStart={(event) => {
                  dragged.current = e.catalog_id;
                  event.dataTransfer.effectAllowed = "move";
                  event.dataTransfer.setData("text/plain", e.catalog_id);
                }}
                onDragEnd={() => {
                  dragged.current = null;
                }}
                onDragOver={(event) => {
                  event.preventDefault();
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  const source = dragged.current;
                  dragged.current = null;
                  if (source) move(source, e.catalog_id);
                }}

                href={
                  e.view_anchor
                    ? `#views/${encodeURIComponent(e.view_anchor)}`
                    : episodeLink(e.catalog_id, e.season, e.episode_id)
                }
              >
                <span className="play">▷</span>

                <p>
                  S{e.season} · E{e.number}
                </p>
                <h2>{e.title}</h2>
                {e.incomplete && <small>{t("alternateIncomplete")}</small>}
                <span>{t("details")} →</span>
              </a>
              <div className="next-actions">
                <button
                  className="quick-watch"
                  data-watch-action="episode"
                  disabled={tracking.pending}
                  aria-label={`${t("watched")}: ${e.title}`}
                  onClick={() =>
                    tracking.run({
                      catalog_id: e.catalog_id,
                      episode_id: e.episode_id,
                      watched: true,
                      ...(e.view_anchor ? { view_anchor: e.view_anchor } : {}),
                    })
                  }
                >
                  ✓ {t("watched")}
                </button>
                {d.up_next.length > 1 && (
                  <>
                    <button
                      disabled={order.isPending || i === 0}
                      aria-label={`${t("moveUp")}: ${e.title}`}
                      onClick={() => {
                        const before = d.up_next[i - 1];
                        if (before) move(e.catalog_id, before.catalog_id);
                      }}
                    >
                      ↑
                    </button>
                    <button
                      disabled={order.isPending || i === d.up_next.length - 1}
                      aria-label={`${t("moveDown")}: ${e.title}`}
                      onClick={() => {
                        const after = d.up_next[i + 1];
                        if (after) move(after.catalog_id, e.catalog_id);
                      }}
                    >
                      ↓
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {order.error && (
        <p className="error" role="alert">
          {order.error.message}
        </p>
      )}
      {d.up_next.length > 10 && (
        <div className="next-show-more">
          <button aria-expanded={showAll} onClick={() => setShowAll(!showAll)}>
            {showAll ? t("showLess") : `${t("showMore")} (${d.up_next.length - 10})`}
          </button>
        </div>
      )}
      <div className="dashboard-columns">
        <section>
          <h2>{t("recent")}</h2>
          {!d.activity.length && <p>{t("empty")}</p>}
          {d.activity.map((e, i) => (
            <a
              className="activity-row"
              key={i}
              href={
                e.view_anchor
                  ? `#views/${encodeURIComponent(e.view_anchor)}`
                  : `#detail/${e.catalog_id.split(":").slice(1).join("/")}`
              }
            >
              <span className="activity-dot" />
              <strong>{e.title}</strong>
              <small>
                {displayDate(e.watched_date, e.watched_time, user.Settings, t("unknown"))}
              </small>
            </a>
          ))}
        </section>
        <section>
          <h2>{t("upcoming")}</h2>
          {!d.upcoming.length && <p>{t("empty")}</p>}
          {d.upcoming.slice(0, 8).map((e) => (
            <a className="activity-row" key={e.id} href={episodeLink(e.catalog_id, e.season, e.id)}>
              <strong>{e.title}</strong>
              <small>
                {displayDate(e.air_date, e.air_time, user.Settings, t("unknown"), true)}
              </small>
            </a>
          ))}
        </section>
      </div>
    </>
  );
}
