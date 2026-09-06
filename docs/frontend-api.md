# 国标麻将前端对接文档

这份文档是给客户端工程师单独拷走用的，不依赖服务端仓库的相对路径。  
拷走时请同时带走下面两个文件。

| 用途 | 本机绝对路径 |
|---|---|
| 本文档 | `/home/ros/work/GB_mahjong_server/docs/frontend-api.md` |
| 协议源文件（以后改协议以这个为准） | `/home/ros/work/GB_mahjong_server/api/proto/gbmj.proto` |
| 番种中文名对照（服务端源码） | `/home/ros/work/GB_mahjong_server/internal/rules/fanlist.go` |
| 开发环境配置（端口 18080） | `/home/ros/work/GB_mahjong_server/configs/dev.yaml` |
| 本机正式端口配置（8080，带 GM） | `/home/ros/work/GB_mahjong_server/configs/server.yaml` |
| 云服务器配置（8080，默认关 GM / pprof） | `/home/ros/work/GB_mahjong_server/configs/prod.yaml` |

服务端是两段：

1. **HTTP JSON**：只做注册、登录、探活。
2. **WebSocket 二进制 Protobuf**：鉴权之后的聊天、好友、房间、打牌、结算。

---

## 0. 完整访问地址（先记住这张表）

下面的 `127.0.0.1` 是本机联调地址。部署到别的机器时，只改主机，**后面的路径不要改**。

| 环境 | 配置文件 | HTTP 根地址 | WebSocket 完整路径 |
|---|---|---|---|
| 开发 | `configs/dev.yaml` | `http://127.0.0.1:18080` | `ws://127.0.0.1:18080/ws` |
| 本机正式端口 | `configs/server.yaml` | `http://127.0.0.1:8080` | `ws://127.0.0.1:8080/ws` |
| 云服务器 | `configs/prod.yaml` | `http://服务器IP:8080` | `ws://服务器IP:8080/ws` |

HTTP 三条完整路径（开发环境）：

| 功能 | 方法 | 完整路径 |
|---|---|---|
| 探活 | `GET` | `http://127.0.0.1:18080/healthz` |
| 注册 | `POST` | `http://127.0.0.1:18080/v1/register` |
| 登录 | `POST` | `http://127.0.0.1:18080/v1/login` |
| GM 网页（管理员，不是游戏客户端） | `GET` | `http://127.0.0.1:18080/gm` |

正式环境把端口 `18080` 换成 `8080` 即可，例如：

- `http://127.0.0.1:8080/v1/register`
- `http://127.0.0.1:8080/v1/login`
- `ws://127.0.0.1:8080/ws`

以后如果网站是 HTTPS，WebSocket 改成 `wss://主机/ws`，HTTP 改成 `https://主机/...`。

---

## 1. 推荐接入顺序

```
POST http://127.0.0.1:18080/v1/register
  → POST http://127.0.0.1:18080/v1/login   （拿到 uid + token）
  → 打开 ws://127.0.0.1:18080/ws
  → 8 秒内发第一条：C2S_AUTH（body 里带 token）
  → 收到 S2C_AUTH 且 code=0
  → 发 C2S_FRIEND_LIST 拉好友（登录后不会自动给好友列表）
  → 大厅：世界聊天 / 开房 / 加入 / 邀请
  → 入座、准备，房主开局
  → 收 S2C_DEAL、S2C_GAME_EVENT，用 C2S_ACTION 打牌
  → 收 S2C_SETTLE，房间还在，准备好可再开一局
```

同一账号再次登录：旧 Token 立刻失效，旧 WebSocket 会被踢掉。

---

## 2. HTTP 接口

公共约定：

- 请求头：`Content-Type: application/json`
- 请求体最大 4KB
- 注册成功 **不会** 返回 Token，必须再调登录

下面完整路径以开发环境为例。正式环境只改端口。

---

### 2.1 探活

- **完整路径**：`GET http://127.0.0.1:18080/healthz`
- **功能**：看服务器是否活着。前端一般不用。
- **传入参数**：无
- **返回消息**：
  - HTTP 200，正文纯文本 `ok`（不是 JSON）

---

### 2.2 注册

- **完整路径**：`POST http://127.0.0.1:18080/v1/register`
- **功能**：创建一个账号。成功后还要再调登录才能拿到 Token。

**传入参数（JSON）**

```json
{ "user": "alice", "pass": "secret" }
```

| 字段 | 类型 | 规则 |
|---|---|---|
| `user` | string | 3–16 位，只能字母、数字、下划线 |
| `pass` | string | 4–64 位 |

**返回消息**

| HTTP | 正文 | 含义 |
|---|---|---|
| 200 | `{"uid":1}` | 成功。`uid` 是用户数字 ID |
| 400 | `{"error":"bad json"}` | JSON 坏了 |
| 400 | `{"error":"invalid username or password"}` | 用户名或密码格式不对 |
| 405 | 文本 `method` | 不是 POST |
| 409 | `{"error":"exists"}` | 用户名已被占用 |

---

### 2.3 登录

- **完整路径**：`POST http://127.0.0.1:18080/v1/login`
- **功能**：校验账号密码，返回 Token。Token 要存下来，后面连 WebSocket 时用。

**传入参数（JSON）** 与注册相同：

```json
{ "user": "alice", "pass": "secret" }
```

**返回消息**

| HTTP | 正文 | 含义 |
|---|---|---|
| 200 | `{"uid":1,"token":"64位十六进制字符串"}` | 成功 |
| 400 | `{"error":"bad json"}` | JSON 坏了 |
| 401 | `{"error":"auth failed"}` | 账号不存在或密码错 |
| 405 | 文本 `method` | 不是 POST |

补充：

- Token 默认 **72 小时**有效。
- 新登录会使这个账号的旧 Token 全部失效。
- 昵称就是注册时的 `user`，没有改名接口。

---

### 2.4 GM 管理员页（游戏客户端不要调）

给服务器管理员填电脑用，不是登录/对局接口。开发环境完整路径：

- 网页：`GET http://127.0.0.1:18080/gm`
- 列表：`GET http://127.0.0.1:18080/gm/api/rooms`，请求头 `X-GM-Token: gm`
- 填充电脑：`POST http://127.0.0.1:18080/gm/api/addbot`，正文 `{"room_id":"012345"}`，同样带头

