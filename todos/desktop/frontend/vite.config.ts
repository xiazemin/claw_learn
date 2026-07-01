import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");

  return {
    base: "./",
    plugins: [react(), tailwindcss(), tsconfigPaths({ ignoreConfigErrors: true })],
    server: {
      proxy: {
        "/api/manage": {
          target: "https://o.wpsgo.com/app/app-base",
          changeOrigin: true,
          timeout: 0,
          configure: (proxy) => {
            proxy.on("proxyReq", (proxyReq) => {
              if (env.DEBUG_TOKEN) {
                proxyReq.setHeader("X-Debug-Token", env.DEBUG_TOKEN);
              }
              if (env.DEBUG_USER_ID) {
                proxyReq.setHeader("X-Debug-User-Id", env.DEBUG_USER_ID);
              }
              if (env.VITE_PROJECT_ID) {
                proxyReq.setHeader("X-Project-Id", env.VITE_PROJECT_ID);
              }
            });
          },
        },
        "/api": {
          target: "http://localhost:4917",
          changeOrigin: false,
        },
      },
    },
    build: {
      outDir: "dist",
      sourcemap: true,
      target: ["chrome108", "es2022"],
    },
  };
});
