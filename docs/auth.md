# Ceddit 双令牌认证系统

## 概述

Ceddit 采用 **Access Token + Refresh Token** 双令牌认证方案：

- **Access Token**：JWT，短期有效（默认 30 分钟），无状态验证。客户端通过 `Authorization: Bearer <token>` 请求头携带。服务端不存储，仅靠签名验证。
- **Refresh Token**：随机字符串，长期有效（默认 15 天），存储在 Redis 中。客户端通过 `httpOnly` Cookie 存储和传输。服务端可主动撤销，实现强制下线。

## 为什么用双令牌

| 单 Token（之前） | 双 Token（现在） |
|---|---|
| Access token 有效期 60 天，泄露后无法撤销 | Access token 仅 30 分钟，泄露窗口极小 |
| 服务端无状态，无法强制用户下线 | Refresh token 存 Redis，可随时删除实现下线 |
| 长期 token 在每次请求中暴露 | 短期 access token 频繁暴露，长期 refresh token 只偶尔传输 |

## 令牌格式

### Access Token（JWT）

```json
{
  "user_id": 10359911913361408,
  "username": "testuser",
  "exp": 1780065395,
  "iss": "ceddit"
}
```

- 签名算法：HMAC-SHA256
- 有效期：`jwt.access_token_ttl` 分钟（默认 30）
- 签名密钥：`jwt.secret`（配置文件）

### Refresh Token

- 生成方式：`crypto/rand` 生成 32 字节随机数，经 Base64URL 编码为 43 字符字符串
- 有效期：`jwt.refresh_token_ttl` 天（默认 15）
- 特点：无结构，纯随机字符串，不可伪造

## Redis 数据结构

采用**双重索引**设计：

```
# 索引一：按 token 查详情
Key:   ceddit:refresh:<token_string>
Type:  String
Value: JSON
TTL:   15 天

# 索引二：按用户查所有 token
Key:   ceddit:user_tokens:<user_id_hex>
Type:  Set (成员为 token_string)
TTL:   15 天（每次写入时重置）
```

### Refresh Token Value (JSON)

```json
{
  "user_id": 10359911913361408,
  "username": "testuser",
  "device_id": "abc123...",
  "created_at": 1780063595,
  "expires_at": 1781359595
}
```

### 为什么是双重索引

- **单用 `refresh:<token>`**：验证 token 有效 + 获取用户信息很快。但要"登出所有设备"就需要扫描所有 key，O(N)。
- **单用 `user_tokens:<user_id>`**：登出所有设备很快。但要验证 token 就需要遍历 set 的所有成员去匹配，O(M)。
- **两者组合**：验证 O(1)，登出 O(M)（M 为用户设备数，通常很小）。

### 双重写入的原子性

创建和轮换操作同时修改两个索引，通过 **Lua 脚本**保证原子性，避免以下竞态：

1. 写入 `refresh:<token>` 成功但 `SADD user_tokens:<uid>` 失败 → token 无法被批量撤销
2. 轮换时删除旧 token 成功但写入新 token 失败 → 用户被意外踢下线

## API 接口

### POST /api/v1/signup — 注册

```
Request:
{
  "username": "testuser",
  "password": "pass123",
  "re_password": "pass123",
  "device_id": "optional-device-id"   // 可选，不传则服务端生成
}

Response (200):
{
  "code": 1000,
  "msg": "success",
  "data": {
    "access_token": "eyJhbG...",
    "expires_in": 1800              // 秒
  }
}
Set-Cookie: refresh_token=<token>; Path=/api/v1; Max-Age=1296000; HttpOnly
```

注册成功后自动登录，直接返回双令牌。

### POST /api/v1/login — 登录

请求和响应格式同 signup。

### POST /api/v1/refresh — 刷新令牌

```
Request:
  (无请求体，refresh token 从 Cookie 自动带上)

Response (200):
{
  "code": 1000,
  "msg": "success",
  "data": {
    "access_token": "eyJhbG...",   // 新的 access token
    "expires_in": 1800
  }
}
Set-Cookie: refresh_token=<new_token>; Path=/api/v1; Max-Age=1296000; HttpOnly
```