电脑坐下后，房间会推 `S2C_ROOM_STATE`，空位变成 `电脑1`…`电脑4`。怎么用看仓库 `README.md` 的「GM 填充电脑」。

---

## 3. WebSocket 怎么发包 / 收包

### 3.1 连接

- **完整路径**：`ws://127.0.0.1:18080/ws`
- **不要**把 Token 放在 URL 参数里，Token 放在连上后的第一条 Protobuf 里。
- 只发 **二进制帧**（Binary），不要发 JSON 文本帧。
- 服务端不拦网页来源（任意 Origin 都能连）。
- 连接数满了：HTTP **503**，正文 `full`，升不成 WebSocket。

时限：

| 情况 | 时间 |
|---|---|
| 连上后必须发出第一条 `C2S_AUTH` | 8 秒，否则连接直接断开（可能没有任何回包） |
| 鉴权之后两次客户端消息间隔 | 超过 120 秒没消息会断开 |
| 服务端 Ping | 每 30 秒一条 WebSocket Ping，请按标准回复 Pong |
| 行牌操作 | 默认 10 秒，超时由服务器代打/代过 |

### 3.2 每一帧都是一个 `Envelope`

协议定义在：`/home/ros/work/GB_mahjong_server/api/proto/gbmj.proto`

先解外层 `Envelope`，再按 `cmd` 解内层 `body`。

```
Envelope {
  uint32 seq  = 1;  // 客户端自己递增的序号
  Cmd    cmd  = 2;  // 见下面命令表
  bytes  body = 3;  // 内层消息的 Protobuf 二进制
  int32  code = 4;  // 0 成功；非 0 失败
}
```

**`seq` 规则**

- 客户端每发一条请求，`seq` 加 1（从 1 开始即可）。
- 只有少数回包会原样带回你的 `seq`：`S2C_AUTH`、`S2C_ERROR`、聊天被限频时的 `S2C_CHAT`。
- 服务器主动推送（房间状态、发牌、牌局事件、结算、好友同步、世界聊天广播）的 `seq` **固定为 0**。  
  前端不要用 `seq` 把推送和某次点击一一对应，要用 `cmd` + 内容。

**成功时通常没有「OK 回执」**

大部分请求成功后，不会回一条 `code=0` 的同名命令，而是推一条业务消息。例如：

- 开房成功 → `S2C_ROOM_STATE`（`seq=0`）
- 申请好友成功 → `S2C_FRIEND_SYNC`
- 出牌成功 → `S2C_GAME_EVENT`
- 过牌成功、还在等别人表态 → **什么也不推**（这是正常的）

失败才走 `S2C_ERROR`（少数例外见错误码）。

### 3.3 命令号 `Cmd`

前端生成代码时用枚举名；手写解码时用数字。

| 数字 | 枚举 | 方向 | 内层 body 类型 |
|---:|---|---|---|
| 0 | `CMD_UNSPECIFIED` | — | 不要发 |
| 1 | `C2S_AUTH` | 客户端→服务器 | `C2SAuth` |
| 2 | `S2C_AUTH` | 服务器→客户端 | `S2CAuth`（失败时 body 可空） |
| 10 | `C2S_CHAT` | 客户端→服务器 | `C2SChat` |
| 11 | `S2C_CHAT` | 服务器→客户端 | `S2CChat`（限频时 body 可空） |
| 20 | `C2S_FRIEND_ASK` | 客户端→服务器 | `C2SFriendAsk` |
| 21 | `C2S_FRIEND_RESP` | 客户端→服务器 | `C2SFriendResp` |
| 22 | `C2S_FRIEND_LIST` | 客户端→服务器 | 无 body |
| 23 | `S2C_FRIEND_SYNC` | 服务器→客户端 | `S2CFriendSync` |
| 40 | `C2S_ROOM_CREATE` | 客户端→服务器 | `C2SRoomCreate`（可空；可带 `turn_sec`/`extra_sec`） |
| 41 | `C2S_ROOM_JOIN` | 客户端→服务器 | `C2SRoomJoin` |
| 42 | `C2S_ROOM_SIT` | 客户端→服务器 | `C2SRoomSit` |
| 43 | `C2S_ROOM_KICK` | 客户端→服务器 | `C2SRoomKick` |
| 44 | `C2S_ROOM_INVITE` | 客户端→服务器 | `C2SRoomInvite` |
| 45 | `C2S_ROOM_START` | 客户端→服务器 | `C2SRoomStart`（空） |
| 46 | `S2C_ROOM_STATE` | 服务器→客户端 | `S2CRoomState` |
| 47 | `C2S_ROOM_LEAVE` | 客户端→服务器 | `C2SRoomLeave`（空） |
| 48 | `C2S_ROOM_READY` | 客户端→服务器 | `C2SRoomReady` |
| 80 | `C2S_ACTION` | 客户端→服务器 | `C2SAction` |
| 81 | `S2C_GAME_EVENT` | 服务器→客户端 | `S2CGameEvent` |
| 82 | `S2C_DEAL` | 服务器→客户端 | `S2CDeal` |
| 83 | `S2C_SETTLE` | 服务器→客户端 | `S2CSettle` |
| 90 | `S2C_ERROR` | 服务器→客户端 | 无 body，看 `code` |

WebSocket 没有 HTTP 那种 `/v1/xxx` 路径。所有消息都走同一个完整路径 `ws://127.0.0.1:18080/ws`，用 `Envelope.cmd` 区分功能。

### 3.4 错误码 `code`

| 数字 | 枚举 | 常见出现位置 | 含义 |
|---:|---|---|---|
| 0 | `CODE_OK` | 所有成功包 | 成功 |
| 1 | `CODE_AUTH_FAIL` | `S2C_AUTH` | Token 无效/过期，或首包不是鉴权 |
| 2 | `CODE_RATE_LIMITED` | `S2C_CHAT` | 聊天太快 |
| 3 | `CODE_PERM_DENIED` | `S2C_ERROR` | 没权限：不是房主却踢人/开局、邀请非好友、没入座却打牌、还没轮到你 |
| 4 | `CODE_NOT_FOUND` | `S2C_ERROR` | 房间不存在，或不在任何房间里却发了房间/出牌请求 |
| 5 | `CODE_BAD_STATE` | `S2C_ERROR` | 状态不对：非法吃碰杠胡、turn_id 过期、座位被占、未准备就开局、已在别的房间、对局中不能入座/准备/踢人 |
| 6 | `CODE_BAD_ACTION` | `S2C_ERROR` | 聊天内容不合法、好友参数不对、未知命令 |
| 7 | `CODE_ROOM_FULL` | `S2C_ERROR` | 房间人数到 8、座位有人、或全服房间数到上限 |
| 8 | `CODE_DUP` | 目前未使用 | 预留 |

