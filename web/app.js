const templates = {
  support: {
    label: "乐源售后助手",
    solutionName: "lecharm-support-agent",
    industry: "beverage",
    owner: "fde-zhouyuan",
    environment: "poc",
    goal: "知识问答 + 工单受理 + 人工升级",
    endpointPath: "/w2a/tickets",
    signalType: "ticket.created",
    knowledgeUri: "./data/lecharm/knowledge_units.jsonl",
    chatMessage: "葡萄汁出现沉淀怎么处理？",
    w2aDescription: "客户反馈葡萄汁出现沉淀，想确认是否还能饮用。",
  },
  qa: {
    label: "知识问答助手",
    solutionName: "knowledge-support-agent",
    industry: "general-support",
    owner: "fde-team",
    environment: "poc",
    goal: "基于知识库回答客户问题并返回引用",
    endpointPath: "/w2a/tickets",
    signalType: "ticket.created",
    knowledgeUri: "./data/knowledge/knowledge_units.jsonl",
    chatMessage: "保修期怎么计算？",
    w2aDescription: "客户咨询设备保修期如何计算。",
  },
  ticket: {
    label: "工单受理助手",
    solutionName: "ticket-triage-agent",
    industry: "after-sales",
    owner: "fde-team",
    environment: "poc",
    goal: "接收工单事件，识别风险并必要时转人工",
    endpointPath: "/w2a/tickets",
    signalType: "ticket.created",
    knowledgeUri: "./data/lecharm/knowledge_units.jsonl",
    chatMessage: "我需要人工客服处理投诉。",
    w2aDescription: "客户投诉瓶内有异物，要求升级处理。",
  },
};

const state = {
  runtime: null,
  selectedTemplate: "support",
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];

const elements = {
  runtimeStatus: $("#runtimeStatus"),
  runtimeSummary: $("#runtimeSummary"),
  currentSolutionLabel: $("#currentSolutionLabel"),
  refreshButton: $("#refreshButton"),
  templateList: $("#templateList"),
  solutionForm: $("#solutionForm"),
  formCompleteness: $("#formCompleteness"),
  solutionNameInput: $("#solutionNameInput"),
  industryInput: $("#industryInput"),
  ownerInput: $("#ownerInput"),
  environmentInput: $("#environmentInput"),
  goalInput: $("#goalInput"),
  endpointPathInput: $("#endpointPathInput"),
  signalTypeInput: $("#signalTypeInput"),
  knowledgeUriInput: $("#knowledgeUriInput"),
  solutionName: $("#solutionName"),
  environmentName: $("#environmentName"),
  chatPath: $("#chatPath"),
  w2aPath: $("#w2aPath"),
  tracePath: $("#tracePath"),
  manifestStatus: $("#manifestStatus"),
  manifestPreview: $("#manifestPreview"),
  validationChecks: $("#validationChecks"),
  chatForm: $("#chatForm"),
  chatMessage: $("#chatMessage"),
  chatResult: $("#chatResult"),
  w2aForm: $("#w2aForm"),
  w2aToken: $("#w2aToken"),
  w2aDescription: $("#w2aDescription"),
  w2aPayload: $("#w2aPayload"),
  w2aResult: $("#w2aResult"),
  traceRefreshButton: $("#traceRefreshButton"),
  traceList: $("#traceList"),
  traceDetail: $("#traceDetail"),
};

function pretty(value) {
  return JSON.stringify(value, null, 2);
}

function setRuntimeStatus(text, kind = "neutral") {
  elements.runtimeStatus.textContent = text;
  elements.runtimeStatus.dataset.kind = kind;
}

function primarySensor() {
  return state.runtime?.sensors?.find((sensor) => sensor.endpointPath) ?? null;
}

function formValues() {
  return {
    solutionName: elements.solutionNameInput.value.trim(),
    industry: elements.industryInput.value.trim(),
    owner: elements.ownerInput.value.trim(),
    environment: elements.environmentInput.value.trim(),
    goal: elements.goalInput.value.trim(),
    endpointPath: elements.endpointPathInput.value.trim(),
    signalType: elements.signalTypeInput.value.trim(),
    knowledgeUri: elements.knowledgeUriInput.value.trim(),
  };
}

function applyTemplate(key) {
  const template = templates[key] ?? templates.support;
  state.selectedTemplate = key;
  elements.currentSolutionLabel.textContent = template.label;
  elements.solutionNameInput.value = template.solutionName;
  elements.industryInput.value = template.industry;
  elements.ownerInput.value = template.owner;
  elements.environmentInput.value = template.environment;
  elements.goalInput.value = template.goal;
  elements.endpointPathInput.value = template.endpointPath;
  elements.signalTypeInput.value = template.signalType;
  elements.knowledgeUriInput.value = template.knowledgeUri;
  elements.chatMessage.value = template.chatMessage;
  elements.w2aDescription.value = template.w2aDescription;

  $$(".template-item").forEach((button) => {
    button.classList.toggle("template-item-active", button.dataset.template === key);
  });
  updateDerivedView();
}

