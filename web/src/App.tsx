import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, APIError } from "./api";
import type { Dashboard, Episode, History, LibraryItem, Status, Title, User } from "./api";
import { en, tr } from "./i18n";
import type { Key } from "./i18n";
import { ListsPage, SettingsPage } from "./pages";
import { AnalyticsPage, CalendarPage } from "./analytics";
import { AlternateView } from "./alternate";
import { MappingPage, Notices } from "./admin";
export type T = (key: Key) => string;
const bases = ["planned", "watching", "completed", "hold", "dropped"] as const;
const nav = ["home", "library", "discover", "lists", "calendar", "statistics", "settings"] as const;
const icons = ["◈", "▤", "⌕", "☷", "▦", "▥", "⚙"];
function subscribe(callback: () => void) {
  window.addEventListener("hashchange", callback);
  return () => {
    window.removeEventListener("hashchange", callback);
  };
}
function route() {
  return location.hash.slice(1) || "home";
}
function image(path: string | undefined, size = "w342") {
  return path ? `https://image.tmdb.org/t/p/${size}${path}` : undefined;
}
function localDay(timezone?: string) {
  if (timezone)
    return new Intl.DateTimeFormat("sv-SE", {
      timeZone: timezone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date());
  const d = new Date();
  return `${String(d.getFullYear())}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}
function ErrorBox({ error, retry, t }: { error: Error; retry?: () => void; t: T }) {
  return (
    <div role="alert" className="error">
      <strong>{t("error")}</strong>
      <p>{error.message}</p>
      {retry && <button onClick={retry}>{t("retry")}</button>}
    </div>
  );
}
function Poster({ title, path }: { title: string; path?: string }) {
  const [failed, setFailed] = useState(false);
  return path && !failed ? (
    <img
      loading="lazy"
      src={image(path)}
      alt={title}
      onError={() => {
        setFailed(true);
      }}
    />
  ) : (
    <div className="poster-placeholder" aria-label={title}>
      W<span>{title}</span>
    </div>
  );
}
function Cards({ results, kind, t }: { results: Title[]; kind?: string; t: T }) {
  return (
    <div className="poster-grid">
      {results
        .filter(
          (item, i, all) =>
            all.findIndex(
              (other) =>
                other.id === item.id && (other.media_type ?? kind) === (item.media_type ?? kind),
            ) === i,
        )
        .map((item) => (
          <a
            className="poster-card"
            key={`${item.media_type ?? kind}:${String(item.id)}`}
            href={`#detail/${item.media_type ?? kind ?? "movie"}/${String(item.id)}`}
          >
            <div className="poster">
              <Poster
                title={item.title ?? item.name ?? t("unknown")}
                path={item.poster_path ?? item.profile_path}
              />
              {item.vote_average !== undefined && (
                <span className="score">★ {item.vote_average.toFixed(1)}</span>
              )}
            </div>
            <strong>{item.title ?? item.name}</strong>
            <small>
              {(item.release_date ?? item.first_air_date)?.slice(0, 4)} · {item.media_type ?? kind}
            </small>
          </a>
        ))}
    </div>
  );
}
export function App() {
  const page = useSyncExternalStore(subscribe, route);
  const [fallbackLang, setFallbackLang] = useState(() =>
    navigator.language.startsWith("tr") ? "tr" : "en",
  );
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<User>("/me"), retry: false });
  const language = me.data?.Settings.language ?? fallbackLang;
  const t: T = (key) => (language === "tr" ? tr : en)[key];
  const [search, setSearch] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);
  useEffect(() => {
    const listener = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (e.target instanceof HTMLElement && e.target.closest("dialog[open]")) return;
      const editing =
        (e.target instanceof HTMLElement && e.target.isContentEditable) ||
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target instanceof HTMLSelectElement;
      if (
        !editing &&
        e.key.toLowerCase() === "w" &&
        document.activeElement instanceof HTMLButtonElement &&
        document.activeElement.dataset.watchAction
      ) {
        e.preventDefault();
        document.activeElement.click();
        return;
      }
      if (!editing && (e.key === "ArrowRight" || e.key === "ArrowLeft")) {
        const cards = Array.from(document.querySelectorAll<HTMLAnchorElement>(".poster-card"));
        if (cards.length) {
          const active = cards.findIndex((card) => card === document.activeElement);
          const next =
            e.key === "ArrowRight"
              ? Math.min(active + 1, cards.length - 1)
              : Math.max(active - 1, 0);
          e.preventDefault();
          cards[next]?.focus();
          return;
        }
      }

      if (e.key === "/" && !editing) {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", listener);
    return () => {
      window.removeEventListener("keydown", listener);
    };
  }, []);
  if (me.isPending) return <div className="center">{t("loading")}</div>;
  if (me.error) {
    if (me.error instanceof APIError && me.error.status === 401)
      return (
        <main className="login">
          <div className="wordmark">
            <span>W</span> watchledger
          </div>
          <div className="login-copy">
            <p className="eyebrow">{t("nextChapter")}</p>
            <h1>{t("intro")}</h1>
            <p>{t("subtitle")}</p>
            <a className="button primary" href="/auth/login">
              {t("signIn")} ↗
            </a>
            <small>{t("loginNote")}</small>
            <button
              className="text-button"
              onClick={() => {
                setFallbackLang(language === "en" ? "tr" : "en");
              }}
            >
              {language === "en" ? "Türkçe" : "English"}
            </button>
          </div>
          <div className="login-art" aria-hidden="true">
            <div>W</div>
            <span>{t("watchRemember")}</span>
          </div>
        </main>
      );
    return (
      <main className="center">
        <ErrorBox
          error={me.error}
          retry={() => {
            void me.refetch();
          }}
          t={t}
        />
      </main>
    );
  }
  if (!me.data) return null;
  const user = me.data;
  const root = page.split("/")[0];
  return (
    <div className="app-shell">
      <aside>
        <a className="wordmark" href="#home">
          <span>W</span> watchledger
        </a>
        <p className="nav-label">{t("collection")}</p>
        <nav>
          {nav.map((item, i) => (
            <a key={item} className={root === item ? "active" : ""} href={`#${item}`}>
              <span aria-hidden="true">{icons[i]}</span>
              {t(item)}
            </a>
          ))}
          {user.Admin && <a href="#admin">◇ {t("admin")}</a>}
        </nav>
        <div className="sidebar-foot">
          <div className="avatar">{user.Name.slice(0, 1) || "W"}</div>
          <div>
            <strong>{user.Name || "Watchledger"}</strong>
            <small>{t("loginNote")}</small>
          </div>
        </div>
      </aside>
      <div className="main">
        <header>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              location.hash = `search/${encodeURIComponent(search)}`;
            }}
          >
            <span aria-hidden="true">⌕</span>
            <input
              ref={searchRef}
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
              }}
              placeholder={t("search")}
              aria-label={t("search")}
            />
            <kbd>/</kbd>
          </form>
          <a href="#settings" className="avatar" aria-label={t("settings")}>
            {user.Name.slice(0, 1) || "W"}
          </a>
        </header>
        <main className="content">
          <Notices t={t} />
          {root === "detail" ? (
            <DetailPage key={page} parts={page.split("/")} user={user} t={t} />
          ) : root === "views" ? (
            <AlternateView key={page} id={decodeURIComponent(page.slice(6))} t={t} />
          ) : root === "library" ? (
            <LibraryPage t={t} />
          ) : root === "search" || root === "discover" ? (
            <DiscoveryPage
              key={page}
              search={root === "search" ? decodeURIComponent(page.slice(7)) : undefined}
              user={user}
              t={t}
            />
          ) : root === "lists" ? (
            <ListsPage t={t} user={user} />
          ) : root === "statistics" ? (
            <AnalyticsPage t={t} />
          ) : root === "calendar" ? (
            <CalendarPage t={t} user={user} />
          ) : root === "settings" ? (
            <SettingsPage user={user} t={t} />
          ) : root === "admin" || root === "mappings" ? (
            <MappingPage t={t} admin={user.Admin} />
          ) : (
            <DashboardPage page={root} user={user} t={t} />
          )}
          <footer>
            {t("attribution")}{" "}
            <a href="https://www.themoviedb.org" target="_blank" rel="noreferrer">
              TMDB
            </a>{" "}
            ·{" "}
            <a href="https://www.justwatch.com" target="_blank" rel="noreferrer">
              JustWatch
            </a>
            <p>{t("incomplete")}</p>
          </footer>
        </main>
      </div>
    </div>
  );
}
function LibraryPage({ t }: { t: T }) {
  const q = useQuery({ queryKey: ["library"], queryFn: () => api<LibraryItem[]>("/library") });
  const [filter, setFilter] = useState("all"),
    [term, setTerm] = useState(""),
    [table, setTable] = useState(false);
  const [order, setOrder] = useState("title"),
    [contentType, setContentType] = useState("all");
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <ErrorBox
        error={q.error}
        retry={() => {
          void q.refetch();
        }}
        t={t}
      />
    );
  const entries = (q.data ?? []).filter(
    (x) =>
      (contentType === "all" || contentType === x.kind) &&
      (filter === "all" || x.base === filter) &&
      x.title.toLocaleLowerCase().includes(term.toLocaleLowerCase()),
  );
  entries.sort((a, b) => {
    if (order === "rating") return (b.rating ?? 0) - (a.rating ?? 0);
    if (order === "added") return b.added_at.localeCompare(a.added_at);
    return a.title.localeCompare(b.title);
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">{t("yourStories")}</p>
          <h1>{t("library")}</h1>
        </div>
        <button
          onClick={() => {
            setTable(!table);
          }}
        >
          {t(table ? "grid" : "table")}
        </button>
      </div>
      <div className="toolbar">
        <input
          aria-label={t("search")}
          placeholder={t("title")}
          value={term}
          onChange={(e) => {
            setTerm(e.target.value);
          }}
        />
        <select
          aria-label={t("status")}
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
          }}
        >
          <option value="all">{t("all")}</option>
          {bases.map((b) => (
            <option key={b} value={b}>
              {t(b)}
            </option>
          ))}
        </select>
        <select
          aria-label={t("kind")}
          value={contentType}
          onChange={(e) => {
            setContentType(e.target.value);
          }}
        >
          {["all", "movie", "tv", "anime"].map((k) => (
            <option key={k} value={k}>
              {t(k as Key)}
            </option>
          ))}
        </select>
        <select
          aria-label={t("sort")}
          value={order}
          onChange={(e) => {
            setOrder(e.target.value);
          }}
        >
          {["title", "rating", "added"].map((k) => (
            <option key={k} value={k}>
              {t(k as Key)}
            </option>
          ))}
        </select>
        <span>
          {entries.length} {t("title")}
        </span>
      </div>
      {!entries.length ? (
        <Empty t={t} />
      ) : table ? (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("title")}</th>
                <th>{t("status")}</th>
                <th>{t("rating")}</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((x) => (
                <tr key={x.id}>
                  <td>
                    <a href={`#detail/${x.id.split(":").slice(1).join("/")}`}>{x.title}</a>
                  </td>
                  <td>{x.status_name ?? t(bases.find((b) => b === x.base) ?? "planned")}</td>
                  <td>{x.rating ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="poster-grid">
          {entries.map((x) => {
            const m = Object.values(x.metadata)[0];
            return (
              <a
                className="poster-card"
                key={x.id}
                href={`#detail/${x.id.split(":").slice(1).join("/")}`}
              >
                <div className="poster">
                  <Poster title={x.title} path={m?.poster_path} />
                  <span className="status-badge">
                    {x.status_name ?? t(bases.find((b) => b === x.base) ?? "planned")}
                  </span>
                </div>
                <strong>{x.title}</strong>
                <small>{x.rating ? `★ ${String(x.rating)}` : t("noRating")}</small>
              </a>
            );
          })}
        </div>
      )}
    </>
  );
}
function Empty({ t }: { t: T }) {
  return (
    <div className="empty">
      <span>◈</span>
      <h2>{t("empty")}</h2>
      <p>{t("noData")}</p>
      <a className="button" href="#discover">
        {t("discover")} ↗
      </a>
    </div>
  );
}
function DiscoveryPage({ search, user, t }: { search?: string; user: User; t: T }) {
  const [advanced, setAdvanced] = useState<Record<string, string>>({});
  const [browse, setBrowse] = useState("popular");
  const [exclude, setExclude] = useState(false);
  const library = useQuery({
    queryKey: ["library"],
    queryFn: () => api<LibraryItem[]>("/library"),
  });
  const [kind, setKind] = useState(search ? "multi" : "movie"),
    [page, setPage] = useState(1),
    [year, setYear] = useState(""),
    [genre, setGenre] = useState(""),
    [score, setScore] = useState(""),
    [votes, setVotes] = useState("50");
  const params = new URLSearchParams({
    language: user.Settings.metadata_language ?? "en",
    page: String(page),
  });
  if (search) {
    params.set("query", search);
    params.set("kind", kind);
  } else {
    if (year) params.set(kind === "tv" ? "first_air_date_year" : "year", year);
    if (genre) params.set("with_genres", genre);
    if (score) params.set("vote_average.gte", score);
    params.set("vote_count.gte", votes);
    if (browse === "upcoming") {
      params.set(
        kind === "tv" ? "first_air_date.gte" : "primary_release_date.gte",
        localDay(user.Settings.timezone),
      );
      params.set("sort_by", kind === "tv" ? "first_air_date.asc" : "primary_release_date.asc");
    } else {
      params.set("sort_by", "popularity.desc");
    }
  }
  for (const [key, value] of Object.entries(advanced)) {
    if (value) params.set(key, value);
  }
  const path = search ? `/search?${params.toString()}` : `/discover/${kind}?${params.toString()}`;
  const q = useQuery({
    queryKey: ["catalog", path],
    queryFn: () =>
      api<{
        results: Title[];
        total_pages: number;
        exhaustive?: boolean;
        search_scope?: string;
        enrichment_failures?: number;
      }>(path),
    retry: false,
  });
  return (
    <>
      <p className="eyebrow">{t("findFavorite")}</p>
      <h1>{search ? `“${search}”` : t("discover")}</h1>
      <div className="toolbar">
        <select
          aria-label={t("kind")}
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setPage(1);
          }}
        >
          {(search ? ["multi", "movie", "tv", "anime", "person"] : ["movie", "tv"]).map((k) => (
            <option key={k} value={k}>
              {k === "multi" ? t("all") : t(k as "movie" | "tv" | "anime" | "person")}
            </option>
          ))}
        </select>
        {!search && (
          <>
            <select
              aria-label={t("browse")}
              value={browse}
              onChange={(e) => {
                setBrowse(e.target.value);
                setVotes(e.target.value === "upcoming" ? "0" : "50");
                setPage(1);
              }}
            >
              <option value="popular">{t("popularity")}</option>
              <option value="upcoming">{t("upcoming")}</option>
            </select>

            <input
              aria-label={t("year")}
              placeholder={t("year")}
              value={year}
              onChange={(e) => {
                setYear(e.target.value);
                setPage(1);
              }}
            />
            <input
              aria-label={t("genre")}
              placeholder={t("genre")}
              value={genre}
              onChange={(e) => {
                setGenre(e.target.value);
                setPage(1);
              }}
            />
            <input
              aria-label={t("score")}
              placeholder={t("score")}
              type="number"
              min="0"
              max="10"
              value={score}
              onChange={(e) => {
                setScore(e.target.value);
                setPage(1);
              }}
            />
            <input
              aria-label={t("minVotes")}
              title={t("minVotes")}
              type="number"
              min="0"
              value={votes}
              onChange={(e) => {
                setVotes(e.target.value);
                setPage(1);
              }}
            />
          </>
        )}
      </div>
      {!search && (
        <details className="panel">
          <summary>{t("filters")}</summary>
          <div className="settings-grid">
            {(
              [
                ["with_original_language", "originalLanguage"],
                ["with_runtime.gte", "minRuntime"],
                ["with_runtime.lte", "maxRuntime"],
                [kind === "tv" ? "first_air_date.gte" : "primary_release_date.gte", "fromDate"],
                [kind === "tv" ? "first_air_date.lte" : "primary_release_date.lte", "toDate"],
                [kind === "tv" ? "person_id" : "with_people", "personID"],
                ["watch_region", "country"],
                ["with_watch_providers", "service"],
              ] as const
            ).map(([key, label]) => (
              <label key={key}>
                {t(label)}
                <input
                  value={advanced[key] ?? ""}
                  onChange={(e) => {
                    setAdvanced({ ...advanced, [key]: e.target.value });
                    setPage(1);
                  }}
                />
              </label>
            ))}
            {(kind === "tv" || kind === "anime") && (
              <label>
                {t("cast")} / {t("crew")}
                <select
                  value={advanced.credit ?? "cast"}
                  onChange={(e) => {
                    setAdvanced({ ...advanced, credit: e.target.value });
                    setPage(1);
                  }}
                >
                  <option value="cast">{t("cast")}</option>
                  <option value="crew">{t("crew")}</option>
                </select>
              </label>
            )}
            <button
              onClick={() => {
                setAdvanced({
                  ...advanced,
                  watch_region: user.Settings.country ?? "TR",
                  with_watch_providers: user.Settings.services?.join("|") ?? "",
                  with_watch_monetization_types: "flatrate",
                });
                setPage(1);
              }}
            >
              {t("myServices")}
            </button>
          </div>
        </details>
      )}
      <label className="check-label">
        <input
          type="checkbox"
          checked={exclude}
          onChange={(e) => {
            setExclude(e.target.checked);
          }}
        />
        {t("excludeWatched")}
      </label>
      {q.data?.exhaustive === false && (
        <p className="notice">
          {t(kind === "anime" ? "animeSearchScope" : "candidateNotice")}{" "}
          {q.data.enrichment_failures ? `${t("error")}: ${String(q.data.enrichment_failures)}` : ""}
        </p>
      )}
      {q.isPending ? (
        <p>{t("loading")}</p>
      ) : q.error ? (
        <ErrorBox
          error={q.error}
          retry={() => {
            void q.refetch();
          }}
          t={t}
        />
      ) : q.data?.results.length ? (
        <Cards
          results={q.data.results.filter(
            (item) =>
              !exclude ||
              !library.data?.some(
                (l) =>
                  l.id ===
                    `${(item.media_type ?? kind) === "anime" ? "anidb" : "tmdb"}:${item.media_type ?? kind}:${String(item.id)}` &&
                  l.base === "completed",
              ),
          )}
          kind={kind}
          t={t}
        />
      ) : (
        <Empty t={t} />
      )}
      <div className="toolbar">
        <button
          disabled={page === 1}
          onClick={() => {
            setPage(page - 1);
          }}
        >
          {t("previousPage")}
        </button>
        <span>{page}</span>
        <button
          disabled={!q.data || page >= q.data.total_pages}
          onClick={() => {
            setPage(page + 1);
          }}
        >
          {t("nextPage")}
        </button>
      </div>
    </>
  );
}
function DetailPage({ parts, user, t }: { parts: string[]; user: User; t: T }) {
  const kind = parts[1] ?? "movie",
    id = parts[2] ?? "0",
    key = `${kind === "anime" ? "anidb" : "tmdb"}:${kind}:${id}`,
    client = useQueryClient();
  const language = user.Settings.metadata_language ?? "en";
  const q = useQuery({
    queryKey: ["detail", kind, id, language, user.Settings.anime_title],
    queryFn: () =>
      api<{ metadata: Title; fetched_at: string; stale: boolean; warning?: string }>(
        `/catalog/${kind}/${id}?language=${encodeURIComponent(language)}`,
      ),
    retry: false,
  });
  const lib = useQuery({ queryKey: ["library"], queryFn: () => api<LibraryItem[]>("/library") });
  const statuses = useQuery({ queryKey: ["statuses"], queryFn: () => api<Status[]>("/statuses") });
  const history = useQuery({
    queryKey: ["history", key],
    queryFn: () => api<History[]>(`/library/${key}/history`),
  });
  const entry = lib.data?.find((x) => x.id === key);
  const [season, setSeason] = useState(1),
    [dateMode, setDateMode] = useState("today"),
    [day, setDay] = useState(() => localDay(user.Settings.timezone)),
    [reveal, setReveal] = useState<string[]>([]);
  const eps = useQuery({
    queryKey: ["season", id, season, language],
    queryFn: () =>
      api<{ episodes: Episode[]; stale?: boolean }>(
        `/catalog/tv/${id}/seasons/${String(season)}?language=${language}`,
      ),
    enabled: kind === "tv",
    retry: false,
  });
  const offers = useQuery({
    queryKey: ["offers", kind, id, user.Settings.country],
    queryFn: () =>
      api<{
        offers: {
          link?: string;
          flatrate?: { provider_id: number; provider_name: string }[];
          rent?: { provider_id: number; provider_name: string }[];
          buy?: { provider_id: number; provider_name: string }[];
        } | null;
        fetched_at: string;
        stale: boolean;
      }>(`/catalog/${kind}/${id}/providers?country=${user.Settings.country ?? "TR"}`),
    enabled: kind !== "person" && kind !== "anime",
    retry: false,
  });
  const save = useMutation({
    mutationFn: (data: { base: string; status_id?: string | null; rating?: number | null }) =>
      api(`/library/${key}`, "PUT", {
        position: entry?.position ?? 0,
        anime_view: entry?.anime_view ?? null,
        rating: entry?.rating ?? null,
        status_id: entry?.status_id ?? null,
        ...data,
      }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["library"] }),
  });
  const [prompt, setPrompt] = useState<{
    title: string;
    yes: string;
    no: string;
    resolve: (value: boolean | null) => void;
  } | null>(null);
  const active = useRef(true);
  const pendingChoice = useRef<((value: boolean | null) => void) | null>(null);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      pendingChoice.current?.(null);
      pendingChoice.current = null;
    };
  }, []);
  const ask = (title: string, yes: string, no: string) =>
    new Promise<boolean | null>((resolve) => {
      pendingChoice.current = resolve;
      setPrompt({ title, yes, no, resolve });
    });
  const track = useMutation({
    mutationFn: async (scope: {
      episode_id?: string;
      season?: number;
      whole?: boolean;
      watched: boolean;
    }) => {
      const date =
        dateMode === "unknown"
          ? {}
          : { day: dateMode === "today" ? localDay(user.Settings.timezone) : day };
      const preview = await api<{ earlier: unknown[]; offer_return_watching: boolean }>(
        `/library/${key}/track`,
        "POST",
        { ...scope, date, preview: true },
      );
      if (!active.current) return;
      let earlier = false;
      if (scope.watched && scope.episode_id && preview.earlier.length > 0) {
        const decision = await ask(t("earlierPrompt"), t("earlier"), t("onlyThis"));
        if (decision === null || !active.current) return;
        earlier = decision;
      }
      let returnWatching = false;
      if (preview.offer_return_watching) {
        const decision = await ask(t("returnPrompt"), t("yes"), t("keep"));
        if (decision === null || !active.current) return;
        returnWatching = decision;
      }
      if (!active.current) return;
      return api(`/library/${key}/track`, "POST", {
        ...scope,
        date,
        earlier,
        return_watching: returnWatching,
      });
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["history", key] });
      await client.invalidateQueries({ queryKey: ["library"] });
      await client.invalidateQueries({ queryKey: ["dashboard"] });
      await client.invalidateQueries({ queryKey: ["alternate"] });
    },
  });
  const bulk = useMutation({
    mutationFn: async (watched: boolean) => {
      for (const item of kind === "tv" ? (q.data?.metadata.seasons ?? []) : []) {
        await api(`/catalog/tv/${id}/seasons/${String(item.season_number)}?language=${language}`);
      }
      track.mutate({ whole: true, watched });
    },
  });
  const refresh = useMutation({
    mutationFn: () => api(`/catalog/${kind}/${id}/refresh?language=${language}`, "POST", {}),
    onSuccess: () => client.invalidateQueries({ queryKey: ["detail", kind, id] }),
  });
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <ErrorBox
        error={q.error}
        retry={() => {
          void q.refetch();
        }}
        t={t}
      />
    );
  if (!q.data) return null;
  const m = q.data.metadata;
  return (
    <>
      {prompt && (
        <ChoiceDialog
          title={prompt.title}
          yes={prompt.yes}
          no={prompt.no}
          submit={t("save")}
          cancel={t("cancel")}
          onClose={(value) => {
            pendingChoice.current = null;
            prompt.resolve(value);
            setPrompt(null);
          }}
        />
      )}
      <a className="back" href="#library">
        ← {t("back")}
      </a>
      <section
        className="detail-hero"
        style={
          m.backdrop_path
            ? {
                backgroundImage: `linear-gradient(90deg,#111820 20%,#111820b0),url(${image(m.backdrop_path, "w1280") ?? ""})`,
              }
            : undefined
        }
      >
        <div className="detail-poster">
          <Poster title={m.title ?? m.name ?? ""} path={m.poster_path ?? m.profile_path} />
        </div>
        <div>
          <p className="eyebrow">
            {kind.toUpperCase()} · {(m.release_date ?? m.first_air_date)?.slice(0, 4)}
          </p>
          <h1>{m.title ?? m.name}</h1>
          <div className="tags">
            {m.status && (
              <span>
                {m.status === "Ended"
                  ? t("ended")
                  : m.status === "Returning Series"
                    ? t("ongoing")
                    : m.status}
              </span>
            )}
            {m.runtime !== undefined && m.runtime > 0 && (
              <span>
                {m.runtime} {t("minutes")}
              </span>
            )}

            {m.genres?.map((g) => (
              <span key={g.id}>{g.name}</span>
            ))}
            {m.vote_average !== undefined && (
              <span>
                ★ {m.vote_average.toFixed(1)} · {m.vote_count} {t("minVotes")}
              </span>
            )}
          </div>
          <p className="synopsis">{m.overview ?? m.biography ?? t("noSynopsis")}</p>
          {kind !== "person" && (
            <div className="toolbar">
              {!entry ? (
                <button
                  className="primary"
                  disabled={save.isPending}
                  onClick={() => {
                    save.mutate({ base: "planned" });
                  }}
                >
                  ＋ {t("add")}
                </button>
              ) : (
                <>
                  <select
                    aria-label={t("status")}
                    value={entry.status_id ?? entry.base}
                    onChange={(e) => {
                      const custom = statuses.data?.find((s) => s.id === e.target.value);
                      save.mutate({
                        base: custom?.base ?? e.target.value,
                        status_id: custom?.id ?? null,
                      });
                    }}
                  >
                    {bases.map((b) => (
                      <option key={b} value={b}>
                        {t(b)}
                      </option>
                    ))}
                    {statuses.data?.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                  <select
                    aria-label={t("rating")}
                    value={entry.rating ?? ""}
                    onChange={(e) => {
                      save.mutate({
                        base: entry.base,
                        rating: e.target.value ? Number(e.target.value) : null,
                      });
                    }}
                  >
                    <option value="">{t("noRating")}</option>
                    {Array.from({ length: 10 }, (_, i) => (
                      <option key={i + 1} value={i + 1}>
                        ★ {i + 1}
                      </option>
                    ))}
                  </select>
                </>
              )}
            </div>
          )}
          <small>
            {q.data.stale ? t("stale") : t("freshness")}:{" "}
            {new Date(q.data.fetched_at).toLocaleString(language)}
          </small>{" "}
          <button
            className="text-button"
            onClick={() => {
              refresh.mutate();
            }}
            disabled={refresh.isPending}
          >
            {t("refresh")}
          </button>
        </div>
      </section>
      {q.data.warning && <p className="notice">{q.data.warning}</p>}
      {kind === "anime" && (
        <p className="notice">
          AniDB · <a href={`https://anidb.net/anime/${id}`}>Source</a> ·{" "}
          <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/">CC-BY-NC-SA 4.0</a> ·{" "}
          {t("anidbNotice")}
        </p>
      )}
      {[save.error, track.error, refresh.error]
        .filter((e): e is Error => !!e)
        .map((e, i) => (
          <ErrorBox key={i} error={e} t={t} />
        ))}
      {kind !== "person" && (
        <section>
          <h2>
            {t("providers")} · {user.Settings.country ?? "TR"}
          </h2>
          {offers.error ? (
            <ErrorBox error={offers.error} t={t} />
          ) : offers.data?.offers ? (
            <div className="offers">
              {(["flatrate", "rent", "buy"] as const).map((k) => (
                <div key={k}>
                  <h3>{t(k === "flatrate" ? "subscription" : k)}</h3>
                  {offers.data?.offers?.[k]?.map((p) => (
                    <a
                      className="chip"
                      key={p.provider_id}
                      href={offers.data?.offers?.link}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {p.provider_name} ↗
                    </a>
                  ))}
                </div>
              ))}
              <small>
                JustWatch · {t("freshness")}:{" "}
                {new Date(offers.data.fetched_at).toLocaleDateString(language)}
              </small>
            </div>
          ) : (
            <p className="muted">{t("noOffers")}</p>
          )}
        </section>
      )}
      {entry && (
        <div className="tracking-bar">
          <label>
            {t("watchDate")}{" "}
            <select
              value={dateMode}
              onChange={(e) => {
                setDateMode(e.target.value);
              }}
            >
              <option value="today">{t("today")}</option>
              <option value="chosen">{t("chosen")}</option>
              <option value="unknown">{t("unknown")}</option>
            </select>
          </label>
          {dateMode === "chosen" && (
            <input
              aria-label={t("watchDate")}
              type="date"
              value={day}
              onChange={(e) => {
                setDay(e.target.value);
              }}
            />
          )}
          {kind === "movie" && (
            <button
              data-watch-action="movie"
              disabled={track.isPending}
              onClick={() => {
                track.mutate({ watched: !history.data?.length });
              }}
            >
              {t(history.data?.length ? "unwatched" : "watched")}
            </button>
          )}
        </div>
      )}
      {entry && kind === "tv" && (
        <div className="toolbar">
          <button
            disabled={bulk.isPending || track.isPending}
            onClick={() => {
              bulk.mutate(true);
            }}
          >
            {t("wholeShow")}
          </button>
          <button
            disabled={bulk.isPending || track.isPending}
            onClick={() => {
              bulk.mutate(false);
            }}
          >
            {t("unwatchShow")}
          </button>
          {bulk.error && <p role="alert">{bulk.error.message}</p>}
        </div>
      )}
      {(kind === "tv" || kind === "anime") && (
        <section>
          <div className="section-heading">
            <h2>{t("episodes")}</h2>
            <select
              aria-label={t("seasons")}
              value={season}
              onChange={(e) => {
                setSeason(Number(e.target.value));
              }}
            >
              {m.seasons?.map((s) => (
                <option key={s.season_number} value={s.season_number}>
                  {s.name} ({s.episode_count})
                </option>
              ))}
            </select>
            {entry && (
              <button
                disabled={track.isPending}
                onClick={() => {
                  track.mutate({ season, watched: false });
                }}
              >
                {t("unwatchSeason")}
              </button>
            )}
            {entry && (
              <button
                disabled={track.isPending}
                onClick={() => {
                  track.mutate({ season, watched: true });
                }}
              >
                {t("wholeSeason")}
              </button>
            )}
          </div>
          {eps.error ? (
            <ErrorBox error={eps.error} t={t} />
          ) : (
            (kind === "anime"
              ? m.episodes?.filter((ep) => ep.season_number === season)
              : eps.data?.episodes
            )?.map((ep) => {
              const eid = `${kind === "anime" ? "anidb" : "tmdb"}:episode:${String(ep.id)}`,
                seen = history.data?.some((h) => h.episode_id === eid);
              const show = seen || user.Settings.spoilers || reveal.includes(eid);
              return (
                <article className="episode" key={ep.id}>
                  <span className="episode-number">
                    {String(ep.episode_number).padStart(2, "0")}
                  </span>
                  <div>
                    <h3>{ep.name}</h3>
                    <small>{ep.air_date ?? t("unknown")}</small>
                    {show ? (
                      <p>{ep.overview}</p>
                    ) : (
                      <button
                        className="text-button"
                        onClick={() => {
                          setReveal([...reveal, eid]);
                        }}
                      >
                        {t("reveal")}
                      </button>
                    )}
                  </div>
                  {entry && (
                    <button
                      data-watch-action="episode"
                      className={seen ? "checked" : ""}
                      disabled={track.isPending}
                      onClick={() => {
                        track.mutate({ episode_id: eid, watched: !seen });
                      }}
                    >
                      {seen ? "✓ " : "＋ "}
                      {t(seen ? "unwatched" : "watched")}
                    </button>
                  )}
                </article>
              );
            })
          )}
        </section>
      )}
      {entry && <AlternateView id={key} t={t} />}
      {entry && (
        <details className="panel">
          <summary>
            {t("history")} ({history.data?.length ?? 0})
          </summary>
          {history.error && <p className="error">{history.error.message}</p>}
          {history.data?.map((event, i) => (
            <div className="activity-row" key={event.episode_id ?? String(i)}>
              <strong>{event.episode_id ?? m.title ?? m.name}</strong>
              <small>
                {event.watched_date ??
                  (event.watched_time
                    ? new Date(event.watched_time).toLocaleString(user.Settings.language, {
                        timeZone: user.Settings.timezone,
                      })
                    : t("unknown"))}
              </small>
              <button
                disabled={track.isPending}
                onClick={() => {
                  track.mutate({
                    ...(event.episode_id ? { episode_id: event.episode_id } : {}),
                    watched: false,
                  });
                }}
              >
                {t("unwatched")}
              </button>
            </div>
          ))}
        </details>
      )}
      {m.credits && (
        <section>
          <h2>{t("cast")}</h2>
          <div className="people">
            {m.credits.cast.slice(0, 20).map((c) => (
              <a key={c.id} href={`#detail/person/${String(c.id)}`}>
                <Poster title={c.name} path={c.profile_path} />
                <strong>{c.name}</strong>
                <small>{c.character}</small>
              </a>
            ))}
          </div>
          <h2>{t("crew")}</h2>
          <div className="crew">
            {m.credits.crew.map((c, i) => (
              <a key={`${String(c.id)}:${String(i)}`} href={`#detail/person/${String(c.id)}`}>
                {c.name}
                <small>{c.job}</small>
              </a>
            ))}
          </div>
        </section>
      )}
      {m.videos && (
        <section>
          <h2>{t("trailers")}</h2>
          {m.videos.results
            .filter((v) => v.site === "YouTube")
            .map((v) => (
              <a
                className="chip"
                key={v.key}
                target="_blank"
                rel="noreferrer"
                href={`https://www.youtube.com/watch?v=${encodeURIComponent(v.key)}`}
              >
                ▷ {v.name}
              </a>
            ))}
        </section>
      )}
      {m.recommendations && (
        <section>
          <h2>{t("related")}</h2>
          <Cards results={m.recommendations.results} kind={kind} t={t} />
        </section>
      )}
      {m.combined_credits && <Filmography credits={m.combined_credits} t={t} />}
    </>
  );
}
function DashboardPage({ page, user, t }: { page: string | undefined; user: User; t: T }) {
  const q = useQuery({ queryKey: ["dashboard"], queryFn: () => api<Dashboard>("/dashboard") });
  const client = useQueryClient();
  const [dragged, setDragged] = useState<string | null>(null);
  const order = useMutation({
    mutationFn: (ids: string[]) => api("/up-next/order", "PUT", { ids }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["dashboard"] }),
  });
  function move(id: string, before: string) {
    const ids = (q.data?.up_next ?? []).map((e) => e.catalog_id).filter((x) => x !== id);
    ids.splice(Math.max(0, ids.indexOf(before)), 0, id);
    order.mutate(ids);
  }
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <ErrorBox
        error={q.error}
        retry={() => {
          void q.refetch();
        }}
        t={t}
      />
    );
  if (!q.data) return null;
  const d = q.data;
  if (page === "statistics")
    return (
      <>
        <p className="eyebrow">{t("watchingStory")}</p>
        <h1>{t("statistics")}</h1>
        <div className="metric-grid">
          <div>
            <span>{t("watchedMovies")}</span>
            <strong>{d.statistics.movies}</strong>
          </div>
          <div>
            <span>{t("watchedEpisodes")}</span>
            <strong>{d.statistics.episodes}</strong>
          </div>
        </div>
        <h2>{t("activity")}</h2>
        <p>{t("unknownDates")}</p>
        {d.statistics.monthly.map((m) => (
          <div className="bar-row" key={m.month}>
            <span>{m.month}</span>
            <meter
              min="0"
              max={Math.max(...d.statistics.monthly.map((x) => x.count), 1)}
              value={m.count}
            />
            <strong>{m.count}</strong>
          </div>
        ))}
      </>
    );
  if (page === "calendar")
    return (
      <>
        <h1>{t("calendar")}</h1>
        <div className="calendar-list">
          {d.upcoming.map((e) => (
            <a href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`} key={e.id}>
              <time>
                {e.air_time
                  ? new Date(e.air_time).toLocaleString(user.Settings.language, {
                      timeZone: user.Settings.timezone,
                    })
                  : e.air_date}
              </time>
              <strong>{e.title}</strong>
              <span>
                S{e.season} · E{e.number}
              </span>
            </a>
          ))}
        </div>
        {!d.upcoming.length && <Empty t={t} />}
      </>
    );
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
        <a href="#library">{t("viewAll")} →</a>
      </div>
      {!d.up_next.length ? (
        <Empty t={t} />
      ) : (
        <div className="next-grid">
          {d.up_next.map((e) => (
            <a
              key={e.episode_id}
              className="next-card"
              draggable
              onDragStart={() => {
                setDragged(e.catalog_id);
              }}
              onDragOver={(event) => {
                event.preventDefault();
              }}
              onDrop={(event) => {
                event.preventDefault();
                if (dragged) move(dragged, e.catalog_id);
              }}

              href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`}
            >
              <span className="play">▷</span>

              <p>
                S{e.season} · E{e.number}
              </p>
              <h2>{e.title}</h2>
              <span>{t("details")} →</span>
            </a>
          ))}
        </div>
      )}
      {order.error && (
        <p className="error" role="alert">
          {order.error.message}
        </p>
      )}
      {d.up_next.length > 1 && (
        <div className="toolbar">
          {d.up_next.slice(1).map((item, i) => (
            <button
              key={item.catalog_id}
              disabled={order.isPending}
              onClick={() => {
                const before = d.up_next[i];
                if (before) move(item.catalog_id, before.catalog_id);
              }}
            >
              ↑ {item.title}
            </button>
          ))}
        </div>
      )}
      <div className="dashboard-columns">
        <section>
          <h2>{t("recent")}</h2>
          {d.activity.map((e, i) => (
            <a
              className="activity-row"
              key={i}
              href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`}
            >
              <span className="activity-dot" />
              <strong>{e.title}</strong>
              <small>
                {e.watched_date ??
                  (e.watched_time ? new Date(e.watched_time).toLocaleDateString() : t("unknown"))}
              </small>
            </a>
          ))}
        </section>
        <section>
          <h2>{t("upcoming")}</h2>
          {d.upcoming.slice(0, 8).map((e) => (
            <a
              className="activity-row"
              key={e.id}
              href={`#detail/${e.catalog_id.split(":").slice(1).join("/")}`}
            >
              <strong>{e.title}</strong>
              <small>
                {e.air_date ?? (e.air_time ? new Date(e.air_time).toLocaleString() : "")}
              </small>
            </a>
          ))}
        </section>
      </div>
    </>
  );
}