注意：打牌动作不合法时，服务端回的是 **`code=5`**，不是 6。

鉴权失败回的是 `S2C_AUTH` + `code=1`，然后连接会断开，不是 `S2C_ERROR`。

---

## 4. 鉴权

所有 WebSocket 命令都发到同一个完整路径：`ws://127.0.0.1:18080/ws`。

### 4.1 客户端发 `C2S_AUTH`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 1`（`C2S_AUTH`）
- **功能**：用登录拿到的 Token 证明「我是谁」。必须是这条连接上的 **第一条**消息。

**传入参数（body = `C2SAuth`）**

```
C2SAuth { string token = 1; }
```

| 字段 | 说明 |
|---|---|
| `token` | `POST http://127.0.0.1:18080/v1/login` 返回的字符串 |

**返回消息**：见下一节 `S2C_AUTH`。

### 4.2 服务器回 `S2C_AUTH`

- **命令**：`cmd = 2`（`S2C_AUTH`）
- **功能**：告诉客户端鉴权结果。

**成功（`code=0`）body = `S2CAuth`**

```
S2CAuth { int64 uid = 1; string name = 2; }
```

| 字段 | 说明 |
|---|---|
| `uid` | 自己的用户 ID |
| `name` | 昵称（等于用户名） |

**失败（`code=1`）**：body 为空，随后连接关闭。

鉴权成功后，如果这个人本来就在某个房间里，会再推一条 `S2C_ROOM_STATE`。  
**对局中途重连不会补发手牌、副露、弃牌**，目前只能看到房间还在打（`phase≠0`）。第一版前端建议：断线就提示「对局中已托管」，重连后等结算。

---

## 5. 世界聊天

全服广播，不是房间内聊天。在线的所有人都能收到，包括自己。

### 5.1 客户端发 `C2S_CHAT`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 10`（`C2S_CHAT`）
- **功能**：发一条世界聊天。

**传入参数（body = `C2SChat`）**

```
C2SChat { bytes text = 1; }
```

| 字段 | 说明 |
|---|---|
| `text` | UTF-8 正文，**1–64 字节**（按字节算，不是按汉字个数。一个汉字通常 3 字节，大约 21 个汉字） |

**返回消息**

- 内容不合法（空，或超过 64 字节）→ `S2C_ERROR`，`code=6`
- 发太快 → `S2C_CHAT`，`code=2`，body 空（限频：大约 2 条/秒，瞬间最多 4 条）
- 成功 → 全服广播 `S2C_CHAT`（见下一节）

### 5.2 服务器推 `S2C_CHAT`

- **命令**：`cmd = 11`（`S2C_CHAT`）
- **功能**：把某个人说的话推给所有在线用户。

**返回消息（成功时 `seq=0`，`code=0`，body = `S2CChat`）**

```
S2CChat { int64 uid = 1; bytes text = 2; }
```

| 字段 | 说明 |
|---|---|
| `uid` | 谁说的 |
| `text` | 原文。**没有昵称**，要用 `uid` 自己查（好友列表里有名字；陌生人可先显示 uid） |

---

## 6. 好友

没有「删除好友」接口。申请对象是对方的 **uid**（数字），不是用户名。

### 6.1 申请好友 `C2S_FRIEND_ASK`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 20`（`C2S_FRIEND_ASK`）
- **功能**：向另一个用户发出好友申请。

**传入参数（body = `C2SFriendAsk`）**

```
C2SFriendAsk { int64 peer = 1; }
```

| 字段 | 说明 |
|---|---|
| `peer` | 对方 uid，不能是自己，必须 > 0 |

**返回消息**

- 成功：双方各收到 `S2C_FRIEND_SYNC`。对方的 `pending` 里会出现你。
- 已是好友再申请：不算失败，只是再同步一次。
- 失败（自己加自己等）→ `S2C_ERROR`，`code=6`。

### 6.2 同意/拒绝 `C2S_FRIEND_RESP`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 21`（`C2S_FRIEND_RESP`）
- **功能**：处理别人发给你的好友申请。

**传入参数（body = `C2SFriendResp`）**

```
C2SFriendResp { int64 peer = 1; bool accept = 2; }
```

| 字段 | 说明 |
|---|---|
| `peer` | 申请人 uid（应来自自己 pending 列表） |
| `accept` | `true` 同意，`false` 拒绝 |

**返回消息**：成功则双方各收到 `S2C_FRIEND_SYNC`。

### 6.3 拉取好友列表 `C2S_FRIEND_LIST`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 22`（`C2S_FRIEND_LIST`）
- **功能**：主动拉一次自己的好友和待处理申请。
- **传入参数**：无 body。
- **返回消息**：`S2C_FRIEND_SYNC`。

登录后请主动拉一次。上下线时，你的好友会收到同步，**你自己不会**因为上线而自动收到自己的列表。

### 6.4 服务器推 `S2C_FRIEND_SYNC`

- **命令**：`cmd = 23`（`S2C_FRIEND_SYNC`）
- **功能**：把最新好友列表推给你。

**返回消息（body = `S2CFriendSync`）**

```
FriendItem { int64 uid = 1; string name = 2; bool online = 3; }

S2CFriendSync {
  repeated FriendItem friends = 1;  // 已是好友
  repeated FriendItem pending = 2;  // 别人向你发起的、尚未处理的申请
}
```

`pending` 只有 **别人申请你** 的；你发出去还没被处理的申请，当前协议里看不到。

好友上线/下线时，服务端会把这份列表推给相关好友，用 `online` 更新绿点。

---

## 7. 房间

友谊房，6 位数字房号（不足 6 位前面补 0，例如 `000042`）。  
一个人同一时间只能在一个房间。房间最多 **8 个成员**（含旁观），对局座位 **4 个**。

