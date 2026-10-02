import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // The bundle is embedded in the app and read from memory, never fetched over
  // a network, so the 500 kB download warning does not apply.
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: 1024 },
})
