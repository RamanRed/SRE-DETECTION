package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

var whitespace = regexp.MustCompile(`\s+`)

const (
	RoleSRELead        = "SRE_LEAD"
	RoleDevOpsEngineer = "DEVOPS_ENGINEER"
	RoleEvaluator      = "EVALUATOR"
)

type AuthResponse struct {
	Authenticated bool
	Token         string
	UserID        string
	Username      string
	Email         string
	Role          string
	AvatarURL     string
	Message       string
}

type AuthClaims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	Expiry int64  `json:"exp"`
}

type AuthService struct {
	secret            []byte
	bootstrapPassword string
	bootstrapRole     string
	demoMode          bool
	clock             Clock
	ttl               time.Duration
}

// NewAuthService retains explicit local-demo behavior for compatibility tests.
// Even in demo mode, issued sessions are signed and expiry checked.
func NewAuthService(_ IDGenerator) *AuthService {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("generate demo authentication key: " + err.Error())
	}
	return &AuthService{secret: key, demoMode: true, clock: SystemClock, ttl: 8 * time.Hour}
}

func NewConfiguredAuthService(sessionSecret, bootstrapPassword, bootstrapRole string, demoMode bool, clock Clock, ttl time.Duration) (*AuthService, error) {
	if clock == nil {
		clock = SystemClock
	}
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	key := []byte(sessionSecret)
	if !demoMode && len(key) < 32 {
		return nil, errors.New("AUTH_SESSION_SECRET must be at least 32 bytes when DEMO_MODE=false")
	}
	if !demoMode && len(bootstrapPassword) < 12 {
		return nil, errors.New("AUTH_BOOTSTRAP_PASSWORD must be at least 12 bytes when DEMO_MODE=false")
	}
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(bootstrapRole) == "" {
		bootstrapRole = "SRE_LEAD"
	}
	return &AuthService{secret: key, bootstrapPassword: bootstrapPassword, bootstrapRole: strings.ToUpper(bootstrapRole), demoMode: demoMode, clock: clock, ttl: ttl}, nil
}

func (s *AuthService) Login(username, password, role *string) (AuthResponse, error) {
	if !s.demoMode {
		provided := ""
		if password != nil {
			provided = *password
		}
		if len(provided) != len(s.bootstrapPassword) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.bootstrapPassword)) != 1 {
			return AuthResponse{}, errorWith(CodeInvalid, "Invalid username or password", nil)
		}
	}
	resolvedUsername := "RamanRed"
	if username != nil && strings.TrimSpace(*username) != "" {
		resolvedUsername = strings.TrimSpace(*username)
	}
	resolvedRole := RoleSRELead
	if role != nil && strings.TrimSpace(*role) != "" {
		resolvedRole = strings.ToUpper(strings.TrimSpace(*role))
	}
	if !s.demoMode {
		resolvedRole = s.bootstrapRole
	}
	lowerUsername := strings.ToLower(resolvedUsername)
	userID := whitespace.ReplaceAllString(lowerUsername, "-")
	token, err := s.sign(AuthClaims{UserID: userID, Role: resolvedRole, Expiry: s.clock().Add(s.ttl).Unix()})
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{
		Authenticated: true, Token: token, UserID: userID, Username: resolvedUsername,
		Email: lowerUsername + "@devops.sre.io", Role: resolvedRole,
		AvatarURL: "https://github.com/" + resolvedUsername + ".png", Message: "Authentication successful",
	}, nil
}

func (s *AuthService) CurrentUser(claims AuthClaims) AuthResponse {
	username := claims.UserID
	if strings.EqualFold(username, "ramanred") {
		username = "RamanRed"
	}
	return AuthResponse{
		Authenticated: true, UserID: claims.UserID, Username: username,
		Email: strings.ToLower(claims.UserID) + "@devops.sre.io", Role: claims.Role,
		AvatarURL: "https://github.com/" + username + ".png", Message: "Session active",
	}
}

func (s *AuthService) VerifyToken(token string) (AuthClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return AuthClaims{}, errors.New("session token is malformed")
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthClaims{}, errors.New("session token signature is malformed")
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return AuthClaims{}, errors.New("session token signature is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AuthClaims{}, errors.New("session token payload is malformed")
	}
	var claims AuthClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.UserID == "" || claims.Role == "" {
		return AuthClaims{}, errors.New("session token claims are invalid")
	}
	if s.clock().Unix() >= claims.Expiry {
		return AuthClaims{}, errors.New("session token has expired")
	}
	return claims, nil
}

func (s *AuthService) sign(claims AuthClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