所有房间命令都走：`ws://127.0.0.1:18080/ws`。

### 7.1 权限

| 谁 | 能做什么 |
|---|---|
| 房主 | 创建、踢人、开局；对局前房主离开 = 整房解散 |
| 成员 | 加入、入座、准备、离开、邀请好友 |
| 任何人 | 对局中途离开请求会被忽略（人还在座位上）；断线则该座位 **立刻托管** |

开局条件：4 个座位都有人，且 4 人都 `ready=true`，只有房主发 `C2S_ROOM_START`。

### 7.2 创建房间 `C2S_ROOM_CREATE`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 40`（`C2S_ROOM_CREATE`）
- **功能**：开一间友谊房。房主 **不会自动坐下**，需要再发入座。
- **传入参数**：`C2SRoomCreate`。两个字段现在都可以不填（0 = 用服务器默认 **10+20 秒**）。以后做房间设置页时再填。

```
C2SRoomCreate {
  uint32 turn_sec  = 1;  // 每回合免费决策秒数。0=默认 10。合法范围 3–120
  uint32 extra_sec = 2;  // 每人整局储备秒数。0=默认 20。合法范围 1–300
}
```

旧客户端继续发空 body 完全没问题，就是 10+20。

**返回消息**

- 成功：推 `S2C_ROOM_STATE`，从字段 `room_id` 读 6 位房号；`turn_sec` / `extra_sec` 是这间房实际用的秒数。
- 已在房间里再创建 → `S2C_ERROR`，`code=5`。
- 全服房间满 → `S2C_ERROR`，`code=7`。

### 7.3 加入房间 `C2S_ROOM_JOIN`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 41`（`C2S_ROOM_JOIN`）
- **功能**：凭房号进入房间。

**传入参数（body = `C2SRoomJoin`）**

```
C2SRoomJoin { string room_id = 1; }
```

| 字段 | 说明 |
|---|---|
| `room_id` | 6 位房号字符串，例如 `"000042"` |

**返回消息**

- 成功：房间内所有人收到 `S2C_ROOM_STATE`。
- 房号不对 → `code=4`。
- 已在另一个房间 → `code=5`。
- 人满（8 人）→ `code=7`。

### 7.4 入座 `C2S_ROOM_SIT`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 42`（`C2S_ROOM_SIT`）
- **功能**：坐到 0–3 号座位。第一版 **0 号位永远是庄/东**。

**传入参数（body = `C2SRoomSit`）**

```
C2SRoomSit { uint32 seat = 1; }
```

| 字段 | 说明 |
|---|---|
| `seat` | 0、1、2 或 3 |

**返回消息**

- 成功：房间内推 `S2C_ROOM_STATE`。
- 对局中不能换座 → `code=5`。
- 座位有别人 → `code=7`。

换座：服务端会先把自己从旧座位清掉，再坐新座位。

### 7.5 准备 `C2S_ROOM_READY`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 48`（`C2S_ROOM_READY`）
- **功能**：入座后点准备 / 取消准备。

**传入参数（body = `C2SRoomReady`）**

```
C2SRoomReady { bool ready = 1; }
```

| 字段 | 说明 |
|---|---|
| `ready` | `true` 准备，`false` 取消准备 |

**返回消息**：成功则推 `S2C_ROOM_STATE`。必须已经入座。对局中不能改准备。结算后服务端会把四人准备都清成 `false`，再开一局必须重新点准备。

### 7.6 开局 `C2S_ROOM_START`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 45`（`C2S_ROOM_START`）
- **功能**：房主开始一局。四人都入座且都准备才能成功。
- **传入参数**：空消息 `C2SRoomStart`。

**返回消息**

- 成功：四人各自收到 `S2C_DEAL`（只含自己的手牌），随后可能有补花事件，然后庄家出牌。
- 条件不满足 → `code=5`。
- 非房主 → `code=3`。

### 7.7 踢人 `C2S_ROOM_KICK`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 43`（`C2S_ROOM_KICK`）
- **功能**：房主在对局前把某人踢出房间。不能踢自己。

**传入参数（body = `C2SRoomKick`）**

```
C2SRoomKick { int64 uid = 1; }
```

| 字段 | 说明 |
|---|---|
| `uid` | 被踢者的用户 ID |

**返回消息**：房间内其他人收到更新后的 `S2C_ROOM_STATE`。被踢的人 **不会收到专门通知**。前端被踢后，下一次房间操作会失败。

### 7.8 邀请好友 `C2S_ROOM_INVITE`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 44`（`C2S_ROOM_INVITE`）
- **功能**：邀请一个 **已经是好友** 的人进房。不会自动把对方拉进来。

**传入参数（body = `C2SRoomInvite`）**

```
C2SRoomInvite { int64 uid = 1; }
```

| 字段 | 说明 |
|---|---|
| `uid` | 被邀请好友的用户 ID |

**返回消息**

- 不是好友 → `code=3`。
- 成功：对方收到一条 `S2C_ROOM_STATE`，其中 `room_id` 和 `invite_hint` 都是房号。前端看到 `invite_hint` 非空，就弹「好友邀请你进房 xxxxxx」，用户同意后再发 `C2S_ROOM_JOIN`。

### 7.9 离开房间 `C2S_ROOM_LEAVE`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 47`（`C2S_ROOM_LEAVE`）
- **功能**：对局前退出房间。
- **传入参数**：空消息 `C2SRoomLeave`。

**返回消息**

- 对局前普通人离开：房间内其他人收到 `S2C_ROOM_STATE`。
- 对局前 **房主离开则房间解散**（其他人目前没有解散推送，下一动作会 `code=4`）。
- 对局中离开：请求被忽略，人还在桌上。

### 7.10 服务器推 `S2C_ROOM_STATE`

- **命令**：`cmd = 46`（`S2C_ROOM_STATE`）
- **功能**：同步房间当前样子（房号、房主、四个座位、阶段）。

**返回消息（body = `S2CRoomState`）**

