// @ts-check
import { test, expect } from '@playwright/test';
import { installBaseMocks, fixture, json, expectRequest } from './helpers.js';

/**
 * Contract smoke tests for the Open Redirects page.
 *
 * These tests intercept the /api/v1/open-redirect* calls the frontend emits
 * and assert the exact wire format (method, URL, body shape). A mismatch --
 * e.g. frontend sends PUT but the backend registered PATCH -- causes a hard
 * test failure, which is precisely the bug class we're guarding against.
 */

const listURL = /\/api\/v1\/open-redirect(?:\?|$)/;
const byIdURL = /\/api\/v1\/open-redirect\/redir-aaa-111(?:\?|$)/;
const testURL = /\/api\/v1\/open-redirect\/redir-aaa-111\/test(?:\?|$)/;
const generateURL = /\/api\/v1\/open-redirect\/redir-aaa-111\/(?:generate|generate-link)(?:\?|$)/;
const knownSourcesURL = /\/api\/v1\/open-redirect\/(?:known-sources|sources)(?:\?|$)/;

/** Install the common open-redirect GETs used by the list view. */
async function installListMocks(page) {
	// Single redirect GET (used when opening the update modal).
	await page.route(byIdURL, async (route) => {
		if (route.request().method() === 'GET') {
			await route.fulfill(json(fixture('openRedirectSingle')));
			return;
		}
		// Non-GET requests on this URL are the mutations under test; let the
		// test-scoped handler take over by falling through.
		await route.fallback();
	});

	// List GET. Narrow to GET; mutations never hit this URL.
	await page.route(listURL, async (route) => {
		if (route.request().method() === 'GET') {
			await route.fulfill(json(fixture('openRedirects')));
			return;
		}
		await route.fallback();
	});

	// Proxy-domains dropdown lookup.
	await page.route('**/api/v1/domain/proxy*', async (route) => {
		await route.fulfill(json({ success: true, data: { rows: [], hasNextPage: false } }));
	});

	await page.route(knownSourcesURL, async (route) => {
		await route.fulfill(json(fixture('openRedirectSources')));
	});

	await page.route('**/api/v1/open-redirect/recommendations*', async (route) => {
		await route.fulfill(json({ success: true, data: [] }));
	});

	await page.route('**/api/v1/open-redirect/stats*', async (route) => {
		await route.fulfill(json({ success: true, data: {} }));
	});
}

test.beforeEach(async ({ page }) => {
	await installBaseMocks(page);
	await installListMocks(page);
});

test('list renders both redirects from the fixture', async ({ page }) => {
	await page.goto('/open-redirects/');
	await expect(page.getByText('Google AMP Redirect')).toBeVisible();
	await expect(page.getByText('LinkedIn Safe Link')).toBeVisible();
});

test('Edit submission uses PATCH /api/v1/open-redirect/<id> with the correct body', async ({
	page
}) => {
	await page.goto('/open-redirects/');
	await expect(page.getByText('Google AMP Redirect')).toBeVisible();

	// Open the update modal by clicking the name cell (which calls openUpdateModal).
	await page.getByRole('button', { name: 'Google AMP Redirect' }).click();

	// Modal fields pre-populate from GET /open-redirect/<id>. Target the
	// "Name" field by placeholder (e.g., Google AMP Redirect) and wait until
	// the modal has rehydrated — the GET resolves asynchronously.
	const nameInput = page.locator('input[placeholder*="Google AMP Redirect"]');
	await expect(nameInput).toHaveValue('Google AMP Redirect', { timeout: 10000 });

	// Change the name.
	await nameInput.fill('Google AMP Redirect (edited)');

	// Wait for the modal's submit button to be present + enabled before arming
	// the request watcher. FormButton applies Tailwind `uppercase` which can
	// make accessible-name matching flaky across engines, so target by
	// attribute under the form.
	const submit = page.locator('form button[type="submit"]').first();
	await expect(submit).toBeVisible({ timeout: 5000 });
	await expect(submit).toBeEnabled();

	// Intercept BOTH PATCH and PUT so the UI always unwinds regardless of
	// which the frontend sends. The assertion on requestPromise is what
	// enforces "must be PATCH".
	await page.route(byIdURL, async (route) => {
		const method = route.request().method();
		if (method === 'PATCH' || method === 'PUT') {
			await route.fulfill(json({ success: true, data: { id: 'redir-aaa-111' } }));
			return;
		}
		await route.fallback();
	});

	// Arm the watcher; a PUT-sending frontend will fail this expectRequest.
	const requestPromise = expectRequest(page, byIdURL, { method: 'PATCH', timeout: 8000 });

	await submit.click();

	const req = await requestPromise;
	expect(req.method()).toBe('PATCH');

	const body = req.postDataJSON();
	expect(body).toMatchObject({
		name: 'Google AMP Redirect (edited)',
		baseURL: 'https://www.google.com/url',
		paramName: 'q',
		platform: 'Google'
	});
});

