import { chromium } from "playwright";

const BASE = process.env.BASE_URL || "http://127.0.0.1:8080";
const THEME = process.env.THEME || "";

const PAGES = [
  ["dashboard", "/dashboard"],
  ["rules", "/rules"],
  ["activity", "/activity"],
  ["settings", "/settings"],
  ["stats", "/stats"],
  ["docs", "/docs"],
];

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  colorScheme: "dark",
});

await context.addCookies([
  {
    name: "mailflow_session",
    value: "demo-session-token-abc123",
    url: BASE,
  },
]);

const page = await context.newPage();
if (THEME) {
  await page.addInitScript((t) => localStorage.setItem("theme", t), THEME);
}

for (const [name, path] of PAGES) {
  await page.goto(BASE + path, { waitUntil: "networkidle" });
  await page.waitForTimeout(800);
  await page.screenshot({
    path: `docs/screenshots/${name}.webp`,
    type: "webp",
    quality: 80,
  });
  console.log(`captured ${name}`);
}

await browser.close();
