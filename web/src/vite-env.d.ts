/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly VITE_WS_HOST?: string;
  readonly VITE_DEVICE_KEY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
