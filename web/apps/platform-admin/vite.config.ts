import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    lib: {
      entry: 'src/index.ts',
      formats: ['es'],
      fileName: 'platform-admin',
    },
    sourcemap: false,
    rolldownOptions: {
      external: [
        'react',
        'react-dom',
        'antd',
        '@ant-design/icons',
        '@ant-design/pro-components',
        '@tanstack/react-query',
        '@forge/api-client',
        '@forge/auth-sdk',
        '@forge/design-system',
      ],
    },
  },
})
