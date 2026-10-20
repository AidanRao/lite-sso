# 身份认证系统接入指南

其他系统通过 OAuth 2.0 授权码模式接入。服务端客户端由接入方后端保管密钥并兑换令牌；无后端的原生客户端应使用公共客户端和 PKCE。

## 1. 接入信息

SSO 管理员需为接入系统登记客户端信息：

| 配置项 | 说明 | 示例 |
| --- | --- | --- |
| `client_id` | 系统标识 | `order-app` |
| `client_type` | `confidential` 服务端客户端，或 `public` 公共客户端 | `confidential` |
| `client_secret` | 服务端客户端密钥，仅保存在后端；公共客户端不使用 | `replace-with-secret` |
| `audiences` | 允许访问的业务 API 标识列表，至少配置一个才能签发令牌 | `["classhopper-api"]` |
| `allowed_scopes` | 应用权限列表；未传 scope 时全部签发 | `["courses:read"]` |
| `homepage_url` | 系统首页地址，用于退出后的 `redirect` 域名校验 | `https://order.example.com` |
| `redirect_uris` | 允许的登录回调地址列表，至少一个 | `["https://order.example.com/auth/sso/callback", "http://localhost:3000/auth/sso/callback"]` |
| `logout_uri` | 可选，全局登出通知地址 | `https://order.example.com/auth/sso/logout` |

管理界面中每行填写一个允许的回调地址；创建和更新客户端 API 使用 `redirect_uris` 字符串数组。每次 OAuth 授权请求仍提交单个 `redirect_uri`，兑换授权码时须使用同一地址。不支持通配符或前缀匹配。

部署此更新前执行 `00017_add_oauth_client_redirect_uris.sql` 迁移，原有单个地址会保留为数组中的一项。回滚迁移时仅保留列表中的第一个地址。

通过管理页面或 `POST /api/admin/oauth-clients` 登记，后端会分配独立的注册标记。请勿直接插入数据库或复用旧记录的标记。

服务端客户端请求示例：

```json
{
  "name": "订单系统",
  "client_id": "order-app",
  "client_type": "confidential",
  "client_secret": "replace-with-secret",
  "audiences": ["classhopper-api"],
  "allowed_scopes": ["courses:read"],
  "homepage_url": "https://order.example.com",
  "redirect_uris": ["https://order.example.com/auth/sso/callback", "http://localhost:3000/auth/sso/callback"],
  "logout_uri": "https://order.example.com/auth/sso/logout"
}
```

公共客户端请求示例（如 Android、iOS、桌面客户端或单页 Web 应用）：

```json
{
  "name": "Android App",
  "client_id": "android-app",
  "client_type": "public",
  "audiences": ["classhopper-api"],
  "allowed_scopes": ["courses:read"],
  "homepage_url": "https://android.example.com",
  "redirect_uris": ["lite-sso-demo://oauth/callback"]
}
```

### 客户端详情与用户授权管理

管理员在 `/admin/clients/:id` 编辑 General 配置，在 `/admin/clients/:id/users` 分页管理关联用户，在 `/admin/clients/:id/advanced` 删除客户端。Users 默认每页 20 条，按最后登录时间倒序、用户 ID 升序排列；页码保存在 URL 中。清空全部作用于整个客户端。

以下 API 都需要 SSO 管理员会话，`:id` 是数据库中的数字 ID：

| 方法与路径 | 说明 |
| --- | --- |
| `GET /api/admin/oauth-clients/:id` | `data.client` 返回配置和 `user_count`，无任何密钥信息 |
| `GET /api/admin/oauth-clients/:id/users?page=1&page_size=20` | `data` 返回 `users`、`total`、`page`、`page_size`；每页最多 100 条 |
| `DELETE /api/admin/oauth-clients/:id/users/:userID` | 移除单个关联，返回 `data.removed_count`，重复移除返回 0 |
| `DELETE /api/admin/oauth-clients/:id/users` | 移除全部关联与授权，返回实际移除数量 |
| `GET /api/admin/oauth-clients/:id/secret` | 按需读取完整密钥，响应禁止缓存 |
| `POST /api/admin/oauth-clients/:id/secret/rotate` | 轮换单一密钥，返回新密钥，旧密钥立即失效，响应禁止缓存 |
| `DELETE /api/admin/oauth-clients/:id` | 仅允许删除无关联用户的客户端 |

