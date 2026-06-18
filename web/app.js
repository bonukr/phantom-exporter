"use strict";

const state = {
    groups: [],
    selectedGroupId: null,
    metrics: [],
    editingMetricId: null,
    editingGroupId: null,
};

// ---- API helpers ----

async function api(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
        opts.headers["Content-Type"] = "application/json";
        opts.body = JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    if (!res.ok) {
        let msg = res.statusText;
        try { msg = (await res.json()).error || msg; } catch (_) {}
        throw new Error(msg);
    }
    if (res.status === 204) return null;
    return res.json();
}

function toast(message, isError) {
    const el = document.getElementById("toast");
    el.textContent = message;
    el.classList.toggle("error", !!isError);
    el.classList.remove("hidden");
    clearTimeout(toast._t);
    toast._t = setTimeout(() => el.classList.add("hidden"), 2600);
}

// ---- Status & usage ----

async function refreshStatus() {
    try {
        const s = await api("GET", "/api/status");
        setPill("pill-config", "Config " + (s.configOk ? "OK" : "ERR"), s.configOk);
        document.getElementById("pill-config").title = s.configSource || "";
        setPill("pill-groups", "Groups " + s.groupCount, null);
        setPill("pill-metrics", "Metrics " + s.metricCount, null);
        setPill("pill-uptime", "Uptime " + formatUptime(s.uptimeSeconds), null);
    } catch (e) { setPill("pill-config", "Config ERR", false); }
}

function setPill(id, text, ok) {
    const el = document.getElementById(id);
    el.textContent = text;
    el.classList.remove("ok", "bad");
    if (ok === true) el.classList.add("ok");
    if (ok === false) el.classList.add("bad");
}

