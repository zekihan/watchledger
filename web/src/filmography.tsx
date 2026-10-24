import { useState } from "react";
import { Cards } from "./catalog-cards";
import type { Title } from "./api";
import type { T } from "./App";
import type { Key } from "./i18n";
export function Filmography({ credits, t }: { credits: { cast: Title[]; crew: Title[] }; t: T }) {
  const [role, setRole] = useState("cast"),
    [kind, setKind] = useState("all"),
    [order, setOrder] = useState("release");
  const [page, setPage] = useState(0);
  const results = (role === "cast" ? credits.cast : credits.crew)
    .filter((item) => kind === "all" || item.media_type === kind)
    .filter(
      (item, index, all) =>
        all.findIndex((other) => other.id === item.id && other.media_type === item.media_type) ===
        index,
    )
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
            setPage(0);
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
          <option value="release">{t("release")}</option>
          <option value="popularity">{t("popularity")}</option>
        </select>
      </div>
      <p role="status">
        {results.length} {t("title")}
      </p>
      {!results.length && <p>{t("empty")}</p>}
      <Cards results={results.slice(page * 24, (page + 1) * 24)} t={t} />
      {results.length > 24 && (
        <div className="toolbar">
          <button disabled={page === 0} onClick={() => setPage(page - 1)}>
            {t("previousPage")}
          </button>
          <span>
            {page + 1} / {Math.ceil(results.length / 24)}
          </span>
          <button disabled={(page + 1) * 24 >= results.length} onClick={() => setPage(page + 1)}>
            {t("nextPage")}
          </button>
        </div>
      )}
    </section>
  );
}