详情页密钥默认显示固定的 `xxx`，点击显示时才请求密钥接口，隐藏时清除页面中的完整值。普通配置更新不接受 `client_secret`；公共客户端没有密钥操作。切换成公共客户端会清除密钥，切回服务端客户端由后端生成新密钥。审计不保存密钥内容。

移除仅撤销当前客户端的关联及授权，保留 SSO 账号、登录会话和其他客户端授权；用户以后可以重新登录。客户端删除在事务内重新检查关联人数；非零时返回 HTTP 409，`data.code = "CLIENT_HAS_USERS"`，`data.user_count` 为当前人数，不会自动清空关联。删除成功后彻底移除记录并清理 Logo，允许重新注册相同 Client ID。

每次客户端注册分配 `registration_id`，每次重新建立已移除的关联分配 `authorization_id`。授权码、OAuth JWT、Refresh Token 和扫码确认绑定这些标记，旧授权不会因重新登录或同名客户端注册而恢复。正常重复登录保留现有标记，既有有效授权不受影响。移除或删除后，SSO 在线校验和刷新立即拒绝旧授权；业务端仅离线验签的 JWT 仍按自身有效期过期。Refresh Token 和授权码存储中的失效记录保留到自身 TTL，由在线标记校验拒绝使用。

### 升级顺序

先执行 `00017_add_oauth_client_redirect_uris.sql` 和 `00018_add_oauth_authorization_identity.sql`，再更新全部服务实例，最后开放用户移除与客户端删除入口。迁移为存量客户端和关联保留空标记，使存量授权继续有效；新注册和新关联必须由新版服务生成标记。迁移完成后旧版实例不能写入缺失标记的新记录，应在维护窗口完成切换，或先停止旧实例的 OAuth 写流量。混用旧版实例时禁止开放移除和删除功能。回滚标记迁移会失去授权隔离保障，不能在已经移除授权后直接回滚到旧服务。

## 2. 接口地址

假设 SSO 服务地址为 `https://sso.aidanrao.top`。

| 用途 | 方法 | 地址 |
| --- | --- | --- |
| 发起登录 | `GET` | `/oauth/authorize` |
| 换取令牌 | `POST` | `/oauth/token` |
| 获取用户信息 | `GET` | `/oauth/userinfo` |
| 公钥集 | `GET` | `/.well-known/jwks.json` |
| 退出登录 | `POST` | `/api/auth/logout` |

## 3. 令牌有效期配置

服务部署配置统一使用 `tokens` 分组，并按签发流程区分令牌。以下为当前生产配置：

| 配置项 | 默认值 | 用途 |
| --- | --- | --- |
| `tokens.oauth.access_token_ttl` | `30m` | OAuth `/oauth/token` 签发给接入客户端的 JWT Access Token |
| `tokens.oauth.authorization_code_ttl` | `5m` | OAuth 授权码有效期 |
| `tokens.oauth.refresh_token_ttl` | `720h` | OAuth Refresh Token 最长期限，轮换不延长 |
| `tokens.session.access_token_ttl` | `15m` | Lite SSO 自身登录会话使用的 Access Token |
| `tokens.session.refresh_token_ttl` | `720h` | Lite SSO 自身登录会话 Refresh Token 的有效期 |
| `tokens.reauth.grant_ttl` | `5m` | 高风险操作重新认证授权凭证的有效期 |

OAuth 接入客户端使用独立的 Access Token 和 Refresh Token。内部登录会话的 Refresh Token 通过 SSO 的登录 Cookie 刷新，不应发送给接入业务 API。

