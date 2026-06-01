package counter

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	redisrepo "ceddit/repository/redis"
)

// rebuildLockKey returns the Redis key used to serialise SDS rebuilds for
// a single entity, preventing concurrent rebuilds from double-counting.
func rebuildLockKey(entityType string, entityID int64) string {
	return fmt.Sprintf("lock:sds-rebuild:%s:%d", entityType, entityID)
}

// rebuildLockTTL is how long a rebuild lock lives before auto-expiring.
// It must be longer than the worst-case rebuild duration.
const rebuildLockTTL = 5 * time.Second

// tryLock attempts to acquire a distributed lock via SET NX EX.
//
// Returns (token, true) on success; the caller MUST release the lock with
// unlockRebuild using the same token. Returns ("", false) if another
// process already holds the lock.
func tryLock(key string) (token string, ok bool) {
	rdb := redisrepo.GetRDB()

	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token = hex.EncodeToString(tokenBytes)

	success, err := rdb.SetNX(key, token, rebuildLockTTL).Result()
	if err != nil {
		return "", false
	}
	return token, success
}

// unlockRebuild releases a distributed lock using the "check token → delete"
// pattern inside a Lua script to avoid deleting another holder's lock.
const unlockLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end
`

// unlock releases the rebuild lock if (and only if) the caller holds it.
func unlock(key, token string) {
	rdb := redisrepo.GetRDB()
	_, _ = rdb.Eval(unlockLua, []string{key}, token).Result()
}
