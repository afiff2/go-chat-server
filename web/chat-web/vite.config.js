import { fileURLToPath, URL } from 'node:url'
import fs from 'fs'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,  // 如果端口被占用就报错，不自动换端口
    // 可以打开下面一行，禁止 Host 校验（有安全风险，仅开发时用）
    // cors: true,
    // disableHostCheck: true
    https: {
      key: fs.readFileSync('/root/Project/go-chat-server/certs/server.key'),
      cert: fs.readFileSync('/root/Project/go-chat-server/certs/server.crt'),
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
})
