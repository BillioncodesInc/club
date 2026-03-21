# Club

[![Latest Release](https://img.shields.io/github/v/release/BillioncodesInc/club)](https://github.com/BillioncodesInc/club/releases/latest)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

**Club** is a phishing simulation and man-in-the-middle framework designed for companies that perform phishing simulation internally or as part of their business, and for aiding red teams in obtaining initial access.

It can be used both as a replacement for Gophish for phishers that are looking for more features and as an aid or alternative for offensive phishing tools like MITM frameworks.

## Quick Start

For systemd-enabled distributions, installation is quick and easy. Run the following on the server:

```bash
curl -fsSL https://raw.githubusercontent.com/BillioncodesInc/club/main/install.sh | bash
```

Remember to copy the admin URL and password.

Prebuilt images of the latest version are also available.

See [production docker compose example](https://github.com/BillioncodesInc/club/blob/main/docker-compose.production.yml) and [the latest images](https://github.com/BillioncodesInc/club/pkgs/container/club).

## Features

### Core Phishing Features

- **Multi-stage phishing flows** - Put together multiple phishing pages
- **Reverse proxy phishing** - Capture sessions to bypass weak MFA
- **Domain proxying** - Configure domains to proxy and mirror content from target sites
- **Flexible scheduling** - Time windows, business hours, or manual delivery
- **Multiple domains** - Auto TLS, custom sites, asset management
- **Advanced delivery** - SMTP configs or custom API Sender with OAuth support
- **Recipient tracking** - Groups, CSV import, repeat offender metrics
- **Analytics** - Timelines, dashboards, per-user event history
- **Automation** - HMAC-signed webhooks, REST API, import/export
- **Multi-tenancy** - Segregated client handling and statistics for service providers
- **Security features** - MFA, SSO, session management, IP filtering
- **Operational tools** - In-app updates, CLI installer, config management

### MITM and Red Team Features

- **Full control** - Modify and capture requests and responses independently
- **DOM rewriting** - Modify content using CSS/jQuery-like selectors or regex
- **Path and param rewriting** - Rewrite URL paths and query parameters on the fly
- **Dynamic obfuscation** - Avoid static detection with dynamically obfuscated landing pages
- **Evasion page** - Customize the pre-lure evasion page
- **Custom deny page** - Decide what bots or evaded visitors see
- **Access control** - Default deny-list until visiting phishing lure URL
- **Advanced filtering** - Use JA4, CIDR and geo-IP to control lure URL access
- **Browser impersonation** - Impersonate JA4 fingerprints in proxied requests
- **Response overwriting** - Shortcut proxying with custom responses
- **Forward proxying** - Use HTTP and SOCKS5 proxies to ensure requests originate from the right location
- **Visual Editor** - Use the visual editor to easily setup a proxy
- **Import compromised OAuth token** - Use compromised tokens to send more phishing via OAuth enabled endpoints

### Extended Features (Ghostsender + Evilginx Integration)

The following features have been integrated from Ghostsender and Evilginx to provide a comprehensive phishing toolkit:

| Feature | Description |
|---------|-------------|
| **SMS Phishing (Smishing)** | Send SMS-based phishing campaigns via configurable providers (Twilio, custom API) |
| **Domain Rotation** | Automatic domain rotation with configurable intervals, health checks, and Telegram notifications |
| **Bot Guard** | Advanced bot detection using browser fingerprinting, behavioral analysis, and challenge-response |
| **JS Injection** | Inject custom JavaScript rules into proxied pages for credential harvesting and DOM manipulation |
| **Headless Bypasser** | Automated headless browser sessions using go-rod for bypassing JavaScript-heavy protections |
| **Link Manager** | URL shortening and link management with proxy-aware link generation and click tracking |
| **Live Map** | Real-time geographical visualization of campaign events on an interactive map |
| **Captured Session Sender** | Replay and forward captured session cookies for account takeover testing |
| **Content Balancer** | Load-balance phishing content across multiple landing pages with weighted distribution |
| **WebServer Rules Generator** | Generate Apache/Nginx rewrite rules for redirector infrastructure |
| **DKIM Signing** | DKIM key generation and email signing for improved deliverability |
| **Attachment Generator** | Dynamic attachment generation (PDF, DOCX, HTML) with embedded tracking |
| **Anti-Detection** | Fingerprint randomization, header manipulation, and TLS fingerprint spoofing |
| **Email Warming** | Gradual email sending warmup to build sender reputation |
| **Enhanced Headers** | Custom email header injection for deliverability optimization |
| **Cookie Export** | Export captured session cookies in Netscape and JSON formats |
| **Chrome Extension** | Browser extension for real-time session capture with Telegram notifications |
| **Turnstile Integration** | Cloudflare Turnstile CAPTCHA integration for bot protection on phishing pages |
| **Telegram Notifications** | Real-time Telegram alerts for captured credentials, sessions, and campaign events |

## Docker Deployment

### Production

```bash
# Download the production docker-compose file
curl -O https://raw.githubusercontent.com/BillioncodesInc/club/main/docker-compose.production.yml

# Start the services
docker compose -f docker-compose.production.yml up -d
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CHROME_PATH` | `/usr/bin/chromium` | Path to Chromium binary for headless bypasser |

## Development Setup

### Prerequisites

- Docker and Docker Compose
- Git
- Make (optional, for convenience commands)

### Quick Start

1. **Clone the repository:**
```bash
git clone https://github.com/BillioncodesInc/club.git
cd club
```

2. **Start the services:**
```bash
make up
# or manually:
docker compose up -d
```

3. **Access the platform:**
- Administration: `http://localhost:8003`
- HTTP Phishing Server: `http://localhost:80`
- HTTPS Phishing Server: `https://localhost:443`

4. **Get admin credentials:**

The **username** and **password** are output in the terminal when you start the services. If you restart the backend service before completing setup by logging in, the username and password will change.

```bash
make backend-password
```

5. **Setup and start phishing:**

Open `https://localhost:8003` and setup the admin account using the credentials from step 4.

## Services and Ports

| Port | Service | Description |
|------|---------|-------------|
| 80 | HTTP Phishing Server | HTTP phishing server for campaigns |
| 443 | HTTPS Phishing Server | HTTPS phishing server with SSL |
| 8002 | Backend API | Backend API server |
| 8003 | Frontend | Development frontend with Vite |
| 8101 | Database Viewer | DBGate database administration |
| 8102 | Mail Server | Mailpit SMTP server with SpamAssassin integration |
| 8103 | Container Logs | Dozzle log viewer |
| 8104 | Container Stats | Docker container statistics |
| 8105 | MITMProxy | MITMProxy web interface |
| 8106 | MITMProxy | MITMProxy external access |
| 8201 | ACME Server | Pebble ACME server for certificates |
| 8202 | ACME Management | Pebble management interface |

## Development Commands

The `makefile` has convenience commands for development:

```bash
# Start all services
make up

# Stop all services
make down

# View logs
make logs

# Restart specific service
make backend-restart
make frontend-restart

# Access service containers
make backend-attach
make frontend-attach

# Reset backend database
make backend-db-reset

# Get backend admin password
make backend-password

# Verify new features compile
make verify-features

# Run smoke tests
make smoke-test
```

## Development Domains

For development we use `.test` for all domains. This must also be handled on the host level. You must either modify the hosts file and add the domains you use or run a local DNS server and ensure all `*.test` domains resolve to `127.0.0.1`.

### Option 1: DNSMasq (Recommended)
```bash
# Add to your DNSMasq configuration
address=/.test/127.0.0.1
```

### Option 2: Hosts File
Add to `/etc/hosts`:
```
127.0.0.1 microsoft.test
127.0.0.1 google.test
... add your development domains here
```

## Development SSL Certificates

The development environment uses Pebble ACME server for automatic SSL certificate generation. In production, configure your preferred ACME provider or upload custom certificates.

If you experience any issues with certificate generation, bring the backend down, clear the local certs and start the backend again:

```bash
make backend-down
make backend-clear-certs
make backend-up
```

## Certificate Warning

When developing it can be nice to ignore certificate warnings, especially when handling complex proxy setups. Use a dedicated browser and skip certificate warnings.

On Ubuntu you can add a custom shortcut for Chromium without cert warnings:

`~/.local/share/applications/chromium-dev.desktop`
```
[Desktop Entry]
Version=1.0
Type=Application
Name=Chromium Phishing Dev
Comment=Chromium for development with SSL certificate errors ignored
Exec=chromium-browser --ignore-certificate-errors --incognito
Icon=chromium-browser
Terminal=false
```

## API Endpoints

### New Feature Endpoints

All endpoints are prefixed with `/api/v1` and require authentication.

| Method | Endpoint | Feature |
|--------|----------|---------|
| GET/POST | `/sms/config` | SMS configuration |
| POST | `/sms/send` | Send SMS message |
| GET/POST | `/domain-rotation/config` | Domain rotation settings |
| POST | `/domain-rotation/rotate` | Trigger manual rotation |
| GET/POST | `/bot-guard/config` | Bot guard configuration |
| POST | `/bot-guard/verify` | Verify bot guard challenge |
| GET/POST | `/js-injection/rules` | JS injection rules |
| GET/POST | `/headless-bypasser/config` | Headless bypasser settings |
| POST | `/headless-bypasser/run` | Run headless bypass |
| GET | `/links` | Get all managed links |
| POST | `/links/shorten` | Shorten a URL |
| DELETE | `/links/:id` | Delete a managed link |
| GET | `/live-map/events` | Get live map events |
| GET | `/live-map/stats` | Get geographical statistics |
| GET/POST | `/captured-session/config` | Captured session settings |
| POST | `/captured-session/send` | Send captured session |
| GET/POST | `/content-balancer/config` | Content balancer settings |
| GET/POST | `/webserver-rules/config` | WebServer rules settings |
| POST | `/webserver-rules/generate` | Generate rewrite rules |
| GET/POST | `/dkim/config` | DKIM configuration |
| POST | `/dkim/generate` | Generate DKIM keys |
| POST | `/dkim/verify` | Verify DKIM setup |
| GET/POST | `/attachment-generator/config` | Attachment generator settings |
| POST | `/attachment-generator/generate` | Generate attachment |
| GET/POST | `/anti-detection/config` | Anti-detection settings |
| GET/POST | `/email-warming/config` | Email warming settings |
| GET/POST | `/enhanced-headers/config` | Enhanced headers settings |
| GET/POST | `/turnstile/config` | Turnstile CAPTCHA settings |
| GET/POST | `/telegram/config` | Telegram notification settings |
| POST | `/telegram/test` | Test Telegram connection |
| GET | `/cookie-export/:id` | Export cookies for event |

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

- You can use, modify, and distribute the software freely
- Perfect for educational, research, and commercial use
- You can run your own instance for security testing or professional services
- **Important**: If you provide the software modified as a network service, you must make your source code available under AGPL-3.0

## Security and Ethical Use

This platform is designed for **authorized security testing only**.

For important information about reporting security vulnerabilities, ethical use requirements, legal responsibilities, and security best practices, please read our [Security Policy](SECURITY.md).

**Important**: Users are solely responsible for ensuring their use complies with all applicable laws and regulations.
