import { QueryFeedback } from "./query-feedback";
import { displayDate } from "./dates";
import type { Settings } from "./api";
import { MappingDatasets } from "./mapping-datasets";
import { AnimeIndex } from "./anime-index";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";
export interface Mapping {
  kind?: "episodes" | "movie-aliases";
  episodes?: Record<
    string,
    {
      catalog_id: string;
      title: string;
      name: string;
      season: number;
      number: number;
      kind: string;
      retired: boolean;
    }
  >;
  proposal_id?: string;
  id: string;
  revision: string;
  relationships: { from: string; to: string }[];
  provenance: {
    source: string;
    version: string;
    note?: string;
    attribution?: { notice: string };
    inferred?: boolean;
  };
  requires_consent: boolean;
  confirmed: boolean;
  reviewed: boolean;
}
export function MappingPage({
  t,
  admin,
  settings = {},
}: {
  t: T;
  admin: boolean;
  settings?: Settings;
}) {
  const client = useQueryClient();
  const q = useQuery({ queryKey: ["mappings"], queryFn: () => api<Mapping[]>("/mappings") });
  const [historyID, setHistoryID] = useState("");
  const history = useQuery({
    queryKey: ["mapping-history", historyID],
    queryFn: () =>
      api<
        {
          revision: string;
          recorded_at: string;
          provenance: Mapping["provenance"];
          relationships: Mapping["relationships"];
        }[]
      >(`/admin/mappings/${historyID}/history`),
    enabled: admin && !!historyID,
  });
  const [selected, setSelected] = useState<Mapping | null>(null);
  const [from, setFrom] = useState(""),
    [to, setTo] = useState(""),
    [note, setNote] = useState("");
  const confirm = useMutation({
    mutationFn: (m: Mapping) => api(`/mappings/${m.id}/confirm`, "POST", { revision: m.revision }),
    onSuccess: async () => {
      for (const key of [
        "mappings",
        "alternate",
        "effective",
        "dashboard",
        "statistics",
        "watched-titles",
      ]) {
        await client.invalidateQueries({ queryKey: [key] });
      }
    },
  });
  const save = useMutation({
    mutationFn: (m: Mapping) =>
      api("/admin/mappings", "POST", {
        id: m.id,
        kind: m.kind ?? "episodes",
        expected_revision: m.revision,
        proposal_id: m.proposal_id,
        relationships: m.relationships,
        provenance: {
          ...(m.proposal_id
            ? m.provenance
            : {
                source: "local-admin-override",
                version: new Date().toISOString(),
                note,
                inferred: false,
              }),
        },
      }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["mappings"] });
      setSelected(null);
    },
  });
  const accounts = useQuery({
    queryKey: ["accounts"],
    queryFn: () =>
      api<{ id: string; name: string; admin: boolean; created_at: string }[]>("/admin/accounts"),
    enabled: admin,
  });
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: () =>
      api<{ id: string; attempts: number; last_error: string; run_at: string }[]>("/admin/jobs"),
    enabled: admin,
  });
  return (
    <>
      <h1>{t("mapping")}</h1>
      <p className="notice">{t("mappingGated")}</p>
      <QueryFeedback query={q} t={t} />
      {[save.error, confirm.error].filter(Boolean).map((e, i) => (
        <p className="error" role="alert" key={i}>
          {e?.message}
        </p>
      ))}
      {admin && (
        <MappingDatasets
          t={t}
          onReview={(m) => {
            setSelected(m);
            setNote(m.provenance.note ?? "");
          }}
        />
      )}
      {admin && (
        <button
          onClick={() => {
            setSelected({
              id: "",
              revision: "",
              relationships: [],
              provenance: { source: "local-admin-override", version: "" },
              requires_consent: false,
              confirmed: false,
              reviewed: true,
            });
          }}
        >
          {t("newMapping")}
        </button>
      )}
      {selected && admin && (
        <section className="panel">
          <h2>{t("reviewMapping")}</h2>
          <label>
            {t("mappingKind")}
            <select
              disabled={!!selected.proposal_id}
              value={selected.kind ?? "episodes"}
              onChange={(e) =>
                setSelected({
                  ...selected,
                  kind: e.target.value === "movie-aliases" ? "movie-aliases" : "episodes",
                })
              }
            >
              <option value="episodes">{t("episodes")}</option>
              <option value="movie-aliases">{t("movieAliases")}</option>
            </select>
          </label>
          <p>{t(selected.kind === "movie-aliases" ? "movieAliasSafety" : "mappingSafety")}</p>
          <div className="toolbar">
            <input
              aria-label={t("sourceEpisode")}
              placeholder={t("sourceEpisode")}
              value={from}
              onChange={(e) => {
                setFrom(e.target.value);
              }}
            />
            <span>→</span>
            <input
              aria-label={t("targetEpisode")}
              placeholder={t("targetEpisode")}
              value={to}
              onChange={(e) => {
                setTo(e.target.value);
              }}
            />
            <button
              disabled={!from || !to || !!selected.proposal_id}
              onClick={() => {
                setSelected({
                  ...selected,
                  relationships: [...selected.relationships, { from, to }],
                });
                setFrom("");
                setTo("");
              }}
            >
              ＋
            </button>
          </div>
          {selected.relationships.map((edge, i) => (
            <div className="activity-row" key={i}>
              <code>
                {edge.from} → {edge.to}
              </code>
              <button
                disabled={!!selected.proposal_id}
                onClick={() => {
                  setSelected({
                    ...selected,
                    relationships: selected.relationships.filter((_, j) => i !== j),
                  });
                }}
              >
                {t("remove")}
              </button>
            </div>
          ))}
          <label>
            {t("reviewNote")}
            <input
              value={note}
              onChange={(e) => {
                setNote(e.target.value);
              }}
            />
          </label>
          <div className="toolbar">
            <button
              disabled={!selected.relationships.length || save.isPending}
              className="primary"
              onClick={() => {
                save.mutate(selected);
              }}
            >
              {t("save")}
            </button>
            <button
              onClick={() => {
                setSelected(null);
              }}
            >
              {t("cancel")}
            </button>
          </div>
        </section>
      )}
      {q.data?.map((m) => (
        <section className="panel" key={m.id}>
          <h2>
            {m.provenance.source} · {m.provenance.version}
          </h2>
          <p>{m.provenance.note}</p>
          {m.provenance.attribution && <p className="muted">{m.provenance.attribution.notice}</p>}
          <small>
            {m.id} · {m.revision}
          </small>
          {m.relationships.map((edge, i) => (
            <div className="activity-row" key={i}>
              <span>
                {episodeLabel(m, edge.from)}
                <small> · {edge.from}</small>
              </span>
              <span>→</span>
              <span>
                {episodeLabel(m, edge.to)}
                <small> · {edge.to}</small>
              </span>
            </div>
          ))}
          {m.requires_consent && (
            <p className="notice">
              {t(m.kind === "movie-aliases" ? "movieAliasSafety" : "mappingSafety")}
            </p>
          )}
          {admin && (
            <button onClick={() => setHistoryID(historyID === m.id ? "" : m.id)}>
              {t("mappingHistory")}
            </button>
          )}
          {admin && historyID === m.id && (
            <div>
              <QueryFeedback query={history} t={t} />
              {history.data?.map((h, i) => (
                <details key={i}>
                  <summary>
                    {h.provenance.version} ·{" "}
                    {displayDate(null, h.recorded_at, settings, t("unknown"), true)}
                  </summary>
                  <p>{h.provenance.note}</p>
                  <p>{h.provenance.attribution?.notice}</p>
                  <ul>
                    {h.relationships.map((edge, n) => (
                      <li key={n}>
                        {edge.from} → {edge.to}
                      </li>
                    ))}
                  </ul>
                </details>
              ))}
            </div>
          )}
          <div className="toolbar">
            {m.requires_consent && !m.confirmed && (
              <button
                className="primary"
                disabled={confirm.isPending || !m.reviewed}
                onClick={() => {
                  if (window.confirm(t("confirmMapping"))) confirm.mutate(m);
                }}
              >
                {t("confirmMapping")}
              </button>
            )}
            {m.confirmed && <span>✓ {t("confirmed")}</span>}
            {admin && (
              <button
                onClick={() => {
                  setSelected(m);
                  setNote(m.provenance.note ?? "");
                }}
              >
                {t("reviewMapping")}
              </button>
            )}
          </div>
        </section>
      ))}
      {admin && (
        <>
          <AnimeIndex t={t} settings={settings} />
          <h2>{t("accounts")}</h2>
          <p>{t("accountsPrivacy")}</p>
          <QueryFeedback query={accounts} t={t} />
          {accounts.data?.map((a) => (
            <div className="activity-row" key={a.id}>
              <strong>{a.name}</strong>
              <small>{a.admin ? t("admin") : t("member")}</small>
            </div>
          ))}
          <h2>{t("refreshFailures")}</h2>
          <QueryFeedback query={jobs} t={t} />
          {jobs.data?.map((j) => (
            <div className="error" key={j.id}>
              <strong>{j.id}</strong>
              <p>{j.last_error}</p>
              <small>
                {t("retry")}: {displayDate(null, j.run_at, settings, t("unknown"), true)} (
                {j.attempts})
              </small>
            </div>
          ))}
        </>
      )}
    </>
  );
}
export function Notices({ t }: { t: T }) {
  const client = useQueryClient();
  const markRead = useMutation({
    mutationFn: (id: string) => api(`/notices/${encodeURIComponent(id)}/read`, "POST", {}),
    onSuccess: () => client.invalidateQueries({ queryKey: ["notices"] }),
  });
  const q = useQuery({
    queryKey: ["notices"],
    queryFn: () =>
      api<{ id: string; kind: string; data: { catalog_id?: string; mapping_id?: string } }[]>(
        "/notices",
      ),
  });
  return (
    <>
      {(q.error || markRead.error) && (
        <p role="alert">
          {q.error?.message ?? markRead.error?.message}
          <button
            onClick={() => {
              markRead.reset();
              void q.refetch();
            }}
          >
            {t("retry")}
          </button>
        </p>
      )}
      {q.data?.map((n) => (
        <div className="notice" role="status" key={n.id}>
          {n.kind === "mapping_changed"
            ? t("mappingChanged")
            : n.kind === "episode_reconciliation"
              ? t("episodeChanged")
              : t("noticeUpdated")}{" "}
          <a
            href={
              n.data.mapping_id
                ? "#mappings"
                : `#detail/${n.data.catalog_id?.split(":").slice(1).join("/") ?? ""}`
            }
          >
            {t("details")} →
          </a>
          <button
            disabled={markRead.isPending}
            onClick={() => {
              markRead.mutate(n.id);
            }}
          >
            {t("markRead")}
          </button>
        </div>
      ))}
    </>
  );
}

function episodeLabel(m: Mapping, id: string) {
  const e = m.episodes?.[id];
  return e
    ? e.kind === "movie"
      ? e.title
      : `${e.title} · ${e.season}:${e.number}${e.name ? ` · ${e.name}` : ""}`
    : id;
}
