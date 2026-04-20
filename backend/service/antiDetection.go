package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-errors/errors"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// AntiDetection provides email anti-detection capabilities:
// - Dirty word scanning (finds spam trigger words)
// - HTML mutation (evades fingerprinting)
// - Text encoding (defeats content-based scanning)
//
// Ported from: ghostsenderintegration/ghost-sender-node/services/anti-detection/
type AntiDetection struct {
	Common
}

// ─── Dirty Word Scanner ──────────────────────────────────────────────

// DirtyWordResult represents a found spam trigger word
type DirtyWordResult struct {
	Word     string `json:"word"`
	Category string `json:"category"`
	Severity string `json:"severity"` // "high", "medium", "low"
	Line     int    `json:"line,omitempty"`
}

// ScanResult is the result of scanning content for dirty words
type ScanResult struct {
	Clean   bool              `json:"clean"`
	Score   int               `json:"score"` // 0-100, higher = more spam-like
	Found   []DirtyWordResult `json:"found"`
	Summary string            `json:"summary"`
}

// dirtyWords maps categories to lists of spam trigger words with severity
var dirtyWords = map[string][]struct {
	word     string
	severity string
}{
	"urgency": {
		{"act now", "high"}, {"limited time", "high"}, {"urgent", "high"},
		{"expires", "medium"}, {"immediately", "medium"}, {"hurry", "medium"},
		{"don't delay", "high"}, {"last chance", "high"}, {"final notice", "high"},
		{"deadline", "medium"}, {"time sensitive", "high"}, {"respond immediately", "high"},
		{"action required", "medium"}, {"important notice", "medium"},
	},
	"financial": {
		{"free", "medium"}, {"winner", "high"}, {"prize", "high"},
		{"cash", "high"}, {"credit card", "high"}, {"investment", "medium"},
		{"no cost", "high"}, {"money back", "high"}, {"guarantee", "medium"},
		{"lowest price", "high"}, {"save big", "medium"}, {"discount", "low"},
		{"billion", "high"}, {"million dollars", "high"}, {"earn money", "high"},
		{"financial freedom", "high"}, {"double your", "high"},
	},
	"suspicious": {
		{"click here", "high"}, {"click below", "high"}, {"verify your account", "high"},
		{"confirm your identity", "high"}, {"update your information", "high"},
		{"suspended", "high"}, {"unauthorized", "high"}, {"unusual activity", "high"},
		{"security alert", "medium"}, {"password expired", "high"},
		{"login attempt", "medium"}, {"verify now", "high"},
		{"your account has been", "high"}, {"we noticed", "medium"},
	},
	"spam_phrases": {
		{"unsubscribe", "low"}, {"opt out", "low"}, {"bulk email", "high"},
		{"mass email", "high"}, {"dear friend", "high"}, {"dear customer", "medium"},
		{"no obligation", "high"}, {"risk free", "high"}, {"satisfaction guaranteed", "medium"},
		{"as seen on", "high"}, {"buy now", "high"}, {"order now", "high"},
		{"special promotion", "medium"}, {"exclusive deal", "medium"},
	},
	"technical": {
		{"viagra", "high"}, {"cialis", "high"}, {"pharmacy", "high"},
		{"enlargement", "high"}, {"weight loss", "high"}, {"diet", "medium"},
		{"miracle", "high"}, {"cure", "medium"},
	},
}