这些键也可通过对应环境变量覆盖，例如 `TOKENS_OAUTH_ACCESS_TOKEN_TTL=30m`。所有环境都必须配置 OAuth issuer、`signing_kid` 与 RSA 私钥；可通过 `TOKENS_OAUTH_ISSUER`、`TOKENS_OAUTH_SIGNING_KID` 和 `TOKENS_OAUTH_SIGNING_PRIVATE_KEY_PEM` 注入，私钥不要提交到仓库。`TOKENS_OAUTH_PREVIOUS_PUBLIC_KEYS` 可配置旧公钥 PEM 的 JSON 对象（键为 kid），轮换后至少保留 35 分钟。Cookie 的 `Secure` 属性由 `server.cookie_secure` 配置，本地 HTTP 示例设为 `false`，测试和生产 HTTPS 配置设为 `true`。完整配置示例见 `conf/prod.yaml` 和 `conf/local.example.yaml`。

部署前先执行数据库迁移，并在管理端为现有客户端补齐 `audiences` 和 `allowed_scopes`。以下查询列出仍无法签发新令牌的客户端：

```sql
SELECT client_id FROM oauth_clients WHERE jsonb_array_length(audiences) = 0;
```

分别核对生产和测试环境的 `TOKENS_OAUTH_ACCESS_TOKEN_TTL` 覆盖值及 `TOKENS_OAUTH_ISSUER`，确保实际签发的 Access Token 有效期为 30 分钟。缺少有效 RSA 私钥时，所有环境都会拒绝启动。接入方完成 JWKS 验签和 Refresh Token 轮换后再切换流量。

## 4. 接入流程

### 4.1 跳转到 SSO 登录

接入系统生成随机 `state` 并保存在本地会话中，然后将用户浏览器跳转至：

```text
https://sso.aidanrao.top/oauth/authorize
  ?response_type=code
  &client_id=order-app
  &redirect_uri=https%3A%2F%2Forder.example.com%2Fauth%2Fsso%2Fcallback
  &state=<random-state>
  &scope=courses%3Aread
```

如果用户未登录，SSO 会先展示登录页面；登录完成后继续回调接入系统。

SSO 登录成功后会写入仅供浏览器授权流程使用的 `sso_session` Cookie。该 Cookie 为 `HttpOnly`、`SameSite=Lax`，路径为 `/`；普通 `/api` 接口仍要求 `Authorization: Bearer <access-token>`，不会接受该 Cookie 代替 Access Token。

### 4.2 公共客户端（PKCE）

没有安全保密能力的客户端应登记为 `public` 客户端，例如 Android / iOS App、桌面客户端和单页 Web 应用。公共客户端不设置或发送 `client_secret`，授权码流程必须使用 PKCE `S256`。回调 URI 需要登记完整值，例如原生应用可使用 `lite-sso-demo://oauth/callback`；SSO 会精确匹配该值。

客户端使用系统浏览器或平台提供的安全授权代理打开授权页，不要在嵌入式 WebView 中收集 SSO 凭据。每次登录生成独立随机 `state` 和 `code_verifier`；`code_challenge` 为 `BASE64URL_NO_PADDING(SHA256(UTF8(code_verifier)))`。`code_verifier` 应符合 PKCE 长度和字符要求，授权请求固定使用 `code_challenge_method=S256`。

授权请求示例：

```text
https://sso.aidanrao.top/oauth/authorize
  ?response_type=code
  &client_id=android-app
  &redirect_uri=lite-sso-demo%3A%2F%2Foauth%2Fcallback
  &state=<random-state>
  &code_challenge=<base64url-sha256-verifier>
  &code_challenge_method=S256
```

回调到 `lite-sso-demo://oauth/callback?code=...&state=...` 后，App 必须先校验 `state`，再使用授权开始时保存的 `code_verifier` 请求令牌：

```http
POST /oauth/token HTTP/1.1
Host: sso.aidanrao.top
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&client_id=android-app&code=<code>&redirect_uri=lite-sso-demo%3A%2F%2Foauth%2Fcallback&code_verifier=<original-verifier>
```

公共客户端不发送 `Authorization: Basic`，也不发送 `client_secret`。成功后可使用 `Authorization: Bearer <access-token>` 调用 `/oauth/userinfo`。响应包含需要安全存储的 `refresh_token`；刷新成功后必须立即替换旧值。建议将临时 `state` / verifier 与登录事务绑定保存，成功或失败后清除，并使用平台提供的安全存储保护本地令牌。

