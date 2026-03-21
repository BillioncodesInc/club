package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"go.uber.org/zap"
)

// ContentBalancer rotates email content, subjects, and sender names per send
// to avoid fingerprinting by email security gateways. It distributes recipients
// across multiple content variants and sender configurations.
type ContentBalancer struct {
	Common
	Logger *zap.SugaredLogger
	mu     sync.Mutex
}

// ContentVariant represents a single variant of email content
type ContentVariant struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Weight  int    `json:"weight"` // Higher weight = more recipients get this variant
}

// SenderVariant represents a sender identity variant
type SenderVariant struct {
	ID          string `json:"id"`
	FromName    string `json:"fromName"`
	FromEmail   string `json:"fromEmail"`
	ReplyTo     string `json:"replyTo,omitempty"`
	Weight      int    `json:"weight"`
	RateLimit   int    `json:"rateLimit"`   // Max emails per hour
	DelayMs     int    `json:"delayMs"`     // Delay between sends in ms
}

// BalanceRequest is a request to balance recipients across variants
type BalanceRequest struct {
	Recipients      []BalanceRecipient `json:"recipients"`
	ContentVariants []ContentVariant   `json:"contentVariants"`
	SenderVariants  []SenderVariant    `json:"senderVariants,omitempty"`
	Strategy        string             `json:"strategy"` // "weighted", "round_robin", "by_domain", "random"
}

// BalanceRecipient is a recipient in a balance request
type BalanceRecipient struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// BalanceAssignment maps a recipient to a content variant and sender
type BalanceAssignment struct {
	Recipient      BalanceRecipient `json:"recipient"`
	ContentVariant ContentVariant   `json:"contentVariant"`
	SenderVariant  *SenderVariant   `json:"senderVariant,omitempty"`
	SendDelay      int              `json:"sendDelayMs"`
	BatchIndex     int              `json:"batchIndex"`
}

// BalanceResult is the result of a content balancing operation
type BalanceResult struct {
	Assignments     []BalanceAssignment    `json:"assignments"`
	TotalRecipients int                    `json:"totalRecipients"`
	VariantCounts   map[string]int         `json:"variantCounts"`
	SenderCounts    map[string]int         `json:"senderCounts,omitempty"`
	EstimatedTime   string                 `json:"estimatedTime"`
	Strategy        string                 `json:"strategy"`
}

// Balance distributes recipients across content and sender variants
func (cb *ContentBalancer) Balance(
	session *model.Session,
	req *BalanceRequest,
) (*BalanceResult, error) {
	isAuthorized, err := IsAuthorized(session, "campaign.create")
	if err != nil {
		cb.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	if len(req.ContentVariants) == 0 {
		return nil, fmt.Errorf("at least one content variant is required")
	}
	if len(req.Recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	var assignments []BalanceAssignment

	switch req.Strategy {
	case "by_domain":
		assignments = cb.balanceByDomain(req)
	case "round_robin":
		assignments = cb.balanceRoundRobin(req)
	case "random":
		assignments = cb.balanceRandom(req)
	default: // "weighted" is default
		assignments = cb.balanceWeighted(req)
	}

	// Calculate stats
	variantCounts := make(map[string]int)
	senderCounts := make(map[string]int)
	for _, a := range assignments {
		variantCounts[a.ContentVariant.ID]++
		if a.SenderVariant != nil {
			senderCounts[a.SenderVariant.ID]++
		}
	}

	return &BalanceResult{
		Assignments:     assignments,
		TotalRecipients: len(req.Recipients),
		VariantCounts:   variantCounts,
		SenderCounts:    senderCounts,
		EstimatedTime:   cb.estimateTime(assignments),
		Strategy:        req.Strategy,
	}, nil
}

// SpinContent applies text spinning to create unique variants from a template.
// Syntax: {option1|option2|option3} — randomly picks one option per send.
func (cb *ContentBalancer) SpinContent(template string) string {
	result := template
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		options := strings.Split(result[start+1:end], "|")
		if len(options) > 0 {
			idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(options))))
			chosen := options[idx.Int64()]
			result = result[:start] + chosen + result[end+1:]
		}
	}
	return result
}

// --- Balancing strategies ---

