import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  server: { host: "127.0.0.1", port: Number(process.env.WAILS_VITE_PORT) || 9256, strictPort: true },
  plugins: [react(), tailwindcss(), wails("./bindings")],
  css: {
    postcss: {
      plugins: [{
        postcssPlugin: "remove-reduced-motion",
        AtRule: {
          // 同时移除依赖样式中的减少动态效果规则
          media(rule) {
            if (rule.params.includes("prefers-reduced-motion")) rule.remove();
          },
        },
      }],
    },
  },
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
});