### 4.3 处理服务端登录回调

SSO 登录成功后跳转到接入系统的回调地址：

```text
https://order.example.com/auth/sso/callback?code=<code>&state=<random-state>
```

接入系统需要：

1. 校验回调中的 `state` 与本地会话中保存的值一致。
2. 读取 `code`，由后端调用令牌接口。
3. 校验完成后删除已保存的 `state`，防止重复使用。

### 4.4 使用 Code 换取 Access Token

```http
POST /oauth/token HTTP/1.1
Host: sso.aidanrao.top
Authorization: Basic base64(order-app:replace-with-secret)
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
code=<code>&
redirect_uri=https%3A%2F%2Forder.example.com%2Fauth%2Fsso%2Fcallback
```

响应示例：

```json
{
  "access_token": "<RS256-JWT>",
  "refresh_token": "<opaque-refresh-token>",
  "expires_in": 1800,
  "token_type": "Bearer",
  "scope": "courses:read"
}
```

JWT 的 `iss` 为当前环境 SSO 地址，`aud` 包含客户端登记的全部 API 标识，客户端未传 `scope` 参数时，`scope` 包含应用当前登记的全部 `allowed_scopes`；显式传入时只签发允许的子集，显式空字符串表示无业务权限，未登记权限返回 `invalid_scope`。授权码保存授权时确定的权限，兑换前重新校验，已撤销权限不能签发。业务 API 应按 JWT header 的 `kid` 从 JWKS 获取公钥，并验证 RS256 签名、`iss`、自身 `aud`、`exp` 和所需 `scope`。JWKS 响应可缓存 5 分钟。旧不透明 OAuth Access Token 在切换后不再受支持。

Access Token 到期时使用同一客户端调用 `POST /oauth/token`，参数为 `grant_type=refresh_token&refresh_token=<当前令牌>`。保密客户端继续使用 Basic 认证；公共客户端传 `client_id`。刷新未传 `scope` 时，读取应用当前全部 `allowed_scopes`；显式传入时使用当前配置允许的子集，不受首次授权权限上限限制。新令牌的 `aud` 同样使用应用当前全部 audiences。因此管理员新增 scope 或 audience 后，已有登录刷新即可获得；移除的权限从新令牌中删除。`allowed_scopes` 为空时，响应和 JWT 不包含业务 scope。成功响应返回新的 Access Token 与 Refresh Token，旧 Refresh Token 立即失效；30 天最长期限不因轮换延长。SSO 退出登录会撤销用户的 OAuth Refresh Token，已签发 JWT 最多继续有效 30 分钟。

### 4.5 获取用户信息

```http
GET /oauth/userinfo HTTP/1.1
Host: sso.aidanrao.top
Authorization: Bearer <access-token>
```

响应示例：

```json
{
  "id": "2e62bf9d-6bd3-4c44-9fd5-ef7130e99999",
  "email": "user@example.com",
  "username": "alice",
  "avatar_url": "https://cdn.example.com/avatar.png"
}
```

接入系统应以 `id` 作为 SSO 用户唯一标识，并在获取用户信息后创建自己的登录会话。

### 4.6 退出登录

系统退出分为接入系统本地会话退出和 SSO 全局会话退出：

1. 用户在接入系统点击退出时，接入系统应先清除自己的本地会话。
2. 如需同时退出 SSO，将浏览器导航到 SSO 的 `/logout?redirect=<编码后的返回地址>`。不再向 API 提交浏览器表单。
3. 前端登出页先提示用户确认退出；取消则返回账号页，不发起登出请求。用户确认后调用 `POST /api/auth/logout`，后端撤销当前会话、清除登录 Cookie，并返回当前用户已登录应用的 `logout_uris` 和经校验的 `redirect_uri`。该 API 始终返回 JSON，不再返回 HTML 或 HTTP 重定向。
4. 前端登出页通过隐藏 iframe 访问这些地址，通知各接入系统清除本地会话；通知完成后跳转，最多等待 5 秒。无通知地址时直接跳转。未提供或未通过校验的返回地址默认使用 `/login`。
5. 如果请求携带 `redirect` 参数，SSO 允许站内相对路径；外部地址必须使用 HTTP(S)，且域名与当前用户已登录应用的 `homepage_url` 域名一致才会跳转；未通过校验时会忽略该参数。

