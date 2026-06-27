const state = {
  runtime: null,
};

const elements = {
  runtimeStatus: document.querySelector("#runtimeStatus"),
  solutionName: document.querySelector("#solutionName"),
  environmentName: document.querySelector("#environmentName"),
  chatPath: document.querySelector("#chatPath"),
  w2aPath: document.querySelector("#w2aPath"),
  tracePath: document.querySelector("#tracePath"),
  chatForm: document.querySelector("#chatForm"),
  chatMessage: document.querySelector("#chatMessage"),
  chatResult: document.querySelector("#chatResult"),
  w2aForm: document.querySelector("#w2aForm"),
  w2aPayload: document.querySelector("#w2aPayload"),
  w2aResult: document.querySelector("#w2aResult"),
  traceList: document.querySelector("#traceList"),
  traceDetail: document.querySelector("#traceDetail"),
  refreshButton: document.querySelector("#refreshButton"),
  traceRefreshButton: document.querySelector("#traceRefreshButton"),
};

function pretty(value) {
  return JSON.stringify(value, null, 2);
}

function setStatus(text, isError = false) {
  elements.runtimeStatus.textContent = text;
  elements.runtimeStatus.classList.toggle("error", isError);
}

function primarySensor() {
  return state.runtime?.sensors?.find((sensor) => sensor.endpointPath) ?? null;
}

function buildSampleSignal() {
  const sensor = primarySensor();
  const now = Date.now();
  return {
    schema_version: "w2a/0.1",
    signal_id: `console-${now}`,
    emitted_at: now,
    source: {
      sensor_id: sensor?.id ?? "ticket_webhook",
      sensor_version: "1.0.0",
      source_type: "ticket-system",
      user_identity: "customer-10086",
      package: "@world2agent/sensor-webhook",
    },
    event: {
      type: sensor?.signalTypes?.[0] ?? "ticket.created",
      occurred_at: now,
      summary: "Customer reported pump E42 error",
    },
    source_event: {
      data: {
        ticketId: "T-10086",
        productModel: "LC-200",
        description: "咖啡机显示 E42，萃取过程中停止工作。",
      },
    },
  };
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
    ...options,
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
  setStatus("Loading");
  try {
    const runtime = await requestJSON("/api/runtime");
    state.runtime = runtime;
    const sensor = primarySensor();
    elements.solutionName.textContent = `${runtime.solution} ${runtime.version ?? ""}`.trim();
    elements.environmentName.textContent = runtime.environment || "-";
    elements.chatPath.textContent = runtime.chatPath || "/chat";
    elements.w2aPath.textContent = sensor?.endpointPath || "-";
    elements.tracePath.textContent = runtime.tracePath || "-";
    elements.w2aPayload.value = pretty(buildSampleSignal());
    setStatus("Ready");
  } catch (error) {
    setStatus("Unavailable", true);
    elements.traceDetail.textContent = pretty(error.payload ?? { error: error.message });
  }
}

async function loadTraces() {
  try {
    const traces = await requestJSON("/api/traces?limit=12");
    elements.traceList.textContent = "";
    if (!traces.length) {
      elements.traceList.textContent = "No traces yet.";
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
  } catch (error) {
    elements.traceList.textContent = error.message;
  }
}

async function loadTrace(traceId) {
  try {
    const trace = await requestJSON(`/api/traces/${encodeURIComponent(traceId)}`);
    elements.traceDetail.textContent = pretty(trace);
  } catch (error) {
    elements.traceDetail.textContent = pretty(error.payload ?? { error: error.message });
  }
}

elements.chatForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  elements.chatResult.textContent = "Sending...";
  try {
    const path = state.runtime?.chatPath || "/chat";
    const payload = await requestJSON(path, {
      method: "POST",
      body: JSON.stringify({ message: elements.chatMessage.value }),
    });
    elements.chatResult.textContent = pretty(payload);
    await loadTraces();
  } catch (error) {
    elements.chatResult.textContent = pretty(error.payload ?? { error: error.message });
  }
});

elements.w2aForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  elements.w2aResult.textContent = "Sending...";
  try {
    const sensor = primarySensor();
    const path = sensor?.endpointPath;
    if (!path) {
      throw new Error("No W2A endpoint is configured.");
    }
    const payload = JSON.parse(elements.w2aPayload.value);
    const response = await requestJSON(path, {
      method: "POST",
      body: JSON.stringify(payload),
    });
    elements.w2aResult.textContent = pretty(response);
    await loadTraces();
  } catch (error) {
    elements.w2aResult.textContent = pretty(error.payload ?? { error: error.message });
  }
});

elements.refreshButton.addEventListener("click", async () => {
  await loadRuntime();
  await loadTraces();
});

elements.traceRefreshButton.addEventListener("click", loadTraces);

loadRuntime().then(loadTraces);
