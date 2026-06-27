import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // Relative base so Monaco's bundled worker/asset URLs resolve correctly under
  // the Wails asset-server origin (offline, no CDN).
  base: './',
  plugins: [vue(), tailwindcss()],
  build: {
    outDir: 'dist',
  },
})
