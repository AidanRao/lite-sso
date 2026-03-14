# `lite-sso` 统一身份认证中心技术架构文档

## 1. 系统概述

`lite-sso` 是身份认证服务 (Identity Provider, IdP)。系统基于 Go 语言开发，严格遵循 OIDC / OAuth 2.0 标准协议。
在架构设计上，采用**“应用层无状态”**的云原生理念：持久化关系型数据落盘至关系型数据库，所有高频、生命周期短的状态型数据统一交由 Redis 管理，从而实现系统的高可用与可水平扩展。

### 1.1 核心特性

* **应用层无状态**：状态管理全面接入 Redis，支持 SSO 节点的多实例横向扩容。
* **标准协议接入**：原生支持 OAuth2 授权码模式，无缝对接 DevOps、GitLab 等开源软件及自研系统。
* **多维登录方式**：支持账号密码（带图形验证码防刷）、邮箱 OTP 无密码登录、扫码授权登录及 GitHub/微信 第三方联邦认证。
* **前后端解耦**：提供纯净的 API 接口，登录页面 UI 可由前端自由定制。

---

## 2. 技术栈与架构方案

### 2.1 核心技术选型

| 模块 | 技术栈 | 选型说明 |
| --- | --- | --- |
| **开发语言** | Go (1.26) | 静态编译，高并发，服务端资源占用极小。 |
| **Web 框架** | `gin-gonic/gin` | 路由性能优异，用于暴露前后端交互 API。 |
| **OAuth2 引擎** | `go-oauth2/oauth2/v4` | 核心引擎。配合 `go-oauth2/redis` 插件，将授权码(Code)和令牌(Token)的生命周期交由 Redis 接管。 |
| **持久化存储** | SQLite 或 PostgreSQL | 配合 `gorm.io/gorm`。保存用户档案、客户端配置等低频变动的持久化数据。 |
| **状态/缓存层** | **Redis** (`redis/go-redis/v9`) | **核心变更**：接管所有短生命周期状态数据（验证码、OTP、扫码状态、防刷限流）。 |
| **密码安全** | `golang.org/x/crypto/bcrypt` | 核心哈希算法，自带随机 Salt 防止彩虹表攻击。 |

### 2.2 部署架构

* **应用服务**：`lite-sso` (Docker 镜像，约 20MB)。
* **中间件**：Redis 实例 (可用轻量级配置，如限制 50MB 内存上限，采用 `allkeys-lru` 淘汰策略)。
* **数据库**：SQLite 数据文件挂载。

---

## 3. 数据库设计 (持久化层)

持久化层仅保存核心业务实体。

### 3.1 `users` (用户主表)

| 字段名 | 数据类型 | 约束条件 | 描述 |
| --- | --- | --- | --- |
| `id` | `VARCHAR(36)` | PK | 用户唯一标识 (UUID) |
| `username` | `VARCHAR(50)` | UNIQUE, NULL | 用户名 |
| `email` | `VARCHAR(100)` | UNIQUE, NOT NULL | 邮箱账号（主通讯与登录凭证） |
| `password_hash` | `VARCHAR(255)` | NULL | Bcrypt 密文（第三方/验证码登录用户可为空） |
| `avatar_url` | `VARCHAR(255)` | NULL | 头像链接 |
| `is_active` | `BOOLEAN` | DEFAULT TRUE | 账号状态 |

### 3.2 `user_third_party` (第三方身份绑定表)

| 字段名 | 数据类型 | 约束条件 | 描述 |
| --- | --- | --- | --- |
| `id` | `INTEGER` | PK, AUTO | 自增主键 |
| `user_id` | `VARCHAR(36)` | FK | 关联主表 `users.id` |
| `provider` | `VARCHAR(20)` | NOT NULL | 平台标识 (如 `github`) |
| `provider_uid` | `VARCHAR(100)` | NOT NULL | 该平台下的唯一用户 ID |

### 3.3 `oauth_clients` (接入应用配置表)

| 字段名 | 数据类型 | 约束条件 | 描述 |
| --- | --- | --- | --- |
| `id` | `INTEGER` | PK, AUTO | 自增主键 |
| `name` | `VARCHAR(50)` | NOT NULL | 应用名称 |
| `client_id` | `VARCHAR(50)` | PK | 客户端 ID |
| `client_secret` | `VARCHAR(255)` | NOT NULL | 客户端机密凭证 (换 Token 使用) |
| `redirect_uris` | `TEXT` | NOT NULL | JSON 数组格式，允许回调的安全地址列表 |

---

## 4. Redis 缓存设计 (状态层)

这是支撑多维度登录核心流转的关键，所有 Key 必须设置明确的 TTL 以防内存泄露。

| 业务场景 | Redis Key 格式 | Value 数据结构 | TTL | 描述 |
| --- | --- | --- | --- | --- |
| **图形验证码** | `lite-sso:captcha:{captcha_id}` | String (验证码真实字符) | 5 分钟 | 用户提交后即刻 `DEL` 销毁。 |
| **邮箱 OTP** | `lite-sso:otp:{email}` | String (6位数字) | 5 分钟 | 用于免密登录的验证凭证。 |
| **防刷限流** | `lite-sso:ratelimit:email:{email}` | String ("1") | 60 秒 | 防止同一邮箱在 1 分钟内重复请求发送验证码。 |
| **扫码状态机** | `lite-sso:qr:{uuid}` | String (`pending` / `scanned` / `confirmed:uid`) | 2 分钟 | 维护移动端扫码与 Web 端轮询的交互状态。 |
| **OAuth2 授权** | `lite-sso:oauth2:code:{code}` | Hash (由 `go-oauth2` 接管) | ~5 分钟 | OIDC 标准流转的核心，临时授权码。 |
| **OAuth2 令牌** | `lite-sso:oauth2:access:{token}` | Hash (由 `go-oauth2` 接管) | 自定义 | 会话/Token 校验缓存。 |

