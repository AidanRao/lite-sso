local raw = redis.call("GET", KEYS[1])
if not raw then
    return {0, "expired"}
end
local qr = cjson.decode(raw)
local operation = ARGV[1]

if operation == "scan" then
    if qr.status ~= "pending" then return {0, "status"} end
    qr.status = "scanned"
    qr.user_id = ARGV[2]
elseif operation == "confirm" then
    if qr.status ~= "scanned" or qr.user_id ~= ARGV[2] then return {0, "user"} end
    qr.status = "confirmed"
    qr.login_ticket = ARGV[3]
elseif operation == "complete" then
    if qr.status ~= "confirmed" or qr.login_ticket ~= ARGV[2] then return {0, "ticket"} end
    if qr.device_id ~= "" and qr.device_id ~= ARGV[3] then return {0, "device"} end
    qr.status = "consumed"
    redis.call("SET", KEYS[1], cjson.encode(qr), "KEEPTTL")
    return {1, qr.user_id, qr.redirect}
else
    return {0, "operation"}
end

redis.call("SET", KEYS[1], cjson.encode(qr), "KEEPTTL")
return {1, qr.status}
