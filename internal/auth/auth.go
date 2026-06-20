package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// VerifyTelegram checks the Telegram Login Widget hash.
func VerifyTelegram(data map[string]string, botToken string) bool {
	hash := data["hash"]
	if hash == "" {
		return false
	}
	var parts []string
	for k, v := range data {
		if k != "hash" {
			parts = append(parts, k+"="+v)
		}
	}
	sort.Strings(parts)
	checkStr := strings.Join(parts, "\n")

	tokenHash := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, tokenHash[:])
	mac.Write([]byte(checkStr))
	return hex.EncodeToString(mac.Sum(nil)) == hash
}

// MakeJWT creates a signed HS256 token: {sub, exp}.
func MakeJWT(userID int64, secret string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]any{
		"sub": userID,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	enc := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(header+"."+enc, secret)
	return header + "." + enc + "." + sig, nil
}

// ParseJWT returns userID or error.
func ParseJWT(token, secret string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid token")
	}
	if sign(parts[0]+"."+parts[1], secret) != parts[2] {
		return 0, fmt.Errorf("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, err
	}
	if exp, ok := claims["exp"].(float64); ok && time.Now().Unix() > int64(exp) {
		return 0, fmt.Errorf("token expired")
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid sub")
	}
	return int64(sub), nil
}

func sign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