function formatUptime(sec) {
    sec = Math.floor(sec || 0);
    const h = Math.floor(sec / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    if (h) return `${h}h ${m}m`;
    if (m) return `${m}m ${s}s`;
    return `${s}s`;
}

async function refreshUsage() {
    try {
        const snap = await api("GET", "/api/stats");
        const box = document.getElementById("usage");
        const paths = Object.keys(snap.perPath || {}).sort();
        let html = `<div class="usage-item"><div class="u-path">Total scrapes</div>
            <div class="u-meta">${snap.totalScrapes}</div></div>`;
        if (paths.length === 0) {
            html += `<div class="u-meta">No scrapes yet.</div>`;
        }
        for (const p of paths) {
            const ps = snap.perPath[p];
            html += `<div class="usage-item">
                <div class="u-path">/metrics/${escapeHtml(p)}</div>
                <div class="u-meta">${ps.scrapes} scrapes · last ${timeAgo(ps.lastTime)}</div>
            </div>`;
        }
        box.innerHTML = html;
    } catch (_) {}
}

function timeAgo(ts) {
    const d = (Date.now() - new Date(ts).getTime()) / 1000;
    if (d < 60) return Math.floor(d) + "s ago";
    if (d < 3600) return Math.floor(d / 60) + "m ago";
    return Math.floor(d / 3600) + "h ago";
}

// ---- Groups ----

async function loadGroups() {
    state.groups = await api("GET", "/api/groups");
    renderGroups();
}

function renderGroups() {
    const list = document.getElementById("group-list");
    list.innerHTML = "";
    for (const g of state.groups) {
        const li = document.createElement("li");
        li.className = "group-item" + (g.id === state.selectedGroupId ? " active" : "");
        li.innerHTML = `<div>
                <div class="g-path">/${escapeHtml(g.path)}</div>
                <div class="g-name">${escapeHtml(g.name || "")}</div>
            </div>
            <button class="g-del" title="Delete">✕</button>`;
        li.querySelector("div").addEventListener("click", () => selectGroup(g.id));
        li.querySelector(".g-del").addEventListener("click", (e) => {
            e.stopPropagation();
            deleteGroup(g);
        });
        list.appendChild(li);
    }
}

async function selectGroup(id) {
    state.selectedGroupId = id;
    renderGroups();
    const g = await api("GET", "/api/groups/" + id);
    state.metrics = g.metrics || [];
    state.selectedGroupPath = g.path;
    document.getElementById("metrics-title").textContent = "Metrics · /" + g.path;
    document.getElementById("btn-add-metric").disabled = false;
    const link = document.getElementById("scrape-link");
    link.href = "/metrics/" + g.path;
    link.textContent = "/metrics/" + g.path;
    document.getElementById("tester-url").textContent = "/metrics/" + g.path;
    document.getElementById("btn-run-scrape").disabled = false;
    renderMetrics();
}

async function deleteGroup(g) {
    if (!confirm(`Delete URL group "/${g.path}" and all its metrics?`)) return;
    try {
        await api("DELETE", "/api/groups/" + g.id);
        if (state.selectedGroupId === g.id) {
            state.selectedGroupId = null;
            state.selectedGroupPath = null;
            state.metrics = [];
            renderMetrics();
            document.getElementById("btn-add-metric").disabled = true;
            document.getElementById("btn-run-scrape").disabled = true;
            document.getElementById("tester-url").textContent = "/metrics/...";
            clearScrape();
        }
        await loadGroups();
        toast("Group deleted");
    } catch (e) { toast(e.message, true); }
}

// ---- Metrics ----

function renderMetrics() {
    const body = document.getElementById("metrics-body");
    const table = document.getElementById("metrics-table");
    const empty = document.getElementById("metrics-empty");
    if (!state.selectedGroupId) {
        table.classList.add("hidden");
        empty.classList.remove("hidden");
        empty.textContent = "Select a URL group to manage metrics.";
        return;
    }
    if (state.metrics.length === 0) {
        table.classList.add("hidden");
        empty.classList.remove("hidden");
        empty.textContent = "No metrics yet. Click + Metric to add one.";
        return;
    }
    empty.classList.add("hidden");
    table.classList.remove("hidden");
    body.innerHTML = "";
    for (const m of state.metrics) {
        const tr = document.createElement("tr");
        const labels = (m.labels || []).map(l =>
            `<span class="tag">${escapeHtml(l.key)}=${escapeHtml(l.value)}</span>`).join("") || "-";
        const overridden = m.override !== null && m.override !== undefined;
        tr.innerHTML = `
            <td><code>${escapeHtml(m.name)}</code></td>
            <td><span class="type-badge">${escapeHtml(m.type)}</span></td>
            <td>${valueModeText(m)}</td>
            <td>${labels}</td>
            <td>
                <div class="live-cell">
                    <input type="number" step="any" value="${overridden ? m.override : ""}"
                        placeholder="auto" />
                    <button class="btn small" data-set>Set</button>
                    <button class="btn small ghost" data-clear ${overridden ? "" : "disabled"}>Auto</button>
                </div>
            </td>
            <td>
                <button class="btn small ghost" data-edit>Edit</button>
                <button class="btn small danger" data-del>Del</button>
            </td>`;
        const input = tr.querySelector("input");
        tr.querySelector("[data-set]").addEventListener("click", () => setValue(m, parseFloat(input.value)));
        tr.querySelector("[data-clear]").addEventListener("click", () => setValue(m, null));
        tr.querySelector("[data-edit]").addEventListener("click", () => openMetricModal(m));
        tr.querySelector("[data-del]").addEventListener("click", () => deleteMetric(m));
        body.appendChild(tr);
    }
}

function valueModeText(m) {
    if (m.valueMode === "fixed") return `fixed (${m.fixedValue})`;
    if (m.valueMode === "range") return `range [${m.minValue}, ${m.maxValue}]`;
    if (m.valueMode === "increment") return `increment (+${m.step || 1})`;
    if (m.valueMode === "ramp") return `ramp [${m.minValue}, ${m.maxValue}] +${m.step || 1}`;
    if (m.valueMode === "step") return `step [${m.minValue}, ${m.maxValue}] / ${m.step || 5} levels`;
    if (m.valueMode === "rate") return `rate (${m.minValue} ${m.step >= 0 ? "+" : ""}${m.step || 0}/s)`;
    return "random";
}

async function setValue(m, value) {
    if (value !== null && Number.isNaN(value)) { toast("Enter a number", true); return; }
    try {
        await api("POST", "/api/metrics-def/" + m.id + "/value", { value });
        toast(value === null ? "Override cleared" : "Value set");
        await selectGroup(state.selectedGroupId);
    } catch (e) { toast(e.message, true); }
}

async function deleteMetric(m) {
    if (!confirm(`Delete metric "${m.name}"?`)) return;
    try {
        await api("DELETE", "/api/metrics-def/" + m.id);
        await selectGroup(state.selectedGroupId);
        toast("Metric deleted");
    } catch (e) { toast(e.message, true); }
}

// ---- Group modal ----

function openGroupModal() {
    state.editingGroupId = null;
    document.getElementById("group-modal-title").textContent = "New URL group";
    document.getElementById("group-path").value = "";
    document.getElementById("group-name").value = "";
    show("group-modal");
}

async function saveGroup() {
    const path = document.getElementById("group-path").value.trim();
    const name = document.getElementById("group-name").value.trim();
    if (!path) { toast("Path is required", true); return; }
    try {
        await api("POST", "/api/groups", { path, name });
        hide("group-modal");
        await loadGroups();
        toast("Group created");
    } catch (e) { toast(e.message, true); }
}

// ---- Metric modal ----

function openMetricModal(metric) {
    state.editingMetricId = metric ? metric.id : null;
    document.getElementById("metric-modal-title").textContent = metric ? "Edit metric" : "New metric";
    document.getElementById("m-name").value = metric ? metric.name : "";
    document.getElementById("m-type").value = metric ? metric.type : "counter";
    document.getElementById("m-desc").value = metric ? metric.description : "";
    document.getElementById("m-mode").value = metric ? metric.valueMode : "random";
    document.getElementById("m-min").value = metric ? metric.minValue : 0;
    document.getElementById("m-max").value = metric ? metric.maxValue : 100;
    document.getElementById("m-fixed").value = metric ? metric.fixedValue : 1;
    document.getElementById("m-step").value = metric && metric.step ? metric.step : 1;
    renderLabelRows(metric ? metric.labels : []);
    updateModeFields();
    show("metric-modal");
}

function renderLabelRows(labels) {
    const box = document.getElementById("label-rows");
    box.innerHTML = "";
    (labels || []).forEach(l => addLabelRow(l.key, l.value));
}

function addLabelRow(key, value) {
    const box = document.getElementById("label-rows");
    const row = document.createElement("div");
    row.className = "label-row";
    row.innerHTML = `<input placeholder="key" value="${escapeAttr(key || "")}" />
        <input placeholder="value" value="${escapeAttr(value || "")}" />
        <button title="Remove">✕</button>`;
    row.querySelector("button").addEventListener("click", () => row.remove());
    box.appendChild(row);
}

function collectLabels() {
    const rows = document.querySelectorAll("#label-rows .label-row");
    const labels = [];
    rows.forEach(r => {
        const [k, v] = r.querySelectorAll("input");
        if (k.value.trim()) labels.push({ key: k.value.trim(), value: v.value });
    });
    return labels;
}

const MODE_FIELDS = {
    random: [],
    range: ["min", "max"],
    fixed: ["fixed"],
    increment: ["min", "step"],
    ramp: ["min", "max", "step"],
    step: ["min", "max", "step"],
    rate: ["min", "step"],
};

function updateModeFields() {
    const mode = document.getElementById("m-mode").value;
    const fields = MODE_FIELDS[mode] || [];
    ["min", "max", "fixed", "step"].forEach(f => {
        document.querySelectorAll(".f-" + f).forEach(e => e.style.display = fields.includes(f) ? "" : "none");
    });
    document.getElementById("m-step-label").textContent =
        mode === "step" ? "Levels" : mode === "rate" ? "Rate/sec" : "Step";
    document.getElementById("m-min-label").textContent =
        (mode === "increment" || mode === "rate") ? "Start" : "Min";
}

async function saveMetric() {
    const payload = {
        name: document.getElementById("m-name").value.trim(),
        type: document.getElementById("m-type").value,
        description: document.getElementById("m-desc").value.trim(),
        valueMode: document.getElementById("m-mode").value,
        minValue: parseFloat(document.getElementById("m-min").value) || 0,
        maxValue: parseFloat(document.getElementById("m-max").value) || 0,
        fixedValue: parseFloat(document.getElementById("m-fixed").value) || 0,
        step: parseFloat(document.getElementById("m-step").value) || 0,
        labels: collectLabels(),
    };
    if (!payload.name) { toast("Name is required", true); return; }
    try {
        if (state.editingMetricId) {
            await api("PUT", "/api/metrics-def/" + state.editingMetricId, payload);
        } else {
            await api("POST", "/api/groups/" + state.selectedGroupId + "/metrics", payload);
        }
        hide("metric-modal");
        await selectGroup(state.selectedGroupId);
        toast("Metric saved");
    } catch (e) { toast(e.message, true); }
}

// ---- Scrape tester ----

async function runScrape() {
    if (!state.selectedGroupPath) { toast("Select a URL group first", true); return; }
    const url = "/metrics/" + state.selectedGroupPath;
    const out = document.getElementById("tester-output");
    const meta = document.getElementById("tester-meta");
    const btn = document.getElementById("btn-run-scrape");
    btn.disabled = true;
    out.textContent = "Calling " + url + " ...";
    meta.innerHTML = "";
    const started = performance.now();
    try {
        const res = await fetch(url, { headers: { "Cache-Control": "no-cache" } });
        const text = await res.text();
        const ms = Math.round(performance.now() - started);
        const lines = text.split("\n").filter(l => l && !l.startsWith("#")).length;
        const okClass = res.ok ? "ok" : "bad";
        meta.innerHTML = `
            <span class="chip ${okClass}">Status <b>${res.status} ${res.statusText}</b></span>
            <span class="chip">Latency <b>${ms} ms</b></span>
            <span class="chip">Size <b>${formatBytes(text.length)}</b></span>
            <span class="chip">Series <b>${lines}</b></span>
            <span class="chip">At <b>${new Date().toLocaleTimeString()}</b></span>`;
        out.textContent = text || "(empty response)";
    } catch (e) {
        meta.innerHTML = `<span class="chip bad">Error <b>${escapeHtml(e.message)}</b></span>`;
        out.textContent = "Request failed.";
    } finally {
        btn.disabled = false;
    }
}

function clearScrape() {
    document.getElementById("tester-meta").innerHTML = "";
    document.getElementById("tester-output").textContent =
        "Select a URL group, then click Run to call its /metrics endpoint and inspect the response.";
}

function formatBytes(n) {
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / 1024 / 1024).toFixed(1) + " MB";
}

