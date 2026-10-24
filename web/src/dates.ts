export function localDay(timezone?: string) {
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

// Source date-only values are not timestamps and must not shift across zones.
export function displayDate(
  day: string | null | undefined,
  timestamp: string | null | undefined,
  settings: { timezone?: string; language?: string },
  unknown: string,
  includeTime = false,
) {
  if (day) return day;
  if (!timestamp) return unknown;
  const date = new Date(timestamp);
  if (!Number.isFinite(date.getTime())) return unknown;
  return new Intl.DateTimeFormat(settings.language ?? "en", {
    timeZone: settings.timezone ?? "UTC",
    dateStyle: "medium",
    ...(includeTime ? { timeStyle: "short" as const } : {}),
  }).format(date);
}
