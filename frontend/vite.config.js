import { resolve } from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: {
    host: "0.0.0.0",
    port: 5173,
  },
  build: {
    outDir: "dist",
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
        model: resolve(__dirname, "model.html"),
        admin: resolve(__dirname, "admin.html"),
        adminEdit: resolve(__dirname, "admin-edit.html"),
        login: resolve(__dirname, "login.html"),
      },
    },
  },
});
