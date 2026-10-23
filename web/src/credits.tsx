import type { T } from "./App";
export function CreditsPage({ t }: { t: T }) {
  return (
    <section className="panel">
      <h1>{t("sourceCredits")}</h1>
      <p>{t("sourceRights")}</p>
      <section>
        <h2>TMDB</h2>
        <img src="/credits/tmdb.svg" alt="TMDB" width="80" style={{ height: "auto" }} />
        <p>{t("attribution")}</p>
        <a href="https://www.themoviedb.org" target="_blank" rel="noreferrer">
          TMDB ↗
        </a>
        {" · "}
        <a href="https://developer.themoviedb.org/docs/faq" target="_blank" rel="noreferrer">
          {t("sourceRules")}
        </a>
      </section>
      <section>
        <h2>JustWatch</h2>
        <p>{t("justWatchCredits")}</p>
        <a href="https://www.justwatch.com" target="_blank" rel="noreferrer">
          JustWatch ↗
        </a>
      </section>
      <section>
        <h2>AniDB</h2>
        <p>{t("anidbCredits")}</p>
        <a href="https://anidb.net/policy" target="_blank" rel="noreferrer">
          AniDB · {t("sourceRules")}
        </a>
        {" · "}
        <a
          href="https://creativecommons.org/licenses/by-nc-sa/4.0/"
          target="_blank"
          rel="noreferrer"
        >
          CC BY-NC-SA 4.0
        </a>
      </section>
      <section>
        <h2>animap</h2>
        <p>{t("animapCredits")}</p>
        <a href="https://github.com/cplieger/animap" target="_blank" rel="noreferrer">
          animap ↗
        </a>
        {" · "}
        <a href="https://opendatacommons.org/licenses/odbl/1-0/" target="_blank" rel="noreferrer">
          ODbL 1.0
        </a>
        {" / "}
        <a href="https://opendatacommons.org/licenses/dbcl/1-0/" target="_blank" rel="noreferrer">
          DbCL 1.0
        </a>
      </section>
      <section>
        <h2>
          <img
            src="https://us.simkl.in/img_favicon/v2/favicon-192x192.png"
            alt=""
            width="18"
            height="18"
          />{" "}
          SIMKL
        </h2>
        <p>{t("simklCredits")}</p>
        <a href="https://simkl.com" target="_blank" rel="noreferrer">
          SIMKL ↗
        </a>
        {" · "}
        <a href="https://api.simkl.org/api-rules" target="_blank" rel="noreferrer">
          {t("sourceRules")}
        </a>
      </section>
    </section>
  );
}
