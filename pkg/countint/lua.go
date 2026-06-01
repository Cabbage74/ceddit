package countint

import "github.com/go-redis/redis"

// countIncrByLua atomically increments the int64 at the given byte offset within
// a Redis String value. If the key does not exist, it is initialised with a
// zero-filled blob of the configured size.
//
// KEYS[1] — count key
// ARGV[1] — byte offset (0-based)
// ARGV[2] — delta (signed integer)
// ARGV[3] — total blob size in bytes (for initialisation)
const countIncrByLua = `
local key   = KEYS[1]
local off   = tonumber(ARGV[1])
local delta = tonumber(ARGV[2])
local size  = tonumber(ARGV[3])

-- fetch or initialise
local val = redis.call('GET', key)
if not val then
    val = string.rep(string.char(0), size)
end

-- read the 8-byte little-endian int64 at offset
-- (Lua tables are 1-indexed, so we store bytes at indices 1..8)
local b = {}
for i = 0, 7 do
    b[i+1] = string.byte(val, off + i + 1) or 0
end
local cur = b[1] + b[2]*256 + b[3]*65536 + b[4]*16777216 +
            b[5]*4294967296 + b[6]*1099511627776 +
            b[7]*281474976710656 + b[8]*72057594037927936

-- apply delta, floor at 0
local nv = cur + delta
if nv < 0 then nv = 0 end

-- write back (little-endian, 1-indexed for table.concat compatibility)
local out = {}
for i = 0, 7 do
    out[i+1] = string.char(math.floor(nv / (256 ^ i)) % 256)
end

local head = string.sub(val, 1, off)
local tail = string.sub(val, off + 9)  -- 9 = 1 (lua 1-index) + 8 (bytes past offset)
local new_val = head .. table.concat(out) .. tail

redis.call('SET', key, new_val)
return nv
`

// CountIncrByScript is a Redis Lua script that atomically increments a field
// inside a CountInt blob. Use Run(...).Result() to execute it.
var CountIncrByScript = redis.NewScript(countIncrByLua)
