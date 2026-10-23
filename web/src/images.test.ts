import { expect, test } from "vitest";
import { imageURL } from "./images";
test("artwork accepts only source-owned image paths and rejects arbitrary remote or CSS payloads", () => {
  expect(imageURL("/synthetic.jpg")).toBe("https://image.tmdb.org/t/p/w342/synthetic.jpg");
  expect(imageURL("https://cdn-eu.anidb.net/images/main/123.jpg")).toBe(
    "https://cdn-eu.anidb.net/images/main/123.jpg",
  );
  for (const path of [
    "https://untrusted.invalid/a.jpg",
    "javascript:alert(1)",
    "/x.jpg) url(https://untrusted.invalid/a)",
    "//untrusted.invalid/a.jpg",
    "https://cdn-eu.anidb.net.evil.invalid/images/main/123.jpg",
  ]) {
    expect(imageURL(path)).toBeUndefined();
  }
});
