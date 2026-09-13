import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import basicSsl from "@vitejs/plugin-basic-ssl";

declare const process: {
  env: Record<string, string | undefined>;
};

const enableHttps =
  process.env.VOICE2CANVAS_HTTPS !== "0" &&
  process.env.V2UI_HTTPS !== "0" &&
  process.env.HTTPS !== "false";

const backendTarget =
  process.env.VOICE2CANVAS_BACKEND_URL ||
  process.env.V2UI_BACKEND_URL ||
  ("http://localhost:" +
    (process.env.VOICE2CANVAS_BACKEND_PORT ||
      process.env.V2UI_BACKEND_PORT ||
      process.env.PORT ||
      "8080"));

export default defineConfig({
  plugins: [react(), ...(enableHttps ? [basicSsl()] : [])],
  server: {
    host: true,
    proxy: {
      "/ws": {
        target: backendTarget,
        ws: true,
        changeOrigin: true,
      },
      "/healthz": {
        target: backendTarget,
        changeOrigin: true,
      },
      "/api": {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
});
