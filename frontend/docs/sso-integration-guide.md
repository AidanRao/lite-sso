# 身份认证系统接入指南

其他系统通过 OAuth 2.0 授权码模式接入。接入方后端负责使用授权码换取令牌、获取用户信息，并创建本系统登录会话。

## 1. 接入信息

SSO 管理员需为接入系统登记客户端信息：

| 配置项 | 说明 | 示例 |
| --- | --- | --- |
| `client_id` | 系统标识 | `order-app` |
| `client_secret` | 系统密钥，仅保存在后端 | `replace-with-secret` |
| `redirect_uris` | 登录回调地址，JSON 数组字符串 | `["https://order.example.com/auth/sso/callback"]` |
| `logout_uris` | 可选，全局登出通知地址 | `["https://order.example.com/auth/sso/logout"]` |

登记示例：

```sql
INSERT INTO oauth_clients (name, client_id, client_secret, redirect_uris, logout_uris)
VALUES (
    '订单系统',
    'order-app',
    'replace-with-secret',
    '["https://order.example.com/auth/sso/callback"]',
    '["https://order.example.com/auth/sso/logout"]'
);
```

## 2. 接口地址

假设 SSO 服务地址为 `https://sso.aidanrao.top`。

| 用途 | 方法 | 地址 |
| --- | --- | --- |
| 发起登录 | `GET` | `/oauth/authorize` |
| 换取令牌 | `POST` | `/oauth/token` |
| 获取用户信息 | `GET` | `/oauth/userinfo` |

## 3. 接入流程

### 3.1 跳转到 SSO 登录

接入系统生成随机 `state` 并保存在本地会话中，然后将用户浏览器跳转至：

```text
https://sso.aidanrao.top/oauth/authorize
  ?response_type=code
  &client_id=order-app
  &redirect_uri=https%3A%2F%2Forder.example.com%2Fauth%2Fsso%2Fcallback
  &state=<random-state>
```

如果用户未登录，SSO 会先展示登录页面；登录完成后继续回调接入系统。

### 3.2 处理登录回调

SSO 登录成功后跳转到接入系统的回调地址：

```text
https://order.example.com/auth/sso/callback?code=<code>&state=<random-state>
```

接入系统需要：

1. 校验回调中的 `state` 与本地会话中保存的值一致。
2. 读取 `code`，由后端调用令牌接口。
3. 校验完成后删除已保存的 `state`，防止重复使用。

### 3.3 使用 Code 换取 Access Token

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
  "access_token": "<access-token>",
  "expires_in": 43200,
  "token_type": "Bearer"
}
```

### 3.4 获取用户信息

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

## 4. Python / Flask 示例

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


if __name__ == "__main__":
    app.run(port=5000, debug=True)
```

访问 `http://localhost:5000/login` 即可发起 SSO 登录。

## 5. 注意事项

- `client_secret` 只能存放在接入系统后端，不能写入浏览器端代码。
- 每次登录必须生成并校验 `state`。
- 当前 SSO 不签发 `refresh_token`，令牌过期后需要重新登录。
- 当前未提供 PKCE，推荐由服务端应用接入。
- 当前 `redirect_uri` 按 Host 校验，生产环境应使用 HTTPS 和独立可信域名。
- 如需联动登出，可登记 `logout_uris`，由 SSO 登出流程通知接入系统清除本地会话。
