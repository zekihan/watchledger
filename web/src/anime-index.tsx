import { displayDate } from "./dates";
import type { Settings } from "./api";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";
export function AnimeIndex({ t, settings = {} }: { t: T; settings?: Settings }) {
  const client = useQueryClient();
  const q = useQuery({
    queryKey: ["anime-index"],
    queryFn: () =>
      api<{ entries: number; checksum?: string; imported_at?: string }>("/admin/anidb/titles"),
  });
  const upload = useMutation({
    mutationFn: async (file: File) => {
      if (file.size > 16 * 1024 * 1024) throw new Error(t("animeIndexSize"));
      const response = await fetch("/api/admin/anidb/titles", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/octet-stream" },
        body: file,
      });
      if (!response.ok) throw new Error(await response.text());
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["anime-index"] });
      await client.invalidateQueries({ queryKey: ["catalog"] });
    },
  });
  return (
    <section className="panel">
      <h2>{t("animeIndex")}</h2>
      <p>{t("animeIndexHint")}</p>
      <p>
        <a href="https://anidb.net/api/anime-titles.xml.gz">{t("animeIndexDownload")}</a> ·{" "}
        <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/">CC-BY-NC-SA 4.0</a>
      </p>
      <label>
        {t("animeIndexFile")}
        <input
          type="file"
          accept=".gz,.xml"
          disabled={upload.isPending}
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) upload.mutate(file);
            e.target.value = "";
          }}
        />
      </label>
      {q.data && (
        <p>
          {t("title")}: {q.data.entries}{" "}
          {q.data.imported_at && (
            <>
              {" "}
              · {t("indexImported")}:{" "}
              {displayDate(null, q.data.imported_at, settings, t("unknown"), true)}
            </>
          )}
        </p>
      )}
      {[q.error, upload.error].filter(Boolean).map((e, i) => (
        <p role="alert" key={i}>
          {e?.message}
        </p>
      ))}
      {upload.isPending && <p role="status">{t("loading")}</p>}
      {upload.isSuccess && <p role="status">{t("updated")}</p>}
    </section>
  );
}
