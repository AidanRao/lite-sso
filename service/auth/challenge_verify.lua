local raw = redis.call("GET", KEYS[1])
if not raw then
    return {0, "missing"}
end

local challenge = cjson.decode(raw)
if challenge.device_id ~= ARGV[1] or challenge.purpose ~= ARGV[2] then
    return {0, "context"}
end
if challenge.status ~= "ACTIVE" then
    if challenge.status == "EXPIRED" then
        return {0, "expired"}
    end
    if challenge.status == "FAILED" then
        return {0, "attempts"}
    end
    return {0, "used"}
end
if tonumber(challenge.expires_at) <= tonumber(ARGV[3]) then
    challenge.status = "EXPIRED"
    redis.call("SET", KEYS[1], cjson.encode(challenge), "KEEPTTL")
    return {0, "expired"}
end
if challenge.code_mac ~= ARGV[4] then
    challenge.failed_attempts = tonumber(challenge.failed_attempts or 0) + 1
    if challenge.failed_attempts >= tonumber(ARGV[5]) then
        challenge.status = "FAILED"
        redis.call("SET", KEYS[1], cjson.encode(challenge), "KEEPTTL")
        return {0, "attempts"}
    end
    redis.call("SET", KEYS[1], cjson.encode(challenge), "KEEPTTL")
    return {0, "mismatch"}
end

challenge.status = "VERIFIED"
redis.call("SET", KEYS[1], cjson.encode(challenge), "KEEPTTL")
return {1, "verified"}