function buildManifest(values) {
  return `apiVersion: solution.codex/v1
kind: Solution
solutionType: customer-support
metadata:
  name: ${values.solutionName || "new-solution"}
  version: 0.1.0
  owner: ${values.owner || "fde-team"}
  industry: ${values.industry || "general"}

perception:
  sensors:
    - id: ticket_webhook
      ref: "@world2agent/sensor-webhook@1.0.0"
      signalTypes:
        - ${values.signalType || "ticket.created"}
      config:
        endpointPath: ${values.endpointPath || "/w2a/tickets"}
  triggers:
    - id: ticket_triage
      sensor: ticket_webhook
      signalType: ${values.signalType || "ticket.created"}
      routeTo: classify_intent

knowledge:
  sources:
    - id: product_manuals
      type: jsonl
      uri: ${values.knowledgeUri || "./data/knowledge_units.jsonl"}
      schema: faq

  schemas:
    - id: faq
      fields:
        - question
        - answer
        - product_model
        - source_ref

components:
  - id: intent_classifier
    category: processor
    ref: registry.intent.beverage-router@1.0.0
    config:
      intents:
        - troubleshooting
        - warranty
        - complaint
        - human_handoff
  - id: retriever
    category: processor
    ref: registry.retriever.local-keyword@1.0.0
    config:
      topK: 5
      requireCitation: true
  - id: answer_generator
    category: processor
    ref: registry.agent.cited-answer@1.2.0
    config:
      style: concise
      requireGrounding: true

workflow:
  entrypoint: classify_intent
  inputMapping:
    chat:
      message: request.message
    w2a_signal:
      message: signal.source_event.data.description
      signalSummary: signal.event.summary
  nodes:
    - id: classify_intent
      component: intent_classifier
      inputs:
        message: inputs.message
    - id: retrieve_knowledge
      component: retriever
      inputs:
        query: inputs.message
    - id: generate_answer
      component: answer_generator
      inputs:
        message: inputs.message
        passages: retrieve_knowledge.passages
        citations: retrieve_knowledge.citations

runtime:
  modelPolicy:
    defaultModel: gpt-4o-mini
    maxLatencyMs: 30000
  observability:
    trace: required
    logInputs: yes

delivery:
  environments:
    - name: ${values.environment || "poc"}
      type: shared_sandbox
      config:
        modelKeyRef: env:OPENAI_API_KEY
        tracePath: ./.solution/traces`;
}

function validationItems(values) {
  return [
    {
      ok: Boolean(values.solutionName),
      text: "方案名称会生成 metadata.name",
    },
    {
      ok: values.endpointPath.startsWith("/"),
      text: "W2A endpointPath 以 / 开头",
    },
    {
      ok: Boolean(values.signalType),
      text: "Signal Type 已声明到 sensor 和 trigger",
    },
    {
      ok: values.knowledgeUri.endsWith(".jsonl"),
      text: "知识源使用 M1 支持的 JSONL",
    },
    {
      ok: Boolean(values.environment),
      text: "交付环境已配置",
    },
    {
      ok: Boolean(state.runtime),
      text: "已连接本地 runtime，可跑闭环",
    },
  ];
}

function renderValidation(values) {
  const items = validationItems(values);
  const ready = items.filter((item) => item.ok).length;
  elements.formCompleteness.textContent = `${ready}/${items.length} ready`;
  elements.manifestStatus.textContent = ready === items.length ? "ready" : "draft";
  elements.manifestStatus.classList.toggle("stage-badge-soft", ready !== items.length);

  elements.validationChecks.textContent = "";
  for (const item of items) {
    const li = document.createElement("li");
    li.className = item.ok ? "check-ok" : "check-warn";
    li.textContent = `${item.ok ? "OK" : "WAIT"} · ${item.text}`;
    elements.validationChecks.appendChild(li);
  }
}

function buildSampleSignal() {
  const values = formValues();
  const sensor = primarySensor();
  const now = new Date().toISOString();
  const signalID = `console-${Date.now()}`;
  return {
    schema_version: "w2a/0.1",
    signal_id: signalID,
    emitted_at: now,
    source: {
      sensor_id: sensor?.id ?? "ticket_webhook",
      sensor_version: "1.0.0",
      package: "@world2agent/sensor-webhook",
      source_type: "ticket-system",
      user_identity: "fde-console",
    },
    event: {
      type: sensor?.signalTypes?.[0] ?? values.signalType,
      occurred_at: now,
      summary: values.goal || "FDE console signal",
    },
    source_event: {
      data: {
        ticketId: `T-${Date.now().toString().slice(-6)}`,
        productModel: "Grape-Classic",
        description: elements.w2aDescription.value.trim() || templates[state.selectedTemplate].w2aDescription,
      },
    },
  };
}

