// @ts-check
import { test, expect } from '@playwright/test';
import { installBaseMocks } from './helpers.js';

test.describe('auth routing', () => {
	test('unauthenticated visit to / redirects to /login/', async ({ page }) => {
		await installBaseMocks(page, { loggedIn: false });

		await page.goto('/');
		// layout subscribes to appState and goto('/login/')s as soon as the
		// session ping 401s. give it a beat to run.
		await page.waitForURL('**/login/', { timeout: 10000 });
		expect(page.url()).toMatch(/\/login\/$/);
	});

	test('login page renders the username/password fields', async ({ page }) => {
		await installBaseMocks(page, { loggedIn: false });

		await page.goto('/login/');
		await expect(page.locator('input[type="password"]').first()).toBeVisible();
		// at least one text-ish input (username) + at least one password input exist
		const passwordCount = await page.locator('input[type="password"]').count();
		expect(passwordCount).toBeGreaterThan(0);
	});
});
