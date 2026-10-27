import { useState } from "react";
import { imageURL } from "./images";

export type Provider = { provider_id: number; provider_name: string; logo_path?: string };

export function ProviderCard({ provider, link }: { provider: Provider; link?: string }) {
  const content = (
    <>
      <ProviderMark provider={provider} />
      <span>{provider.provider_name}</span>
      {link && (
        <span className="provider-arrow" aria-hidden="true">
          ↗
        </span>
      )}
    </>
  );
  return link ? (
    <a className="provider-card" href={link} target="_blank" rel="noreferrer">
      {content}
    </a>
  ) : (
    <span className="provider-card">{content}</span>
  );
}

export function ProviderMark({ provider }: { provider: Provider }) {
  const [failed, setFailed] = useState(false);
  const logo = imageURL(provider.logo_path, "w92");
  return (
    <span className="provider-mark" aria-hidden="true">
      {logo && !failed ? (
        <img src={logo} alt="" loading="lazy" onError={() => setFailed(true)} />
      ) : (
        provider.provider_name.trim().charAt(0).toLocaleUpperCase()
      )}
    </span>
  );
}
