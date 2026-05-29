package auth

import (
	"strconv"

	redisclient "github.com/go-redis/redis"

	cedditredis "ceddit/repository/redis"
)

// getRDB is a lazy accessor — main.go must call redis.Init() before any auth function.
func getRDB() *redisclient.Client {
	return cedditredis.GetRDB()
}

func refreshKey(token string) string {
	return prefixRefresh + token
}

func userTokenKey(userID int64) string {
	return prefixUserToken + formatUserID(userID)
}

func formatUserID(userID int64) string {
	return strconv.FormatInt(userID, 16)
}

func runLuaCreateTokens(tokenStr, tokenValue string, userID int64, ttlSeconds int64) error {
	keys := []string{refreshKey(tokenStr), userTokenKey(userID)}
	args := []any{tokenStr, tokenValue, ttlSeconds}
	return getRDB().Eval(luaCreateTokens, keys, args...).Err()
}

func runLuaRefreshTokens(oldToken, newToken, newValue string, userID int64, ttlSeconds int64) error {
	keys := []string{refreshKey(oldToken), refreshKey(newToken), userTokenKey(userID)}
	args := []any{oldToken, newToken, newValue, ttlSeconds}
	result, err := getRDB().Eval(luaRefreshTokens, keys, args...).Int()
	if err != nil {
		return err
	}
	if result == -1 {
		return ErrTokenLeaked
	}
	return nil
}

func runLuaRevokeAll(userID int64) (int, error) {
	keys := []string{userTokenKey(userID)}
	args := []any{prefixRefresh}
	count, err := getRDB().Eval(luaRevokeAll, keys, args...).Int()
	if err != nil {
		return 0, err
	}
	return count, nil
}
