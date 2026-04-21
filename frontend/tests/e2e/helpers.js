// @ts-check
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));

/**
 * Load a JSON fixture by name (no extension).
 *
 * @param {string} name
 */
export function fixture(name) {
	const p = resolve(__dirname, 'fixtures', `${name}.json`);
	return JSON.parse(readFileSync(p, 'utf8'));
}

/**
 * Serialize a fixture to a Playwright-compatible JSON response body.
 *
 * @param {unknown} obj
 */
export function json(obj) {
	return {
		status: 200,
		contentType: 'application/json',
		body: JSON.stringify(obj)
	};
}

/**
 * Minimal auth/setup mocks common to every page: session ping, install status,
 * feature flags, update check, SSO check. These answer 200 with safe defaults
 * so the app boots past the layout guards into the actual route under test.
 *
 * Mount these BEFORE page.goto() so they intercept the bootstrap traffic.
 *
 * @param {import('@playwright/test').Page} page
 * @param {{ loggedIn?: boolean }} opts
 */
export async function installBaseMocks(page, opts = {}) {
	const { loggedIn = true } = opts;

	// RootLoader polls /healthz every second; must answer 200 before the
	// app flips isReady=true and starts rendering route content.
	await page.route('**/api/v1/healthz*', async (route) => {
		await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
	});

	await page.route('**/api/v1/session/ping', async (route) => {
		if (!loggedIn) {
			await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ success: false, error: 'unauthorized' }) });
			return;
		}
		await route.fulfill(json(fixture('session')));
	});

	await page.route('**/api/v1/install*', async (route) => {
		await route.fulfill(json({ success: true, data: { installed: true } }));
	});

	await page.route('**/api/v1/features*', async (route) => {
		await route.fulfill(json({ success: true, data: { sso: false } }));
	});

	await page.route('**/api/v1/update/available/cached*', async (route) => {
		await route.fulfill(json({ success: true, data: { updateAvailable: false } }));
	});

	// Generic option fallback FIRST, then specific override for is_installed.
	// Playwright matches the LAST-registered route that patterns the URL, so
	// this order ensures is_installed wins over the catch-all.
	await page.route('**/api/v1/option/**', async (route) => {
		await route.fulfill(json({ success: true, data: { value: '' } }));
	});

	// session.js checks option/is_installed immediately after a successful ping;
	// the layout refuses to render non-install routes until this returns 'true'.
	await page.route('**/api/v1/option/is_installed*', async (route) => {
		await route.fulfill(json({ success: true, data: { value: 'true' } }));
	});

	await page.route('**/api/v1/sso/**', async (route) => {
		await route.fulfill(json({ success: true, data: false }));
	});
}

/**
 * Capture the first request that matches a URL-method filter. Returns a
 * promise resolving to { url, method, postData } once matched; rejects on
 * timeout.
 *
 * Use this to ASSERT on the exact wire format the frontend emits, so the
 * test fails fast when the frontend drifts from the backend contract.
 *
 * @param {import('@playwright/test').Page} page
 * @param {RegExp} urlPattern
 * @param {{ method?: string; timeout?: number }} opts
 */
export function expectRequest(page, urlPattern, opts = {}) {
	const { method, timeout = 5000 } = opts;
	return page.waitForRequest(
		(req) => {
			if (!urlPattern.test(req.url())) return false;
			if (method && req.method().toUpperCase() !== method.toUpperCase()) return false;
			return true;
		},
		{ timeout }
	);
}