// ScanForDirtyWords scans email content for spam trigger words
func (a *AntiDetection) ScanForDirtyWords(
	ctx context.Context,
	session *model.Session,
	content string,
) (*ScanResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	result := &ScanResult{
		Clean: true,
		Found: make([]DirtyWordResult, 0),
	}

	// Strip HTML tags for text analysis
	plainText := stripHTMLTags(content)
	lowerText := strings.ToLower(plainText)
	lines := strings.Split(lowerText, "\n")

	for category, words := range dirtyWords {
		for _, w := range words {
			for lineNum, line := range lines {
				if strings.Contains(line, strings.ToLower(w.word)) {
					result.Found = append(result.Found, DirtyWordResult{
						Word:     w.word,
						Category: category,
						Severity: w.severity,
						Line:     lineNum + 1,
					})
					result.Clean = false
				}
			}
		}
	}

	// Calculate spam score
	score := 0
	for _, f := range result.Found {
		switch f.Severity {
		case "high":
			score += 15
		case "medium":
			score += 8
		case "low":
			score += 3
		}
	}
	if score > 100 {
		score = 100
	}
	result.Score = score

	if result.Clean {
		result.Summary = "No spam trigger words found. Content looks clean."
	} else {
		result.Summary = fmt.Sprintf("Found %d trigger word(s) with a spam score of %d/100. Consider rephrasing flagged content.", len(result.Found), score)
	}

	return result, nil
}

// ─── HTML Mutator ────────────────────────────────────────────────────

// MutationMethod defines the type of HTML mutation to apply
type MutationMethod int

const (
	MutationZeroWidth     MutationMethod = iota // Insert zero-width characters
	MutationHTMLEntities                        // Replace chars with HTML entities
	MutationInvisibleSpan                       // Wrap chars in invisible spans
	MutationCSSContent                          // Use CSS ::after pseudo-elements
	MutationCommentSplit                        // Insert HTML comments between chars
)

// MutateHTML applies anti-fingerprinting mutations to HTML content
func (a *AntiDetection) MutateHTML(
	ctx context.Context,
	session *model.Session,
	html string,
	method MutationMethod,
	intensity float64, // 0.0 to 1.0 - percentage of characters to mutate
) (string, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return "", errs.Wrap(err)
	}
	if !isAuthorized {
		return "", errs.ErrAuthorizationFailed
	}

	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	switch method {
	case MutationZeroWidth:
		return mutateZeroWidth(html, intensity), nil
	case MutationHTMLEntities:
		return mutateHTMLEntities(html, intensity), nil
	case MutationInvisibleSpan:
		return mutateInvisibleSpan(html, intensity), nil
	case MutationCSSContent:
		return mutateCSSContent(html, intensity), nil
	case MutationCommentSplit:
		return mutateCommentSplit(html, intensity), nil
	default:
		return html, nil
	}
}

// mutateZeroWidth inserts zero-width characters between text characters
func mutateZeroWidth(html string, intensity float64) string {
	zwChars := []string{
		"\u200B", // zero-width space
		"\u200C", // zero-width non-joiner
		"\u200D", // zero-width joiner
		"\uFEFF", // zero-width no-break space
	}

	return processHTMLText(html, func(text string) string {
		var result strings.Builder
		for _, r := range text {
			result.WriteRune(r)
			if rand.Float64() < intensity {
				result.WriteString(zwChars[rand.Intn(len(zwChars))])
			}
		}
		return result.String()
	})
}

// mutateHTMLEntities replaces characters with their HTML entity equivalents
func mutateHTMLEntities(html string, intensity float64) string {
	return processHTMLText(html, func(text string) string {
		var result strings.Builder
		for _, r := range text {
			if rand.Float64() < intensity && r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				result.WriteString(fmt.Sprintf("&#%d;", r))
			} else {
				result.WriteRune(r)
			}
		}
		return result.String()
	})
}

// mutateInvisibleSpan wraps characters in invisible spans with random content
func mutateInvisibleSpan(html string, intensity float64) string {
	return processHTMLText(html, func(text string) string {
		var result strings.Builder
		for _, r := range text {
			result.WriteRune(r)
			if rand.Float64() < intensity {
				// Insert invisible span with random text
				randChar := string(rune('a' + rand.Intn(26)))
				result.WriteString(fmt.Sprintf(`<span style="display:none;font-size:0;line-height:0;max-height:0;overflow:hidden;mso-hide:all;">%s</span>`, randChar))
			}
		}
		return result.String()
	})
}

