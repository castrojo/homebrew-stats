// @ts-check
import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://castrojo.github.io',
  base: '/homebrew-stats',
  output: 'static',
  trailingSlash: 'always',
});
