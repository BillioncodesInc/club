package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// DKIMConfig holds configuration for a single DKIM domain
type DKIMConfig struct {
	Domain           string `json:"domain"`
	Selector         string `json:"selector"`
	PrivateKeyPEM    string `json:"privateKeyPem"`
	Algorithm        string `json:"algorithm"`        // rsa-sha256
	Canonicalization string `json:"canonicalization"` // relaxed/relaxed
	HeaderFields     string `json:"headerFields"`     // from:to:subject:date:message-id
}

// DKIMKeyPair holds a generated DKIM key pair and DNS record
type DKIMKeyPair struct {
	Domain       string `json:"domain"`
	Selector     string `json:"selector"`
	PrivateKey   string `json:"privateKey"`
	PublicKey    string `json:"publicKey"`
	DNSName      string `json:"dnsName"`
	DNSType      string `json:"dnsType"`
	DNSValue     string `json:"dnsValue"`
}

// DKIMVerifyResult holds the result of a DKIM verification
type DKIMVerifyResult struct {
	Valid     bool   `json:"valid"`
	Domain    string `json:"domain"`
	Selector  string `json:"selector"`
	Algorithm string `json:"algorithm"`
	Error     string `json:"error,omitempty"`
}

// DKIM provides DKIM signing, verification, and key management
type DKIM struct {
	Common
}

// GenerateKeyPair generates a new RSA-2048 DKIM key pair for a domain
func (d *DKIM) GenerateKeyPair(domain, selector string) (*DKIMKeyPair, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if selector == "" {
		selector = "default"
	}

	// Generate RSA-2048 key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key to PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key to PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Generate DNS TXT record value
	pubKeyBase64 := base64.StdEncoding.EncodeToString(publicKeyBytes)

	dnsName := fmt.Sprintf("%s._domainkey.%s", selector, domain)
	dnsValue := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubKeyBase64)

	d.Logger.Infow("generated DKIM key pair",
		"domain", domain,
		"selector", selector,
		"dnsName", dnsName,
	)

	return &DKIMKeyPair{
		Domain:     domain,
		Selector:   selector,
		PrivateKey: string(privateKeyPEM),
		PublicKey:  string(publicKeyPEM),
		DNSName:    dnsName,
		DNSType:    "TXT",
		DNSValue:   dnsValue,
	}, nil
}