function updateDerivedView() {
  const values = formValues();
  elements.manifestPreview.textContent = buildManifest(values);
  renderValidation(values);
  elements.w2aPayload.value = pretty(buildSampleSignal());
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
  });
  const payload = await response.json();
  if (!response.ok) {
    const error = new Error(payload?.error?.message ?? `HTTP ${response.status}`);
    error.payload = payload;
    throw error;
  }
  return payload;
}

async function loadRuntime() {
  setRuntimeStatus("Loading runtime");
  try {
    const runtime = await requestJSON("/api/runtime");
    state.runtime = runtime;
    const sensor = primarySensor();
    elements.solutionName.textContent = `${runtime.solution} ${runtime.version ?? ""}`.trim();
    elements.environmentName.textContent = runtime.environment || "-";
    elements.chatPath.textContent = runtime.chatPath || "/chat";
    elements.w2aPath.textContent = sensor?.endpointPath || "-";
    elements.tracePath.textContent = runtime.tracePath || "-";
    elements.runtimeSummary.textContent = `${runtime.environment || "poc"} · ${sensor?.endpointPath || "no W2A endpoint"}`;
    setRuntimeStatus("Runtime ready", "ok");
  } catch (error) {
    state.runtime = null;
    elements.runtimeSummary.textContent = error.message;
    setRuntimeStatus("Runtime unavailable", "error");
    elements.traceDetail.textContent = pretty(error.payload ?? { error: error.message });
  }
  updateDerivedView();
}

async function loadTraces(selectTraceID = "") {
  try {
    const traces = await requestJSON("/api/traces?limit=12");
    elements.traceList.textContent = "";
    if (!traces.length) {
      elements.traceList.textContent = "暂无 trace。先发送一次 Chat 或 W2A。";
      elements.traceDetail.textContent = "";
      return;
    }
    for (const trace of traces) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "trace-item";
      button.innerHTML = `<strong>${trace.traceId}</strong><span>${trace.status ?? "-"} · ${trace.trigger?.type ?? "-"}</span>`;
      button.addEventListener("click", () => loadTrace(trace.traceId));
      elements.traceList.appendChild(button);
    }
    await loadTrace(selectTraceID || traces[0].traceId);
  } catch (error) {
    elements.traceList.textContent = error.message;
  }
}

async function loadTrace(traceId) {
  if (!traceId) {
    return;
  }
  try {
    const trace = await requestJSON(`/api/traces/${encodeURIComponent(traceId)}`);
    elements.traceDetail.textContent = pretty(trace);
  } catch (error) {
    elements.traceDetail.textContent = pretty(error.payload ?? { error: error.message });
  }
}

elements.templateList.addEventListener("click", (event) => {
  const button = event.target.closest("[data-template]");
  if (button) {
    applyTemplate(button.dataset.template);
  }
});

elements.solutionForm.addEventListener("input", updateDerivedView);
elements.w2aDescription.addEventListener("input", updateDerivedView);

elements.chatForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  elements.chatResult.textContent = "Sending...";
  try {
    const path = state.runtime?.chatPath || "/chat";
    const response = await requestJSON(path, {
      method: "POST",
      body: JSON.stringify({ message: elements.chatMessage.value.trim() }),
    });
    elements.chatResult.textContent = pretty(response);
    await loadTraces(response.traceId);
  } catch (error) {
    elements.chatResult.textContent = pretty(error.payload ?? { error: error.message });
  }
});

elements.w2aForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  elements.w2aResult.textContent = "Sending...";
  try {
    const sensor = primarySensor();
    const path = sensor?.endpointPath || formValues().endpointPath;
    const headers = {};
    const token = elements.w2aToken.value.trim();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
    const payload = JSON.parse(elements.w2aPayload.value);
    const response = await requestJSON(path, {
      method: "POST",
      headers,
      body: JSON.stringify(payload),
    });
    elements.w2aResult.textContent = pretty(response);
    await loadTraces(response.traceId);
    updateDerivedView();
  } catch (error) {
    elements.w2aResult.textContent = pretty(error.payload ?? { error: error.message });
  }
});

elements.refreshButton.addEventListener("click", async () => {
  await loadRuntime();
  await loadTraces();
});

elements.traceRefreshButton.addEventListener("click", () => loadTraces());

applyTemplate("support");
loadRuntime().then(() => loadTraces());
