import { episodeLink } from "./episode-links";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import type { Dashboard, User } from "./api";
import type { T } from "./App";
export function AnalyticsPage({ t, language = "en" }: { t: T; language?: string }) {
  const [allMonths, setAllMonths] = useState(false);
  const [allGenres, setAllGenres] = useState(false);
  const q = useQuery({
    queryKey: ["statistics"],
    queryFn: () =>
      api<{
        movies: number;
        episodes: number;
        estimated_minutes: number;
        unknown_runtime: number;
        partial_native_episodes: number;
        partial_movies: number;
        partial_source_records: number;
        unresolved_linked_records: number;
        derived_date_units: number;
        monthly: { month: string; count: number }[];
        genres: { name: string; count: number }[];
      }>("/statistics"),
  });
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <p className="error" role="alert">
        {q.error.message}
        <button
          onClick={() => {
            void q.refetch();
          }}
        >
          {t("retry")}
        </button>
      </p>
    );
  if (!q.data) return null;
  const d = q.data;
  const format = (value: number) => new Intl.NumberFormat(language).format(value);
  const months = allMonths ? d.monthly : d.monthly.slice(-12);
  const genres = allGenres ? d.genres : d.genres.slice(0, 8);
  const monthMax = Math.max(1, ...months.map((item) => item.count));
  const genreMax = Math.max(1, ...genres.map((item) => item.count));
  return (
    <>
      <h1>{t("statistics")}</h1>
      <div className="metric-grid">
        <div>
          <span>{t("watchedMovies")}</span>
          <strong>{format(d.movies)}</strong>
        </div>
        <div>
          <span>{t("watchedEpisodes")}</span>
          <strong>{format(d.episodes)}</strong>
        </div>
        <div>
          <span>{t("estimatedRuntime")}</span>
          <strong>{format(d.estimated_minutes)}</strong>
          <small>
            {t("runtimeUnknownCount")}: {format(d.unknown_runtime)}
          </small>
        </div>
      </div>
      <p className="muted">{t("canonicalAnimeStats")}</p>
      {(d.partial_movies ?? 0) > 0 && (
        <p>
          {t("partialMovieStats")}: {d.partial_movies}
        </p>
      )}
      {(d.partial_native_episodes ?? 0) > 0 && (
        <p>
          {t("partialNativeStats")}: {d.partial_native_episodes} · {t("sourceRecords")}:{" "}
          {d.partial_source_records}
        </p>
      )}
      {(d.unresolved_linked_records ?? 0) > 0 && (
        <p className="notice">
          {t("unresolvedStats")}: {d.unresolved_linked_records}
        </p>
      )}
      {(d.derived_date_units ?? 0) > 0 && (
        <p className="muted">
          {t("derivedStatsDates")}: {d.derived_date_units}
        </p>
      )}
      <div className="statistics-charts">
        <section className="statistics-chart">
          <h2>{t("activity")}</h2>
          <p className="muted">{t("unknownDates")}</p>
          {d.monthly.length > 12 && !allMonths && (
            <p className="muted">{t("recentActiveMonths")}</p>
          )}
          {!months.length && <p>{t("empty")}</p>}
          {months.map((m) => (
            <div className="bar-row" key={m.month}>
              <span>{m.month}</span>
              <meter aria-label={m.month} min="0" max={monthMax} value={m.count} />
              <strong>{format(m.count)}</strong>
            </div>
          ))}
          {d.monthly.length > 12 && (
            <button
              className="statistics-toggle"
              aria-label={`${t(allMonths ? "showLess" : "showMore")}: ${t("activity")}`}
              aria-expanded={allMonths}
              onClick={() => setAllMonths(!allMonths)}
            >
              {allMonths ? t("showLess") : `${t("showMore")} (${format(d.monthly.length - 12)})`}
            </button>
          )}
        </section>
        <section className="statistics-chart">
          <h2>{t("genreBreakdown")}</h2>
          {!genres.length && <p>{t("empty")}</p>}
          {genres.map((g) => (
            <div className="bar-row" key={g.name}>
              <span>{g.name}</span>
              <meter aria-label={g.name} min="0" max={genreMax} value={g.count} />
              <strong>{format(g.count)}</strong>
            </div>
          ))}
          {d.genres.length > 8 && (
            <button
              className="statistics-toggle"
              aria-label={`${t(allGenres ? "showLess" : "showMore")}: ${t("genreBreakdown")}`}
              aria-expanded={allGenres}
              onClick={() => setAllGenres(!allGenres)}
            >
              {allGenres ? t("showLess") : `${t("showMore")} (${format(d.genres.length - 8)})`}
            </button>
          )}
        </section>
      </div>
    </>
  );
}
export function CalendarPage({ t, user }: { t: T; user: User }) {
  const zone = user.Settings.timezone ?? "UTC",
    lang = user.Settings.language ?? "en";
  const [month, setMonth] = useState(() =>
    new Intl.DateTimeFormat("sv-SE", { timeZone: zone, year: "numeric", month: "2-digit" }).format(
      new Date(),
    ),
  );
  const [view, setView] = useState(() =>
    window.matchMedia?.("(max-width: 680px)").matches ? "list" : "month",
  );
  const q = useQuery({
    queryKey: ["calendar", month, zone, user.Settings.metadata_language, user.Settings.anime_title],
    queryFn: () => api<Dashboard["upcoming"]>(`/calendar?month=${month}`),
  });
  const y = Number(month.slice(0, 4)),
    m = Number(month.slice(5)) - 1,
    offset = (new Date(y, m, 1).getDay() + 6) % 7,
    days = new Date(y, m + 1, 0).getDate();
  const date = (e: Dashboard["upcoming"][number]) =>
    e.air_date ??
    (e.air_time
      ? new Intl.DateTimeFormat("sv-SE", {
          timeZone: zone,
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
        }).format(new Date(e.air_time))
      : "");
  return (
    <>
      <div className="page-heading">
        <h1>{t("calendar")}</h1>
        <div className="toolbar">
          <input
            aria-label={t("month")}
            type="month"
            value={month}
            onChange={(e) => {
              if (e.target.value) setMonth(e.target.value);
            }}
          />
          <select
            aria-label={t("calendar")}
            value={view}
            onChange={(e) => {
              setView(e.target.value);
            }}
          >
            <option value="month">{t("month")}</option>
            <option value="list">{t("listView")}</option>
          </select>
        </div>
      </div>
      {q.error && (
        <p className="error" role="alert">
          {q.error.message}{" "}
          <button
            onClick={() => {
              void q.refetch();
            }}
          >
            {t("retry")}
          </button>
        </p>
      )}
      {q.isPending && <p>{t("loading")}</p>}
      {view === "month" ? (
        <div
          className="calendar-scroll"
          role="region"
          aria-label={`${t("calendar")}: ${month}`}
          tabIndex={0}
        >
          <div className="month-grid" role="list" aria-label={month}>
            {Array.from({ length: 7 }, (_, i) => (
              <div className="weekday" aria-hidden="true" key={`w${String(i)}`}>
                {new Intl.DateTimeFormat(lang, { weekday: "short" }).format(
                  new Date(2026, 0, 5 + i),
                )}
              </div>
            ))}
            {Array.from({ length: offset }, (_, i) => (
              <div className="calendar-cell blank" aria-hidden="true" key={`blank${String(i)}`} />
            ))}
            {Array.from({ length: days }, (_, i) => {
              const day = `${month}-${String(i + 1).padStart(2, "0")}`;
              const events = q.data?.filter((e) => date(e) === day) ?? [];
              const renderEvent = (e: Dashboard["upcoming"][number]) => (
                <a key={e.id} href={episodeLink(e.catalog_id, e.season, e.id)}>
                  <strong>{e.title}</strong>
                  <small>
                    S{e.season} E{e.number}
                    {e.air_time
                      ? ` · ${new Date(e.air_time).toLocaleTimeString(lang, { timeZone: zone, hour: "2-digit", minute: "2-digit" })}`
                      : ""}
                  </small>
                </a>
              );
              return (
                <div className="calendar-cell" role="listitem" aria-label={day} key={day}>
                  <time dateTime={day}>{i + 1}</time>
                  {events.slice(0, 2).map(renderEvent)}
                  {events.length > 2 && (
                    <details className="calendar-more">
                      <summary aria-label={`${t("showMore")}: ${day}`}>
                        {t("showMore")} ({events.length - 2})
                      </summary>
                      {events.slice(2).map(renderEvent)}
                    </details>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ) : (
        <div className="calendar-list">
          {q.data?.map((e) => (
            <a key={e.id} href={episodeLink(e.catalog_id, e.season, e.id)}>
              <time>
                {e.air_time
                  ? new Date(e.air_time).toLocaleString(lang, { timeZone: zone })
                  : e.air_date}
              </time>
              <strong>{e.title}</strong>
              <small>
                S{e.season} E{e.number}
              </small>
            </a>
          ))}
          {q.data?.length === 0 && <p>{t("empty")}</p>}
        </div>
      )}
    </>
  );
}
