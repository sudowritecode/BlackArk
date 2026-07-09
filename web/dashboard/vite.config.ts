import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

const backendTarget = "http://localhost:3000";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      "/api": backendTarget,
      "/health": backendTarget,
    },
  },
});
