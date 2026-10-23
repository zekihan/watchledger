export function imageURL(path: string | undefined, size = "w342"): string | undefined {
  if (!path) return undefined;
  if (/^https:\/\/cdn-eu\.anidb\.net\/images\/main\/[1-9][0-9]*\.(jpg|png|webp)$/.test(path)) {
    return path;
  }
  if (!/^\/[A-Za-z0-9_-]+\.(jpg|jpeg|png|webp)$/.test(path)) return undefined;
  return `https://image.tmdb.org/t/p/${size}${path}`;
}
