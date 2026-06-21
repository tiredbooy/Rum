import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  base: "./",
  build: {
    rollupOptions: {
      output: {
        // Split the big, rarely-changing vendors out of the app shell so they
        // cache independently and load in parallel, instead of one ~620 KB
        // monolith that is re-parsed whenever app code changes.
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          if (id.includes("framer-motion") || id.includes("/motion/")) return "vendor-motion";
          if (id.includes("radix-ui") || id.includes("@radix-ui")) return "vendor-radix";
          if (id.includes("@tanstack")) return "vendor-tanstack";
          if (id.includes("react-router")) return "vendor-router";
          // react / react-dom / scheduler stay in the catch-all vendor chunk:
          // splitting them out creates a circular chunk with `vendor`.
          return "vendor";
        },
      },
    },
  },
});
