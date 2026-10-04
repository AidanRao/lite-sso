# Redis key layout

All production keys begin with `{environment}:sso:`. The environment comes from `ENV`; an empty value becomes `local`. `keys.go` owns the suffixes. Callers using `kv.Store` pass suffixes to `NewNamespacedStore`. The OAuth refresh store uses the same suffix helpers and adds the environment prefix before its atomic Lua operations.

| Suffix | Purpose | Lifetime |
| --- | --- | --- |
| `captcha:{id}` | Image CAPTCHA answer | CAPTCHA store TTL |
| `auth:challenge:{id}` | Email OTP challenge and attempts | `auth.otp_expire` |
| `auth:rate:{scope}:{id}` | OTP send and verify rate limits | Rate-limit window |
| `auth:fail:{scope}:{id}` | Password and OTP failure counters | Guard window |
| `auth:accounts:{scope}:{id}` | Distinct accounts attempted from a device or IP | Guard window |
| `qr:{code}` | QR login state | 5 minutes |
| `oauth:state:{state}` | Third-party login state | 5 minutes |
| `oauth:pending-binding:{id}` | Pending third-party account binding | 5 minutes |
| `oauth:authorization-code:{sha256(code)}` | OAuth authorization code metadata | `tokens.oauth.authorization_code_ttl`, consumed on first exchange |
| `oauth:refresh:token:{sha256(token)}` | OAuth refresh grant | Until grant expiry |
| `oauth:refresh:user:{userID}` | Set of refresh grant keys for user-wide revocation | Until latest grant expiry |
| `oauth:refresh:revoked:{userID}` | User-wide refresh revocation cutoff | 30 days |
| `webauthn:ceremony:{id}` | Passkey ceremony state | `passkey.ceremony_ttl` |
| `reauth:grant:{sha256(token)}` | Reauthentication grant | `tokens.reauth.grant_ttl` |
| `reauth:session:{sessionID}` | Reauthentication grant by session | `tokens.reauth.grant_ttl` |
| `reauth:email:{id}` | Reauthentication email challenge | Challenge expiry |
| `session:{sessionID}` | Development or test session fixture | Fixture TTL |

First-party user sessions and refresh token hashes live in the database. OAuth access tokens are signed JWTs and have no Redis record. The OAuth refresh store is separate from the authorization-code store because its grants need atomic rotation and user-wide revocation.

The previous OAuth library keys `{environment}:sso:{JWT}` and `{environment}:sso:{random UUID}` are no longer written. Existing records expire after their access-token TTL. Existing authorization-code keys also expire naturally; codes issued before this change cannot be exchanged after deployment. Existing `oauth:refresh:*` keys retain their names and remain valid.
