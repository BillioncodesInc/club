import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// manualChunks: split heavy vendor libraries into their own chunks so they load
// on-demand (monaco/leaflet are imported dynamically on routes that need them)
// and don't bloat per-route bundles. the SvelteKit static adapter is a pure
// build-time export-to-HTML step and is unaffected by client chunk topology.
function manualChunks(id) {
	if (!id.includes('node_modules')) return;
	if (id.includes('monaco-editor') || id.includes('monaco-vim')) return 'monaco';
	if (
		id.includes('/leaflet/') ||
		id.includes('leaflet.markercluster') ||
		id.includes('leaflet.heat') ||
		id.includes('leaflet-src')
	) {
		return 'leaflet';
	}
	if (id.includes('/node_modules/d3') || /node_modules\/d3-[^/]+\//.test(id)) return 'd3';
	if (id.includes('papaparse')) return 'editor-utils';
}

export default defineConfig({
	plugins: [sveltekit()],
	build: {
		sourcemap: true,
		cssMinify: true,
		minify: true,
		assetsInlineLimit: 4096,
		rollupOptions: {
			output: {
				compact: true,
				manualChunks
			}
		}
	}
});
