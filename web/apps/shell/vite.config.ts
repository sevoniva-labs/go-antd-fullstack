import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
    build: {
      outDir: '../../dist',
      emptyOutDir: true,
      sourcemap: false,
      chunkSizeWarningLimit: 500,
      rolldownOptions: {
        output: {
          chunkFileNames: 'assets/chunk-[hash].js',
          codeSplitting: {
            groups: [
              {
                name: 'react-runtime',
                test: /node_modules[\\/](?:react|react-dom|scheduler)(?:[\\/]|$)/,
                priority: 50,
              },
              {
                name: 'ant-design-pro',
                test: /node_modules[\\/]@ant-design[\\/]pro-/,
                entriesAware: true,
                maxSize: 1024 * 1024,
                priority: 45,
              },
              {
                name: 'ant-design',
                test: /node_modules[\\/](?:antd|@ant-design[\\/]|@rc-component[\\/]|rc-[^\\/]+)(?:[\\/]|$)/,
                entriesAware: true,
                maxSize: 1024 * 1024,
                priority: 40,
              },
              {
                name: 'tanstack',
                test: /node_modules[\\/]@tanstack[\\/]/,
                entriesAware: true,
                priority: 35,
              },
              {
                name: 'vendor',
                test: /node_modules/,
                entriesAware: true,
                maxSize: 1024 * 1024,
                priority: 10,
              },
            ],
          },
        },
      },
    },
  })