// mutateCSSContent uses CSS ::after pseudo-elements with base64 content
func mutateCSSContent(html string, intensity float64) string {
	return processHTMLText(html, func(text string) string {
		var result strings.Builder
		for i, r := range text {
			if rand.Float64() < intensity {
				encoded := base64.StdEncoding.EncodeToString([]byte(string(r)))
				className := fmt.Sprintf("c%d", i)
				result.WriteString(fmt.Sprintf(`<style>.%s::after{content:attr(data-c)}</style><span class="%s" data-c="%s"></span>`, className, className, encoded))
			} else {
				result.WriteRune(r)
			}
		}
		return result.String()
	})
}

// mutateCommentSplit inserts HTML comments between characters
func mutateCommentSplit(html string, intensity float64) string {
	return processHTMLText(html, func(text string) string {
		var result strings.Builder
		for _, r := range text {
			result.WriteRune(r)
			if rand.Float64() < intensity {
				// Random comment content to avoid pattern detection
				commentLen := rand.Intn(8) + 1
				comment := make([]byte, commentLen)
				for i := range comment {
					comment[i] = byte('a' + rand.Intn(26))
				}
				result.WriteString(fmt.Sprintf("<!--%s-->", string(comment)))
			}
		}
		return result.String()
	})
}

// processHTMLText applies a function to text nodes only, preserving HTML tags
func processHTMLText(html string, fn func(string) string) string {
	// Simple regex-based approach: split on HTML tags, process text between them
	tagRegex := regexp.MustCompile(`(<[^>]+>)`)
	parts := tagRegex.Split(html, -1)
	tags := tagRegex.FindAllString(html, -1)

	var result strings.Builder
	for i, part := range parts {
		if part != "" {
			result.WriteString(fn(part))
		}
		if i < len(tags) {
			result.WriteString(tags[i])
		}
	}
	return result.String()
}

// ─── Text Encoder ────────────────────────────────────────────────────

// EncodingMethod defines the type of text encoding to apply
type EncodingMethod int

const (
	EncodingUnicodeSubstitution EncodingMethod = iota // Replace with look-alike Unicode chars
	EncodingQuotedPrintableFull                       // QP-encode every character
	EncodingSVGImageLetters                           // Replace letters with inline SVG images
)

// EncodeText applies anti-scanning encoding to text content
func (a *AntiDetection) EncodeText(
	ctx context.Context,
	session *model.Session,
	text string,
	method EncodingMethod,
) (string, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return "", errs.Wrap(err)
	}
	if !isAuthorized {
		return "", errs.ErrAuthorizationFailed
	}

	switch method {
	case EncodingUnicodeSubstitution:
		return unicodeSubstitution(text), nil
	case EncodingQuotedPrintableFull:
		return quotedPrintableFullEncode(text), nil
	case EncodingSVGImageLetters:
		return svgImageLetters(text), nil
	default:
		return text, nil
	}
}

// Unicode mathematical character substitutions (look-alike characters)
// Ported from: ghost-sender-node/services/anti-detection/text-encoder.js
var mathSubstitutions = map[rune]rune{
	'a': '\U0001D44E', 'b': '\U0001D44F', 'c': '\U0001D450', 'd': '\U0001D451',
	'e': '\U0001D452', 'f': '\U0001D453', 'g': '\U0001D454', 'h': '\u210E',
	'i': '\U0001D456', 'j': '\U0001D457', 'k': '\U0001D458', 'l': '\U0001D459',
	'm': '\U0001D45A', 'n': '\U0001D45B', 'o': '\U0001D45C', 'p': '\U0001D45D',
	'q': '\U0001D45E', 'r': '\U0001D45F', 's': '\U0001D460', 't': '\U0001D461',
	'u': '\U0001D462', 'v': '\U0001D463', 'w': '\U0001D464', 'x': '\U0001D465',
	'y': '\U0001D466', 'z': '\U0001D467',
	'A': '\U0001D434', 'B': '\U0001D435', 'C': '\U0001D436', 'D': '\U0001D437',
	'E': '\U0001D438', 'F': '\U0001D439', 'G': '\U0001D43A', 'H': '\U0001D43B',
	'I': '\U0001D43C', 'J': '\U0001D43D', 'K': '\U0001D43E', 'L': '\U0001D43F',
	'M': '\U0001D440', 'N': '\U0001D441', 'O': '\U0001D442', 'P': '\U0001D443',
	'Q': '\U0001D444', 'R': '\U0001D445', 'S': '\U0001D446', 'T': '\U0001D447',
	'U': '\U0001D448', 'V': '\U0001D449', 'W': '\U0001D44A', 'X': '\U0001D44B',
	'Y': '\U0001D44C', 'Z': '\U0001D44D',
	'0': '\U0001D7CE', '1': '\U0001D7CF', '2': '\U0001D7D0', '3': '\U0001D7D1',
	'4': '\U0001D7D2', '5': '\U0001D7D3', '6': '\U0001D7D4', '7': '\U0001D7D5',
	'8': '\U0001D7D6', '9': '\U0001D7D7',
}