// ---- utils ----

function show(id) { document.getElementById(id).classList.remove("hidden"); }
function hide(id) { document.getElementById(id).classList.add("hidden"); }
function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
function escapeAttr(s) { return escapeHtml(s); }

// ---- wiring ----

function init() {
    document.getElementById("btn-add-group").addEventListener("click", openGroupModal);
    document.getElementById("group-save").addEventListener("click", saveGroup);
    document.getElementById("btn-add-metric").addEventListener("click", () => openMetricModal(null));
    document.getElementById("metric-save").addEventListener("click", saveMetric);
    document.getElementById("btn-add-label").addEventListener("click", () => addLabelRow("", ""));
    document.getElementById("m-mode").addEventListener("change", updateModeFields);
    document.getElementById("btn-run-scrape").addEventListener("click", runScrape);
    document.getElementById("btn-clear-scrape").addEventListener("click", clearScrape);
    document.querySelectorAll("[data-close]").forEach(b =>
        b.addEventListener("click", () => document.querySelectorAll(".modal-backdrop").forEach(m => m.classList.add("hidden"))));
    document.querySelectorAll(".modal-backdrop").forEach(bd =>
        bd.addEventListener("click", e => { if (e.target === bd) bd.classList.add("hidden"); }));

    loadGroups();
    refreshStatus();
    refreshUsage();
    setInterval(refreshStatus, 5000);
    setInterval(refreshUsage, 5000);
}

document.addEventListener("DOMContentLoaded", init);
