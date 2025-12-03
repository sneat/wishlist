// @ts-check
import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  site: 'https://wishlist.mcmillan.id.au',
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