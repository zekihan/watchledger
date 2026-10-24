import { expect, test } from "vitest";
import { importMessage } from "./import-messages";
import { en, tr } from "./i18n";
test("migration actions and source warnings are understandable in both UI languages", () => {
  expect(importMessage("preserve_existing", (key) => en[key])).toBe("Keep existing app values");
  expect(importMessage("preserve_existing", (key) => tr[key])).toBe(
    "Mevcut uygulama değerlerini koru",
  );
  expect(importMessage("episode_identity_review_required", (key) => tr[key])).toContain(
    "kimliklerini",
  );
  expect(importMessage("unsupported_section:synthetic_extra", (key) => tr[key])).toContain(
    "synthetic_extra",
  );
  expect(importMessage("future_source_code", (key) => en[key])).toBe(
    "Source diagnostic: future_source_code",
  );
});
