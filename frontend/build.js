import { rmSync, cpSync, mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const dist = join(__dirname, '..', 'internal', 'web', 'dist');
const src = join(__dirname, 'src');
const root = join(__dirname, '..');

// Clean dist directory
if (existsSync(dist)) {
  rmSync(dist, { recursive: true });
}
mkdirSync(dist, { recursive: true });

// Copy static files
cpSync(join(src, 'index.html'), join(dist, 'index.html'));
cpSync(join(src, 'styles.css'), join(dist, 'styles.css'));

// Copy favicon
const faviconSrc = join(root, 'copilot_proxy.png');
if (existsSync(faviconSrc)) {
  cpSync(faviconSrc, join(dist, 'favicon.png'));
}

console.log('[build.js] ✓ Static files copied to', dist);