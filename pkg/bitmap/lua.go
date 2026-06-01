package bitmap

import (
	"fmt"

	"github.com/go-redis/redis"
)

// toggleLua atomically checks the current value of a bit and, if different
// from the desired value, sets it. Returns {changed, old_value}.
//
// KEYS[1] — bitmap key
// ARGV[1] — bit offset
// ARGV[2] — action: "set" (write 1) or "clear" (write 0)
//
// Returns a two-element array: [changed, old_value] where
//   - changed = 1 if the bit was modified, 0 if it was already in the
//     desired state
//   - old_value = the bit value before the operation (0 or 1)
const toggleLua = `
local key    = KEYS[1]
local offset = tonumber(ARGV[1])
local action = ARGV[2]

local old = redis.call('GETBIT', key, offset)

if (action == 'set' and old == 1) or (action == 'clear' and old == 0) then
    return {0, old}
end

local new_val = 0
if action == 'set' then
    new_val = 1
end

redis.call('SETBIT', key, offset, new_val)
return {1, old}
`

// ToggleScript is a Redis Lua script for atomically toggling a bitmap bit.
// It is idempotent: repeated calls with the same arguments produce the same
// final state and only report "changed" on the first call.
var ToggleScript = redis.NewScript(toggleLua)

// ToggleResult holds the output of a toggle operation.
type ToggleResult struct {
	Changed  bool  // true if the bit was actually flipped
	OldValue int64 // bit value before the operation (0 or 1)
}

// InitScripts pre-loads the bitmap Lua scripts into Redis so subsequent
// calls can use EVALSHA instead of sending the full script body.
//
// Accepts a *redis.Client to avoid circular imports with repository/redis.
func InitScripts(rdb *redis.Client) error {
	if err := ToggleScript.Load(rdb).Err(); err != nil {
		return fmt.Errorf("load bitmap toggle script: %w", err)
	}
	return nil
}