func unicodeSubstitution(text string) string {
	var result strings.Builder
	for _, r := range text {
		if sub, ok := mathSubstitutions[r]; ok && rand.Float64() > 0.2 {
			result.WriteRune(sub)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func quotedPrintableFullEncode(text string) string {
	var result strings.Builder
	lineLen := 0
	for _, r := range text {
		if r == '\r' || r == '\n' {
			result.WriteRune(r)
			lineLen = 0
			continue
		}
		buf := make([]byte, utf8.UTFMax)
		n := utf8.EncodeRune(buf, r)
		encoded := ""
		for i := 0; i < n; i++ {
			encoded += fmt.Sprintf("=%02X", buf[i])
		}
		if lineLen+len(encoded) > 75 {
			result.WriteString("=\r\n")
			lineLen = 0
		}
		result.WriteString(encoded)
		lineLen += len(encoded)
	}
	return result.String()
}

func svgImageLetters(text string) string {
	var result strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			fontSize := 14
			width := 10
			if r == 'i' || r == 'l' || r == '1' {
				width = 8
			}
			svg := fmt.Sprintf(
				`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><text x="0" y="%d" font-family="Arial,sans-serif" font-size="%d" fill="currentColor">%c</text></svg>`,
				width, fontSize+2, fontSize, fontSize, r,
			)
			encoded := base64.StdEncoding.EncodeToString([]byte(svg))
			result.WriteString(fmt.Sprintf(
				`<img src="data:image/svg+xml;base64,%s" alt="" style="display:inline;vertical-align:baseline;height:%dpx;width:%dpx;border:0;">`,
				encoded, fontSize, width,
			))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// stripHTMLTags removes HTML tags from content for text analysis
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// ─── Available Methods ───────────────────────────────────────────────

// GetMutationMethods returns available HTML mutation methods
func (a *AntiDetection) GetMutationMethods() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": 0, "name": "Zero-Width Characters", "description": "Insert invisible zero-width Unicode characters between letters"},
		{"id": 1, "name": "HTML Entities", "description": "Replace characters with HTML numeric entity equivalents"},
		{"id": 2, "name": "Invisible Spans", "description": "Insert hidden span elements with random content"},
		{"id": 3, "name": "CSS Content", "description": "Use CSS ::after pseudo-elements with base64 data attributes"},
		{"id": 4, "name": "Comment Split", "description": "Insert HTML comments between characters to break pattern matching"},
	}
}

// GetEncodingMethods returns available text encoding methods
func (a *AntiDetection) GetEncodingMethods() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": 0, "name": "Unicode Substitution", "description": "Replace ASCII with visually identical Unicode mathematical characters"},
		{"id": 1, "name": "Quoted-Printable Full", "description": "QP-encode every character (requires Content-Transfer-Encoding: quoted-printable)"},
		{"id": 2, "name": "SVG Image Letters", "description": "Replace letters with tiny inline SVG images (HTML output)"},
	}
}