```
SeatInfo {
  int64  uid    = 1;
  string name   = 2;
  bool   ready  = 3;
  bool   online = 4;  // 当前版本始终为 false，不要用来画在线点
}

S2CRoomState {
  string room_id     = 1;  // 6 位房号
  int64  owner       = 2;  // 房主 uid
  repeated SeatInfo seats = 3;  // 固定 4 个，下标 0–3
  uint32 phase       = 4;  // 见下表
  string invite_hint = 5;  // 邀请时=房号，平时为空
  uint32 turn_sec    = 6;  // 每回合免费思考秒数，默认 10
  uint32 extra_sec   = 7;  // 每人整局储备秒数，默认 20
}
```

空座位：`uid=0`，`name=""`。

GM 填充的电脑：`name` 为 `电脑1`…`电脑4`（按座位），`uid` 是负数，`ready` 一般为 true。前端按普通人显示名字即可，不必特殊处理。对局中途不会再出现新电脑。

**`phase`（牌局阶段）**

| 值 | 含义 | 前端该做什么 |
|---:|---|---|
| 0 | 空闲（等人 / 刚结算完） | 显示房间、准备按钮 |
| 1 | 某人自己回合（要出牌/暗杠/加杠/自摸） | 若是自己，显示手牌操作 |
| 2 | 有人刚出牌，别人要表态（吃碰杠胡/过） | 若能操作，显示按钮 |
| 3 | 有人加杠，别人可抢杠胡 | 只显示胡 / 过 |
| 4 | 已终局（几乎一闪而过） | 随后会收到结算，然后 phase 回到 0 |

旁观成员（加入了房间但没入座）能收到 `S2C_ROOM_STATE`，**收不到**发牌和打牌事件。

---

## 8. 牌怎么编码

每张牌 1 个字节：`(花色 << 4) | 点数`。Protobuf 里 `uint32 tile` 只使用低 8 位。`0` 表示没牌。

| 花色 | 高 4 位 | 点数 | 例子（十六进制 = 十进制） |
|---|---|---|---|
| 万 | `1` | 1–9 | 一万 `0x11`=17，九万 `0x19`=25 |
| 条 | `2` | 1–9 | 一条 `0x21`=33，九条 `0x29`=41 |
| 饼 | `3` | 1–9 | 一饼 `0x31`=49，九饼 `0x39`=57 |
| 风 | `4` | 1东 2南 3西 4北 | 东 `0x41`=65，北 `0x44`=68 |
| 箭 | `5` | 1中 2发 3白 | 中 `0x51`=81，白 `0x53`=83 |
| 花 | `6` | 1梅 2兰 3竹 4菊 5春 6夏 7秋 8冬 | 梅 `0x61`=97，冬 `0x68`=104 |

手牌数组、事件里的 `tiles`、结算里的 `hands`，都是 **原始字节**，每个字节一张牌，不要当 UTF-8 字符串显示。

---

## 9. 对局

对局命令同样走：`ws://127.0.0.1:18080/ws`。

### 9.1 客户端出招 `C2S_ACTION`

- **通道**：`ws://127.0.0.1:18080/ws`
- **命令**：`cmd = 80`（`C2S_ACTION`）
- **功能**：出牌、吃、碰、杠、胡、过。

**传入参数（body = `C2SAction`）**

```
C2SAction {
  uint32     turn_id = 1;
  ActionType type    = 2;
  uint32     tile    = 3;
  uint32     chi_mid = 4;
}
```

| 字段 | 说明 |
|---|---|
| `turn_id` | 必须等于最近一次 `S2C_DEAL` / `S2C_GAME_EVENT` / `S2C_SETTLE` 里的 `turn_id`。过期会 `code=5` |
| `type` | 动作种类，见下表 |
| `tile` | 相关的那张牌（有的动作可省略） |
| `chi_mid` | 只有吃牌必填：顺子中间那张的编码 |

**`ActionType`（客户端真正能发的只有下面这些）**

| 数字 | 枚举 | 何时发 | `tile` | `chi_mid` |
|---:|---|---|---|---|
| 1 | `ACT_PASS` | 别人出牌后 / 抢杠时，你要过 | 不需要 | 不需要 |
| 2 | `ACT_DISCARD` | 自己回合出牌 | **必填**，打出的那张 | 不需要 |
| 3 | `ACT_CHI` | 上家刚打出的牌，且墙里还有牌 | 可省略（用刚打出的牌） | **必填**，顺子中间那张 |
| 4 | `ACT_PENG` | 别人刚打的牌，手里至少 2 张相同 | 可省略 | 不需要 |
| 5 | `ACT_GANG_MING` | 别人刚打的牌，手里 3 张相同，墙未空 | 可省略 | 不需要 |
| 6 | `ACT_GANG_AN` | 自己回合，手里 4 张相同 | **必填** | 不需要 |
| 7 | `ACT_GANG_JIA` | 自己回合，已碰且手里还有第 4 张 | **必填** | 不需要 |
| 8 | `ACT_HU` | 自摸 / 点和 / 抢杠 | 不需要 | 不需要 |

不要发：`ACT_DRAW`(9)、`ACT_BUHUA`(10)。那是服务器推给你的。  
不要用 `C2S_ACTION` 发入座/准备，那是房间命令。

**返回消息**

- 出牌/吃碰杠成功 → 推 `S2C_GAME_EVENT`（胡了则直接 `S2C_SETTLE`）。
- 过牌成功、还在等别人表态 → **什么也不推**。
- 非法动作 / `turn_id` 过期 → `S2C_ERROR`，`code=5`。
- 还没轮到你 / 没入座 → `S2C_ERROR`，`code=3`。

**吃牌 `chi_mid` 怎么填**

吃的是三张连续数牌，`chi_mid` 永远是 **中间那张**。

例：上家打出 **三万** `0x13`（十进制 19）：

| 你想吃成 | 你手里还要有 | `chi_mid` |
|---|---|---|
| 一二三万 | 一万、二万 | 二万 `0x12`（18） |
| 二三四万 | 二万、四万 | 三万 `0x13`（19） |
| 三四五万 | 四万、五万 | 四万 `0x14`（20） |

只能吃 **上家**（出牌人座位 + 1）的牌。风、箭、花不能吃。

**自己回合能做什么**

- 必须出牌（或先胡/暗杠/加杠）
- 吃碰之后的这一回合 **不能杠**
- 海底（墙上没牌了）不能暗杠/加杠
- 自摸胡：起和 **8 番**，花不计入这 8 番；报了但不够番或不成牌 = **错和**，整局立刻结束

