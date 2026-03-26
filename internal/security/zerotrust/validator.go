package zerotrust

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Identity struct {
	ServiceName string    `json:"service_name"`
	Token       string    `json:"token"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Policy struct {
	AllowedServices []string
	AllowedPaths    []string
	RequireToken    bool
}

type Validator struct {
	mu       sync.RWMutex
	secret   string
	policies map[string]*Policy
	tokens   map[string]*Identity
}

func New(secret string) *Validator {
	return &Validator{
		secret:   secret,
		policies: make(map[string]*Policy),
		tokens:   make(map[string]*Identity),
	}
}

func (v *Validator) IssueToken(serviceName string, ttl time.Duration) *Identity {
	now := time.Now()
	nonce := make([]byte, 16)
	rand.Read(nonce)
	data := fmt.Sprintf("%s:%d:%s", serviceName, now.UnixNano(), hex.EncodeToString(nonce))
	mac := hmac.New(sha256.New, []byte(v.secret))
	mac.Write([]byte(data))
	token := hex.EncodeToString(mac.Sum(nil))

	identity := &Identity{
		ServiceName: serviceName,
		Token:       token,
		IssuedAt:    now,
		ExpiresAt:   now.Add(ttl),
	}

	v.mu.Lock()
	v.tokens[token] = identity
	v.mu.Unlock()

	return identity
}

func (v *Validator) ValidateToken(token string) (*Identity, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	identity, exists := v.tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}
	if time.Now().After(identity.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}
	return identity, nil
}

func (v *Validator) SetPolicy(serviceName string, policy *Policy) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.policies[serviceName] = policy
}

func (v *Validator) CheckAccess(serviceName string, targetService string, path string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	policy, exists := v.policies[targetService]
	if !exists {
		return fmt.Errorf("no policy defined for service '%s': access denied by default", targetService)
	}

	// Check if service is in allowed list
	allowed := false
	for _, svc := range policy.AllowedServices {
		if svc == serviceName || svc == "*" {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("service '%s' is not allowed to access '%s'", serviceName, targetService)
	}

	// Check path restriction
	if len(policy.AllowedPaths) > 0 {
		pathAllowed := false
		for _, p := range policy.AllowedPaths {
			if p == path || p == "*" {
				pathAllowed = true
				break
			}
		}
		if !pathAllowed {
			return fmt.Errorf("path '%s' is not allowed for service '%s'", path, serviceName)
		}
	}

	return nil
}

func (v *Validator) RevokeToken(token string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.tokens, token)
}

func (v *Validator) ActiveTokenCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	count := 0
	now := time.Now()
	for _, id := range v.tokens {
		if now.Before(id.ExpiresAt) {
			count++
		}
	}
	return count
}
