/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_WUJIE_PRODUCTION_APPROVED?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

interface Window {
  __FORGE_CONFIG__?: Partial<import('./app/config/runtime').RuntimeConfig>
}
