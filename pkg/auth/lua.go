package auth

const (
	prefixRefresh   = "ceddit:refresh:"
	prefixUserToken = "ceddit:user_tokens:"

	luaCreateTokens = `
		local key_refresh = KEYS[1]
		local key_user_set = KEYS[2]
		local token_str = ARGV[1]
		local token_value = ARGV[2]
		local ttl = ARGV[3]

		redis.call('SET', key_refresh, token_value, 'EX', ttl)
		redis.call('SADD', key_user_set, token_str)
		redis.call('EXPIRE', key_user_set, ttl)
		return 0
	`

	luaRefreshTokens = `
		local key_old = KEYS[1]
		local key_new = KEYS[2]
		local key_user_set = KEYS[3]
		local old_token = ARGV[1]
		local new_token = ARGV[2]
		local new_value = ARGV[3]
		local ttl = ARGV[4]

		if redis.call('EXISTS', key_old) == 0 then
			return -1
		end

		redis.call('DEL', key_old)
		redis.call('SET', key_new, new_value, 'EX', ttl)
		redis.call('SREM', key_user_set, old_token)
		redis.call('SADD', key_user_set, new_token)
		redis.call('EXPIRE', key_user_set, ttl)
		return 0
	`

	luaRevokeAll = `
		local key_user_set = KEYS[1]
		local refresh_prefix = ARGV[1]

		local members = redis.call('SMEMBERS', key_user_set)
		for i, token in ipairs(members) do
			redis.call('DEL', refresh_prefix .. token)
		end
		redis.call('DEL', key_user_set)
		return #members
	`
)