**别人出牌后能做什么**

优先级：胡 > 碰/直杠 > 吃。多人要胡时，从点炮者逆时针最近的人获得。

**超时（默认 10+20 秒）**

每回合都有 **10 秒免费决策时间**。这 10 秒内出手，不扣后面的储备。  
10 秒用完后，开始扣这个人的 **20 秒储备**。这 20 秒是 **这个人整局共用的**，不会每回合重置；别人超时扣的是别人自己的 20 秒。  
储备也用完还不操作：自己回合服务器帮你打出刚摸的那张（没有则打手牌最后一张）；吃碰杠胡阶段帮你过。  
断线：该座位立刻进入托管，按上面规则自动打（不额外扣储备，如果还在免费 10 秒内）。

前端必须自己画倒计时，见下面 `wait_ms` / `turn_ms` / `extra_left_ms`。本地到 0 就关掉按钮，**不要等**服务器再推一条「过」。

服务端 **不会** 下发「你现在可以碰/吃」的按钮掩码（协议里有 `mask` 字段，但当前恒为 0）。前端要自己根据手牌和上一个事件算按钮。

### 9.2 发牌 `S2C_DEAL`

- **命令**：`cmd = 82`（`S2C_DEAL`）
- **功能**：开局把你自己的手牌发给你。每人一条，**看不到别人的手牌**。

**返回消息（body = `S2CDeal`）**

```
S2CDeal {
  uint32 seat    = 1;  // 你的座位 0–3
  bytes  hand    = 2;  // 自己手牌，每字节一张，已补花、已排序
  uint32 banker  = 3;  // 庄家座位，第一版恒为 0
  uint32 wind    = 4;  // 圈风，第一版恒为东 0x41
  uint32 turn_id = 5;  // 此后出牌用这个
  uint32 wait_ms = 6;  // 距被代打还剩多少毫秒（见 9.4）
  repeated uint32 extra_left_ms = 7;  // 固定 4 个数，四个座位剩余储备
  uint32 turn_ms = 8;  // 每回合免费毫秒，默认 10000
}
```

开局：闲家 13 张，庄家 14 张（都是补花后的张数）。庄家先打。

**补花动画注意：** `hand` 已经是补完花的结果。后面可能还会推 `ACT_BUHUA`。那些补花事件只用来显示「谁补了哪张花」，**不要再从 `hand` 里扣掉一次**，否则手牌会少。

### 9.3 牌局事件 `S2C_GAME_EVENT`

- **命令**：`cmd = 81`（`S2C_GAME_EVENT`）
- **功能**：把桌上发生的一张动作（出牌、吃碰杠、摸牌、补花）推给四人。

**返回消息（body = `S2CGameEvent`）**

```
S2CGameEvent {
  uint32     turn_id   = 1;
  ActionType type      = 2;
  uint32     seat      = 3;  // 动作发生在哪个座位
  uint32     tile      = 4;  // 关键的一张牌
  bytes      tiles     = 5;  // 额外牌，每字节一张
  uint32     wall_left = 6;  // 牌墙剩余张数
  uint32     mask      = 7;  // 当前恒为 0，忽略
  uint32     wait_ms   = 8;  // 距被代打/代过还剩多少毫秒（见 9.4）
  repeated uint32 extra_left_ms = 9;  // 固定 4 个数
  uint32     turn_ms   = 10; // 每回合免费毫秒
}
```

四人都会收到大部分事件。例外：

| type | 自己（该座位）看到 | 其他人看到 |
|---|---|---|
| `ACT_DRAW` 摸牌 | `tile`=摸到的牌 | `tile=0`，`tiles` 空（只知道谁摸了一张） |
| `ACT_GANG_AN` 暗杠 | `tile=0`，`tiles` 空（牌面保密） | 同样不给牌面，只知道谁暗杠了 |
| 其它 | 全桌相同 | 全桌相同 |

**各事件字段怎么用**

| type | `tile` | `tiles` | 前端更新 |
|---|---|---|---|
| `ACT_DISCARD` 出牌 | 打出的牌 | 空 | 从该座位手牌/刚摸张去掉，放入弃牌河 |
| `ACT_CHI` 吃 | 吃进来的那张（刚打出的牌） | 从手牌拿出的另外两张 | 该座位亮一副顺子；上家弃牌河拿走最后一张 |
| `ACT_PENG` 碰 | 碰的那张 | 从手牌拿出的两张 | 亮刻子；弃牌河拿走最后一张 |
| `ACT_GANG_MING` 直杠 | 杠的那张 | 从手牌拿出的三张 | 亮杠；随后通常有摸牌/补花 |
| `ACT_GANG_AN` 暗杠 | 0 | 空 | 该座位手牌少 4 张，亮扣着的杠（不要显示牌面） |
| `ACT_GANG_JIA` 加杠 | 加杠的那张 | 空 | 已有碰变成杠；进入抢杠阶段 |
| `ACT_DRAW` 摸牌 | 见上 | 空 | 只有自己把 `tile` 加入手牌 |
| `ACT_BUHUA` 补花 | 打出的花 | 空 | 该座位花数 +1；对局中若花来自手牌/摸张，要从手牌去掉 |
| `ACT_HU` | 一般 **不会** 作为游戏事件出现 | — | 胡了直接走 `S2C_SETTLE` |
| `ACT_PASS` | 一般 **不会** 广播 | — | 不要等「过」的动画 |

摸到花时：可能先看到 `ACT_BUHUA`，再看到最终那张非花的 `ACT_DRAW`。不要把花当普通牌打出去。

### 9.4 倒计时怎么画（10+20）

默认 **每回合 10 秒免费 + 每人整局 20 秒储备**。开房后看 `S2C_ROOM_STATE.turn_sec` / `extra_sec`。对局中每次 `S2C_DEAL`、`S2C_GAME_EVENT` 都会带当前剩余时间。

| 字段 | 谁有 | 含义 |
|---|---|---|
| `turn_ms` | DEAL / GAME_EVENT | 每回合免费时长，默认 10000 |
| `extra_left_ms` | 同上，**固定 4 个数** | 座位 0–3 各自还剩多少储备 |
| `wait_ms` | 同上，**每人一条不一样** | 如果你正在决策：你距被代打/代过还剩多久（免费+你还能用的储备）。如果你不用操作：当前最着急那位还剩多久 |

