package api

import (
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

const (
	defaultJWKSCacheTTL = 15 * time.Minute
	envAuth0TestMode    = "AUTH0_TEST_MODE"
	envTestJWTSecret    = "TEST_JWT_SECRET"
	envLocalAuthMode    = "LOCAL_AUTH_MODE"
	envLocalAuthSecret  = "LOCAL_AUTH_SHARED_SECRET"
	envJWKSCacheTTL     = "JWKS_CACHE_TTL"
)

// Auth validates incoming JWT tokens.
type Auth struct {
	JWKS       *keyfunc.JWKS
	Audience   string
	Issuer     string
	TestMode   bool
	TestSecret []byte

	parser      *jwt.Parser
	keyCache    sync.Map
	keyCacheTTL time.Duration
}

type cachedKey struct {
	key       any
	expiresAt time.Time
}

// NewAuth creates a new Auth instance.
func NewAuth(jwks *keyfunc.JWKS, audience, issuer string) *Auth {
	a := &Auth{JWKS: jwks, Audience: audience, Issuer: issuer}
	a.keyCacheTTL = parseCacheTTL()

	if mode := strings.ToLower(os.Getenv(envLocalAuthMode)); mode != "" {
		switch mode {
		case "hs256":
			secret := os.Getenv(envLocalAuthSecret)
			if secret == "" {
				panic("LOCAL_AUTH_SHARED_SECRET must be set when LOCAL_AUTH_MODE=hs256")
			}
			a.TestMode = true
			a.TestSecret = []byte(secret)
		default:
			panic("unsupported LOCAL_AUTH_MODE value")
		}
	} else if os.Getenv(envAuth0TestMode) == "1" {
		secret := os.Getenv(envTestJWTSecret)
		if secret == "" {
			panic("TEST_JWT_SECRET must be set when AUTH0_TEST_MODE=1")
		}
		a.TestMode = true
		a.TestSecret = []byte(secret)
	}

	if a.TestMode {
		a.parser = jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	} else {
		a.parser = jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	}
	return a
}

func parseCacheTTL() time.Duration {
	ttl := defaultJWKSCacheTTL
	if raw := os.Getenv(envJWKSCacheTTL); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			panic("invalid JWKS_CACHE_TTL")
		}
		ttl = parsed
	}
	return ttl
}

// UserIDFromAuthHeader extracts the user identifier from the Authorization header.
func (a *Auth) UserIDFromAuthHeader(h string) (string, error) {
	if h == "" {
		return "", errMissingAuthorization
	}
	token, err := bearerTokenFromString(h)
	if err != nil {
		return "", err
	}
	return a.UserIDFromBearer(token)
}

// UserIDFromBearer extracts the user identifier from a bearer token presented as raw bytes.
func (a *Auth) UserIDFromBearer(token []byte) (string, error) {
	if len(token) == 0 {
		return "", errBadAuthorization
	}

	tokenStr := readOnlyString(token)
	var parsedToken *jwt.Token
	var err error
	if a.TestMode {
		parsedToken, err = a.parser.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return a.TestSecret, nil
		})
	} else {
		parsedToken, err = a.parser.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return a.keyForToken(t)
		})
	}
	if err != nil {
		return "", err
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	now := time.Now().Add(time.Minute).Unix()
	if !claims.VerifyExpiresAt(now, true) {
		return "", errors.New("token expired")
	}
	if !claims.VerifyNotBefore(now, false) {
		return "", errors.New("token not valid yet")
	}
	if !claims.VerifyIssuedAt(now, false) {
		return "", errors.New("token used before issued")
	}
	if a.Audience != "" && !claims.VerifyAudience(a.Audience, false) {
		return "", errors.New("invalid audience")
	}
	if a.Issuer != "" && !claims.VerifyIssuer(a.Issuer, false) {
		return "", errors.New("invalid issuer")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing sub")
	}

	return sub, nil
}

func (a *Auth) keyForToken(token *jwt.Token) (any, error) {
	if a.JWKS == nil {
		return nil, errors.New("jwks not configured")
	}

	kid, _ := token.Header["kid"].(string)
	if kid != "" && a.keyCacheTTL > 0 {
		if cached, ok := a.keyCache.Load(kid); ok {
			entry := cached.(cachedKey)
			if time.Now().Before(entry.expiresAt) {
				return entry.key, nil
			}
			a.keyCache.Delete(kid)
		}
	}

	key, err := a.JWKS.Keyfunc(token)
	if err != nil {
		return nil, err
	}

	if kid != "" && a.keyCacheTTL > 0 {
		a.keyCache.Store(kid, cachedKey{key: key, expiresAt: time.Now().Add(a.keyCacheTTL)})
	}
	return key, nil
}
