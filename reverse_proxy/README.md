# Reverse Proxy Configuration for PhishingClub

## Why Use a Reverse Proxy?

Placing a reverse proxy (Nginx or Caddy) in front of PhishingClub's Go proxy server provides several evasion benefits:

1. **TLS Fingerprint Masking**: Go's `crypto/tls` library has a distinct JA3/JA4 fingerprint that security tools can detect. Nginx and Caddy use OpenSSL/BoringSSL, which produces a different, more common fingerprint.

2. **WebSocket Support**: Some target sites use WebSocket connections. The reverse proxy handles the upgrade transparently.

3. **Rate Limiting**: The reverse proxy can add rate limiting to prevent abuse detection.

4. **IP Whitelisting**: Restrict admin panel access to specific IPs while keeping the phishing proxy open.

5. **Logging**: Separate access logs for forensic analysis.

## Architecture

```
Internet → Reverse Proxy (443) → PhishingClub Go Proxy (127.0.0.1:8443)
                                → PhishingClub Admin UI (127.0.0.1:3000)
```

## Quick Start

### Option A: Nginx

```bash
# Install Nginx
sudo apt install nginx -y

# Copy config
sudo cp nginx.conf /etc/nginx/sites-available/phishingclub
sudo ln -s /etc/nginx/sites-available/phishingclub /etc/nginx/sites-enabled/

# Generate self-signed cert (for testing)
sudo mkdir -p /etc/nginx/ssl
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/nginx/ssl/phishingclub.key \
  -out /etc/nginx/ssl/phishingclub.crt \
  -subj "/CN=*.yourdomain.com"

# Or use Let's Encrypt wildcard cert
# sudo certbot certonly --dns-cloudflare -d "*.yourdomain.com" -d "yourdomain.com"

# Test and reload
sudo nginx -t && sudo systemctl reload nginx
```

### Option B: Caddy (Recommended)

```bash
# Install Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy -y

# Update DOMAIN_NAME in Caddyfile
sed -i 's/DOMAIN_NAME/yourdomain.com/g' Caddyfile

# Run Caddy (auto-obtains SSL certificates)
sudo caddy run --config Caddyfile
```

## Configuration Notes

### PhishingClub Configuration

When using a reverse proxy, configure PhishingClub to:

1. **Bind to localhost only**: Set the proxy listen address to `127.0.0.1:8443`
2. **Trust X-Forwarded-For**: Enable the `X-Real-IP` and `X-Forwarded-For` headers for correct client IP detection
3. **Handle CF-Connecting-IP**: If behind Cloudflare, the `CF-Connecting-IP` header contains the real client IP

### Client IP Priority

The correct client IP extraction priority should be:

```
1. CF-Connecting-IP (if behind Cloudflare)
2. X-Real-IP (set by Nginx/Caddy)
3. Rightmost non-private IP in X-Forwarded-For
4. RemoteAddr (direct connection)
```

### Cloudflare Integration

If using Cloudflare in front of the reverse proxy:

```
Internet → Cloudflare → Reverse Proxy (443) → PhishingClub (8443)
```

- Cloudflare provides an additional TLS fingerprint layer
- Use Cloudflare Origin Certificates for the reverse proxy
- Enable "Full (Strict)" SSL mode in Cloudflare
- The `CF-Connecting-IP` header will contain the real visitor IP