接入系统的 `logout_uri` 用于接收 SSO 的全局退出通知，建议实现为幂等接口。该接口被 SSO 退出中转页以浏览器 iframe 方式访问，因此应支持 `GET` 请求，不依赖请求体，并在收到请求后清除当前浏览器对应的本地登录 Cookie 或会话。

示例：

```http
GET /auth/sso/logout HTTP/1.1
Host: order.example.com
Cookie: order_session=<local-session>
```

响应可返回 `204 No Content` 或轻量 HTML，SSO 不会读取响应内容。

## 5. Python / Flask 示例

安装依赖：

```bash
python -m pip install Flask requests
```

设置配置：

```bash
export FLASK_SECRET_KEY='replace-with-random-secret'
export SSO_BASE_URL='http://localhost:8080'
export SSO_CLIENT_ID='order-app'
export SSO_CLIENT_SECRET='replace-with-secret'
export SSO_REDIRECT_URI='http://localhost:5000/auth/sso/callback'
```

`app.py`：

```python
import hmac
import os
import secrets
from urllib.parse import urlencode

import requests
from flask import Flask, abort, jsonify, redirect, request, session


app = Flask(__name__)
app.secret_key = os.environ["FLASK_SECRET_KEY"]

SSO_BASE_URL = os.environ["SSO_BASE_URL"].rstrip("/")
CLIENT_ID = os.environ["SSO_CLIENT_ID"]
CLIENT_SECRET = os.environ["SSO_CLIENT_SECRET"]
REDIRECT_URI = os.environ["SSO_REDIRECT_URI"]


@app.get("/login")
def login():
    state = secrets.token_urlsafe(32)
    session["sso_state"] = state
    query = urlencode(
        {
            "response_type": "code",
            "client_id": CLIENT_ID,
            "redirect_uri": REDIRECT_URI,
            "state": state,
        }
    )
    return redirect(f"{SSO_BASE_URL}/oauth/authorize?{query}")


@app.get("/auth/sso/callback")
def callback():
    expected_state = session.pop("sso_state", "")
    state = request.args.get("state", "")
    code = request.args.get("code", "")

    if not expected_state or not hmac.compare_digest(expected_state, state):
        abort(400, "Invalid state")
    if not code:
        abort(400, "Missing code")

    token_response = requests.post(
        f"{SSO_BASE_URL}/oauth/token",
        auth=(CLIENT_ID, CLIENT_SECRET),
        data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT_URI,
        },
        timeout=10,
    )
    token_response.raise_for_status()
    access_token = token_response.json()["access_token"]

    user_response = requests.get(
        f"{SSO_BASE_URL}/oauth/userinfo",
        headers={"Authorization": f"Bearer {access_token}"},
        timeout=10,
    )
    user_response.raise_for_status()
    user = user_response.json()

    # 生产系统在此使用 user["id"] 关联本地用户并创建本地会话。
    session["user"] = {
        "sso_user_id": user["id"],
        "email": user.get("email"),
        "username": user.get("username"),
    }
    return jsonify(session["user"])


@app.get("/auth/sso/logout")
def sso_logout():
    session.clear()
    return "", 204


if __name__ == "__main__":
    app.run(port=5000, debug=True)
```

访问 `http://localhost:5000/login` 即可发起 SSO 登录。

## 6. 注意事项

- `client_secret` 只能用于 `confidential` 服务端客户端并存放在接入系统后端；`public` 客户端不使用 secret。
- 每次登录必须生成并校验 `state`。
- OAuth Refresh Token 每次使用后轮换，客户端必须保存新值并丢弃旧值。
- 公共客户端必须使用 PKCE `S256`；服务端客户端可继续使用现有 secret 流程，也可选用 PKCE `S256`。
- `redirect_uri` 必须与管理端 `redirect_uris` 列表中的一项完全一致（包括路径、查询参数和末尾斜杠），且不得包含片段（`#`）。服务端回调生产环境应使用 HTTPS；原生公共客户端可登记精确的自定义 Scheme 回调。
- 退出登录携带的 `redirect` 只校验域名是否与已登记的 `homepage_url` 一致，不要求路径与 `homepage_url` 相同。
- 如需联动登出，可登记 `logout_uri`，由 SSO 登出流程通知接入系统清除本地会话。


