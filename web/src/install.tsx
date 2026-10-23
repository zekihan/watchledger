import type { T } from "./App";
import type { useInstall } from "./use-install";
export function InstallPanel({ t, state }: { t: T; state: ReturnType<typeof useInstall> }) {
  return (
    <section className="panel">
      <h2>{t("installTitle")}</h2>
      <p>{t(state.installed ? "installedApp" : "installHint")}</p>
      {!state.installed && state.available && (
        <button
          disabled={state.pending}
          onClick={() => {
            void state.install();
          }}
        >
          {t("installApp")}
        </button>
      )}
      {state.failed && <p role="alert">{t("installFailed")}</p>}
    </section>
  );
}
