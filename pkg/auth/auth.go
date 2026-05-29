package auth

import (
	"ceddit/pkg/jwt"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	redisclient "github.com/go-redis/redis"
	"github.com/spf13/viper"
)

const refreshCookieName = "refresh_token"

var (
	ErrTokenLeaked   = errors.New("refresh token has been compromised, please re-login")
	ErrTokenNotFound = errors.New("refresh token not found")
)

// TokenPair is returned on login/signup/refresh.
type TokenPair struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// RefreshValue is the JSON value stored at refresh:<token>.
type RefreshValue struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	DeviceID  string `json:"device_id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// CreateTokens generates a new token pair for a user login or signup.
// Returns the token pair and the raw refresh token string (for Set-Cookie).
func CreateTokens(userID int64, username, deviceID string) (*TokenPair, string, error) {
	if deviceID == "" {
		deviceID = generateDeviceID()
	}

	accessToken, err := jwt.GenToken(userID, username)
	if err != nil {
		return nil, "", err
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().Unix()
	ttlDays := viper.GetInt("jwt.refresh_token_ttl")
	ttlSeconds := int64(ttlDays) * 24 * 3600

	val := RefreshValue{
		UserID:    userID,
		Username:  username,
		DeviceID:  deviceID,
		CreatedAt: now,
		ExpiresAt: now + ttlSeconds,
	}
	valJSON, err := json.Marshal(val)
	if err != nil {
		return nil, "", err
	}

	if err := runLuaCreateTokens(refreshToken, string(valJSON), userID, ttlSeconds); err != nil {
		return nil, "", err
	}

	accessTTL := viper.GetInt("jwt.access_token_ttl")
	return &TokenPair{
		AccessToken: accessToken,
		ExpiresIn:   accessTTL * 60,
	}, refreshToken, nil
}

// RefreshTokens validates the old refresh token and returns a new pair (rotation).
// If the old token does not exist in Redis, it is considered leaked and
// ErrTokenLeaked is returned.
func RefreshTokens(oldRefreshToken string) (*TokenPair, string, error) {
	// 1. Look up the old token to get user info.
	oldVal, err := getRefreshValue(oldRefreshToken)
	if err == redisclient.Nil {
		// Token key doesn't exist at all — potentially leaked.
		return nil, "", ErrTokenLeaked
	}
	if err != nil {
		return nil, "", err
	}

	// 2. Generate new refresh token and access token.
	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, "", err
	}

	accessToken, err := jwt.GenToken(oldVal.UserID, oldVal.Username)
	if err != nil {
		return nil, "", err
	}

	now := time.Now().Unix()
	ttlDays := viper.GetInt("jwt.refresh_token_ttl")
	ttlSeconds := int64(ttlDays) * 24 * 3600

	newVal := RefreshValue{
		UserID:    oldVal.UserID,
		Username:  oldVal.Username,
		DeviceID:  oldVal.DeviceID,
		CreatedAt: now,
		ExpiresAt: now + ttlSeconds,
	}
	newValJSON, err := json.Marshal(newVal)
	if err != nil {
		return nil, "", err
	}

	// 3. Atomic rotation via Lua.
	if err := runLuaRefreshTokens(oldRefreshToken, newRefreshToken, string(newValJSON), oldVal.UserID, ttlSeconds); err != nil {
		return nil, "", err
	}

	accessTTL := viper.GetInt("jwt.access_token_ttl")
	return &TokenPair{
		AccessToken: accessToken,
		ExpiresIn:   accessTTL * 60,
	}, newRefreshToken, nil
}

// RevokeAllTokens removes all refresh tokens for a user (logout all devices).
func RevokeAllTokens(userID int64) error {
	_, err := runLuaRevokeAll(userID)
	return err
}

// GetRefreshValue looks up a refresh token in Redis and returns its value.
func GetRefreshValue(token string) (*RefreshValue, error) {
	return getRefreshValue(token)
}

// SetRefreshCookie sets the httpOnly cookie containing the refresh token.
func SetRefreshCookie(c *gin.Context, refreshToken string) {
	ttlDays := viper.GetInt("jwt.refresh_token_ttl")
	c.SetCookie(
		refreshCookieName,
		refreshToken,
		ttlDays*24*3600,
		"/api/v1",
		"",
		false, // Secure=false for local dev without HTTPS; use true in production
		true,  // httpOnly
	)
}

// ClearRefreshCookie removes the refresh token cookie.
func ClearRefreshCookie(c *gin.Context) {
	c.SetCookie(
		refreshCookieName,
		"",
		-1,
		"/api/v1",
		"",
		false,
		true,
	)
}

// GetRefreshTokenFromCookie extracts the refresh token from the request cookie.
func GetRefreshTokenFromCookie(c *gin.Context) (string, error) {
	return c.Cookie(refreshCookieName)
}

// --- internal helpers ---

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateDeviceID() string {
	b := make([]byte, 16)
	rand.Read(b) // errors are impossible at OS level for /dev/urandom
	return base64.RawURLEncoding.EncodeToString(b)
}

func getRefreshValue(token string) (*RefreshValue, error) {
	data, err := getRDB().Get(refreshKey(token)).Result()
	if err != nil {
		return nil, err
	}
	var v RefreshValue
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		return nil, err
	}
	return &v, nil
}
