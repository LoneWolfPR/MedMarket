import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The app is served through Traefik on port 80, so the browser reaches both
// the dev server and the API at the same origin (http://localhost). HMR needs
// its websocket pointed at the public port (80) rather than the internal 5173.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    port: 5173,
    strictPort: true,
    hmr: { clientPort: 80 },
  },
})