function ChoiceDialog({
  title,
  yes,
  no,
  submit,
  cancel,
  onClose,
}: {
  title: string;
  yes: string;
  no: string;
  submit: string;
  cancel: string;
  onClose: (value: boolean | null) => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [value, setValue] = useState(false);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      onCancel={(e) => {
        e.preventDefault();
        onClose(null);
      }}
      className="choice-dialog"
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          onClose(value);
        }}
      >
        <h2>{title}</h2>
        <label className="check-label">
          <input
            autoFocus
            type="radio"
            name="choice"
            checked={!value}
            onChange={() => {
              setValue(false);
            }}
          />
          {no}
        </label>
        <label className="check-label">
          <input
            type="radio"
            name="choice"
            checked={value}
            onChange={() => {
              setValue(true);
            }}
          />
          {yes}
        </label>
        <div className="toolbar">
          <button type="submit" className="primary">
            {submit}
          </button>
          <button
            type="button"
            onClick={() => {
              onClose(null);
            }}
          >
            {cancel}
          </button>
        </div>
      </form>
    </dialog>
  );
}

function Filmography({ credits, t }: { credits: { cast: Title[]; crew: Title[] }; t: T }) {
  const [role, setRole] = useState("cast"),
    [kind, setKind] = useState("all"),
    [order, setOrder] = useState("release");
  const results = (role === "cast" ? credits.cast : credits.crew)
    .filter((item) => kind === "all" || item.media_type === kind)
    .sort((a, b) =>
      order === "popularity"
        ? (b.popularity ?? 0) - (a.popularity ?? 0)
        : (b.release_date ?? b.first_air_date ?? "").localeCompare(
            a.release_date ?? a.first_air_date ?? "",
          ),
    );
  return (
    <section>
      <h2>{t("filmography")}</h2>
      <div className="toolbar">
        <select
          aria-label={t("credits")}
          value={role}
          onChange={(e) => {
            setRole(e.target.value);
          }}
        >
          <option value="cast">{t("cast")}</option>
          <option value="crew">{t("crew")}</option>
        </select>
        <select
          aria-label={t("kind")}
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
          }}
        >
          {["all", "movie", "tv"].map((k) => (
            <option key={k} value={k}>
              {t(k as Key)}
            </option>
          ))}
        </select>
        <select
          aria-label={t("sort")}
          value={order}
          onChange={(e) => {
            setOrder(e.target.value);
          }}
        >
          <option value="release">{t("release")}</option>
          <option value="popularity">{t("popularity")}</option>
        </select>
      </div>
      <Cards results={results} t={t} />
    </section>
  );
}
