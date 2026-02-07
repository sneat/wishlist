// @ts-check
import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  site: import.meta.env.PUBLIC_SITE_URL || 'http://localhost:4321',
  outDir: '../backend/pb_public',
  vite: {
    plugins: [tailwindcss()]
  },
  image: {
    domains: ["cf.geekdo-images.com"],
  },
  experimental: {
    svgo: true,
  }
});