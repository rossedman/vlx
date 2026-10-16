// Renders an HTML page to a PNG at 2x with headless Chromium.
//   node hack/render.cjs page.html out.png WIDTH HEIGHT
const { chromium } = require("playwright");
const path = require("path");
(async () => {
  const [, , input, output, w, h] = process.argv;
  const browser = await chromium.launch();
  const page = await browser.newPage({ deviceScaleFactor: 2, viewport: { width: +w, height: +h } });
  await page.goto("file://" + path.resolve(input));
  await page.waitForTimeout(400);
  await page.screenshot({ path: output, omitBackground: true });
  await browser.close();
})();
