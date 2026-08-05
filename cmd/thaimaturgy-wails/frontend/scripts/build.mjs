import { cpSync, mkdirSync, rmSync, watch } from 'node:fs';
import { join } from 'node:path';

const root = new URL('..', import.meta.url).pathname;
const src = join(root, 'src');
const dist = join(root, 'dist');

function build() {
  rmSync(dist, { recursive: true, force: true });
  mkdirSync(dist, { recursive: true });
  cpSync(src, dist, { recursive: true });
  console.log(`built ${dist}`);
}

build();

if (process.argv.includes('--watch')) {
  watch(src, { recursive: true }, build);
}
