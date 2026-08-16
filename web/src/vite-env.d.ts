/// <reference types="vite/client" />

interface Window {
  __FORGE_CONFIG__?: Partial<import('./app/config/runtime').RuntimeConfig>
}
