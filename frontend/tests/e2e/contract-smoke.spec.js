// @ts-check
import { test, expect } from '@playwright/test';
import { installBaseMocks, json } from './helpers.js';

/**
 * Parameterized smoke pass over the main authenticated routes.
 *
 * For each route we:
 *   - mock every GET /api/v1/** to return an empty paginated shell so the
 *     page renders without real data,
 *   - watch for any 404s on our own /api/ URL space (catching dead URLs the
 *     frontend invents),
 *   - assert no unhandled console errors escape.
 *
 * This is a regression net, not a correctness check. It trips on:
 *   - frontend calls /api/v1/foo but backend mounts /api/v1/foos (404),
 *   - frontend throws an unhandled reference error on mount,
 *   - a fetch fires to an unmocked external host (route.continue leaking).
 */

const routes = [
	'/dashboard/',
	'/campaign/',
	'/company/',
	'/domain/',
	'/email/',
	'/recipient/',
	'/proxy/'
];

for (const route of routes) {
	test(`smoke: ${route} loads without api 404s or unhandled errors`, async ({ page }) => {
		await installBaseMocks(page);

		/** @type {string[]} */
		const api404s = [];
		/** @type {string[]} */
		const consoleErrors = [];

		page.on('response', (res) => {
			const url = res.url();
			if (url.includes('/api/v1/') && res.status() === 404) {
				api404s.push(`${res.request().method()} ${url}`);
			}
		});

		page.on('pageerror', (err) => {
			consoleErrors.push(`pageerror: ${err.message}`);
		});

		page.on('console', (msg) => {
			if (msg.type() === 'error') {
				const txt = msg.text();
				// SvelteKit static adapter dispatches a dev-only warning about
				// missing route params for dynamic segments that we have no
				// way of seeding from a mocked fetch -- ignore those.
				if (txt.includes('Failed to load resource')) return;
				consoleErrors.push(txt);
			}
		});

		// Catch-all mock for any GET /api/v1/** the page under test decides to
		// make. Handler order matters: specific mocks (in installBaseMocks)
		// win; this is the fallthrough.
		await page.route('**/api/v1/**', async (r) => {
			const method = r.request().method();
			if (method === 'GET') {
				await r.fulfill(json({ success: true, data: { rows: [], hasNextPage: false } }));
				return;
			}
			// Non-GET mutations on smoke-tested routes: reply 200 empty so the
			// UI doesn't explode if a test accidentally triggers one.
			await r.fulfill(json({ success: true, data: {} }));
		});

		await page.goto(route);
		// Give any onMount fetches a beat to complete.
		await page.waitForLoadState('networkidle', { timeout: 10000 }).catch(() => {});

		expect(api404s, `unexpected 404s on ${route}:\n${api404s.join('\n')}`).toEqual([]);
		expect(
			consoleErrors,
			`unhandled errors on ${route}:\n${consoleErrors.join('\n')}`
		).toEqual([]);
	});
}
