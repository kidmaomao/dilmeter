import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import vuetify from 'vite-plugin-vuetify';
import checker from 'vite-plugin-checker';
import wasm from 'vite-plugin-wasm';

const targetPort = 8030;
const targetUrl = `http://localhost:${targetPort}`;
const configDir = path.dirname(fileURLToPath(import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
    define: {
        __IS_STANDALONE__: process.env.STANDALONE === 'true',
    },
    plugins: [
        vue(),
        vuetify(),
        checker({
            vueTsc: true,
            // The legacy inspector pages still have Vue inject typings that
            // are intentionally checked separately. Do not make a production
            // bundle fail after Vite has successfully compiled the app.
            enableBuild: false,
        }),
        wasm(),
    ],
    server: {
        port: 8031,
        proxy: {
            '/ws': {
                target: targetUrl,
                changeOrigin: true,
                ws: true,
            },
            '/res': {
                target: targetUrl,
                changeOrigin: true,
            },
            '/api': {
                target: targetUrl,
                changeOrigin: true,
            },
        },
    },
    resolve: {
        alias: {
            '@': path.resolve(configDir, './src'),
        },
    },
    optimizeDeps: {
        exclude: [
            "brotli-dec-wasm",
        ],
    },
    build: {
        // Release builds omit debug maps and compact JavaScript; local debugging can opt in.
        sourcemap: process.env.DILMETER_DEBUG_FRONTEND === '1',
        minify: process.env.DILMETER_DEBUG_FRONTEND === '1' ? false : 'esbuild',
    },
});
