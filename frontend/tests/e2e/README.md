# Frontend Contract Smoke Tests (Playwright)

These are **contract tests**, not integration tests. They run the frontend's
production build (`vite build` + `vite preview`) against a fully mocked
backend. Every outbound `/api/v1/**` request is intercepted via
`page.route()`, asserted on (method, URL, body shape), then answered with a
local JSON fixture.

They catch the class of bug where the frontend drifts from the backend HTTP
contract, for example:

- Frontend sends `PUT /api/v1/open-redirect/<id>` but the backend mounted
  `PATCH /api/v1/open-redirect/<id>`.
- Frontend sends `POST /api/v1/foo/delete` but the backend expects
  `DELETE /api/v1/foo/<id>`.
- Frontend sends `{ foo: "bar" }` but the backend binds on `{ name: "bar" }`.

**They do NOT replace backend unit tests.** They prove the wire between
front and back is correctly named and shaped, nothing more.

## Running

```bash
cd frontend
npm run test:e2e              # headless run
npm run test:e2e:ui           # interactive UI mode
npm run test:e2e:report       # open the HTML report from the last run
```

The `webServer` block in `playwright.config.js` runs `npm run build && npm
run preview` on first test invocation (port 4173) and reuses the running
server between local runs.

## Layout

```
tests/e2e/
  fixtures/                    # small local JSON fixtures
  helpers.js                   # installBaseMocks, fixture, json, expectRequest
  auth.spec.js                 # unauth redirect + login render
  open-redirects.spec.js       # full CRUD + test + generate-link contract
  contract-smoke.spec.js       # parameterized "does it render without 404s"
```

## Writing a new mock + assertion pattern

```js
import { test, expect } from '@playwright/test';
import { installBaseMocks, fixture, json, expectRequest } from './helpers.js';

test('Foo page: Save uses PATCH /api/v1/foo/<id> with correct body', async ({ page }) => {
	await installBaseMocks(page);

	// 1. Mock the GETs needed to render the page.
	await page.route(/\/api\/v1\/foo(?:\?|$)/, (r) => r.fulfill(json(fixture('fooList'))));

	// 2. Drive the page.
	await page.goto('/foo/');
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.locator('input[name="title"]').fill('new title');

	// 3. Arm the contract assertion BEFORE the action.
	const reqPromise = expectRequest(page, /\/api\/v1\/foo\/foo-1/, { method: 'PATCH' });

	// 4. Short-circuit the mutation with a happy-path mock.
	await page.route(/\/api\/v1\/foo\/foo-1/, (r) =>
		r.fulfill(json({ success: true, data: null }))
	);

	await page.getByRole('button', { name: 'Save' }).click();

	// 5. Inspect the exact wire format.
	const req = await reqPromise;
	expect(req.method()).toBe('PATCH');
	expect(req.postDataJSON()).toMatchObject({ title: 'new title' });
});
```

## Rules of thumb

- One `expectRequest` per mutation. That's the assertion.
- Specific `page.route` patterns win over catch-alls; order doesn't matter,
  but specificity does.
- Use `route.fallback()` inside a handler if you want another registered
  handler to take over (useful when a URL matches both the list GET mock
  and the mutation mock).
- Keep fixtures small -- ~15 lines each. They are contract documents, not
  dataset dumps.
- NEVER call the real backend from a test. If a request escapes the mock
  net, the test should fail.
