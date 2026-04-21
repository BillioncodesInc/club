import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for frontend contract smoke tests.
 *
 * These tests run against the VITE PREVIEW build (production output) with
 * all outbound /api/ requests intercepted via page.route(). They are NOT
 * integration tests -- they are contract tests designed to catch bugs where
 * the frontend calls the wrong HTTP method, wrong URL, or wrong body shape
 * relative to the backend contract.
 *
 * See tests/e2e/README.md for extension patterns.
 */
export default defineConfig({
	testDir: './tests/e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: [['list'], ['html', { open: 'never' }]],
	use: {
		baseURL: 'http://localhost:4173',
		trace: 'on-first-retry',
		// the app runs fully client-side (SSR disabled, static adapter); no cookies
		// survive a fresh tab -- every navigation starts unauthenticated, which is
		// exactly what we want for contract tests that drive state via mocks.
		viewport: { width: 1280, height: 900 }
	},
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] }
		}
	],
	webServer: {
		command: 'npm run build && npm run preview',
		url: 'http://localhost:4173',
		reuseExistingServer: !process.env.CI,
		timeout: 120000,
		stdout: 'pipe',
		stderr: 'pipe'
	}
});
