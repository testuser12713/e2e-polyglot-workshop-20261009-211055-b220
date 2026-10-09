// Package auth issues and verifies the bearer tokens of the workshop area and
// seeds the first employee at startup.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

// SigningMethod is the only algorithm this API issues and accepts. Pinning it
// prevents the classic "alg: none" / algorithm-confusion substitution.
const SigningMethod = "HS256"

// Claims is the token payload: the employee id (sub), the employee e-mail and
// the expiry as a Unix timestamp. It deliberately carries no password or hash.
type Claims struct {
	Sub   int    `json:"sub"`
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
}

// Sign returns a signed HS256 JWT for the given claims.
func Sign(secret string, claims Claims) (string, error) {
	if secret == "" {
		return "", errors.New("auth: signing secret is empty")
	}
	if claims.Exp == 0 {
		return "", errors.New("auth: claims must carry an expiry")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   claims.Sub,
		"email": claims.Email,
		"exp":   claims.Exp,
	})
	return token.SignedString([]byte(secret))
}

// Verify parses and validates a token, accepting only HS256 and requiring a
// valid, unexpired exp claim.
func Verify(secret, tokenString string) (*Claims, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{SigningMethod}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("auth: invalid token: %w", err)
	}
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("auth: unexpected claims type")
	}

	claims := &Claims{}
	sub, err := numericSub(mapClaims["sub"])
	if err != nil {
		return nil, err
	}
	claims.Sub = sub
	if email, ok := mapClaims["email"].(string); ok {
		claims.Email = email
	}
	if exp, err := numericSub(mapClaims["exp"]); err == nil {
		claims.Exp = int64(exp)
	}
	return claims, nil
}

// numericSub normalises the numeric JSON values (float64 after decoding, or
// json.Number from a decoder) into an int.
func numericSub(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("auth: invalid sub claim: %w", err)
		}
		return int(i), nil
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0, fmt.Errorf("auth: invalid sub claim: %w", err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("auth: missing or invalid sub claim")
	}
}
