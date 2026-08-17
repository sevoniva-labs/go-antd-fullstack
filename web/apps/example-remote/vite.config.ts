import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

const appRoot = fileURLToPath(new URL('.', import.meta.url));

export default defineConfig({
  base: './',
  plugins: [react()],
  resolve: {
    alias: {
      '@': resolve(appRoot, 'src'),
    },
  },
  server: {
    cors: {
      origin: 'http://localhost:5173',
    },
    headers: {
      'Cache-Control': 'no-store',
      'Referrer-Policy': 'no-referrer',
      'X-Content-Type-Options': 'nosniff',
    },
  },
  preview: {
    cors: {
      origin: ['http://localhost:5173', 'http://127.0.0.1:4173'],
    },
    headers: {
      'Cache-Control': 'no-store',
      'Referrer-Policy': 'no-referrer',
      'X-Content-Type-Options': 'nosniff',
    },
  },
  build: {
    sourcemap: false,
    chunkSizeWarningLimit: 500,
    rolldownOptions: {
      output: {
        strictExecutionOrder: true,
        codeSplitting: {
          groups: [
            {
              name: 'react-runtime',
              test: /node_modules[\\/](?:react|react-dom|scheduler)(?:[\\/]|$)/,
              priority: 30,
            },
            {
              name: 'ant-design',
              test: /node_modules[\\/](?:antd|@ant-design[\\/]|@rc-component[\\/]|rc-[^\\/]+)(?:[\\/]|$)/,
              maxSize: 400 * 1024,
              priority: 20,
            },
            {
              name: 'vendor',
              test: /node_modules/,
              maxSize: 350 * 1024,
              priority: 10,
            },
          ],
        },
      },
    },
  },
});
