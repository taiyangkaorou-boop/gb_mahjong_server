# 国标麻将服务端

单进程 Go 服务：HTTP 登录 + WebSocket/Protobuf 对局。友谊房单局，SQLite 存用户/好友，算番复用 [zheng-fan/GB-Mahjong](https://github.com/zheng-fan/GB-Mahjong)（CGO）。

当前发布：`v0.0.1`。GitHub：[taiyangkaorou-boop/GB_mahjong_server](https://github.com/taiyangkaorou-boop/GB_mahjong_server)。

- `main`：已发布快照（打过 tag 的版本）
- `dev`：日常改代码请在这个分支

```bash
git clone --recurse-submodules https://github.com/taiyangkaorou-boop/GB_mahjong_server.git
cd GB_mahjong_server
git checkout dev    # 要改代码时切到开发分支
```

算番库是子模块 `third_party/GB-Mahjong`。如果这个目录是空的，执行：`git submodule update --init --recursive`。

## 功能一览

| 功能 | 说明 | 怎么用 |
|---|---|---|
| 注册/登录 | 用户名+密码，返回 Token | `POST /v1/register` `POST /v1/login` |
| 长连接鉴权 | WS 首包带 Token | `C2S_AUTH` |
| 好友 | 申请、同意、在线同步、邀请进房 | `C2S_FRIEND_*` `C2S_ROOM_INVITE` |
| 世界聊天 | 每用户 2 条/秒，突发 4，正文最多 64 字节 | `C2S_CHAT` |
| 房间 | 6 位房号，房主开局/踢人/解散 | `C2S_ROOM_*` |
| GM 填充电脑 | 网页把房间空位填成电脑，AI 与内置机器人相同 | 浏览器打开 `/gm` |
| 对局 | 国标单局：补花、庄先打、吃碰杠胡、截和、荒庄 | `C2S_ACTION` |
| 结算 | 8 番起和（不含花）、底分 8 拆账、错和 -30 | `S2C_SETTLE` |

## 前端对接

做客户端时请直接看（可整份拷走）：

- 对接文档：`docs/frontend-api.md`
- 协议文件：`api/proto/gbmj.proto`

文档里每条接口都写了完整 URL（例如 `http://127.0.0.1:18080/v1/login`、`ws://127.0.0.1:18080/ws`）、功能、入参和回包。

## 环境

- Go 1.18+（需 CGO）
- g++（C++11）
- protoc（改协议时需要）

```bash
chmod +x scripts/*.sh
./scripts/gen_proto.sh          # 生成 internal/pb
go test ./pkg/tile ./internal/settle ./internal/game ./internal/ai
go test ./cgo/gbmj              # 会编译 C++ 算番库
go test ./...
go build -o server ./cmd/server
./server configs/server.yaml
# 另开终端
go run ./cmd/bot -addr http://127.0.0.1:8080 -n 4
```

健康检查：`curl http://127.0.0.1:8080/healthz` 应返回 `ok`。

数据文件默认 `./data/gbmj.db`。本机 8080 被占用时可改 `http_addr`，或用 `./server configs/dev.yaml`（18080）。pprof：`http://127.0.0.1:8080/debug/pprof/`。

GM 页面：服务器起来后用浏览器打开 `http://127.0.0.1:18080/gm`（开发端口）或 `http://127.0.0.1:8080/gm`。密码填配置里的 `gm_token`，默认 `gm`。把 `gm_token` 留空则关闭 GM。

## 怎么测

### 1. 单元测试（不需要先开服务器）

在仓库根目录：

```bash
go test ./...
```

算番相关测试会编译 C++，需要 g++。改协议后先 `./scripts/gen_proto.sh`。

`cmd/bot`（智能机器人）、`cmd/stress`（压测工具）和 `internal/pb`（自动生成的协议代码）没有单测，这是正常的。

### 2. 带策略的自动一局（不用开 HTTP）

四个 AI 在同一张牌桌上打到结算，会吃碰杠胡，能和则和，不会乱报不够 8 番的胡：

```bash
go test ./internal/ai -count=1 -timeout 60s
go test ./internal/ai -run TestBankerHuOnStackedWall -v   # 叠好的牌墙，庄家必须自摸
go test ./internal/ai -run TestFourBotsOneHand -v         # 随机牌墙打完一局
```

`TestBankerHuOnStackedWall` 必须胡。随机牌墙不保证每局都胡（国标 8 番门槛），但必须打到结算。

### 3. 真服务器功能测试（四个机器人打一局）

注册登录 → WebSocket 鉴权 → 开房入座准备 → 开局打牌 → 收到结算。机器人与上面同一套策略。

需要两个终端。

**终端 1，启动服务器（开发端口 18080）：**

```bash
go build -o bin/server ./cmd/server && go build -o bin/bot ./cmd/bot
./bin/server configs/dev.yaml
```

看到 `listen :18080` 就成功了。可再检查：`curl http://127.0.0.1:18080/healthz` 应返回 `ok`。

**终端 2，放四个机器人进去打：**

```bash
./bin/bot -addr http://127.0.0.1:18080 -n 4
```

成功：四个机器人各自打印一行 `settle ok`，带 `kind/fan/scores` 以及自己这局的 `chi/peng/gang/hu` 次数，命令退出码为 0。  
`kind`：0 荒庄（国标 8 番门槛下很常见）、1 自摸、2 点炮、3 抢杠。日志里除了出牌，还可能出现吃碰杠胡，不保证每一局都胡。  
失败：有人打印 `bot(s) failed`，一直停在 `deal` 没有结算，或 `kind=4` 错和（报了不够 8 番、或副露坏了）。改过服务端后要重新 `go build -o bin/server` 再启动，否则机器人连的还是旧进程。

好友和世界聊天看：`go test ./internal/social ./cmd/server`。

### 4. GM 填充电脑（需要先开服务器）

人不够 4 个时，用网页把空座位填成电脑。电脑和 `cmd/bot` 用同一套 `internal/ai`，会吃碰杠胡。

1. 启动服务器：`./bin/server configs/dev.yaml`
2. 游戏里开房、自己入座（也可以先不入座）
3. 浏览器打开 `http://127.0.0.1:18080/gm`
4. 密码填 `gm`，点保存
5. 填 6 位房号，点「填充电脑」（或先点「刷新列表」再点该房间的按钮）
6. 游戏里应看到空位变成「电脑1」…「电脑4」，并且已经准备
7. 真人点准备后，房主开局即可

也可以不用网页：

```bash
curl -s -H 'X-GM-Token: gm' http://127.0.0.1:18080/gm/api/rooms
curl -s -H 'X-GM-Token: gm' -H 'Content-Type: application/json' \
  -d '{"room_id":"012345"}' http://127.0.0.1:18080/gm/api/addbot
```

对局已经开始时不能再加电脑。四个座位都有人时会提示房间满。结算后电脑仍保持准备，方便再开一局。电脑的 `uid` 是负数，客户端显示座位上的 `name` 即可。

### 5. 压力测试（需要先开服务器）

压测程序是笨的：打牌只打最后一张、别人出牌一律过，这样压的是服务器而不是本机算番。不要用 `./bin/bot` 开很多房，那个会先把压测机 CPU 打满。

先抬高本机文件句柄（连接一多否则会 `too many open files`）：

```bash
ulimit -n 65535
go build -o bin/server ./cmd/server && go build -o bin/stress ./cmd/stress
./bin/server configs/dev.yaml
```

另一个终端，**从轻到重**，看清一档再加码（不要一上来 2000 房）：

```bash
./bin/stress -addr http://127.0.0.1:18080 -mode login -n 100
./bin/stress -addr http://127.0.0.1:18080 -mode idle  -n 200 -sec 30
./bin/stress -addr http://127.0.0.1:18080 -mode play  -rooms 10
./bin/stress -addr http://127.0.0.1:18080 -mode chat  -n 50 -qps 10 -sec 20
```

成功：最后一行 `stress ...` 汇总，退出码 0。`play` 必须每房都结算，`wrong_hu` 必须是 0（笨机器人不会报胡；`kind=0` 荒庄很正常）。  
失败：有 `fail`/`drop`/`login_fail`，或进程非 0 退出。10 房稳定后再试 `-rooms 20`、`50`。

看服务器卡在哪（本机已开 pprof）：

```bash
curl -s http://127.0.0.1:18080/healthz
go tool pprof http://127.0.0.1:18080/debug/pprof/profile?seconds=20
go tool pprof http://127.0.0.1:18080/debug/pprof/heap
```

登录很慢多半是 bcrypt + SQLite 单连接；人多一聊天就卡是世界聊天要广播给所有人。账号名形如 `st00001`，重跑会直接登录已有用户。

## HTTP

### POST /v1/register

参数 JSON：`user` 3–16 位字母数字下划线，`pass` 4–64 位。

返回：`{"uid":1}`。重名 409。

### POST /v1/login

同上。返回：`{"uid":1,"token":"..."}`。同一账号新登录会使旧 Token 失效。

### GET /healthz

返回 `ok`。

### GM（管理员网页，不是给游戏客户端用的）

配置了 `gm_token` 才会开启。开发/正式配置默认密码是 `gm`。

| 功能 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 网页 | `GET` | `/gm` | 几个按钮：填房号、填充电脑、刷新房间列表 |
| 房间列表 | `GET` | `/gm/api/rooms` | 请求头 `X-GM-Token` |
| 填充电脑 | `POST` | `/gm/api/addbot` | JSON `{"room_id":"012345"}`，把头里带 `X-GM-Token` |

成功时 `addbot` 返回该房间四个座位，`added` 是这次新坐下的电脑数量。电脑昵称 `电脑1`…`电脑4`，进房后自动准备。

## WebSocket `/ws`

每个二进制帧是一条 `Envelope`（见 `api/proto/gbmj.proto`）。

```
Envelope { seq, cmd, body, code }
```

必须先发 `C2S_AUTH`，body 为 `{ token }`。成功回 `S2C_AUTH { uid, name }`。

`code != 0` 表示失败：1 鉴权 2 频控 3 没权限 4 找不到 5 状态不对 6 非法动作 7 房间满。

### 聊天

- 请求 `C2S_CHAT { text }`，`text` 最长 64 字节 UTF-8。
- 广播 `S2C_CHAT { uid, text }`，不带昵称。超频回 `code=2`。

### 好友

- `C2S_FRIEND_ASK { peer }` 申请
- `C2S_FRIEND_RESP { peer, accept }` 同意/拒绝
- `C2S_FRIEND_LIST` 拉取
- 推送 `S2C_FRIEND_SYNC { friends, pending }`，含在线状态

### 房间权限

房主：创建（已创建者）、解散（对局前离开即解散）、开局、踢人。  
成员：加入、入座、准备、邀请好友进房。  
对局中离开不拆房，服务器按超时托管（自动打牌/过）。

开局：4 人入座且都准备，仅房主 `C2S_ROOM_START`。

### 战斗动作 `C2S_ACTION`

| type | 含义 | 额外字段 |
|---|---|---|
| DISCARD | 出牌 | tile |
| CHI | 吃（仅上家打的牌） | chi_mid=顺子中间牌编码 |
| PENG / GANG_MING | 碰 / 直杠 | tile 可省略（用刚打出的牌） |
| GANG_AN / GANG_JIA | 暗杠 / 加杠 | tile |
| HU | 和 | |
| PASS | 过 | |

`turn_id` 必须等于服务器最近事件里的值，过期包会被丢弃。

服务器事件：`S2C_DEAL` 只给本人手牌；`S2C_GAME_EVENT` 出牌/副露全桌，暗杠不带牌面，摸牌只给本人看张。终局 `S2C_SETTLE`：`hu_kind` 0荒庄 1自摸 2点炮 3抢杠 4错和；`fans[].id` 对照下面番种表。

## 牌编码

1 字节：`(花色<<4)|点数`

- 花色 1万 2条 3饼 4风 5箭 6花
- 例：一万 `0x11`，九饼 `0x39`，东 `0x41`，中 `0x51`，梅 `0x61`

## 规则（第一版）

- 144 张（含 8 花），头摸尾补，不留王牌
- 开局补花后庄家 14 张先打
- 8 番起和，花不计入门槛，过门槛后再加花
- 胡 > 碰/直杠 > 吃；截和（点炮者逆时针最近）
- 只抢加杠；吃碰当回合不能杠
- 最后一张是花则荒庄
- 错和立刻终盘：错和 -30，其余 +10
- 自摸：三家各付 `(8+番)`；点炮：点炮者 `(8+番)`，另两家各 8
- 操作时限 10 秒（`configs/server.yaml` 可改）；断线后该座位立即托管（超时打牌 / 过）

## 配置 `configs/server.yaml`

- `http_addr` 监听地址
- `data_dir` SQLite 目录
- `token_ttl_hours` 登录有效期
- `action_timeout_ms` 行牌超时
- `reconnect_sec` 预留（当前新连接直接顶掉旧连接）
- `max_conns` / `max_rooms` 上限
- `chat_max_bytes` 聊天长度
- `pprof` 是否挂到默认 HTTP 上（本进程用 DefaultServeMux，默认开启）
- `gm_token` GM 网页密码；留空则关闭 `/gm`。公网请改成自己的密码
- `log_level` 日志级别，默认 `info`。可选：`trace` `info` `warn` `error` `fatal`

## 日志

日志打到标准错误（终端里能看见），英文前缀，前面带级别。例：`[INFO] room started room=123456`。和牌时番种用中文名（和国标番种表一致）。

| 级别 | 你会看到什么 | 进程会不会停 |
|---|---|---|
| `trace` | 每个函数怎么走（打牌、房间命令、协议分发）。房间很多时不要开，会刷屏 | 否 |
| `info` | 启动、登录成功、开房/开局/结算。有人和牌时会打出番种，例如 `fans=七对(19):24,自摸(80):1` | 否 |
| `warn` | 密码错、房间满、限流、非法动作、顶号、托管代打。还能继续跑 | 否 |
| `error` | 数据库/编码/算番库出错，或某条连接、某一桌 panic 被接住 | 否 |
| `fatal` | 建目录、打开数据库、监听端口失败。这时没法提供服务 | **是，进程退出** |

默认 `info`。排错时把 yaml 改成 `log_level: trace` 再启动服务器。不要把密码或完整 Token 写进日志。

某一桌或某条 WebSocket 里出了意料之外的 panic，服务器会记一条 error 并继续跑（那一桌可能会被拆掉），不会把整个进程打死。

## 工程结构

- `cmd/server` 进程入口（含 `/gm` 管理员页）
- `internal/logx` 分级日志（trace/info/warn/error/fatal）和 panic recover
- `cmd/bot` 四机器人对打验收（与 `internal/ai` 同一套策略）
- `cmd/stress` 压力测试：登录/挂机/多房笨打/聊天好友
- `internal/ai` 座位视角 + 吃碰杠胡决策
- `api/proto` 协议
- `internal/game` 牌局状态机（不算番）
- `internal/settle` 起和番与拆账
- `internal/rules` 算番接口
- `cgo/gbmj` 唯一 CGO 包，桥接 C++ 库
- `third_party/GB-Mahjong` 上游源码（MIT）
- `pkg/tile` 1 字节编解码

## 单元测试覆盖

| 包 | 覆盖的要点 |
|---|---|
| `pkg/tile` | 42 张往返、编码、牌墙张数、删牌/排序/门风 |
| `cgo/gbmj` | 上游 18/332 番样例、解析失败、副露字符串、听牌 |
| `internal/rules` | 番名表、CGO 引擎对接 |
| `internal/settle` | 自摸/点炮/错和拆账、花不计入起和番 |
| `internal/game` | 庄先打、截和、禁杠、海底、荒庄、错和、抢杠胡、托管代打 |
| `internal/ai` | 8 番才胡、打孤张、上家吃、碰碰和型碰、叠牌墙自摸、四人对打一局 |
| `internal/room` | 房主权限、满员、入座准备、开局发牌出牌、对局中断线不离座、GM 填充电脑 |
| `internal/auth` / `persist/sqlite` | 注册登录、Token 过期、好友表 |
| `internal/social` | 聊天令牌桶、申请同意拒绝 |
| `internal/netx` / `config` / `user` / `logx` | 协议编解码、配置、昵称、日志级别与 recover |
| `cmd/server` | `/healthz` 注册登录 HTTP、WS 鉴权失败/成功、聊天、开房、GM 填充电脑 |

## 番种 id

与库 `fan_t` 一致，见 `internal/rules/fanlist.go`（1 大四喜 … 81 花牌，82 明暗杠）。客户端用 id 自己显示中文，服务端结算包不重复下发长名字。

## 第一版不做

四圈换座、金币排位、观战录像、多进程、Redis/MySQL、短信登录。

GM 以后还可以加：踢出电脑、改牌、跳过等待。现在只有「填充电脑」。电脑不会聊天、不能当房主。如果服务器挂在公网，请立刻改掉 `gm_token`，不需要 GM 时把这一项留空。

日志目前只打到终端，没有写成文件、也不会按天切割。人多或压测时保持 `info`，不要开 `trace`。
