import { useState } from "react";
import type { T } from "./App";
import type { Episode } from "./api";
import { imageURL } from "./images";
export function EpisodeSummary({
  episode,
  identity,
  show,
  detail,
  t,
  reveal,
}: {
  episode: Episode;
  identity: string;
  show: boolean;
  detail: boolean;
  t: T;
  reveal: () => void;
}) {
  const [failed, setFailed] = useState<string | null>(null);
  const image = imageURL(episode.still_path, "w780");
  return (
    <>
      <time dateTime={episode.air_date}>{episode.air_date || t("unknown")}</time>
      {episode.runtime !== undefined && episode.runtime > 0 && (
        <span>
          {" "}
          · {episode.runtime} {t("minutes")}
        </span>
      )}
      {detail && (
        <p className="muted">
          {t("sourceIdentity")}: {identity}
          {episode.native_number ? ` · ${episode.native_number}` : ""}
        </p>
      )}
      {show ? (
        <>
          {detail && image && failed !== image && (
            <img
              className="episode-still"
              src={image}
              alt={episode.name}
              loading="lazy"
              onError={() => {
                setFailed(image);
              }}
            />
          )}
          {detail && image && failed === image && <p>{t("noPoster")}</p>}
          <p>{episode.overview || t("noSynopsis")}</p>
        </>
      ) : (
        <button className="text-button" onClick={reveal}>
          {t("reveal")}
        </button>
      )}
    </>
  );
}