## 5. 接入 App 确认电脑扫码登录

扫码方向是电脑登录 SSO，App 提供当前用户的身份确认。只允许获准应用使用 OAuth Access Token 确认；SSO 自身会话令牌、Cookie、Client Secret 均不能代替扫码 OAuth 令牌。

管理员在应用配置中保留已有权限，增加 `qr:login`，并在 audiences 中增加 `sso-api`。该权限允许用户通过应用确认浏览器 SSO 登录，并继续进入其他接入系统，应只授予可信应用。客户端普通授权和刷新请求均可省略 scope；无需首次扫码增量授权。旧令牌缺少权限时刷新一次即可获得当前配置权限。

二维码是以下 JSON 文本的图像：

```json
{"type":"lite-sso.qr-login","version":1,"issuer":"https://sso.example.com","code":"12345678-1234-4234-8234-123456789abc"}
```

App 必须识别版本，精确匹配配置的 issuer，并将请求发往自己配置的 SSO 服务；不能按二维码中的任意 URL 发送令牌。二维码不包含 Access Token、登录 ticket、浏览器凭证或 redirect。

所有接口位于 `/api/auth/qr`，沿用 `{code, message, data}` 响应。二维码固定生成后 5 分钟过期；扫描、确认、轮询均不续期。所有响应设置 `Cache-Control: no-store`。

| 接口 | 请求 | 响应 data / 行为 |
| --- | --- | --- |
| `GET /generate` | 可选 `redirect`，仅本站相对路径 | `code`、`qr_payload`、`browser_token`、`expires_at`（UTC RFC3339） |
| `GET /poll` | query `code`；原设备 Cookie 与 `X-QR-Browser-Token` | `status`、`expires_at`；confirmed 时附 `login_ticket` |
| `POST /scan` | OAuth Bearer；JSON `{code}` | 绑定用户和应用，返回 `status`、`user_id`、`account_name`、`target_name`、`environment`、`created_at`、`expires_at` |
| `POST /confirm` | OAuth Bearer；JSON `{code}` | `{confirmed: true, cancelled: false}`，同一用户和应用可安全重试 |
| `POST /reject` | OAuth Bearer；JSON `{code}` | `{confirmed: false, cancelled: true}` |
| `POST /complete` | 原设备 Cookie、浏览器凭证 header；JSON `{code, login_ticket}` | 一次性创建浏览器会话，返回现有登录结果并写入登录 Cookie |
| `POST /cancel` | 原设备 Cookie、浏览器凭证 header；JSON `{code}` | `{cancelled: true}`，刷新二维码或离开登录页时调用 |

浏览器凭证只留在发起页面内存中，不放入二维码、URL、日志或手机响应。状态为 `pending → scanned → confirmed → consumed`，取消或拒绝进入 `cancelled`。同一个用户与扫描应用可重复扫描、确认；其他用户或应用不能接管。App 显示账号、目标系统、电脑请求信息，并要求用户明确确认。网络超时后可重试同一操作，浏览器完成只能消费一次；完成失败则重新生成二维码。

### 登录环境信息

`GET /api/user/devices` 的每个设备与 `POST /api/auth/qr/scan` 的预览统一通过 `environment` 返回服务端解析的展示字段：

```json
{
  "environment": {
    "browser": "Chrome",
    "browser_version": "130.0.0.0",
    "os": "macOS",
    "os_version": "10.15.7",
    "device_type": "desktop",
    "device_name": "",
    "display_name": "Chrome · macOS",
    "ip": "203.0.113.10",
    "location": "中国 · 北京"
  }
}
```

