/// <reference types="astro/client" />

interface ImportMetaEnv {
  readonly PUBLIC_API_URL?: string;
  readonly PUBLIC_SITE_URL?: string;
  readonly PUBLIC_BGO_LOCALE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
