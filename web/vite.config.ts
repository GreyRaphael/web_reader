import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: fileURLToPath(new URL('../internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          katex: ['katex'],
          mermaid: ['mermaid'],
          markdown: ['markdown-it'],
          prism: ['prismjs'],
          dompurify: ['dompurify'],
          panzoom: ['panzoom'],
          vue: ['vue'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8848',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.ts'],
    restoreMocks: true,
  },
})