func (cb *ContentBalancer) balanceWeighted(req *BalanceRequest) []BalanceAssignment {
	totalWeight := 0
	for _, v := range req.ContentVariants {
		w := v.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	assignments := make([]BalanceAssignment, 0, len(req.Recipients))
	recipientIdx := 0
	batchIdx := 0

	for i, variant := range req.ContentVariants {
		w := variant.Weight
		if w <= 0 {
			w = 1
		}

		var count int
		if i == len(req.ContentVariants)-1 {
			count = len(req.Recipients) - recipientIdx
		} else {
			count = int(float64(w) / float64(totalWeight) * float64(len(req.Recipients)))
		}

		for j := 0; j < count && recipientIdx < len(req.Recipients); j++ {
			a := BalanceAssignment{
				Recipient:      req.Recipients[recipientIdx],
				ContentVariant: variant,
				BatchIndex:     batchIdx,
			}

			// Assign sender if available
			if len(req.SenderVariants) > 0 {
				senderIdx := recipientIdx % len(req.SenderVariants)
				sender := req.SenderVariants[senderIdx]
				a.SenderVariant = &sender
				a.SendDelay = sender.DelayMs
			}

			assignments = append(assignments, a)
			recipientIdx++
		}
		batchIdx++
	}

	return assignments
}

func (cb *ContentBalancer) balanceRoundRobin(req *BalanceRequest) []BalanceAssignment {
	assignments := make([]BalanceAssignment, 0, len(req.Recipients))

	for i, recipient := range req.Recipients {
		variantIdx := i % len(req.ContentVariants)
		a := BalanceAssignment{
			Recipient:      recipient,
			ContentVariant: req.ContentVariants[variantIdx],
			BatchIndex:     variantIdx,
		}
		if len(req.SenderVariants) > 0 {
			senderIdx := i % len(req.SenderVariants)
			sender := req.SenderVariants[senderIdx]
			a.SenderVariant = &sender
			a.SendDelay = sender.DelayMs
		}
		assignments = append(assignments, a)
	}

	return assignments
}

func (cb *ContentBalancer) balanceByDomain(req *BalanceRequest) []BalanceAssignment {
	// Group recipients by email domain
	byDomain := make(map[string][]int)
	for i, r := range req.Recipients {
		parts := strings.Split(r.Email, "@")
		domain := "unknown"
		if len(parts) == 2 {
			domain = parts[1]
		}
		byDomain[domain] = append(byDomain[domain], i)
	}

	assignments := make([]BalanceAssignment, len(req.Recipients))
	domainIdx := 0
	for _, indices := range byDomain {
		variantIdx := domainIdx % len(req.ContentVariants)
		for _, i := range indices {
			a := BalanceAssignment{
				Recipient:      req.Recipients[i],
				ContentVariant: req.ContentVariants[variantIdx],
				BatchIndex:     domainIdx,
			}
			if len(req.SenderVariants) > 0 {
				senderIdx := domainIdx % len(req.SenderVariants)
				sender := req.SenderVariants[senderIdx]
				a.SenderVariant = &sender
				a.SendDelay = sender.DelayMs
			}
			assignments[i] = a
		}
		domainIdx++
	}

	return assignments
}

func (cb *ContentBalancer) balanceRandom(req *BalanceRequest) []BalanceAssignment {
	assignments := make([]BalanceAssignment, 0, len(req.Recipients))

	for _, recipient := range req.Recipients {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(req.ContentVariants))))
		a := BalanceAssignment{
			Recipient:      recipient,
			ContentVariant: req.ContentVariants[idx.Int64()],
			BatchIndex:     int(idx.Int64()),
		}
		if len(req.SenderVariants) > 0 {
			sIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(req.SenderVariants))))
			sender := req.SenderVariants[sIdx.Int64()]
			a.SenderVariant = &sender
			a.SendDelay = sender.DelayMs
		}
		assignments = append(assignments, a)
	}

	return assignments
}

func (cb *ContentBalancer) estimateTime(assignments []BalanceAssignment) string {
	totalDelayMs := 0
	for _, a := range assignments {
		totalDelayMs += a.SendDelay
	}
	d := time.Duration(totalDelayMs) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
