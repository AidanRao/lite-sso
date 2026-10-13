-- Compare-and-set preserves the original TTL across processes. State transitions
-- are validated in Go against the exact snapshot supplied in ARGV[1].
local raw = redis.call("GET", KEYS[1])
if not raw then return -1 end
if raw ~= ARGV[1] then return 0 end
redis.call("SET", KEYS[1], ARGV[2], "KEEPTTL")
return 1