---

## 5. 接口设计与流转说明

### 5.1 OIDC 标准协议接口

| Method | 路由路径 | 核心功能 |
| --- | --- | --- |
| GET | `/oauth/authorize` | **授权入口**。校验 Client 参数并展示登录页，成功后颁发 Code。 |
| POST | `/oauth/token` | **换取令牌**。子系统后端凭 Code (此时会在 Redis 中验真并销毁 Code) 换取 Access Token。 |
| GET | `/oauth/userinfo` | **获取资料**。凭 Token 获取当前用户基础信息。 |

### 5.2 前端业务认证接口

**1. 密码体系**

* `GET /api/auth/captcha` -> 生成图片，验证码存入 Redis。
* `POST /api/auth/login/password` -> 比对 Redis 中的验证码及 DB 中的密码。

**2. 邮箱无密码体系**

* `POST /api/auth/email/send` -> 检查 Redis 限流 Key，生成 OTP 存入 Redis 并发邮件。
* `POST /api/auth/login/email` -> 取出 Redis 的 OTP 进行对比。

**3. 扫码登录体系**

* `GET /api/auth/qr/generate` -> 生成 UUID，在 Redis 中设置 `pending` 状态。
* `GET /api/auth/qr/poll` -> Web 端轮询读取 Redis 中该 UUID 的最新状态。
* `POST /api/auth/qr/scan` -> 移动端调用，将 Redis 中状态改为 `scanned`。
* `POST /api/auth/qr/confirm` -> 移动端调用，将 Redis 状态改为 `confirmed:{user_id}`。

**4. 第三方登录体系**

* `GET /api/auth/third/{provider}` -> 跳转到第三方授权页面（GitHub/微信）
* `GET /api/auth/third/{provider}/callback` -> 第三方授权回调处理
* `POST /api/auth/third/bind` -> 绑定第三方账号到现有用户

### 5.3 API 响应规范

**成功响应格式：**
```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

**错误响应格式：**
```json
{
  "code": 400,
  "message": "错误描述",
  "data": null
}
```

**常用错误码：**
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未授权/Token过期
- `403`: 权限不足
- `429`: 请求频率过高
- `500`: 服务器内部错误

---

## 6. 安全配置规范

### 6.1 会话与令牌管理

* **Access Token 过期时间**: 12小时
* **无 Refresh Token 机制**: 简化设计，避免复杂的安全管理
* **Token 存储**: Redis Hash 结构，包含用户ID、权限范围、过期时间等信息

### 6.2 账户安全策略

* **密码哈希**: Bcrypt 算法，cost=12
* **账户锁定机制**: 可配置失败次数阈值（默认5次），锁定时间30分钟
* **登录失败记录**: Redis 记录失败次数，自动过期清理
* **无密码复杂度要求**: 简化用户体验

### 6.3 第三方登录配置

* **GitHub OAuth2**: 标准 OAuth2 授权码模式
* **微信登录**: 支持开放平台/公众号授权
* **第三方账号绑定**: 支持一个用户绑定多个第三方账号

---

## 7. 部署与运维

### 7.1 Docker 容器化部署

```yaml
# docker-compose.yml 示例
version: '3.8'
services:
  lite-sso:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_TYPE=sqlite
      - DB_PATH=/data/lite-sso.db
      - REDIS_URL=redis:6379
    volumes:
      - ./data:/data
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 50mb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

### 7.2 数据库迁移性设计

* **使用 GORM 接口抽象**: 避免 SQLite 特定语法
* **配置化数据库连接**: 通过环境变量切换数据库类型
* **数据库迁移脚本**: 支持版本化数据库结构变更
* **SQLite 文件挂载**: 数据持久化到宿主机

### 7.3 配置管理

**环境变量配置示例：**
```bash
# 数据库配置
DB_TYPE=sqlite
DB_PATH=/data/lite-sso.db

# Redis配置
REDIS_URL=redis:6379
REDIS_PASSWORD=

# 安全配置
ACCESS_TOKEN_EXPIRE=12h
MAX_LOGIN_ATTEMPTS=5
LOCKOUT_DURATION=30m

# 第三方应用配置
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
WECHAT_APP_ID=
WECHAT_APP_SECRET=
```

---

## 8. 系统解耦优势总结

引入 Redis 后，`lite-sso` 在工程上实现了三大飞跃：

1. **防重放与幂等性**：验证码、授权码 (`Code`) 在 Redis 中完成验证后，通过原子操作（如 `GETDEL` 或 Lua 脚本）直接删除，彻底杜绝重放攻击。
2. **分布式支持**：系统不再强依赖于单台服务器的物理内存。如果未来有高可用需求，可以通过 Nginx 将请求随机分发到多个 `lite-sso` Docker 容器上，因为状态全部汇聚在 Redis 中。
3. **运维便利性**：可以直接通过 Redis 可视化工具（如 Another Redis Desktop Manager）实时监控当前的活跃登录会话、扫码请求量以及正在流转的授权码，极大提升排错效率。

### 8.1 设计原则总结

* **简单优先**: 针对小规模用户场景，避免过度设计
* **容器化部署**: Docker 标准化部署，简化运维
* **配置驱动**: 环境变量配置，无需复杂配置文件
* **可扩展架构**: 保持代码可迁移性，为未来扩展预留空间