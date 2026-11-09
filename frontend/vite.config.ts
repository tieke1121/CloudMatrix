import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Vite 构建配置，配置 Vue 插件与 Vitest 测试环境
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  server: {
    port: 5173
  },
  test: {
    globals: true,
    environment: 'jsdom'
  }
} as any)
