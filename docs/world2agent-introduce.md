# World2Agent (W2A) — 项目技术全景

[World2Agent](https://github.com/machinepulse-ai/world2agent) 是一个开源协议，标准化了 AI Agent 感知真实世界的方式。核心理念：World → Sensor → Agent —— 传感器监听数据源，按统一 Schema 发射结构化信号，Agent 接收信号后决策行动。

## 一、协议核心：Signal 信封

所有传感器输出的信号遵循同一个 W2ASignalL73-L111 接口，当前版本 w2a/0.1。信封结构包含：

| 字段 | 必须 | 职责 |
| --- | --- | --- |
| signal_id | ✅ | UUID v4，去重和追踪 |
| schema_version | ✅ | 协议版本，消费者必须拒绝不认识的版本 |
| emitted_at | ✅ | 传感器发射时间 (UTC ms) |
| source | ✅ | 发射者身份：sensor_id(npm包名)、sensor_version、source_type(平台分组)、user_identity、package |
| event | ✅ | 标准化事件分类 + 自然语言摘要 |
| source_event | 可选 | 自描述原始数据：schema(JSON Schema draft-07) + data |
| attachments | 可选 | 内容附件（Tagged Union：inline 内联 / reference 外部引用） |
| _meta | 可选 | 扩展点，消费者必须忽略未知键 |

关键设计约束：

event.summary 是信号的灵魂 —— Agent 只读这一行就必须能决定是否行动。遵循 Actor→Action→Object→Context→Impact 模式
event.type 遵循 domain.entity.action 三段式命名，action 必须是过去时动词，domain 是抽象源空间而非平台名
event vs source_event vs attachments 三者严格分离：标准化分类 / 结构化机器数据 / 非结构化内容

## 二、技术栈

| 层面 | 技术选型 | 说明 |
| --- | --- | --- |
| 协议定义 | TypeScript (自包含，无运行时依赖) |  |
| Schema 生成 | typescript-json-schema | 从 TS 类型生成 schema.json，用于校验 |
| Schema 校验 | ajv + ajv-formats |  |
| 传感器开发 | @world2agent/sdk (TypeScript + Zod) | defineSensor() 定义传感器，createSignal() 构造信号，z.object() 定义配置 Schema |
| 传感器分发 | npm | 每个传感器是独立 npm 包，npm publish 即发布，SensorHub 是发现层 |
| 传输层 | stdout pipe / HTTP POST / WebSocket / SSE / 自定义 | stdoutTransport()、httpTransport() 等，支持 fanout 多目标 |
| Agent 运行时桥接 | Claude Code Plugin / Hermes / OpenClaw | 三种已支持的一等公民运行时，信号注入机制各异 |
| 包管理 | npm (@world2agent scope) | 协议包 @world2agent/protocol，SDK @world2agent/sdk，传感器 @world2agent/sensor-* |
| CI 校验 | npm run validate:examples | 对示例文件做 JSON Schema 校验 |
| 版本策略 | 目录版本化 (schema/0.1/) | 破坏性变更升目录，增量变更原地修改 |

## 三、传感器开发范式

一个传感器约 50 行代码，核心模式如下（参见 build-a-sensor.md）：

```ts
import { defineSensor } from "@world2agent/sdk/sensor";
import { createSignal } from "@world2agent/sdk";
import { z } from "zod";

export default defineSensor({
  id: "my-sensor",
  version: "0.1.0",
  source_type: "my-source",
  auth: { type: "api_key", fields: [{ name: "token", label: "API Token", sensitive: true }] },
  configSchema: z.object({ token: z.string() }),
  async start(ctx) {
    const interval = setInterval(async () => {
      const data = await fetchMySource(ctx.config.token);
      const signal = createSignal(this, { event: {...}, source_event: {...}, attachments: [...] });
      await ctx.emit(signal);
    }, 60_000);
    return () => clearInterval(interval); // cleanup
  },
});
```

传感器生命周期：定义 → start() → 周期性 emit → cleanup。start 返回清理函数，消费者负责调用。

## 四、多传感器组合与传输

通过 runAll() 同时运行多个传感器，用 fanout() 将信号分发到多个传输目标：

```ts
import { runAll } from "@world2agent/sdk/sensor";
import { fanout, stdoutTransport, httpTransport } from "@world2agent/sdk/transports";

await runAll([
  { spec: github, config: { token: "xxx" } },
  { spec: steam, config: { userId: "xxx" } },
], {
  onSignal: fanout([ stdoutTransport(), httpTransport({ url: "..." }) ]),
});
```

也可 CLI 管道：w2a-sensor-github | your-agent

## 五、架构全景

```text
World (APIs, Calendars, GitHub, X, Steam, ...)
  │
  ▼
Sensors (npm packages, W2A Protocol)
  │   ←→ SensorHub (发现/发布/安装)
  │
  ├── Direct path → Agent
  │
  └── Graph path (Roadmap) → Graph Layer (compose/enrich/filter) → Agent
```

Graph 层当前为 RFC 状态，核心约束是 输入输出均为 W2A Protocol，因此对 Agent 完全透明。部署方式：自托管或第三方托管（如 Karpo）。

## 六、Agent 运行时集成

| 运行时 | 桥接方式 | 信号注入机制 |
| --- | --- | --- |
| Claude Code | Plugin (world2agent) | MCP 工具 + plugin channel |
| Hermes | @world2agent/hermes-sensor-bridge | Webhook → AIAgent.run_conversation() |
| OpenClaw | @world2agent/openclaw-sensor-bridge | Webhook → /hooks/agent handler |
| 任意运行时 | CLI pipe | stdout → stdin |

## 七、安全模型

传感器信号驱动 Agent 感知和行动，不可信传感器等同于不可信指令源
仅安装来源可信的开源传感器，安装前审查代码
schema_version 强制版本协商，消费者拒绝未知版本