// SignEmail generates a DKIM-Signature header for an email
func (d *DKIM) SignEmail(config *DKIMConfig, headers map[string]string, body string) (string, error) {
	if config.PrivateKeyPEM == "" {
		return "", fmt.Errorf("private key is required")
	}
	if config.Domain == "" {
		return "", fmt.Errorf("domain is required")
	}

	// Parse private key
	block, _ := pem.Decode([]byte(config.PrivateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	selector := config.Selector
	if selector == "" {
		selector = "default"
	}

	canonicalization := config.Canonicalization
	if canonicalization == "" {
		canonicalization = "relaxed/relaxed"
	}

	headerFields := config.HeaderFields
	if headerFields == "" {
		headerFields = "from:to:subject:date:message-id:mime-version:content-type"
	}

	// Canonicalize body (relaxed)
	canonBody := canonicalizeBodyRelaxed(body)
	bodyHash := sha256.Sum256([]byte(canonBody))
	bodyHashB64 := base64.StdEncoding.EncodeToString(bodyHash[:])

	// Build DKIM-Signature header (without b= value)
	timestamp := time.Now().Unix()
	dkimHeader := fmt.Sprintf(
		"v=1; a=rsa-sha256; c=%s; d=%s; s=%s; t=%d; h=%s; bh=%s; b=",
		canonicalization,
		config.Domain,
		selector,
		timestamp,
		headerFields,
		bodyHashB64,
	)

	// Canonicalize headers (relaxed)
	var headerData strings.Builder
	for _, field := range strings.Split(headerFields, ":") {
		field = strings.TrimSpace(field)
		if val, ok := headers[field]; ok {
			headerData.WriteString(canonicalizeHeaderRelaxed(field, val))
			headerData.WriteString("\r\n")
		} else {
			// Try case-insensitive lookup
			for k, v := range headers {
				if strings.EqualFold(k, field) {
					headerData.WriteString(canonicalizeHeaderRelaxed(field, v))
					headerData.WriteString("\r\n")
					break
				}
			}
		}
	}
	// Add DKIM-Signature header itself (without trailing CRLF)
	headerData.WriteString(canonicalizeHeaderRelaxed("dkim-signature", dkimHeader))

	// Sign
	hash := sha256.Sum256([]byte(headerData.String()))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Build final DKIM-Signature header
	finalHeader := fmt.Sprintf("DKIM-Signature: %s%s", dkimHeader, signatureB64)

	d.Logger.Infow("signed email with DKIM",
		"domain", config.Domain,
		"selector", selector,
	)

	return finalHeader, nil
}

// VerifyDKIMHeader performs a basic parse of a DKIM-Signature header
func (d *DKIM) VerifyDKIMHeader(rawHeader string) *DKIMVerifyResult {
	result := &DKIMVerifyResult{}

	// Find DKIM-Signature header
	idx := strings.Index(rawHeader, "DKIM-Signature:")
	if idx == -1 {
		result.Error = "no DKIM-Signature header found"
		return result
	}

	sigLine := rawHeader[idx+len("DKIM-Signature:"):]
	// Parse tag=value pairs
	tags := parseDKIMTags(sigLine)

	result.Domain = tags["d"]
	result.Selector = tags["s"]
	result.Algorithm = tags["a"]
	result.Valid = result.Domain != "" && result.Selector != "" && tags["b"] != ""

	if !result.Valid && result.Error == "" {
		result.Error = "incomplete DKIM-Signature: missing required tags"
	}

	return result
}

// GenerateDNSRecord generates the DNS TXT record for an existing key pair
func (d *DKIM) GenerateDNSRecord(domain, selector, publicKeyPEM string) (string, string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", "", fmt.Errorf("failed to decode public key PEM")
	}

	pubKeyBase64 := base64.StdEncoding.EncodeToString(block.Bytes)
	dnsName := fmt.Sprintf("%s._domainkey.%s", selector, domain)
	dnsValue := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubKeyBase64)

	return dnsName, dnsValue, nil
}

// canonicalizeBodyRelaxed applies relaxed body canonicalization per RFC 6376
func canonicalizeBodyRelaxed(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		// Reduce sequences of WSP to single SP
		var prev rune
		var cleaned strings.Builder
		for _, r := range line {
			if r == ' ' || r == '\t' {
				if prev != ' ' {
					cleaned.WriteRune(' ')
				}
				prev = ' '
			} else {
				cleaned.WriteRune(r)
				prev = r
			}
		}
		// Remove trailing whitespace
		result = append(result, strings.TrimRight(cleaned.String(), " "))
	}
	// Remove trailing empty lines
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\r\n") + "\r\n"
}

// canonicalizeHeaderRelaxed applies relaxed header canonicalization per RFC 6376
func canonicalizeHeaderRelaxed(name, value string) string {
	// Lowercase header name
	name = strings.ToLower(strings.TrimSpace(name))
	// Unfold header value and reduce WSP
	value = strings.ReplaceAll(value, "\r\n", "")
	value = strings.ReplaceAll(value, "\n", "")
	var prev rune
	var cleaned strings.Builder
	for _, r := range value {
		if r == ' ' || r == '\t' {
			if prev != ' ' {
				cleaned.WriteRune(' ')
			}
			prev = ' '
		} else {
			cleaned.WriteRune(r)
			prev = r
		}
	}
	return name + ":" + strings.TrimSpace(cleaned.String())
}

// parseDKIMTags parses tag=value pairs from a DKIM-Signature header
func parseDKIMTags(sig string) map[string]string {
	tags := make(map[string]string)
	// Remove whitespace and split by semicolons
	sig = strings.ReplaceAll(sig, "\r\n", "")
	sig = strings.ReplaceAll(sig, "\n", "")
	parts := strings.Split(sig, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx > 0 {
			key := strings.TrimSpace(part[:eqIdx])
			value := strings.TrimSpace(part[eqIdx+1:])
			tags[key] = value
		}
	}
	return tags
}