Refresh token 每次使用后**轮换（rotation）**：旧的被删除，新的写入。一旦检测到旧 token 在 Redis 中不存在（可能是泄露后被攻击者使用并轮换了），返回错误要求重新登录。

### POST /api/v1/logout — 登出

```
Request:
  (无请求体，refresh token 从 Cookie 自动带上)

Response (200):
{
  "code": 1000,
  "msg": "success",
  "data": null
}
Set-Cookie: refresh_token=; Path=/api/v1; Max-Age=-1; HttpOnly   // 清除 cookie
```

登出行为：删除该用户的**所有** refresh token（所有设备登出）。无 cookie 时也返回成功（幂等）。

### 受保护接口

所有业务接口（community、post、vote）需要 access token：

```
GET /api/v1/community
Authorization: Bearer <access_token>
```

错误码：

| Code | 含义 | 应对 |
|------|------|------|
| 1006 | `need auth` — 未提供 token | 跳转登录 |
| 1007 | `invalid auth` — token 无效 | 跳转登录 |
| 1008 | `auth expired` — token 过期 | 调用 `/refresh` 换取新 token |

## 完整流程图

```
用户                    前端                    服务端                    Redis
 |                       |                       |                        |
 |  输入用户名密码        |                       |                        |
 |---------------------->|                       |                        |
 |                       |  POST /api/v1/login   |                        |
 |                       |---------------------->|                        |
 |                       |                       |  GenToken(user, pass)  |
 |                       |                       |  generateRefreshToken()|
 |                       |                       |----------------------->|
 |                       |                       |  Lua: SET refresh:<t>  |
 |                       |                       |  Lua: SADD user:<uid>  |
 |                       |                       |<-----------------------|
 |                       |  {access_token,       |                        |
 |                       |   expires_in}          |                        |
 |                       |  Set-Cookie: refr...   |                        |
 |                       |<----------------------|                        |
 |                       |                       |                        |
 |  === 30分钟后 access token 过期 ===             |                        |
 |                       |                       |                        |
 |                       |  POST /api/v1/refresh |                        |
 |                       |  Cookie: refr=<old>   |                        |
 |                       |---------------------->|                        |
 |                       |                       |  GET refresh:<old>     |
 |                       |                       |----------------------->|
 |                       |                       |  (验证存在)            |
 |                       |                       |<-----------------------|
 |                       |                       |                        |
 |                       |                       |  生成新 refresh token  |
 |                       |                       |  生成新 access token   |
 |                       |                       |----------------------->|
 |                       |                       |  Lua: DEL refresh:<old>|
 |                       |                       |  Lua: SET refresh:<new>|
 |                       |                       |  Lua: SREM old / SADD  |
 |                       |                       |<-----------------------|
 |                       |  {access_token,       |                        |
 |                       |   expires_in}          |                        |
 |                       |  Set-Cookie: refr=<n> |                        |
 |                       |<----------------------|                        |
 |                       |                       |                        |
 |  === 用户主动登出 ===   |                       |                        |
 |                       |                       |                        |
 |                       |  POST /api/v1/logout  |                        |
 |                       |  Cookie: refr=<t>     |                        |
 |                       |---------------------->|                        |
 |                       |                       |  GET refresh:<t>       |
 |                       |                       |----------------------->|
 |                       |                       |  拿到 user_id          |
 |                       |                       |<-----------------------|
 |                       |                       |----------------------->|
 |                       |                       |  Lua: SMEMBERS set      |
 |                       |                       |  Lua: DEL each refresh |
 |                       |                       |  Lua: DEL set           |
 |                       |                       |<-----------------------|
 |                       |  {success}            |                        |
 |                       |  Set-Cookie: (clear)  |                        |
 |                       |<----------------------|                        |
```

## Lua 脚本

项目中三个 Lua 脚本在 `pkg/auth/lua.go`：

### luaCreateTokens

