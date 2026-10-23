import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { T } from "./App";
import type { Mapping } from "./admin";
interface Dataset {
  checksum: string;
  version: string;
  source_revision: string;
  generated_at: string;
  attribution: { notice: string; license: string; contents_license: string };
}
interface Proposal {
  proposal_id: string;
  existing_id: string;
  existing_revision: string;
  revision: string;
  relationships: { from: string; to: string }[];
  requires_consent: boolean;
  provenance: Mapping["provenance"];
  attribution: Dataset["attribution"];
  unresolved: {
    candidate: { source_episode: string; native_number: number; inferred: boolean };
    reason: string;
  }[];
}
export function MappingDatasets({ t, onReview }: { t: T; onReview: (m: Mapping) => void }) {
  const client = useQueryClient();
  const [version, setVersion] = useState("");
  const [checksum, setChecksum] = useState("");
  const [dataset, setDataset] = useState("");
  const [aid, setAid] = useState("");
  const q = useQuery({
    queryKey: ["mapping-datasets"],
    queryFn: () => api<Dataset[]>("/admin/mapping-datasets"),
  });
  const upload = useMutation({
    mutationFn: async (file: File) => {
      if (file.size > 32 * 1024 * 1024) throw new Error(t("importSize"));
      const response = await fetch(
        `/api/admin/mapping-datasets?version=${encodeURIComponent(version)}&checksum=${encodeURIComponent(checksum)}`,
        {
          method: "POST",
          credentials: "same-origin",
          headers: { "Content-Type": "application/json" },
          body: file,
        },
      );
      if (!response.ok) throw new Error(await response.text());
    },
    onSuccess: () => client.invalidateQueries({ queryKey: ["mapping-datasets"] }),
  });
  const resolve = useMutation({
    mutationFn: () =>
      api<Proposal>(`/admin/mapping-datasets/${dataset}/anime/${aid}/resolve`, "POST", {}),
  });
  const busy = upload.isPending || resolve.isPending;
  return (
    <section className="panel">
      <h2>{t("mappingDatasets")}</h2>
      <p>{t("mappingDatasetHint")}</p>
      <p>
        <a href="https://github.com/cplieger/animap/releases">animap releases</a> ·{" "}
        <a href="https://opendatacommons.org/licenses/odbl/1-0/">ODbL 1.0</a> /{" "}
        <a href="https://opendatacommons.org/licenses/dbcl/1-0/">DbCL 1.0</a>
      </p>
      <fieldset disabled={busy}>
        <label>
          {t("datasetVersion")}
          <input value={version} onChange={(e) => setVersion(e.target.value)} />
        </label>
        <label>
          SHA-256
          <input value={checksum} onChange={(e) => setChecksum(e.target.value.trim())} />
        </label>
        <label>
          {t("datasetFile")}
          <input
            type="file"
            accept=".json"
            disabled={!version || !/^[a-f0-9]{64}$/.test(checksum)}
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) upload.mutate(file);
              e.target.value = "";
            }}
          />
        </label>
        <label>
          {t("datasetVersion")}
          <select
            value={dataset}
            onChange={(e) => {
              setDataset(e.target.value);
              resolve.reset();
            }}
          >
            <option value="">—</option>
            {q.data?.map((d) => (
              <option key={d.checksum} value={d.checksum}>
                {d.version} · {d.source_revision}
              </option>
            ))}
          </select>
        </label>
        <label>
          AniDB ID
          <input
            type="number"
            min="1"
            value={aid}
            onChange={(e) => {
              setAid(e.target.value);
              resolve.reset();
            }}
          />
        </label>
        <button disabled={!dataset || !/^[1-9][0-9]*$/.test(aid)} onClick={() => resolve.mutate()}>
          {t("resolveDataset")}
        </button>
      </fieldset>
      {busy && <p role="status">{t("loading")}</p>}
      {[q.error, upload.error, resolve.error].filter(Boolean).map((e, i) => (
        <p role="alert" key={i}>
          {e?.message}
        </p>
      ))}
      {q.data?.find((d) => d.checksum === dataset) && (
        <p className="muted">{q.data.find((d) => d.checksum === dataset)?.attribution.notice}</p>
      )}
      {resolve.data && (
        <div className="preview">
          <h3>
            {t("reviewMapping")}: {resolve.data.relationships.length}
          </h3>
          <p>
            {t("unresolved")}: {resolve.data.unresolved.length}
          </p>
          {resolve.data.requires_consent && <p>{t("mappingSafety")}</p>}
          <ul>
            {resolve.data.relationships.slice(0, 50).map((e) => (
              <li key={e.from + e.to}>
                {e.from} → {e.to}
              </li>
            ))}
          </ul>
          <ul>
            {resolve.data.unresolved.slice(0, 50).map((x, i) => (
              <li key={i}>
                {x.candidate.source_episode || x.candidate.native_number} · {x.reason}
              </li>
            ))}
          </ul>
          <button
            disabled={!resolve.data.relationships.length}
            onClick={() => {
              const p = resolve.data;
              onReview({
                id: p.existing_id,
                revision: p.existing_revision,
                proposal_id: p.proposal_id,
                relationships: p.relationships,
                provenance: p.provenance,
                requires_consent: p.requires_consent,
                reviewed: false,
                confirmed: false,
              });
            }}
          >
            {t("reviewMapping")}
          </button>
        </div>
      )}
    </section>
  );
}
