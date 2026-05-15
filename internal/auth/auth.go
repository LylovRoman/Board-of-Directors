package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Login  string `json:"login"`
	Issued int64  `json:"iat"`
	Expiry int64  `json:"exp"`
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash string, password string) error {
	if hash == "" {
		return errors.New("password is not set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return errors.New("invalid login or password")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password cannot exceed %d characters", MaxPasswordLength)
	}
	return nil
}

func IssueToken(secret string, userID int64, login string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is required")
	}
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Login:  login,
		Issued: now.Unix(),
		Expiry: now.Add(ttl).Unix(),
	}

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + sign(secret, unsigned), nil
}

func ParseToken(secret string, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token")
	}

	unsigned := parts[0] + "." + parts[1]
	expected := sign(secret, unsigned)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid token")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token")
	}

	var claims Claims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, errors.New("invalid token")
	}
	if claims.UserID <= 0 || claims.Login == "" {
		return nil, errors.New("invalid token")
	}
	if claims.Expiry <= time.Now().UTC().Unix() {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}

func BearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("missing bearer token")
	}
	return parts[1], nil
}

func sign(secret string, unsigned string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func NormalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func ValidateLogin(login string) error {
	login = NormalizeLogin(login)
	if len(login) < 3 {
		return errors.New("login must be at least 3 characters")
	}
	if len(login) > 32 {
		return errors.New("login cannot exceed 32 characters")
	}
	for _, r := range login {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return errors.New("login can contain only letters, numbers, dots, dashes, and underscores")
	}
	if _, err := strconv.ParseInt(login, 10, 64); err == nil {
		return errors.New("login cannot be only digits")
	}
	return nil
}
