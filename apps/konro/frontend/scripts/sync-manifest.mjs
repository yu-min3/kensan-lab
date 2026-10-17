// PWA launch colors also come from the platform's token source.
import { readFile, writeFile } from "node:fs/promises";
const tokens = JSON.parse(await readFile(new URL("../../../../packages/design-tokens/tokens.json", import.meta.url), "utf8"));
const target = new URL("../public/manifest.webmanifest", import.meta.url);
const manifest = JSON.parse(await readFile(target, "utf8"));
manifest.background_color = `hsl(${tokens.color.light.background.hsl})`;
manifest.theme_color = manifest.background_color;
await writeFile(target, JSON.stringify(manifest, null, 2) + "\n");
