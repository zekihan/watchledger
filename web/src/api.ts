export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  return apiBody<T>(path, method, body === undefined ? undefined : JSON.stringify(body), signal);
}

// Preserve signed backup bytes, including JSON number representations.
export async function apiBody<T>(
  path: string,
  method: string,
  body?: string,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    signal,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body,
  });
  if (!response.ok) throw new APIError(response.status, await response.text());
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
export interface Settings {
  up_next_order?: string[];
  language?: string;
  metadata_language?: string;
  country?: string;
  timezone?: string;
  anime_view?: string;
  anime_title?: string;
  spoilers?: boolean;
  services?: number[];
}
export interface User {
  ID: string;
  Name: string;
  Admin: boolean;
  Settings: Settings;
}
export interface Credit {
  source?: string;
  id: number;
  name: string;
  character?: string;
  job?: string;
  profile_path?: string;
}
export interface Title {
  native_tags?: { id: string; name: string }[];
  episodes?: Episode[];
  id: number;
  title?: string;
  name?: string;
  overview?: string;
  biography?: string;
  poster_path?: string;
  backdrop_path?: string;
  profile_path?: string;
  release_date?: string;
  first_air_date?: string;
  runtime?: number;
  popularity?: number;
  vote_average?: number;
  vote_count?: number;
  media_type?: string;
  status?: string;
  genres?: { id: number; name: string }[];
  seasons?: { season_number: number; name: string; episode_count: number }[];
  credits?: { cast: Credit[]; crew: Credit[] };
  combined_credits?: { cast: Title[]; crew: Title[] };
  recommendations?: { results: Title[] };
  videos?: { results: { key: string; site: string; name: string }[] };
}
export interface LibraryItem {
  id: string;
  title: string;
  kind: string;
  metadata: Record<string, Title>;
  base: string;
  status_id: string | null;
  status_name: string | null;
  rating: number | null;
  position: number;
  anime_view: string | null;
  added_at: string;
}
export interface Episode {
  runtime?: number;
  native_number?: string;
  kind?: string;
  id: number;
  name: string;
  overview?: string;
  still_path?: string;
  season_number: number;
  episode_number: number;
  air_date?: string;
}
export interface History {
  episode_id: string | null;
  watched_date: string | null;
  watched_time: string | null;
}
export interface Status {
  id: string;
  name: string;
  base: string;
  used: boolean;
}
export interface Rule {
  op: string;
  children?: Rule[];
  field?: string;
  value?: string;
}
export interface List {
  id: string;
  name: string;
  kind: string;
  rule: Rule | null;
  sort: string;
  members: string[];
  dependents?: string[];
}
export interface Dashboard {
  up_next: {
    view_anchor?: string;
    incomplete?: boolean;
    catalog_id: string;
    title: string;
    episode_id: string;
    season: number;
    number: number;
  }[];
  upcoming: {
    id: string;
    catalog_id: string;
    title: string;
    season: number;
    number: number;
    air_date: string | null;
    air_time: string | null;
  }[];
  activity: {
    view_anchor?: string | null;
    catalog_id: string;
    title: string;
    watched_date: string | null;
    watched_time: string | null;
    created_at: string;
  }[];
  statistics: { movies: number; episodes: number; monthly: { month: string; count: number }[] };
}

export async function downloadExport() {
  const response = await fetch("/api/export", { credentials: "same-origin" });
  if (!response.ok) throw new APIError(response.status, await response.text());
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "watchledger-before-simkl-import.json";
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
