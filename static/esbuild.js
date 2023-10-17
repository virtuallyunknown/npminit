import { build } from 'esbuild';

await build({
    entryPoints: ['src/**/*.ts'],
    platform: 'node',
    target: 'node16',
    format: 'esm',
    bundle: false,
    outdir: './out',
    sourcemap: false,
    logLevel: 'warning',
});