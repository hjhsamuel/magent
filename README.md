# magent

多 agent runtime 任务执行平台。各 runtime 独立执行，互不通信；Web Server 通过 HTTP 接收操作，通过 SSE 向用户推送消息。

## 整体架构

```text
                      ╭─────────────────────╮
                      │     用户 / Web      │
                      ╰─────────────────────╯
                           │           ▲
              HTTP 请求    │           │    SSE 推送
          提交任务 · 审批  │           │  进度 · 审批 · 结果
                           ▼           │
                      ╭─────────────────────╮
                      │       magent        │
                      │     Web Server      │
                      ╰─────────────────────╯
                                 ⇅
                        下发任务 · 回传事件
                                 ⇅
           ┌─────────────────────┼─────────────────────┐
           ⇅                     ⇅                     ⇅
  ╭─────────────────╮   ╭─────────────────╮   ╭─────────────────╮
  │      Codex      │   │    Runtime B    │   │    Runtime C    │
  ╰─────────────────╯   ╰─────────────────╯   ╰─────────────────╯

                 各 runtime 独立执行，彼此不交互
```

## 模块关联

```text
Web 接入层 [规划]
    |
    +-- HTTP API --------------------------------------+
    |                                                  |
    +-- SSE <--- 事件分发 [规划] <--- 统一事件流          |
                         |                             v
                         +-------------------> internal/service
                                               [业务实现待补充]
                                                      |
                 +------------------------------------+------------------+
                 |                                    |                  |
                 v                                    v                  v
internal/runtime/manager.go                  Store 访问层 [规划]      pkg/fx
    |                                                 |             下载 / 归档
    +-- Manager                                       v
    |   +-- shard[0..15]                     internal/store/schema
    |       +-- connectionID -> ManagedRuntime        |
    |                                                 +-- Session
    +-- ManagedRuntime                                +-- Turn / Message
        +-- Events: 独立消费与有界缓冲                 +-- Skill
        +-- Turn: 按 sessionID + turnID 聚合正文        +-- MCP
        +-- Approval: 透传审批请求与清理通知
        |
        v
internal/runtime/runtime.go
    Runtime 接口 / Session / Turn / Event / Approval
        ^
        | 实现接口
        +--------------------------------------------+
        |                                            |
internal/runtime/codex                        其他适配器 [规划]
    |
    +-- runtime.go           会话 / 轮次 / 回复 / 关闭
    +-- events.go            协议事件 -> 统一事件
    +-- mapping.go           协议对象 -> 平台对象
    +-- message.go           双向 RPC / 请求响应关联
    +-- transport.go         消息传输接口
        +-- runtime_stdio.go
        +-- runtime_socket.go
        +-- runtime_websocket.go
```

## 请求与推送链路

```text
任务提交

  User --HTTP--> API --> Service --> ManagedRuntime --> Adapter --> Runtime
                           |
                           +--> Store: 保存任务与 runtime 标识映射


执行事件

  Runtime --> Adapter --> ManagedRuntime --> Event Dispatcher
                              |                      |
                              |                      +--> Service / Store
                              |                      |
                              |                      +--> SSE --> User
                              |
                              +-- 正文增量
                              +-- 审批请求 / 请求清理
                              +-- 轮次结束 + 最后一条助手正文


审批闭环

  Runtime --审批请求--> Adapter --> ManagedRuntime --> Dispatcher --SSE--> User
     ^                                                                    |
     |                                                                    |
     +--- Respond(原请求 ID) <--- 原连接 <--- Service <--- API <---HTTP-------+
                                                归属与有效状态校验


连接关闭

  Service --> Manager.Remove / Close
                         |
                         v
                  ManagedRuntime.Close
                         |
                         v
                  Codex Runtime.Close
                         |
                         +--> 停止接受新建 / 恢复会话
                         |
                         +--> 退订已登记会话（共用 3 秒超时）
                         |
                         +--> 关闭连接（退订失败也执行）
                         |
                         +--> 返回关闭结果 / 退订错误
```

## 事件与连接隔离

```text
                            Runtime Manager
                                   |
                 +-----------------+-----------------+
                 |                 |                 |
              shard 0           shard 1          shard 15
              独立锁            独立锁            独立锁
                 |                 |                 |
           connection A      connection B      connection C
                 |                 |                 |
           event worker A    event worker B    event worker C
                 |                 |                 |
           bounded buffer    bounded buffer    bounded buffer
                 |                 |                 |
                 +-----------------+-----------------+
                                   |
                                   v
                       Event Dispatcher [规划]
                       每条事件流由单一组件消费
                                   |
                    按用户 / 任务 / 会话分发
                                   |
                 +-----------------+-----------------+
                 |                 |                 |
            SSE buffer A      SSE buffer B      SSE buffer C
              [规划]            [规划]            [规划]
                 |                 |                 |
               User A            User B            User C

  connection ID + session ID + turn ID --> 执行归属
  connection ID + request ID           --> 审批回复路由

  轮次结束  --> 业务层判断任务是否完成
  SSE 断开  --> 仅解除推送订阅 [规划]，不隐式关闭 runtime
  连接关闭  --> 退订并释放连接，不等同于删除会话或取消远端任务
```