```
SET refresh:<new_token> <json_value> EX <ttl>
SADD user_tokens:<user_id> <new_token>
EXPIRE user_tokens:<user_id> <ttl>
```

### luaRefreshTokens

```
EXISTS refresh:<old_token>                    // 不存在则返回 -1（泄露）
DEL refresh:<old_token>
SET refresh:<new_token> <json_value> EX <ttl>
SREM user_tokens:<user_id> <old_token>
SADD user_tokens:<user_id> <new_token>
EXPIRE user_tokens:<user_id> <ttl>
```

### luaRevokeAll

```
SMEMBERS user_tokens:<user_id>
for each token: DEL refresh:<token>
DEL user_tokens:<user_id>
```

## 安全考量

### 已处理

- **强制下线**：通过 Redis 双重索引，logout 可删除用户所有 refresh token
- **Token 轮换**：每次 refresh 生成新 refresh token，旧 token 即时失效
- **泄露检测**：refresh 时发现旧 token 不存在（已被他人轮换），返回 `ErrTokenLeaked`
- **Cookie 安全**：refresh token 仅存在于 `httpOnly` Cookie，JS 无法读取，防御 XSS
- **密码存储**：加盐 MD5（预置 `secret` 常量作为 salt）
- **Lua 原子操作**：双重索引的写入不会出现不一致

### 当前未处理（已知 trade-off）

- **Access token 30 分钟暴露窗口**：access token 无法主动撤销。如果用户在 12:00 被强制下线，攻击者拿到的 access token 在 12:30 前仍然有效。这是无状态 JWT 的固有权衡——牺牲即时撤销换取无需每次请求查 Redis 的性能。
- **密码哈希**：用的是 MD5 + 固定 salt，不够安全。后续可升级为 bcrypt。
- **HTTPS**：当前 `SetCookie` 中 `Secure=false`，部署到生产环境需改为 `true`（需 HTTPS）。

## 配置

```yaml
# config.yaml
jwt:
  secret: "cabbage"          # JWT 签名密钥（生产环境改为强随机值）
  access_token_ttl: 30      # Access token 有效期（分钟）
  refresh_token_ttl: 15     # Refresh token 有效期（天）
```

## 代码结构

```
pkg/auth/
  auth.go       # CreateTokens, RefreshTokens, RevokeAllTokens
                # SetRefreshCookie, ClearRefreshCookie
                # GetRefreshTokenFromCookie, GetRefreshValue
                # TokenPair, RefreshValue 结构体
  lua.go        # Lua 脚本常量
  redis.go      # Redis 操作封装（双重索引读写）

pkg/jwt/
  jwt.go        # GenToken, ParseToken（从 viper 读取配置）

middleware/
  auth.go       # JWTAuthMiddleware（区分过期 vs 无效）

service/
  user.go       # SignUp, LogIn, RefreshTokens, RevokeUserTokens

controller/
  user.go       # SignUpHandler, LogInHandler, RefreshHandler, LogoutHandler

models/
  paramSignUp.go  # +DeviceID
  paramLogIn.go   # +DeviceID
```

## 前端对接指南

### 登录/注册后

1. 从 response body 拿到 `access_token` 和 `expires_in`
2. 将 `access_token` 存在内存（或 sessionStorage）
3. `expires_in` 用于设置刷新定时器（提前 1-2 分钟刷新）
4. refresh token 由浏览器自动管理（httpOnly Cookie）

### 发送 API 请求

```javascript
fetch('/api/v1/community', {
  headers: {
    'Authorization': `Bearer ${accessToken}`
  },
  credentials: 'include'  // 让 cookie 自动带上
})
```

### Access token 过期时

```javascript
// 收到 code 1008 时
const res = await fetch('/api/v1/refresh', {
  method: 'POST',
  credentials: 'include'
})
const { data } = await res.json()
accessToken = data.access_token  // 更新内存中的 token
// 重试原始请求
```

### 登出

```javascript
await fetch('/api/v1/logout', {
  method: 'POST',
  credentials: 'include'
})
// 清除内存中的 access_token，跳转到登录页
```
