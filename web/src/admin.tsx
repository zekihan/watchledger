import { AnimeIndex } from "./anime-index";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";
interface Mapping {
  id: string;
  revision: string;
  relationships: { from: string; to: string }[];
  provenance: { source: string; version: string; note?: string; inferred?: boolean };
  requires_consent: boolean;
  confirmed: boolean;
  reviewed: boolean;
}
export function MappingPage({ t, admin }: { t: T; admin: boolean }) {
  const client = useQueryClient();
  const q = useQuery({ queryKey: ["mappings"], queryFn: () => api<Mapping[]>("/mappings") });
  const [selected, setSelected] = useState<Mapping | null>(null);
  const [from, setFrom] = useState(""),
    [to, setTo] = useState(""),
    [note, setNote] = useState("");
  const confirm = useMutation({
    mutationFn: (m: Mapping) => api(`/mappings/${m.id}/confirm`, "POST", { revision: m.revision }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["mappings"] }),
  });
  const save = useMutation({
    mutationFn: (m: Mapping) =>
      api("/admin/mappings", "POST", {
        id: m.id,
        expected_revision: m.revision,
        relationships: m.relationships,
        provenance: {
          source: "local-admin-override",
          version: new Date().toISOString(),
          note,
          inferred: false,
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
      {q.isPending && <p>{t("loading")}</p>}
      {[q.error, save.error, confirm.error].filter(Boolean).map((e, i) => (
        <p className="error" role="alert" key={i}>
          {e?.message}
        </p>
      ))}
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
          <p>{t("mappingSafety")}</p>
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
              disabled={!from || !to}
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
          <small>
            {m.id} · {m.revision}
          </small>
          {m.relationships.map((edge, i) => (
            <div className="activity-row" key={i}>
              <code>{edge.from}</code>
              <span>→</span>
              <code>{edge.to}</code>
            </div>
          ))}
          {m.requires_consent && <p className="notice">{t("mappingSafety")}</p>}
          <div className="toolbar">
            {m.requires_consent && !m.confirmed && (
              <button
                className="primary"
                disabled={confirm.isPending}
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
          <AnimeIndex t={t} />
          <h2>{t("accounts")}</h2>
          <p>{t("accountsPrivacy")}</p>
          {accounts.error && <p className="error">{accounts.error.message}</p>}
          {accounts.data?.map((a) => (
            <div className="activity-row" key={a.id}>
              <strong>{a.name}</strong>
              <small>{a.admin ? t("admin") : t("member")}</small>
            </div>
          ))}
          <h2>{t("refreshFailures")}</h2>
          {jobs.error && <p className="error">{jobs.error.message}</p>}
          {jobs.data?.map((j) => (
            <div className="error" key={j.id}>
              <strong>{j.id}</strong>
              <p>{j.last_error}</p>
              <small>
                {t("retry")}: {new Date(j.run_at).toLocaleString()} ({j.attempts})
              </small>
            </div>
          ))}
        </>
      )}
    </>
  );
}
export function Notices({ t }: { t: T }) {
  const q = useQuery({
    queryKey: ["notices"],
    queryFn: () =>
      api<{ id: string; kind: string; data: { catalog_id?: string; mapping_id?: string } }[]>(
        "/notices",
      ),
  });
  return (
    <>
      {q.data?.map((n) => (
        <div className="notice" role="status" key={n.id}>
          {n.kind === "mapping_changed" ? t("mappingChanged") : t("episodeChanged")}{" "}
          <a
            href={
              n.data.mapping_id
                ? "#mappings"
                : `#detail/${n.data.catalog_id?.split(":").slice(1).join("/") ?? ""}`
            }
          >
            {t("details")} →
          </a>
        </div>
      ))}
    </>
  );
}
