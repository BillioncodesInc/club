# Pre-Built Redirector / Lure Page Templates

These are ready-to-use HTML templates ported from Evilginx that can be used as **before-landing pages** in Phishing Club campaign templates. They serve as intermediate pages that visitors see before being redirected to the actual phishing proxy page.

## Available Templates

| Template | Purpose | Use Case |
|---|---|---|
| `coming_soon/` | "Coming Soon" countdown page | Generic lure that auto-redirects after a delay |
| `download_example/` | File download prompt page | Lures victim into clicking a "download" button |
| `error404/` | Fake 404 error page | Decoy for bots/scanners; real targets get redirected |
| `geo_blocked/` | Geographic restriction page | Shown to blocked countries/IPs |
| `maintenance/` | Maintenance mode page | Professional-looking "under maintenance" decoy |
| `turnstile/` | Cloudflare Turnstile verification | Bot-filtering pre-lure with Turnstile integration |
| `verification_required/` | Email verification prompt | Lures victim into entering email before redirect |

## How to Use in Phishing Club

1. Go to **Pages** in the sidebar
2. Create a new page
3. Copy the HTML content from any template above
4. In your **Campaign Template**, set this page as the "Before Landing Page"
5. The template will auto-redirect to your proxy URL after the configured delay or action

## Customization

Each template uses CSS custom properties (variables) for easy theming:

```css
:root {
    --primary-h: 220;    /* Hue */
    --primary-s: 75%;    /* Saturation */
    --primary-l: 55%;    /* Lightness */
}
```

Change these values to match your target brand colors.

## Template Placeholders

Templates support Phishing Club's standard template placeholders:

- `{{.URL}}` — The phishing proxy URL (redirect target)
- `{{.FirstName}}` — Recipient's first name
- `{{.LastName}}` — Recipient's last name
- `{{.Email}}` — Recipient's email address
