/// <reference types="vitest/config" />
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
  test: {
    // Components need a DOM. jsdom is an in-process implementation — no
    // browser, no Docker, so `task frontend:test` stays as fast as `tsc`.
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    // Tailwind's plugin would otherwise compile the stylesheet on every run to
    // produce class names no assertion reads.
    css: false,
    // Undo spies and `vi.stubGlobal` between tests so one file's fetch stub
    // can't leak into the next.
    restoreMocks: true,
    unstubGlobals: true,
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/vite-env.d.ts', 'src/test/**', 'src/**/*.test.{ts,tsx}'],
    },
  },
})
