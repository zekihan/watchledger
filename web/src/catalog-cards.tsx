import { useState } from "react";
import { imageURL as image } from "./images";
import type { Key } from "./i18n";
import type { Title } from "./api";
import type { T } from "./App";
export function Poster({ title, path }: { title: string; path?: string }) {
  const [failedPath, setFailedPath] = useState<string | undefined>();
  const url = image(path);
  return url && url !== failedPath ? (
    <img
      loading="lazy"
      src={url}
      alt={title}
      onError={() => {
        setFailedPath(url);
      }}
    />
  ) : (
    <div className="poster-placeholder" aria-label={title}>
      W<span>{title}</span>
    </div>
  );
}
export function Cards({ results, kind, t }: { results: Title[]; kind?: string; t: T }) {
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
              {(item.release_date ?? item.first_air_date)?.slice(0, 4)} ·{" "}
              {t(
                (["movie", "tv", "anime", "person"].includes(item.media_type ?? kind ?? "")
                  ? (item.media_type ?? kind)
                  : "unknown") as Key,
              )}
            </small>
          </a>
        ))}
    </div>
  );
}
