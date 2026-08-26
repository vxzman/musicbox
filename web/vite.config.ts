import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 生产构建产物由 Go 后端内嵌（同源，无 CORS）。
// 开发期 vite dev server 将 /api 与 /events 代理到本地守护进程。
export default defineConfig({
  plugins: [
    vue({
      template: {
        compilerOptions: {
          // @material/web 自定义元素（md-*）不作为 Vue 组件解析
          isCustomElement: (tag) => tag.startsWith('md-'),
        },
      },
    }),
  ],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8082', changeOrigin: true },
      '/events': { target: 'http://127.0.0.1:8082', changeOrigin: true },
    },
  },
})
