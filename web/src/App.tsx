import { PageControls } from "./page-controls";
import { Filmography } from "./filmography";
import { Cards, Poster } from "./catalog-cards";
import { DashboardPage } from "./dashboard";
import { EpisodeSummary } from "./episode-details";
import { episodeLink } from "./episode-links";
import { useInstall } from "./use-install";
import { CreditsPage } from "./credits";
import { ChoiceDialog } from "./tracking-ui";
import { useTrackingDialogFocus } from "./use-tracking-dialog-focus";
import { displayDate, localDay } from "./dates";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { imageURL as image } from "./images";
import { api, APIError } from "./api";
import type { Episode, History, LibraryItem, Status, Title, User } from "./api";
import { en, tr } from "./i18n";
import type { Key } from "./i18n";
import { ListsPage, SettingsPage } from "./pages";
import { AnalyticsPage, CalendarPage } from "./analytics";
import { AlternateView } from "./alternate";
import { MappingPage, Notices } from "./admin";
export type T = (key: Key) => string;
const bases = ["planned", "watching", "completed", "hold", "dropped"] as const;
const nav = [
  "home",
  "library",
  "discover",
  "lists",
  "calendar",
  "statistics",
  "settings",
  "credits",
] as const;
const icons = ["◈", "▤", "⌕", "☷", "▦", "▥", "⚙", "ⓘ"];
function subscribe(callback: () => void) {
  window.addEventListener("hashchange", callback);
  return () => {
    window.removeEventListener("hashchange", callback);
  };
}
function route() {
  return location.hash.slice(1) || "home";
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
function decodeRoute(value: string): string | null {
  try {
    return decodeURIComponent(value);
  } catch {
    return null;
  }
}
export function App() {
  const installation = useInstall();
  const client = useQueryClient();
  const page = useSyncExternalStore(subscribe, route);
  const [fallbackLang, setFallbackLang] = useState(() =>
    navigator.language.startsWith("tr") ? "tr" : "en",
  );
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<User>("/me"), retry: false });
  const identity = me.error ? null : (me.data?.ID ?? null);
  const [cacheIdentity, setCacheIdentity] = useState<string | null>(null);
  useEffect(() => {
    if (identity === cacheIdentity) return;
    let current = true;
    // Cancel before removal so an old in-flight response cannot repopulate
    // another account's cache. Keep /me, which establishes the boundary.
    void client.cancelQueries({ predicate: (query) => query.queryKey[0] !== "me" }).then(() => {
      if (!current) return;
      client.removeQueries({ predicate: (query) => query.queryKey[0] !== "me" });
      client.getMutationCache().clear();
      setCacheIdentity(identity);
    });
    return () => {
      current = false;
    };
  }, [identity, cacheIdentity, client]);
  const language = (me.data?.Settings.language ?? fallbackLang) === "tr" ? "tr" : "en";
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
        if (cards.length && document.activeElement?.classList.contains("poster-card")) {
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
  if (cacheIdentity !== identity) return <div className="center">{t("loading")}</div>;
  const user = { ...me.data, Settings: { ...me.data.Settings, language } };
  const root = page.split("/")[0];
  return (
    <div className="app-shell" key={user.ID}>
      <a
        className="skip-link"
        href="#content"
        onClick={(event) => {
          event.preventDefault();
          document.getElementById("content")?.focus();
        }}
      >
        {t("skipContent")}
      </a>
      <aside>
        <a className="wordmark" href="#home">
          <span>W</span> watchledger
        </a>
        <p className="nav-label">{t("collection")}</p>
        <nav aria-label={t("navigation")}>
          {nav.map((item, i) => (
            <a
              key={item}
              aria-current={root === item ? "page" : undefined}
              className={root === item ? "active" : ""}
              href={`#${item}`}
            >
              <span aria-hidden="true">{icons[i]}</span>
              {t(item === "credits" ? "sourceCredits" : item)}
            </a>
          ))}
          {user.Admin && (
            <a href="#admin" aria-current={root === "admin" ? "page" : undefined}>
              ◇ {t("admin")}
            </a>
          )}
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
        <main id="content" tabIndex={-1} className="content">
          <Notices t={t} />
          {(root === "search" && decodeRoute(page.slice(7)) === null) ||
          (root === "views" && decodeRoute(page.slice(6)) === null) ? (
            <p role="alert">{t("invalidLink")}</p>
          ) : root === "creator" ? (
            <NativeCreatorPage id={page.split("/")[1] ?? ""} t={t} user={user} />
          ) : root === "detail" ? (
            <DetailPage key={page} parts={page.split("/")} user={user} t={t} />
          ) : root === "views" ? (
            <AlternateView
              key={page}
              id={decodeRoute(page.slice(6)) ?? ""}
              t={t}
              timezone={user.Settings.timezone}
            />
          ) : root === "library" ? (
            <LibraryPage t={t} user={user} />
          ) : root === "search" || root === "discover" ? (
            <DiscoveryPage
              key={page}
              search={root === "search" ? (decodeRoute(page.slice(7)) ?? "") : undefined}
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
            <SettingsPage user={user} t={t} installation={installation} />
          ) : root === "credits" ? (
            <CreditsPage t={t} />
          ) : root === "admin" || root === "mappings" ? (
            <MappingPage t={t} admin={user.Admin} settings={user.Settings} />
          ) : (
            <DashboardPage user={user} t={t} />
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
            {" · "}
            <a href="#credits">{t("sourceCredits")}</a>
            <p>{t("incomplete")}</p>
          </footer>
        </main>
      </div>
    </div>
  );
}
function LibraryPage({ t, user }: { t: T; user: User }) {
  const [page, setPage] = useState(0);
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
      (filter === "all" || x.base === filter || `custom:${x.status_id ?? ""}` === filter) &&
      x.title
        .toLocaleLowerCase(user.Settings.language)
        .includes(term.trim().toLocaleLowerCase(user.Settings.language)),
  );
  entries.sort((a, b) => {
    if (order === "rating") return (b.rating ?? 0) - (a.rating ?? 0);
    if (order === "added") return b.added_at.localeCompare(a.added_at);
    return a.title.localeCompare(b.title, user.Settings.language);
  });
  const currentPage = Math.min(page, Math.max(0, Math.ceil(entries.length / 48) - 1));
  const visible = entries.slice(currentPage * 48, (currentPage + 1) * 48);
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
            setPage(0);
          }}
        />
        <select
          aria-label={t("status")}
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
            setPage(0);
          }}
        >
          <option value="all">{t("all")}</option>
          {bases.map((b) => (
            <option key={b} value={b}>
              {t(b)}
            </option>
          ))}
          {[
            ...new Map(
              (q.data ?? [])
                .filter((item) => item.status_id && item.status_name)
                .map((item) => [item.status_id, item.status_name]),
            ).entries(),
          ].map(([id, name]) => (
            <option key={id} value={`custom:${id ?? ""}`}>
              {name}
            </option>
          ))}
        </select>
        <select
          aria-label={t("kind")}
          value={contentType}
          onChange={(e) => {
            setContentType(e.target.value);
            setPage(0);
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
            setPage(0);
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
              {visible.map((x) => (
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
          {visible.map((x) => {
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
      <PageControls
        page={currentPage}
        count={entries.length}
        size={48}
        label={t("library")}
        change={setPage}
        t={t}
      />
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
    queryKey: ["watched-titles"],
    queryFn: () => api<string[]>("/library/watched"),
    enabled: exclude,
    retry: false,
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
  } else if (kind !== "anime") {
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
        fetched_at?: string;
        stale?: boolean;
        enrichment_failures?: number;
      }>(path),
    retry: false,
  });
  const visibleResults = (q.data?.results ?? []).filter(
    (item) =>
      !exclude ||
      !library.data?.includes(
        `${(item.media_type ?? kind) === "anime" ? "anidb" : "tmdb"}:${item.media_type ?? kind}:${String(item.id)}`,
      ),
  );
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
            setAdvanced({});
            setGenre("");
            setPage(1);
          }}
        >
          {(search ? ["multi", "movie", "tv", "anime", "person"] : ["movie", "tv", "anime"]).map(
            (k) => (
              <option key={k} value={k}>
                {k === "multi" ? t("all") : t(k as "movie" | "tv" | "anime" | "person")}
              </option>
            ),
          )}
        </select>
        {!search && kind !== "anime" && (
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
      {!search && kind !== "anime" && (
        <details className="panel">
          <summary>{t("filters")}</summary>
          <p>{t("filterIDsHint")}</p>
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
            {kind === "movie" &&
              (
                [
                  ["with_cast", "castIDs"],
                  ["with_crew", "crewIDs"],
                ] as const
              ).map(([key, label]) => (
                <label key={key}>
                  {t(label)}
                  <input
                    value={advanced[key] ?? ""}
                    onChange={(event) => {
                      setAdvanced({ ...advanced, [key]: event.target.value });
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
              disabled={!user.Settings.services?.length}
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
            {!user.Settings.services?.length && <small>{t("noSubscriptions")}</small>}
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
      {exclude && <p className="muted">{t("excludeWatchedHint")}</p>}
      {q.data?.exhaustive === false && (
        <p className="notice">
          {t(
            kind === "anime"
              ? q.data.search_scope === "native_hot"
                ? "nativeHotScope"
                : q.data.search_scope === "imported_title_index"
                  ? "animeIndexScope"
                  : "animeSearchScope"
              : "candidateNotice",
          )}{" "}
          {q.data.enrichment_failures ? `${t("error")}: ${String(q.data.enrichment_failures)}` : ""}
        </p>
      )}
      {kind === "anime" && q.data?.fetched_at && (
        <p>
          {q.data.stale ? t("stale") : t("freshness")}:{" "}
          {displayDate(null, q.data.fetched_at, user.Settings, t("unknown"), true)}
        </p>
      )}
      {q.isPending || (exclude && library.isPending) ? (
        <p>{t("loading")}</p>
      ) : q.error || (exclude && library.error) ? (
        <ErrorBox
          error={(q.error ?? library.error) as Error}
          retry={() => {
            void q.refetch();
            if (exclude) void library.refetch();
          }}
          t={t}
        />
      ) : q.data ? (
        <>
          <p role="status">
            {t("pageMatches")}: {visibleResults.length}
          </p>
          {visibleResults.length ? (
            <Cards results={visibleResults} kind={kind} t={t} />
          ) : (
            <p className="empty">{t("noPageMatches")}</p>
          )}
        </>
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
  const effective = useQuery({
    queryKey: ["effective", key],
    queryFn: () =>
      api<
        Record<
          string,
          { direct: boolean; derived: boolean; explicit_unwatch?: boolean; sources: string[] }
        >
      >(`/library/${key}/effective`),
    enabled: !!entry,
  });

  const requestedSeason =
    parts[3] === "season" && /^\d+$/.test(parts[4] ?? "") ? Number(parts[4]) : 1;
  const selectedEpisode =
    parts[5] === "episode" && /^[1-9]\d*$/.test(parts[6] ?? "") ? parts[6] : undefined;
  const season = Number.isSafeInteger(requestedSeason) ? requestedSeason : 1;
  const [castLimit, setCastLimit] = useState(20);
  const [dateMode, setDateMode] = useState("today"),
    [day, setDay] = useState(() => localDay(user.Settings.timezone)),
    [reveal, setReveal] = useState<string[]>([]);
  const eps = useQuery({
    queryKey: ["season", id, season, language],
    queryFn: () =>
      api<{
        episodes: Episode[];
        stale?: boolean;
        name?: string;
        overview?: string;
        poster_path?: string;
        air_date?: string;
      }>(`/catalog/tv/${id}/seasons/${String(season)}?language=${language}`),
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
          free?: { provider_id: number; provider_name: string }[];
          ads?: { provider_id: number; provider_name: string }[];
        } | null;
        fetched_at: string;
        stale: boolean;
      }>(`/catalog/${kind}/${id}/providers?country=${user.Settings.country ?? "TR"}`),
    enabled: kind !== "person" && kind !== "anime",
    retry: false,
  });
  const save = useMutation({
    mutationFn: (data: { base?: string; status_id?: string | null; rating?: number | null }) =>
      entry ? api(`/library/${key}`, "PATCH", data) : api(`/library/${key}`, "PUT", data),
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
      if (scope.watched && dateMode === "chosen" && !day) throw new Error(t("chooseWatchDate"));
      const date =
        dateMode === "unknown"
          ? {}
          : { day: dateMode === "today" ? localDay(user.Settings.timezone) : day };
      const preview = await api<{
        earlier: unknown[];
        offer_return_watching: boolean;
        linked_changes?: { episode_id: string; catalog_id: string; title: string }[] | null;
        linked_plan?: string;
      }>(`/library/${key}/track`, "POST", { ...scope, date, preview: true });
      if (!active.current) return;
      let earlier = false;
      if (scope.watched && scope.episode_id && preview.earlier.length > 0) {
        const decision = await ask(t("earlierPrompt"), t("earlier"), t("onlyThis"));
        if (decision === null || !active.current) return;
        earlier = decision;
      }
      let includeLinked = false;
      if (!scope.watched && preview.linked_changes?.length) {
        const names = preview.linked_changes
          .slice(0, 12)
          .map((x) => x.title + " · " + x.episode_id)
          .join("\n");
        const decision = await ask(
          t("linkedUnwatchPrompt") +
            "\n" +
            names +
            (preview.linked_changes.length > 12 ? "\n…" : ""),
          t("unwatchLinked"),
          t("cancel"),
        );
        if (!decision || !active.current) return;
        includeLinked = true;
      }
      let returnWatching = false;
      if (preview.offer_return_watching) {
        const decision = await ask(
          t(preview.linked_changes?.length ? "linkedReturnPrompt" : "returnPrompt"),
          t("yes"),
          t("keep"),
        );
        if (decision === null || !active.current) return;
        returnWatching = decision;
      }
      if (!active.current) return;
      return api(`/library/${key}/track`, "POST", {
        ...scope,
        date,
        earlier,
        return_watching: returnWatching,
        include_linked: includeLinked,
        linked_plan: includeLinked ? preview.linked_plan : undefined,
      });
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["history", key] });
      await client.invalidateQueries({ queryKey: ["watched-titles"] });
      await client.invalidateQueries({ queryKey: ["library"] });
      await client.invalidateQueries({ queryKey: ["dashboard"] });
      await client.invalidateQueries({ queryKey: ["alternate"] });
      await client.invalidateQueries({ queryKey: ["effective"] });
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
  const trackingFocus = useTrackingDialogFocus(track.isPending || bulk.isPending);
  const refresh = useMutation({
    mutationFn: () =>
      api<NonNullable<typeof q.data>>(
        `/catalog/${kind}/${id}/refresh?language=${language}`,
        "POST",
        {},
      ),
    onSuccess: (data) => {
      client.setQueryData(["detail", kind, id, language, user.Settings.anime_title], data);
    },
  });
  useEffect(() => {
    if (selectedEpisode && q.data && (kind === "anime" || eps.data))
      document.getElementById("episode-details")?.focus();
  }, [selectedEpisode, q.data, eps.data, kind]);
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
  const seasonEpisodes =
    kind === "anime" ? m.episodes?.filter((ep) => ep.season_number === season) : eps.data?.episodes;
  const visibleEpisodes = selectedEpisode
    ? seasonEpisodes?.filter((ep) => String(ep.id) === selectedEpisode)
    : seasonEpisodes;
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
            trackingFocus.requestRestore();
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
            {t(
              (["movie", "tv", "anime", "person"].includes(kind) ? kind : "unknown") as Key,
            ).toLocaleUpperCase(user.Settings.language)}{" "}
            · {(m.release_date ?? m.first_air_date)?.slice(0, 4)}
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
                ★ {m.vote_average.toFixed(1)} · {m.vote_count} {t("votes")}
              </span>
            )}
          </div>
          <p className="synopsis">{m.overview ?? m.biography ?? t("noSynopsis")}</p>
          {!!m.native_tags?.length && (
            <details>
              <summary>{t("nativeTags")}</summary>
              <p>{t("nativeTagsHint")}</p>
              <div className="tags">
                {m.native_tags.map((tag) => (
                  <span key={tag.id}>{tag.name}</span>
                ))}
              </div>
            </details>
          )}
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
                    disabled={save.isPending}
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
                    disabled={save.isPending}
                    value={entry.rating ?? ""}
                    onChange={(e) => {
                      save.mutate({
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
            {displayDate(null, q.data.fetched_at, user.Settings, t("unknown"), true)}
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
          AniDB · <a href={`https://anidb.net/anime/${id}`}>{t("sourceDetails")}</a> ·{" "}
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
            <ErrorBox
              error={offers.error}
              retry={() => {
                void offers.refetch();
              }}
              t={t}
            />
          ) : offers.data?.offers ? (
            <div className="offers">
              {(["flatrate", "rent", "buy", "free", "ads"] as const).map((k) => (
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
                JustWatch · {offers.data.stale ? t("stale") : t("freshness")}:{" "}
                {displayDate(null, offers.data.fetched_at, user.Settings, t("unknown"))}
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
              disabled={track.isPending}
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
              disabled={track.isPending}
              required
              value={day}
              onChange={(e) => {
                setDay(e.target.value);
              }}
            />
          )}
          {kind === "movie" && (
            <button
              data-watch-action="movie"
              disabled={track.isPending || effective.isPending || !!effective.error}
              onClick={() => {
                trackingFocus.capture();
                track.mutate({
                  watched: !(history.data?.length || effective.data?.[key]?.derived),
                });
              }}
            >
              {t(history.data?.length || effective.data?.[key]?.derived ? "unwatched" : "watched")}
            </button>
          )}
        </div>
      )}
      {entry && kind === "tv" && (
        <div className="toolbar">
          <button
            disabled={bulk.isPending || track.isPending}
            onClick={() => {
              trackingFocus.capture();
              bulk.mutate(true);
            }}
          >
            {t("wholeShow")}
          </button>
          <button
            disabled={bulk.isPending || track.isPending}
            onClick={() => {
              trackingFocus.capture();
              bulk.mutate(false);
            }}
          >
            {t("unwatchShow")}
          </button>
          {bulk.error && <p role="alert">{bulk.error.message}</p>}
        </div>
      )}
      {(kind === "tv" || kind === "anime") && (
        <section id="episode-details" tabIndex={-1}>
          <div className="section-heading">
            <h2>{t(selectedEpisode ? "episodeDetails" : "seasonDetails")}</h2>
            <select
              aria-label={t("seasons")}
              value={season}
              onChange={(e) => {
                location.hash = `detail/${kind}/${id}/season/${e.target.value}`;
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
                  trackingFocus.capture();
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
                  trackingFocus.capture();
                  track.mutate({ season, watched: true });
                }}
              >
                {t("wholeSeason")}
              </button>
            )}
          </div>
          {selectedEpisode && (
            <a className="button" href={`#detail/${kind}/${id}/season/${String(season)}`}>
              {t("allEpisodes")}
            </a>
          )}
          {kind === "tv" && !selectedEpisode && eps.data && (
            <div className="season-summary">
              {eps.data.poster_path && (
                <Poster title={eps.data.name ?? t("seasonScope")} path={eps.data.poster_path} />
              )}
              <div>
                <h3>{eps.data.name ?? `${t("seasonScope")} ${String(season)}`}</h3>
                <time dateTime={eps.data.air_date}>{eps.data.air_date ?? t("unknown")}</time>
                <p>{eps.data.overview ?? t("noSynopsis")}</p>
              </div>
            </div>
          )}
          {kind === "tv" && eps.isPending && <p>{t("loading")}</p>}
          {eps.data?.stale && <p role="status">{t("stale")}</p>}
          {selectedEpisode && visibleEpisodes?.length === 0 && (
            <p role="status">{t("missingEpisode")}</p>
          )}
          {eps.error ? (
            <ErrorBox
              error={eps.error}
              t={t}
              retry={() => {
                void eps.refetch();
              }}
            />
          ) : (
            visibleEpisodes?.map((ep) => {
              const eid = `${kind === "anime" ? "anidb" : "tmdb"}:episode:${String(ep.id)}`,
                seen = !!(
                  effective.data?.[eid]?.direct ||
                  effective.data?.[eid]?.derived ||
                  history.data?.some((h) => h.episode_id === eid)
                );
              const show = seen || user.Settings.spoilers || reveal.includes(eid);
              return (
                <article
                  className="episode"
                  key={ep.id}
                  aria-labelledby={`episode-heading-${kind}-${String(ep.id)}`}
                >
                  <span className="episode-number">
                    {ep.native_number ?? String(ep.episode_number).padStart(2, "0")}
                  </span>
                  <div>
                    {selectedEpisode ? (
                      <h3 id={`episode-heading-${kind}-${String(ep.id)}`}>{ep.name}</h3>
                    ) : (
                      <h3 id={`episode-heading-${kind}-${String(ep.id)}`}>
                        <a href={episodeLink(key, season, eid)}>{ep.name}</a>
                      </h3>
                    )}
                    {effective.data?.[eid]?.derived && <small>{t("derivedWatch")}</small>}
                    <EpisodeSummary
                      key={eid}
                      episode={ep}
                      identity={eid}
                      show={!!show}
                      detail={!!selectedEpisode}
                      t={t}
                      reveal={() => {
                        setReveal([...reveal, eid]);
                      }}
                    />
                  </div>
                  {entry && (
                    <button
                      data-watch-action="episode"
                      aria-describedby={`episode-heading-${kind}-${String(ep.id)}`}
                      className={seen ? "checked" : ""}
                      disabled={track.isPending || effective.isPending || !!effective.error}
                      onClick={() => {
                        trackingFocus.capture();
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
      {effective.error && (
        <ErrorBox
          error={effective.error}
          retry={() => {
            void effective.refetch();
          }}
          t={t}
        />
      )}
      {entry && <AlternateView id={key} t={t} timezone={user.Settings.timezone} />}
      {entry && (
        <details className="panel">
          <summary>
            {t("sourceHistory")} ({history.data?.length ?? 0})
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
                  trackingFocus.capture();
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
            {m.credits.cast.slice(0, castLimit).map((c, index) => (
              <a
                key={`${c.source ?? "tmdb"}:${String(c.id)}:${String(index)}`}
                href={
                  c.source === "anidb"
                    ? `#creator/${String(c.id)}`
                    : `#detail/person/${String(c.id)}`
                }
              >
                <Poster title={c.name} path={c.profile_path} />
                <strong>{c.name}</strong>
                <small>{c.character}</small>
              </a>
            ))}
          </div>
          {m.credits.cast.length > 20 && (
            <div className="toolbar">
              {castLimit < m.credits.cast.length && (
                <button onClick={() => setCastLimit(castLimit + 20)}>{t("showMore")}</button>
              )}
              {castLimit > 20 && <button onClick={() => setCastLimit(20)}>{t("showLess")}</button>}
              <span role="status">
                {Math.min(castLimit, m.credits.cast.length)} / {m.credits.cast.length}
              </span>
            </div>
          )}
          <h2>{t("crew")}</h2>
          <div className="crew">
            {m.credits.crew.map((c, i) => (
              <a
                key={`${String(c.id)}:${String(i)}`}
                href={
                  c.source === "anidb"
                    ? `#creator/${String(c.id)}`
                    : `#detail/person/${String(c.id)}`
                }
              >
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
function NativeCreatorPage({ id, t, user }: { id: string; t: T; user: User }) {
  const q = useQuery({
    queryKey: ["native-creator", id, user.Settings.metadata_language, user.Settings.anime_title],
    queryFn: () =>
      api<Title & { source_url: string; truncated: boolean }>(
        `/anidb/creators/${encodeURIComponent(id)}`,
      ),
    retry: false,
  });
  if (q.isPending) return <p>{t("loading")}</p>;
  if (q.error)
    return (
      <ErrorBox
        error={q.error}
        t={t}
        retry={() => {
          void q.refetch();
        }}
      />
    );
  if (!q.data) return null;
  return (
    <>
      <a className="back" href="#library">
        ← {t("back")}
      </a>
      <section className="detail-hero">
        <div className="detail-poster">
          <Poster title={q.data.name ?? ""} path={q.data.profile_path} />
        </div>
        <div>
          <p className="eyebrow">AniDB · {t("nativeCreator")}</p>
          <h1>{q.data.name}</h1>
          <p>{t("nativeCreatorScope")}</p>
          <a
            href={`https://anidb.net/cr${encodeURIComponent(id)}`}
            target="_blank"
            rel="noreferrer"
          >
            {t("sourceDetails")} ↗
          </a>
        </div>
      </section>
      {q.data.truncated && <p role="status">{t("nativeCreatorLimit")}</p>}
      {q.data.combined_credits && <Filmography credits={q.data.combined_credits} t={t} />}
    </>
  );
}
