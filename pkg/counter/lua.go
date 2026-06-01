package counter

import (
	"fmt"

	redisrepo "ceddit/repository/redis"

	"github.com/go-redis/redis"
)

// flushLua atomically reads a delta from a Redis Hash field, applies it to a
// CountInt SDS blob, and deletes the field — all inside a single Redis
// transaction (Lua scripts are atomic).
//
// KEYS[1] — aggregation bucket key (Hash); e.g. agg:v1:post:456:2024100114
// KEYS[2] — CountInt SDS key (String);       e.g. pcnt:456
// ARGV[1] — hash field name (same as byte offset, as string)
// ARGV[2] — SDS byte offset (integer)
// ARGV[3] — total SDS blob size in bytes
// ARGV[4] — active aggregation set key (for cleanup when bucket empties)
//
// Returns the new counter value after applying the delta.
const flushLua = `
local agg_key   = KEYS[1]
local sds_key   = KEYS[2]
local field     = ARGV[1]
local off       = tonumber(ARGV[2])
local size      = tonumber(ARGV[3])
local active_set = ARGV[4]

-- Read delta from aggregation bucket
local delta_str = redis.call('HGET', agg_key, field)
if not delta_str then
    return redis.error_reply('field not found')
end
local delta = tonumber(delta_str)
if delta == 0 then
    -- Nothing to do; clean up the zero field anyway
    redis.call('HDEL', agg_key, field)
    return 0
end

-- Read or initialise SDS blob
local val = redis.call('GET', sds_key)
if not val then
    val = string.rep(string.char(0), size)
end

-- Decode current int64 at offset (little-endian)
local b = {}
for i = 0, 7 do
    b[i+1] = string.byte(val, off + i + 1) or 0
end
local cur = b[1] + b[2]*256 + b[3]*65536 + b[4]*16777216 +
            b[5]*4294967296 + b[6]*1099511627776 +
            b[7]*281474976710656 + b[8]*72057594037927936

-- Apply delta, floor at 0
local nv = cur + delta
if nv < 0 then nv = 0 end

-- Write back (little-endian)
local out = {}
for i = 0, 7 do
    out[i+1] = string.char(math.floor(nv / (256 ^ i)) % 256)
end

local head = string.sub(val, 1, off)
local tail = string.sub(val, off + 9)  -- 9 = 1 (lua 1-index) + 8 bytes
local new_val = head .. table.concat(out) .. tail

redis.call('SET', sds_key, new_val)

-- Delete the processed field from the aggregation bucket
redis.call('HDEL', agg_key, field)

-- If the bucket is now empty, remove it from the active set and delete the key
if redis.call('HLEN', agg_key) == 0 then
    redis.call('SREM', active_set, agg_key)
    redis.call('DEL', agg_key)
end

return nv
`

// FlushScript is a Redis Lua script that atomically moves an accumulated delta
// from an aggregation bucket into the target CountInt SDS key.
//
// Use Run(...).Result() to execute it.
var FlushScript = redis.NewScript(flushLua)

// InitCounterScripts pre-loads the counter Lua scripts into Redis so that
// subsequent calls can use EVALSHA instead of sending the full script body.
func InitCounterScripts() error {
	rdb := redisrepo.GetRDB()
	if err := FlushScript.Load(rdb).Err(); err != nil {
		return fmt.Errorf("load counter flush script: %w", err)
	}
	return nil
}