**建议算法（收到包的那一刻重新开始减）：**

```
收到 DEAL 或 GAME_EVENT：
  start = 本机当前时间
  remain0 = wait_ms
  myExtra = extra_left_ms[我的座位号]

每一帧：
  remain = remain0 - (现在 - start)
  若 remain <= 0：关掉我的出牌/吃碰杠胡按钮，不要再发动作
  否则：
    free = remain - myExtra   // 还在免费 10 秒里则 > 0
    若 free > 0：大数字显示 free，旁边「延时」显示 myExtra（不动）
    若 free <= 0：大数字和「延时」都显示 remain（已经在扣储备）
```

四个座位头像旁都可以写 `extra_left_ms[座位]/1000` 秒，让大家看见谁还剩多少延时。

注意：

- 别人点「过」服务器 **不会** 再推一包，你的 `wait_ms` 不用变。
- 补花 `ACT_BUHUA` 也会带 `wait_ms`，请用最新值覆盖倒计时，不要每次都当成重新 10+20。
- 电脑玩家会立刻出牌，倒计时可能一闪而过，这是正常的。
- 超时这一手被代打后，下一手这个人 **仍然有 10 秒免费**；只是储备不会补回来。

### 9.5 结算 `S2C_SETTLE`

- **命令**：`cmd = 83`（`S2C_SETTLE`）
- **功能**：一局结束，告诉四人谁赢、多少番、怎么拆分。四人各收到一份相同内容。随后会再推 `S2C_ROOM_STATE`（`phase=0`，准备全关）。

**返回消息（body = `S2CSettle`）**

```
FanItem { uint32 id = 1; uint32 score = 2; }

S2CSettle {
  uint32 winner      = 1;  // 和牌座位 0–3；荒庄/错和时不可靠，见下
  int32  hu_kind     = 2;
  int32  fan         = 3;  // 总番（含花）
  repeated FanItem fans = 4;
  repeated int32 scores = 5;  // 长度 4，座位 0–3 本局分差
  bytes  hands       = 6;  // 四人亮牌，见下
  uint32 discarder   = 7;  // 点炮/抢杠者座位；自摸/荒庄时不可靠
}
```

**`hu_kind`**

| 值 | 含义 | 怎么读 winner / discarder / scores |
|---:|---|---|
| 0 | 荒庄 | 不用看 winner。`scores` 全 0 |
| 1 | 自摸 | `winner` 是胡的人。`discarder` 可能是 0，**不要当成点炮者** |
| 2 | 点炮 | `winner` 胡牌，`discarder` 点炮 |
| 3 | 抢杠 | `winner` 胡牌，`discarder` 加杠的人 |
| 4 | 错和 | **`winner` 不可靠**（常为 0）。谁错和看 `scores`：该座位 **-30**，另外三家各 **+10** |

**拆账（给结算面板用）**

- 自摸：另外三家各付 `(8 + 总番)`
- 点炮/抢杠：点炮者付 `(8 + 总番)`，另外两家各付 `8`
- 错和：错和者 -30，其余 +10
- 荒庄：0

**`fans`**：番种列表。`id` 对照第 11 节中文名；`score` 是该番种的番数。客户端自己显示中文，服务端不下发名字。

**`hands` 亮牌格式**

四家剩余手牌用 `0xFF` 分隔：

```
座位0手牌 + 0xFF + 座位1手牌 + 0xFF + 座位2手牌 + 0xFF + 座位3手牌 + 0xFF
```

按 `0xFF` 切开得到 4 段，每一段里每个字节一张牌。副露不在这个字段里，前端应对局过程中自己记下吃碰杠。

---

## 10. 前端建议的本地状态

服务端不推「轮到谁、你可以点哪些按钮」，需要前端根据事件维护：

1. 记下自己的 `seat`、`hand`、四家 `melds`（吃碰杠）、四家 `discards`、四家 `flowers`、`wall_left`、`turn_id`。
2. 收到 `ACT_DRAW` 且 `tile≠0`：这是自己摸牌，进入「自己回合」。
3. 收到别人的 `ACT_DISCARD`：进入「表态阶段」。自己是出牌者下家才判断吃；人人判断碰/杠/胡。
4. 收到 `ACT_GANG_JIA` 且不是自己：进入「抢杠」，只判断胡或过。
5. 收到吃/碰且 `seat` 是自己：进入自己回合（只能出牌，不能杠）。
6. 发出 `PASS` 后不要等回执；等下一个事件即可。
7. `turn_id` 一变，作废上一回合还没发出去的点击。

座位门风（第一版庄=0）：0 东、1 南、2 西、3 北。圈风第一版永远是东。

---

## 11. 番种 id

与结算 `fans[].id` 对应。`0` 是无效，不要显示。  
中文名源码：`/home/ros/work/GB_mahjong_server/internal/rules/fanlist.go`

| id | 名称 | id | 名称 | id | 名称 |
|---:|---|---:|---|---:|---|
| 1 | 大四喜 | 2 | 大三元 | 3 | 绿一色 |
| 4 | 九莲宝灯 | 5 | 四杠 | 6 | 连七对 |
| 7 | 十三幺 | 8 | 清幺九 | 9 | 小四喜 |
| 10 | 小三元 | 11 | 字一色 | 12 | 四暗刻 |
| 13 | 一色双龙会 | 14 | 一色四同顺 | 15 | 一色四节高 |
| 16 | 一色四步高 | 17 | 三杠 | 18 | 混幺九 |
| 19 | 七对 | 20 | 七星不靠 | 21 | 全双刻 |
| 22 | 清一色 | 23 | 一色三同顺 | 24 | 一色三节高 |
| 25 | 全大 | 26 | 全中 | 27 | 全小 |
| 28 | 清龙 | 29 | 三色双龙会 | 30 | 一色三步高 |
| 31 | 全带五 | 32 | 三同刻 | 33 | 三暗刻 |
| 34 | 全不靠 | 35 | 组合龙 | 36 | 大于五 |
| 37 | 小于五 | 38 | 三风刻 | 39 | 花龙 |
| 40 | 推不倒 | 41 | 三色三同顺 | 42 | 三色三节高 |
| 43 | 无番和 | 44 | 妙手回春 | 45 | 海底捞月 |
| 46 | 杠上开花 | 47 | 抢杠和 | 48 | 碰碰和 |
| 49 | 混一色 | 50 | 三色三步高 | 51 | 五门齐 |
| 52 | 全求人 | 53 | 双暗杠 | 54 | 双箭刻 |
| 55 | 全带幺 | 56 | 不求人 | 57 | 双明杠 |
| 58 | 和绝张 | 59 | 箭刻 | 60 | 圈风刻 |
| 61 | 门风刻 | 62 | 门前清 | 63 | 平和 |
| 64 | 四归一 | 65 | 双同刻 | 66 | 双暗刻 |
| 67 | 暗杠 | 68 | 断幺 | 69 | 一般高 |
| 70 | 喜相逢 | 71 | 连六 | 72 | 老少副 |
| 73 | 幺九刻 | 74 | 明杠 | 75 | 缺一门 |
| 76 | 无字 | 77 | 边张 | 78 | 坎张 |
| 79 | 单钓将 | 80 | 自摸 | 81 | 花牌 |
| 82 | 明暗杠 | | | | |

