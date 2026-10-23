export function episodeLink(catalogID: string, season: number, episodeID: string): string {
  const [source, kind, id] = catalogID.split(":");
  const [episodeSource, entity, episode] = episodeID.split(":");
  const base = `#detail/${kind ?? "tv"}/${id ?? "0"}`;
  if (
    (source !== "tmdb" && source !== "anidb") ||
    (kind !== "tv" && kind !== "anime") ||
    source !== episodeSource ||
    entity !== "episode" ||
    !/^[1-9]\d*$/.test(episode ?? "") ||
    !Number.isSafeInteger(season) ||
    season < 0
  )
    return base;
  return `${base}/season/${String(season)}/episode/${episode}`;
}