这两个响应不再包含顶层 `ip` 或原始 `user_agent`。Web 和 App 直接展示 `environment.display_name`、`environment.ip`，无需各自解析 UA；服务端与客户端应一起发布此接口变更。`device_type` 为 `desktop`、`mobile`、`tablet`、`bot` 或 `unknown`；未识别的字段为空字符串，缺少可识别信息时 `display_name` 为“未知设备”。版本仅表示 UA 报告的版本，可能被浏览器缩减，不代表核验后的真实系统版本或设备型号。

扫码预览描述生成二维码时发起登录的浏览器请求；完成登录的 session 保存浏览器完成请求时的信息。session 列表展示登录时保存的环境；浏览器和系统由保存的 UA 解析，地点读取创建 session 时保存的快照。地点字段需要执行 `00013_add_session_location.sql` 迁移，已有记录保留空地点，不回填。`device_id` 表示浏览器 Cookie 环境，环境信息仅用于展示，不参与认证授权。

通用入口为 `util/requestmeta.Extract`、`util/useragent.Parse`，Web 层通过 `handler.LoginMetadataFromRequest` 组合设备 ID，Service 层接收 `LoginMetadata`。`util/requestmeta.ParseEnvironment` 解析环境信息，`dto.ToEnvironmentResponse` 只映射响应字段。`util/ipgeo` 使用 ip2region 离线解析 IP 地点，服务启动时共享一个支持 IPv4、IPv6 并发查询的实例；国家、省份、城市合并为 `environment.location`，过滤占位值和重复名称。扫码预览保存生成二维码时的地点，session 保存完成登录时的地点；地点仅用于展示，不影响认证。无效 IP、内网 IP 或查不到地点时返回空字符串，Web 和 App 展示“未知”。服务端保留最多 512 字节、已清除控制字符的 UA 用于诊断。IP 沿用 `server.trust_proxy_headers`：关闭时取连接地址，开启时优先读取 `X-Vercel-Forwarded-For`、其次 `X-Forwarded-For` 的第一个合法 IP；部署入口必须覆盖客户端伪造的转发头。无效 IP 返回空字符串。

| HTTP / data.code | App 或浏览器行为 |
| --- | --- |
| `401 / UNAUTHORIZED` | App 刷新 OAuth 令牌并重试一次，仍失败则引导登录 |
| `403 / QR_TOKEN_PERMISSION_MISSING` | 应用当前配置允许，但令牌缺少 scope / audience；刷新并重试一次 |
| `403 / QR_CLIENT_FORBIDDEN` | 应用不存在或当前配置未获扫码权限；提示联系管理员，不循环刷新 |
| `403 / USER_INACTIVE` | 账号不存在或停用，停止操作 |
| `403 / QR_CODE_INVALID_USER` | 二维码已被其他用户或应用扫描 |
| `403 / QR_BROWSER_INVALID` | 原浏览器凭证或设备不匹配 |
| `409 / QR_CODE_INVALID_STATUS` | 状态不允许或 ticket 无效，重新生成 |
| `410 / QR_CODE_EXPIRED` | 要求电脑刷新、App 重新扫描 |
| `500 / QR_SERVICE_UNAVAILABLE` | 服务或存储故障，保留与过期不同的提示 |

扫描、确认、拒绝均验证令牌以及数据库当前应用权限和账号状态；浏览器完成前也重新检查确认应用和用户，因此撤销扫码权限立即阻止未完成请求。已发往其他业务 API 的旧 JWT 仍按其自身有效期与验证策略处理。

### ip2region 数据部署

默认从 `data/ip2region` 读取 `ip2region_v4.xdb` 与 `ip2region_v6.xdb`，可通过 `geoip.data_dir` / `GEOIP_DATA_DIR` 指定目录。两个容器镜像均携带数据文件，缺失或格式错误会阻止服务启动。数据来源、固定版本和替换步骤见仓库 `data/ip2region/README.md`。登录地点属于公网 IP 推测结果，VPN、代理和运营商出口会影响定位；更新数据库只影响之后生成的地点快照。

## 账号状态与注销

