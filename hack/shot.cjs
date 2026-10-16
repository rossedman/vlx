// Renders an HTML file to a PNG with headless Chromium.
//   node hack/shot.cjs in.html out.png [selector]
const { chromium } = require("playwright");
(async () => {
  const [, , input, output, selector = ".wrap"] = process.argv;
  const browser = await chromium.launch();
  const page = await browser.newPage({ deviceScaleFactor: 2, viewport: { width: 1800, height: 1400 } });
  await page.goto("file://" + require("path").resolve(input));
  await page.waitForTimeout(300);
  await (await page.$(selector)).screenshot({ path: output, omitBackground: true });
  await browser.close();
})();
