import path from "path"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// Dual build target:
//   VITE_TARGET=pages  -> ./dist, base "/Immortal/"  (GitHub Pages)
//   otherwise          -> ../internal/web/landing/static, base "/"  (embedded in Go binary)
const isPages = process.env.VITE_TARGET === "pages"

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
  base: isPages ? "/Immortal/" : "/",
  build: {
    outDir: isPages
      ? path.resolve(__dirname, "./dist")
      : path.resolve(__dirname, "../internal/web/landing/static"),
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
