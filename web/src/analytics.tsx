import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import type { Dashboard, User } from "./api";
import type { T } from "./App";
export function AnalyticsPage({ t }: { t: T }) {
  const q = useQuery({
    queryKey: ["statistics"],
    queryFn: () =>
      api<{
        movies: number;
        episodes: number;
        estimated_minutes: number;
        unknown_runtime: number;
        partial_native_episodes: number;
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
  return (
    <>
      <h1>{t("statistics")}</h1>
      <div className="metric-grid">
        <div>
          <span>{t("watchedMovies")}</span>
          <strong>{d.movies}</strong>
        </div>
        <div>
          <span>{t("watchedEpisodes")}</span>
          <strong>{d.episodes}</strong>
        </div>
        <div>
          <span>{t("runtime")}</span>
          <strong>{d.estimated_minutes}</strong>
          <small>
            {t("unknownValue")}: {d.unknown_runtime}
          </small>
        </div>
      </div>
      <p className="muted">{t("canonicalAnimeStats")}</p>
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
      <h2>{t("activity")}</h2>
      <p className="muted">{t("unknownDates")}</p>
      {!d.monthly.length && <p>{t("empty")}</p>}
      {d.monthly.map((m) => (
        <div className="bar-row" key={m.month}>
          <span>{m.month}</span>
          <meter min="0" max={Math.max(...d.monthly.map((x) => x.count), 1)} value={m.count} />
          <strong>{m.count}</strong>
        </div>
      ))}
      <h2>{t("genreBreakdown")}</h2>
      {!d.genres.length && <p>{t("empty")}</p>}
      {d.genres.map((g) => (
        <div className="bar-row" key={g.name}>
          <span>{g.name}</span>
          <meter min="0" max={Math.max(...d.genres.map((x) => x.count), 1)} value={g.count} />
          <strong>{g.count}</strong>
        </div>
      ))}
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
  const [view, setView] = useState("month");
  const q = useQuery({
    queryKey: ["calendar", month],
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
          {q.error.message}
        </p>
      )}
      {q.isPending && <p>{t("loading")}</p>}
      {view === "month" ? (
        <div className="month-grid" role="grid" aria-label={month}>
          {Array.from({ length: 7 }, (_, i) => (
            <div className="weekday" key={`w${String(i)}`}>
              {new Intl.DateTimeFormat(lang, { weekday: "short" }).format(new Date(2026, 0, 5 + i))}
            </div>
          ))}
          {Array.from({ length: offset }, (_, i) => (
            <div className="calendar-cell blank" key={`blank${String(i)}`} />
          ))}
          {Array.from({ length: days }, (_, i) => {
            const day = `${month}-${String(i + 1).padStart(2, "0")}`;
            return (
              <div className="calendar-cell" role="gridcell" key={day}>
                <time dateTime={day}>{i + 1}</time>
                {q.data
                  ?.filter((e) => date(e) === day)
                  .map((e) => (
                    <a key={e.id} href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`}>
                      <strong>{e.title}</strong>
                      <small>
                        S{e.season} E{e.number}
                        {e.air_time
                          ? ` · ${new Date(e.air_time).toLocaleTimeString(lang, { timeZone: zone, hour: "2-digit", minute: "2-digit" })}`
                          : ""}
                      </small>
                    </a>
                  ))}
              </div>
            );
          })}
        </div>
      ) : (
        <div className="calendar-list">
          {q.data?.map((e) => (
            <a key={e.id} href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`}>
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