起和：不含花至少 8 番；过门槛后再加花。错报和会整局按错和结算。

---

## 12. 规则摘要（做 UI 文案用）

- 144 张（含 8 花），没有王牌
- 开局补花后庄家 14 张先打
- 只抢加杠，不抢暗杠/直杠
- 最后一张是花，或牌墙摸尽无人胡：荒庄
- 第一版不做：四圈换座、金币、观战录像、短信登录

---

## 13. Protobuf 怎么接到网页 / 客户端

先把协议文件拷到客户端工程，源文件绝对路径：

```
/home/ros/work/GB_mahjong_server/api/proto/gbmj.proto
```

用 `protobufjs` 的例子（浏览器）。把下面的 `PROTO_PATH` 换成你拷到客户端后的实际绝对路径：

```javascript
import protobuf from "protobufjs";

const PROTO_PATH = "/home/ros/work/GB_mahjong_server/api/proto/gbmj.proto";
const root = await protobuf.load(PROTO_PATH);
const Envelope = root.lookupType("gbmj.Envelope");
const C2SAuth = root.lookupType("gbmj.C2SAuth");
const S2CAuth = root.lookupType("gbmj.S2CAuth");

function send(ws, seq, cmd, innerType, inner) {
  const body = innerType.encode(innerType.create(inner)).finish();
  const buf = Envelope.encode(
    Envelope.create({ seq, cmd, body, code: 0 })
  ).finish();
  ws.send(buf);
}

const ws = new WebSocket("ws://127.0.0.1:18080/ws");
ws.binaryType = "arraybuffer";
ws.onopen = () => {
  send(ws, 1, 1, C2SAuth, { token });
};
ws.onmessage = (ev) => {
  const env = Envelope.decode(new Uint8Array(ev.data));
  if (env.cmd === 2 && env.code === 0) {
    const auth = S2CAuth.decode(env.body);
    console.log("uid", auth.uid.toString(), auth.name);
  }
};
```

注意：`uid` 在 Protobuf 里是 `int64`，JS 里可能是 `Long`，显示前要转成字符串或 Number。

在服务端仓库里生成 JS 静态代码（输出目录请自己换成客户端工程的绝对路径）：

```bash
npx pbjs -t static-module -w es6 \
  /home/ros/work/GB_mahjong_server/api/proto/gbmj.proto \
  -o /绝对路径/客户端工程/gbmj.js
npx pbts /绝对路径/客户端工程/gbmj.js \
  -o /绝对路径/客户端工程/gbmj.d.ts
```

---

## 14. 联调时容易踩的坑

1. 注册完整路径是 `http://127.0.0.1:18080/v1/register`，成功后必须再登录 `http://127.0.0.1:18080/v1/login`。WebSocket 不能只用 uid。
2. 先连 `ws://127.0.0.1:18080/ws`，首包必须是 `C2S_AUTH`，超时 8 秒。
3. 只发 Binary Protobuf，不要发 JSON。
4. 成功往往没有 ACK，要等对应推送；`PASS` 可能完全没有推送。
5. `S2C_DEAL.hand` 已含补花结果，后面的 `ACT_BUHUA` 不要再扣手牌。
6. 暗杠和别人的摸牌没有牌面。
7. 非法出牌的错误码是 `5` 不是 `6`。
8. 结算 `hu_kind` 优先于 `winner`。错和看分数 -30。
9. `SeatInfo.online` 和 `S2CGameEvent.mask` 现在没用。
10. 对局中重连没有补状态，只能等结算或提示托管中。倒计时也只在 `DEAL`/`GAME_EVENT` 里，重连后没有单独的补时钟包。
11. 踢人、房主解散，对方可能收不到通知。
12. 世界聊天按 **字节** 限 64，不是 64 个字。
13. 必须用新的 `gbmj.proto` 重新生成代码，旧生成文件没有 `wait_ms` / `turn_sec`。空的 `C2S_ROOM_CREATE` 仍然合法（就是默认 10+20）。
14. 本地倒计时到 0 就关按钮。过牌本来就不会推送，超时代过同样没有推送。

---

## 15. 最小房间流程（四人）

开发环境完整地址：HTTP `http://127.0.0.1:18080`，WebSocket `ws://127.0.0.1:18080/ws`。

1. 四人分别：`POST http://127.0.0.1:18080/v1/register` → `POST http://127.0.0.1:18080/v1/login` → 连接 `ws://127.0.0.1:18080/ws` → `C2S_AUTH`。
2. A 发 `C2S_ROOM_CREATE` → 从 `S2C_ROOM_STATE.room_id` 读到房号。
3. B/C/D 发 `C2S_ROOM_JOIN`，`room_id` 填刚拿到的 6 位房号。
4. 四人发 `C2S_ROOM_SIT`（`seat` 分别为 0/1/2/3），再发 `C2S_ROOM_READY { ready: true }`。
5. 房主 A 发 `C2S_ROOM_START`。
6. 各自处理 `S2C_DEAL` / `S2C_GAME_EVENT`，用最新 `turn_id` 发 `C2S_ACTION`。
7. 收到 `S2C_SETTLE` 结束本局。