test('Delete uses DELETE /api/v1/open-redirect/<id>', async ({ page }) => {
	await page.goto('/open-redirects/');
	await expect(page.getByText('Google AMP Redirect')).toBeVisible();

	const deletePromise = expectRequest(page, byIdURL, { method: 'DELETE', timeout: 5000 });

	await page.route(byIdURL, async (route) => {
		if (route.request().method() === 'DELETE') {
			await route.fulfill(json({ success: true, data: null }));
			return;
		}
		await route.fallback();
	});

	// Open row action menu. The ellipsis button is the first actionless
	// button inside the row for Google AMP Redirect; simplest reliable
	// selector is the Delete button inside the resulting DeleteAlert flow.
	const row = page.locator('tr', { hasText: 'Google AMP Redirect' }).first();
	await row.getByRole('button').last().click(); // ellipsis
	await page.getByRole('button', { name: /Delete/i }).first().click();
	// Confirm in the DeleteAlert dialog
	await page.getByRole('button', { name: /^(Delete|Confirm|Yes)/i }).last().click();

	const req = await deletePromise;
	expect(req.method()).toBe('DELETE');
});

test('Test Redirect fires POST /api/v1/open-redirect/<id>/test and renders hop chain', async ({
	page
}) => {
	await page.goto('/open-redirects/');
	await expect(page.getByText('Google AMP Redirect')).toBeVisible();

	const testReqPromise = expectRequest(page, testURL, { method: 'POST', timeout: 5000 });

	await page.route(testURL, async (route) => {
		await route.fulfill(json(fixture('openRedirectTestResult')));
	});

	const row = page.locator('tr', { hasText: 'Google AMP Redirect' }).first();
	await row.getByRole('button').last().click();
	await page.getByRole('button', { name: /Test Redirect/i }).click();

	const req = await testReqPromise;
	expect(req.method()).toBe('POST');
	expect(req.url()).toMatch(/\/open-redirect\/redir-aaa-111\/test$/);

	// Result modal should show the working banner and hop chain.
	await expect(page.getByText(/Redirect is working/i)).toBeVisible();
	await expect(page.getByText('example.com').first()).toBeVisible();
});

test('Generate Link fires POST /api/v1/open-redirect/<id>/generate(-link)', async ({ page }) => {
	await page.goto('/open-redirects/');
	await expect(page.getByText('Google AMP Redirect')).toBeVisible();

	const genReqPromise = expectRequest(page, generateURL, { method: 'POST', timeout: 5000 });

	await page.route(generateURL, async (route) => {
		await route.fulfill(
			json({ success: true, data: { redirectURL: 'https://www.google.com/url?q=https%3A%2F%2Fphish.example' } })
		);
	});

	const row = page.locator('tr', { hasText: 'Google AMP Redirect' }).first();
	await row.getByRole('button').last().click();
	await page.getByRole('button', { name: /Generate Link/i }).click();

	// Modal opens with a target URL field.
	const targetInput = page.locator('input[placeholder*="proxy-domain"], input[placeholder*="login"]').first();
	await targetInput.fill('https://phish.example/login');

	await page.getByRole('button', { name: /Generate Link/i }).last().click();

	const req = await genReqPromise;
	expect(req.method()).toBe('POST');
	const body = req.postDataJSON();
	expect(body).toMatchObject({ targetURL: 'https://phish.example/login' });
});
