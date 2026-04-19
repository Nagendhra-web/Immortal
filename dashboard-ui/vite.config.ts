import path from "path"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  base: "/dashboard/",
  build: {
    outDir: path.resolve(__dirname, "../internal/api/dashboard/static"),
    emptyOutDir: true,
    rollupOptions: {
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "app-[hash].js",
        assetFileNames: (info) => {
          if (info.name?.endsWith(".css")) return "app.css"
          return "assets/[name]-[hash][extname]"
        },
      },
    },
  },
})
