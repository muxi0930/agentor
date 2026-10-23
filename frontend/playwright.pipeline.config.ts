import { defineConfig } from "@playwright/test";
import { tmpdir } from "node:os";
import { join } from "node:path";
export default defineConfig({ testDir: "./tests/pipeline", testMatch: "*.spec.ts", workers: 1, use: { baseURL: "http://127.0.0.1:9267", viewport: { width: 1280, height: 860 }, launchOptions: { channel: "chrome" } }, webServer: { command: "npm run dev -- --port 9267", url: "http://127.0.0.1:9267/tests/pipeline/index.html", reuseExistingServer: true }, reporter: "list", outputDir: join(tmpdir(), "agentor-pipeline-playwright") });