用户信息的 `status` 为 `active`（正常）、`disabled`（禁用）或 `pending_deletion`（待注销），替代原来的 `is_active`。`/api/user/profile`、登录结果的 `user` 和 `/oauth/userinfo` 均不携带注销申请时间或截止时间；SSO 刷新结果只增加 `status`。

前端发现 `pending_deletion` 后进入 `/account/deletion`，主动调用以下接口获取详情：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/user/account-deletion` | 查询状态、申请时间、截止时间及是否仍可撤销 |
| POST | `/api/user/account-deletion` | 验证身份后申请注销，返回 202 |
| DELETE | `/api/user/account-deletion` | 截止前验证身份并撤销注销 |

接口使用 SSO Bearer Token。POST 和 DELETE 复用现有 `X-Reauth-Token` 或近期登录验证规则；不能用用户信息弹窗的文字输入代替后端身份验证。弹窗只在前端校验用户名或已验证邮箱（均不存在时使用用户 ID）及准确短语“注销我的账号”，这些文字不发送至注销 API。

申请成功后立即撤销全部 SSO 会话并清除当前浏览器 Cookie。冷静期为服务端计算的完整 24 小时，邮箱、用户名、第三方身份在此期间仍被占用。重新登录只创建绑定本次申请的注销管理会话，允许查看状态、撤销、退出和必要的二次验证；不能管理账号资料、访问管理接口、授权应用或扫码批准其他浏览器。原登录跳转目标会被替换为注销状态页。

GET 和 POST 成功响应的 `data` 示例：

```json
{
  "status": "pending_deletion",
  "deletion_requested_at": "2026-10-15T02:00:00Z",
  "deletion_scheduled_at": "2026-10-16T02:00:00Z",
  "can_cancel": true
}
```

重复申请不延长截止时间。达到截止时间即不可撤销，DELETE 返回 409 和 `data.code=ACCOUNT_DELETION_DUE`；处于冷静期的账号访问普通受保护接口返回 403 和 `data.code=ACCOUNT_PENDING_DELETION`，不附带注销详情。撤销成功后须重新登录，旧会话、旧 OAuth 刷新授权、旧验证码和未完成的绑定流程不会恢复。

最终注销会删除本系统的账号资料、邮箱及来源、第三方绑定、Passkey、会话、应用关联和功能名单，释放对应唯一身份。重新注册分配新的用户 ID。清理不会删除第三方平台账号、第三方头像地址或接入应用的业务数据。服务自行启动每分钟数据库扫描，并在重启后补做；同身份登录、注册或绑定时也会先处理到期注销，因此服务停机后的清理可能延迟，但不会延长可撤销窗口。

OAuth 授权码兑换、刷新、userinfo 和扫码接口会检查账号状态及持久化撤销记录。Redis 暂时不可用不会让旧刷新授权在撤销注销后恢复；Redis 和自有头像清理使用独立持久化任务重试。接入应用仅离线验签的已签发 JWT 仍可在其原有效期内有效，本次没有增加下游退出通知或业务数据清理协议。

状态值、任务类型及冷静期规则由 Go 代码验证，新增迁移不添加 SQL CHECK 约束。

后端冷静期统一由 `service/user/deletion.go` 的 `DeletionCoolingPeriod` 常量定义，线上保持 `24 * time.Hour`，本地调试可临时缩短。前端说明固定为一天，实际截止时间始终读取服务端响应。

升级时先应用 `00014_add_account_deletion.sql`，再部署新服务；该迁移将 `is_active` 转成 `status`，旧服务与新结构不兼容，应在维护窗口完成切换。既有禁用账号保持 `disabled`。审计日志继续按原有 90 天规则保留。

本地验证：

```bash
GOCACHE=/private/tmp/lite-sso-go-cache go test ./...
ACCOUNT_DELETION_TEST_DATABASE_DSN='<isolated PostgreSQL DSN>' go test ./service/user -run DeletionPostgres
MIGRATION_TEST_DATABASE_DSN='<isolated PostgreSQL DSN>' go test ./cmd/migrate
cd frontend
npm test
npm run build
```

PostgreSQL 测试在指定数据库内建立并清理独立 schema，不使用生产数据库。未提供相应 DSN 时，这些集成测试会明确跳过。
