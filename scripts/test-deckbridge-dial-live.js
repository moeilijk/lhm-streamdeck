#!/usr/bin/env node
"use strict";

const zlib = require("zlib");
const vm = require("vm");

const baseUrl = process.env.DECKBRIDGE_URL || "http://127.0.0.1:34075";
const wsUrl = baseUrl.replace(/^http/, "ws");
const encoderBaseIndex = 1000;

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function fetchJson(path) {
  const res = await fetch(baseUrl + path);
  assert(res.ok, "GET " + path + " failed: " + res.status);
  return res.json();
}

async function fetchText(path) {
  const res = await fetch(baseUrl + path);
  assert(res.ok, "GET " + path + " failed: " + res.status);
  return res.text();
}

async function postJson(path, body) {
  const res = await fetch(baseUrl + path, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error("POST " + path + " failed: " + res.status + " " + await res.text());
  }
  return res.json();
}

async function waitFor(label, read, predicate, timeoutMs = 4000) {
  const deadline = Date.now() + timeoutMs;
  let latest = null;
  while (Date.now() < deadline) {
    latest = await read();
    if (predicate(latest)) return latest;
    await sleep(100);
  }
  throw new Error("timed out waiting for " + label);
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function dialSlots(state) {
  return (state.slots || []).filter((slot) => slot.actionId === "com.moeilijk.lhm.dial");
}

function findSlot(state, context) {
  return dialSlots(state).find((slot) => slot.context === context);
}

function selectedSourcePage(slot) {
  const pages = (slot.settings && slot.settings.pages) || [];
  return pages.find((page) => page && page.sensorUid && page.readingId && (page.readingLabel || page.title));
}

function paletteColor(index) {
  const palette = [
    { foregroundColor: "#005128", highlightColor: "#009e00" },
    { foregroundColor: "#003f73", highlightColor: "#00a2ff" },
    { foregroundColor: "#5a3b87", highlightColor: "#b06cff" },
    { foregroundColor: "#6a4a00", highlightColor: "#ffbf33" },
    { foregroundColor: "#6f1d1b", highlightColor: "#ff5a4f" },
    { foregroundColor: "#004b50", highlightColor: "#00d6d6" },
    { foregroundColor: "#4d3d00", highlightColor: "#d8d000" },
    { foregroundColor: "#00421f", highlightColor: "#39d98a" },
    { foregroundColor: "#4b184f", highlightColor: "#ff66d8" },
    { foregroundColor: "#5b2b00", highlightColor: "#ff8a1f" },
    { foregroundColor: "#173b64", highlightColor: "#66c2ff" },
    { foregroundColor: "#3f4f13", highlightColor: "#b5e853" },
    { foregroundColor: "#4f2333", highlightColor: "#ff7aa8" },
    { foregroundColor: "#1d4a45", highlightColor: "#5ef2c2" },
    { foregroundColor: "#2f2d6b", highlightColor: "#8f8cff" },
    { foregroundColor: "#5a3216", highlightColor: "#d98b45" },
  ];
  return palette[index % palette.length];
}

function pageFromSource(sourcePage, index) {
  const colors = paletteColor(index);
  return {
    sourceProfileId: sourcePage.sourceProfileId || "",
    sensorUid: sourcePage.sensorUid,
    readingId: String(sourcePage.readingId),
    readingLabel: (sourcePage.readingLabel || sourcePage.title || "Reading") + " E2E " + (index + 1),
    title: "E2E Page " + (index + 1),
    titleFontSize: 0,
    valueFontSize: 0,
    showTitleInGraph: true,
    min: 0,
    max: 100,
    format: "",
    divisor: "",
    graphUnit: "",
    isValid: true,
    titleColor: "#b7b7b7",
    foregroundColor: colors.foregroundColor,
    backgroundColor: "#000000",
    highlightColor: colors.highlightColor,
    valueTextColor: "#ffffff",
    graphMode: "both",
    graphHeightPct: 100,
    graphLineThickness: 1,
    textStroke: false,
    textStrokeColor: "#000000",
    smoothingAlpha: 0,
    thresholds: [],
    suppressedGlobalIDs: [],
    snoozeDurations: [],
    currentThresholdId: "",
  };
}

function controlledSettings(sourcePage) {
  const pages = [0, 1, 2].map((idx) => pageFromSource(sourcePage, idx));
  pages[0].thresholds = [{
    id: "e2e-threshold",
    name: "E2E Threshold",
    text: "E2E Snooze",
    textColor: "#ffffff",
    enabled: true,
    operator: ">=",
    value: 0,
    backgroundColor: "#333300",
    foregroundColor: "#999900",
    highlightColor: "#ffff00",
    valueTextColor: "#ffff00",
  }];
  pages[0].snoozeDurations = [0];
  pages[0].currentThresholdId = "e2e-threshold";
  return { activeIndex: 0, pages };
}

function openPropertyInspectorSocket(context) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(wsUrl);
    ws.messages = [];
    ws.addEventListener("message", (event) => {
      try {
        ws.messages.push(JSON.parse(event.data));
      } catch {
        ws.messages.push(event.data);
      }
    });
    const timer = setTimeout(() => reject(new Error("WebSocket open timeout")), 3000);
    ws.addEventListener("open", () => {
      clearTimeout(timer);
      ws.send(JSON.stringify({ event: "registerPropertyInspector", uuid: context }));
      resolve(ws);
    });
    ws.addEventListener("error", () => {
      clearTimeout(timer);
      reject(new Error("WebSocket connection failed"));
    });
  });
}

async function waitForWsMessage(ws, predicate, label, timeoutMs = 4000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const index = ws.messages.findIndex(predicate);
    if (index !== -1) {
      const msg = ws.messages[index];
      ws.messages.splice(0, index + 1);
      return msg;
    }
    await sleep(50);
  }
  throw new Error("timed out waiting for websocket message: " + label);
}

function sendToPlugin(ws, slot, payload) {
  ws.send(JSON.stringify({
    event: "sendToPlugin",
    context: slot.context,
    action: slot.actionId,
    payload,
  }));
}

async function sendDialSettings(ws, slot, settings) {
  sendToPlugin(ws, slot, { dialSetSettings: settings });
}

async function requestGlobalThresholds(ws, slot) {
  ws.messages.length = 0;
  sendToPlugin(ws, slot, { requestDialCatalog: true });
  const msg = await waitForWsMessage(ws, (candidate) => {
    return candidate && candidate.payload && Array.isArray(candidate.payload.globalThresholds);
  }, "global threshold catalog");
  return msg.payload.globalThresholds;
}

async function deleteGlobalThresholdsByName(settingsWs, settingsSlot, catalogWs, catalogSlot, name) {
  let globals = await requestGlobalThresholds(catalogWs, catalogSlot);
  let matches = globals.filter((threshold) => threshold.name === name);
  if (matches.length === 0) return;

  matches.forEach((threshold) => {
    sendToPlugin(settingsWs, settingsSlot, { deleteGlobalThreshold: threshold.id });
  });

  await waitFor("global threshold cleanup", async () => {
    globals = await requestGlobalThresholds(catalogWs, catalogSlot);
    return globals;
  }, (latest) => !latest.some((threshold) => threshold.name === name), 5000);
}

async function waitForSlot(context, predicate, label) {
  const state = await waitFor(label, () => fetchJson("/api/state"), (state) => {
    const slot = findSlot(state, context);
    return slot && predicate(slot, state);
  });
  return findSlot(state, context);
}

function imageDataForSlot(images, keyIndex) {
  const item = images.find((img) => img.keyIndex === keyIndex);
  return item && item.feedbackImageDataUrl;
}

async function waitForImage(keyIndex, predicate, label, timeoutMs = 4000) {
  const images = await waitFor(label, () => fetchJson("/api/images"), (images) => {
    const data = imageDataForSlot(images, keyIndex);
    return data && predicate(data);
  }, timeoutMs);
  return imageDataForSlot(images, keyIndex);
}

function paeth(a, b, c) {
  const p = a + b - c;
  const pa = Math.abs(p - a);
  const pb = Math.abs(p - b);
  const pc = Math.abs(p - c);
  if (pa <= pb && pa <= pc) return a;
  if (pb <= pc) return b;
  return c;
}

function decodePngDataUrl(dataUrl) {
  const base64 = dataUrl.replace(/^data:image\/png;base64,/, "");
  const buf = Buffer.from(base64, "base64");
  assert(buf.slice(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])), "invalid PNG signature");
  let pos = 8;
  let width = 0;
  let height = 0;
  let colorType = 0;
  const idat = [];
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos);
    const type = buf.slice(pos + 4, pos + 8).toString("ascii");
    const data = buf.slice(pos + 8, pos + 8 + len);
    pos += 12 + len;
    if (type === "IHDR") {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      colorType = data[9];
      assert(data[8] === 8, "only 8-bit PNGs supported");
      assert(data[12] === 0, "interlaced PNGs not supported");
    } else if (type === "IDAT") {
      idat.push(data);
    } else if (type === "IEND") {
      break;
    }
  }
  const bpp = colorType === 6 ? 4 : colorType === 2 ? 3 : 0;
  assert(bpp > 0, "unsupported PNG color type " + colorType);
  const inflated = zlib.inflateSync(Buffer.concat(idat));
  const stride = width * bpp;
  const raw = Buffer.alloc(height * stride);
  let src = 0;
  for (let y = 0; y < height; y++) {
    const filter = inflated[src++];
    const row = y * stride;
    const prev = y > 0 ? row - stride : -1;
    for (let x = 0; x < stride; x++) {
      const value = inflated[src++];
      const left = x >= bpp ? raw[row + x - bpp] : 0;
      const up = prev >= 0 ? raw[prev + x] : 0;
      const upLeft = prev >= 0 && x >= bpp ? raw[prev + x - bpp] : 0;
      if (filter === 0) raw[row + x] = value;
      else if (filter === 1) raw[row + x] = (value + left) & 255;
      else if (filter === 2) raw[row + x] = (value + up) & 255;
      else if (filter === 3) raw[row + x] = (value + Math.floor((left + up) / 2)) & 255;
      else if (filter === 4) raw[row + x] = (value + paeth(left, up, upLeft)) & 255;
      else throw new Error("unsupported PNG filter " + filter);
    }
  }
  return {
    width,
    height,
    pixel(x, y) {
      const i = (y * width + x) * bpp;
      return {
        r: raw[i],
        g: raw[i + 1],
        b: raw[i + 2],
        a: bpp === 4 ? raw[i + 3] : 255,
      };
    },
  };
}

function isSeparatorColor(px) {
  return px.r === 54 && px.g === 62 && px.b === 70 && px.a === 255;
}

function hasPageIndicator(decoded) {
  for (let y = decoded.height - 10; y < decoded.height - 2; y++) {
    for (let x = 70; x < decoded.width - 70; x++) {
      const p = decoded.pixel(x, y);
      if (p.r >= 180 && p.r <= 205 && p.g >= 188 && p.g <= 210 && p.b >= 196 && p.b <= 220) {
        return true;
      }
    }
  }
  return false;
}

async function testPiAddFlow(ws, slot, sourcePage) {
  const original = clone(slot.settings || { activeIndex: 0, pages: [] });
  const originalCount = (original.pages || []).length;
  const piUtils = await fetchText("/com.moeilijk.lhm/pi_utils.js");
  const dialPi = await fetchText("/com.moeilijk.lhm/dial_pi.js");
  const listeners = {};
  const addBtn = {
    dataset: {},
    addEventListener(type, handler) { listeners[type] = handler; },
  };
  const sandbox = {
    console,
    JSON,
    setTimeout,
    clearTimeout,
    navigator: { appVersion: "DeckBridgeLiveTest" },
    location: { hostname: "127.0.0.1" },
    document: {
      querySelector() { return null; },
      querySelectorAll() { return []; },
      getElementById(id) {
        if (id === "addPageBtn") return addBtn;
        if (id === "pageSensorSelect") return { value: sourcePage.sensorUid };
        if (id === "pageReadingSelect") return { value: String(sourcePage.readingId), disabled: false };
        return null;
      },
      createElement() { return {}; },
      addEventListener() {},
      body: { appendChild() {} },
    },
    window: null,
    websocket: ws,
    uuid: slot.context,
    actionInfo: { action: slot.actionId },
    currentSettings: clone(original),
    currentCatalog: {
      readings: [{
        id: String(sourcePage.readingId),
        sensorUid: sourcePage.sensorUid,
        label: (sourcePage.readingLabel || sourcePage.title) + " Live Add",
        unit: "%",
        type: "Usage",
      }],
    },
    pageSelectionDraft: {
      sensorUid: sourcePage.sensorUid,
      readingId: String(sourcePage.readingId),
    },
    renderPages() {},
  };
  sandbox.window = sandbox;
  sandbox.addEventListener = () => {};
  vm.createContext(sandbox);
  vm.runInContext(piUtils, sandbox);
  vm.runInContext(dialPi, sandbox);
  sandbox.websocket = ws;
  sandbox.uuid = slot.context;
  sandbox.actionInfo = { action: slot.actionId };
  sandbox.currentSettings = clone(original);
  sandbox.currentCatalog = {
    readings: [{
      id: String(sourcePage.readingId),
      sensorUid: sourcePage.sensorUid,
      label: (sourcePage.readingLabel || sourcePage.title) + " Live Add",
      unit: "%",
      type: "Usage",
    }],
  };
  sandbox.pageSelectionDraft = {
    sensorUid: sourcePage.sensorUid,
    readingId: String(sourcePage.readingId),
  };
  sandbox.renderPages = () => {};

  sandbox.bindAddPageControl();
  assert(typeof listeners.click === "function", "Add button click handler was not bound");
  listeners.click({ preventDefault() {}, stopPropagation() {} });

  const expected = paletteColor(originalCount);
  await waitForSlot(slot.context, (updated) => {
    const pages = (updated.settings && updated.settings.pages) || [];
    const added = pages[originalCount];
    return pages.length === originalCount + 1 &&
      added &&
      added.foregroundColor === expected.foregroundColor &&
      added.highlightColor === expected.highlightColor;
  }, "PI Add to persist through DeckBridge state");
}

async function testPiThresholdAddFlow(ws, slot) {
  const state = await fetchJson("/api/state");
  const liveSlot = findSlot(state, slot.context);
  assert(liveSlot, "dial slot missing before threshold Add test");
  const original = clone(liveSlot.settings || { activeIndex: 0, pages: [] });
  const originalThresholdCount = (((original.pages || [])[0] || {}).thresholds || []).length;
  const piUtils = await fetchText("/com.moeilijk.lhm/pi_utils.js");
  const dialPi = await fetchText("/com.moeilijk.lhm/dial_pi.js");
  const listeners = {};
  const addThresholdBtn = {
    dataset: {},
    addEventListener(type, handler) { listeners[type] = handler; },
  };
  const newThresholdName = {
    dataset: {},
    value: "E2E Added Threshold",
    addEventListener() {},
  };
  const sandbox = {
    console,
    JSON,
    setTimeout,
    clearTimeout,
    navigator: { appVersion: "DeckBridgeLiveTest" },
    location: { hostname: "127.0.0.1" },
    document: {
      querySelector(selector) {
        if (selector === "#addThresholdBtn") return addThresholdBtn;
        if (selector === "#newThresholdName") return newThresholdName;
        return null;
      },
      querySelectorAll() { return []; },
      getElementById(id) {
        if (id === "pageList") return { selectedIndex: 0 };
        return null;
      },
      createElement() { return {}; },
      addEventListener() {},
      body: { appendChild() {} },
    },
    window: null,
    websocket: ws,
    uuid: slot.context,
    actionInfo: { action: slot.actionId },
    currentSettings: clone(original),
    currentCatalog: { readings: [] },
    renderPages() {},
  };
  sandbox.window = sandbox;
  sandbox.addEventListener = () => {};
  vm.createContext(sandbox);
  vm.runInContext(piUtils, sandbox);
  vm.runInContext(dialPi, sandbox);
  sandbox.websocket = ws;
  sandbox.uuid = slot.context;
  sandbox.actionInfo = { action: slot.actionId };
  sandbox.currentSettings = clone(original);
  sandbox.renderPages = () => {};

  sandbox.bindThresholdControls();
  assert(typeof listeners.click === "function", "Threshold Add click handler was not bound");
  listeners.click({ preventDefault() {}, stopPropagation() {} });

  await waitForSlot(slot.context, (updated) => {
    const thresholds = (((updated.settings || {}).pages || [])[0] || {}).thresholds || [];
    return thresholds.length === originalThresholdCount + 1 &&
      thresholds.some((threshold) => threshold.name === "E2E Added Threshold");
  }, "PI threshold Add to persist through DeckBridge state");
}

async function testPiViewOptionsFlow(ws, slot) {
  const state = await fetchJson("/api/state");
  const liveSlot = findSlot(state, slot.context);
  assert(liveSlot, "dial slot missing before view options test");
  const next = clone(liveSlot.settings || { activeIndex: 0, pages: [] });
  next.defaultView = "overview";
  next.indicatorStyle = "count";
  await sendDialSettings(ws, slot, next);
  await waitForSlot(slot.context, (updated) => {
    return updated.settings &&
      updated.settings.defaultView === "overview" &&
      updated.settings.indicatorStyle === "count";
  }, "PI view options persist through DeckBridge state");
}

async function testPiBulkAddFlow(ws, slot, sourcePage) {
  const state = await fetchJson("/api/state");
  const liveSlot = findSlot(state, slot.context);
  assert(liveSlot, "dial slot missing before bulk Add test");
  const original = { activeIndex: 0, pages: [] };
  await sendDialSettings(ws, slot, original);
  await waitForSlot(slot.context, (updated) => {
    const pages = (updated.settings && updated.settings.pages) || [];
    return pages.length === 0 && updated.settings.activeIndex === 0;
  }, "PI bulk Add empty start state");
  const originalCount = (original.pages || []).length;
  const piUtils = await fetchText("/com.moeilijk.lhm/pi_utils.js");
  const dialPi = await fetchText("/com.moeilijk.lhm/dial_pi.js");
  const listeners = {};
  const bulkPreviewBtn = {
    dataset: {},
    addEventListener(type, handler) { listeners["preview:" + type] = handler; },
  };
  const bulkAddBtn = {
    dataset: {},
    disabled: true,
    addEventListener(type, handler) { listeners["add:" + type] = handler; },
  };
  const bulkRule = {
    value: "sensor-readings",
    dataset: {},
    addEventListener(type, handler) { listeners["rule:" + type] = handler; },
  };
  const pageSensorSelect = { value: sourcePage.sensorUid };
  const pageReadingSelect = { value: String(sourcePage.readingId), disabled: false };
  const previewOptions = [];
  const bulkPreviewList = {
    options: previewOptions,
    appendChild(option) { previewOptions.push(option); },
  };
  Object.defineProperty(bulkPreviewList, "innerHTML", {
    set() { previewOptions.length = 0; },
    get() { return ""; },
  });
  const sandbox = {
    console,
    JSON,
    setTimeout,
    clearTimeout,
    navigator: { appVersion: "DeckBridgeLiveTest" },
    location: { hostname: "127.0.0.1" },
    document: {
      querySelector() { return null; },
      querySelectorAll() { return []; },
      getElementById(id) {
        if (id === "bulkPreviewBtn") return bulkPreviewBtn;
        if (id === "bulkAddBtn") return bulkAddBtn;
        if (id === "bulkRule") return bulkRule;
        if (id === "bulkPreviewList") return bulkPreviewList;
        if (id === "pageSensorSelect") return pageSensorSelect;
        if (id === "pageReadingSelect") return pageReadingSelect;
        if (id === "pageList") return { selectedIndex: 0 };
        return null;
      },
      createElement() { return { value: "", textContent: "", selected: false }; },
      addEventListener() {},
      body: { appendChild() {} },
    },
    window: null,
    websocket: ws,
    uuid: slot.context,
    actionInfo: { action: slot.actionId },
    currentSettings: clone(original),
    currentCatalog: {
      sensors: [{ uid: sourcePage.sensorUid, name: "E2E Sensor" }],
      readings: [
        {
          id: String(sourcePage.readingId),
          sensorUid: sourcePage.sensorUid,
          label: (sourcePage.readingLabel || sourcePage.title) + " Bulk A",
          unit: "%",
          type: "Usage",
        },
        {
          id: String(Number(sourcePage.readingId) + 1),
          sensorUid: sourcePage.sensorUid,
          label: (sourcePage.readingLabel || sourcePage.title) + " Bulk B",
          unit: "%",
          type: "Usage",
        },
      ],
    },
    pageSelectionDraft: {
      sensorUid: sourcePage.sensorUid,
      readingId: String(sourcePage.readingId),
    },
    renderPages() {},
  };
  sandbox.window = sandbox;
  sandbox.addEventListener = () => {};
  vm.createContext(sandbox);
  vm.runInContext(piUtils, sandbox);
  vm.runInContext(dialPi, sandbox);
  sandbox.websocket = ws;
  sandbox.uuid = slot.context;
  sandbox.actionInfo = { action: slot.actionId };
  sandbox.currentSettings = clone(original);
  const selectedLabel = (sourcePage.readingLabel || sourcePage.title) + " Bulk #1";
  const otherLabel = (sourcePage.readingLabel || sourcePage.title) + " Bulk #2";
  const otherSensorUid = sourcePage.sensorUid + "-other";
  const wrongUnitSensorUid = sourcePage.sensorUid + "-wrong-unit";
  sandbox.currentCatalog = {
    sensors: [
      { uid: sourcePage.sensorUid, name: "E2E Sensor" },
      { uid: otherSensorUid, name: "E2E Sensor Other" },
      { uid: wrongUnitSensorUid, name: "E2E Sensor Wrong Unit" },
    ],
    readings: [
      {
        id: String(sourcePage.readingId),
        sensorUid: sourcePage.sensorUid,
        label: selectedLabel,
        unit: "%",
        type: "Usage",
      },
      {
        id: String(Number(sourcePage.readingId) + 1),
        sensorUid: sourcePage.sensorUid,
        label: otherLabel,
        unit: "%",
        type: "Usage",
      },
      {
        id: String(Number(sourcePage.readingId) + 100),
        sensorUid: otherSensorUid,
        label: selectedLabel,
        unit: "%",
        type: "Usage",
      },
      {
        id: String(Number(sourcePage.readingId) + 101),
        sensorUid: otherSensorUid,
        label: otherLabel,
        unit: "%",
        type: "Usage",
      },
      {
        id: String(Number(sourcePage.readingId) + 200),
        sensorUid: wrongUnitSensorUid,
        label: selectedLabel,
        unit: "W",
        type: "Power",
      },
    ],
  };
  sandbox.pageSelectionDraft = {
    sensorUid: sourcePage.sensorUid,
    readingId: String(sourcePage.readingId),
  };
  sandbox.renderPages = () => {};

  sandbox.bindBulkControls();
  assert(typeof listeners["preview:click"] === "function", "Bulk Preview click handler was not bound");
  assert(typeof listeners["add:click"] === "function", "Bulk Add click handler was not bound");
  listeners["preview:click"]({ preventDefault() {} });
  assert(previewOptions.length === 2, "All-readings preview did not render selected-sensor candidates");
  assert(previewOptions.every((option) => option.textContent.indexOf("E2E Sensor — ") === 0), "All-readings preview included another sensor");
  assert(previewOptions.some((option) => option.textContent.endsWith(selectedLabel)), "All-readings preview missing selected reading");
  assert(previewOptions.some((option) => option.textContent.endsWith(otherLabel)), "All-readings preview missing other selected-sensor reading");

  bulkRule.value = "matching-reading";
  listeners["rule:change"]();
  assert(previewOptions.length === 0, "Bulk rule change did not clear stale preview");
  assert(bulkAddBtn.disabled === true, "Bulk rule change did not disable stale Add");
  listeners["add:click"]({ preventDefault() {} });

  bulkRule.value = "sensor-readings";
  listeners["rule:change"]();
  listeners["preview:click"]({ preventDefault() {} });
  assert(previewOptions.length === 2, "All-readings preview did not regenerate after rule invalidation");
  previewOptions[1].selected = false;
  listeners["add:click"]({ preventDefault() {} });

  const afterAllReadings = await waitForSlot(slot.context, (updated) => {
    const pages = (updated.settings && updated.settings.pages) || [];
    return pages.length === originalCount + 1 &&
      pages.some((page) => (page.readingLabel || "") === selectedLabel) &&
      !pages.some((page) => (page.readingLabel || "") === otherLabel);
  }, "PI bulk Add to persist selected preview through DeckBridge state");

  sandbox.currentSettings = clone(afterAllReadings.settings);
  bulkRule.value = "matching-reading";
  listeners["preview:click"]({ preventDefault() {} });
  const matchingLabels = previewOptions.map((option) => option.textContent).join("|");
  assert(previewOptions.length === 1, "Matching-reading preview did not exclude already configured exact candidate");
  assert(!matchingLabels.includes("E2E Sensor — " + selectedLabel), "Matching-reading preview still included configured selected sensor reading");
  assert(matchingLabels.includes("E2E Sensor Other — " + selectedLabel), "Matching-reading preview missing other sensor same reading");
  assert(!matchingLabels.includes(otherLabel), "Matching-reading preview included another reading");
  assert(!matchingLabels.includes("Wrong Unit"), "Matching-reading preview included same label with different type/unit");
  listeners["add:click"]({ preventDefault() {} });

  await waitForSlot(slot.context, (updated) => {
    const added = ((updated.settings && updated.settings.pages) || []).slice(originalCount);
    return added.length === 2 &&
      added.filter((page) => (page.readingLabel || "") === selectedLabel).length === 2 &&
      !added.some((page) => (page.readingLabel || "") === otherLabel);
  }, "PI matching-reading bulk Add to persist exact cross-sensor pages");

  const afterExact = await waitForSlot(slot.context, (updated) => {
    const pages = (updated.settings && updated.settings.pages) || [];
    return pages.length === originalCount + 2;
  }, "PI exact bulk Add state settled");

  sandbox.currentSettings = clone(afterExact.settings);
  bulkRule.value = "matching-numbered-readings";
  listeners["rule:change"]();
  listeners["preview:click"]({ preventDefault() {} });
  const numberedLabels = previewOptions.map((option) => option.textContent).join("|");
  assert(previewOptions.length === 2, "Matching-numbered preview did not exclude already configured numbered candidates");
  assert(!numberedLabels.includes("E2E Sensor — " + selectedLabel), "Matching-numbered preview still included selected sensor #1");
  assert(numberedLabels.includes("E2E Sensor — " + otherLabel), "Matching-numbered preview missing selected sensor #2");
  assert(!numberedLabels.includes("E2E Sensor Other — " + selectedLabel), "Matching-numbered preview still included other sensor #1");
  assert(numberedLabels.includes("E2E Sensor Other — " + otherLabel), "Matching-numbered preview missing other sensor #2");
  assert(!numberedLabels.includes("Wrong Unit"), "Matching-numbered preview included same numbered label with different type/unit");
  listeners["add:click"]({ preventDefault() {} });

  await waitForSlot(slot.context, (updated) => {
    const added = ((updated.settings && updated.settings.pages) || []).slice(originalCount + 2);
    return added.length === 2 &&
      added.every((page) => (page.readingLabel || "") === otherLabel);
  }, "PI matching-numbered bulk Add to persist selected numbered pages");
}

async function testGlobalThresholdFlow(settingsWs, settingsSlot, dialWs, dialSlot, sourcePage) {
  const name = "E2E Global Threshold";
  let globalId = "";
  await deleteGlobalThresholdsByName(settingsWs, settingsSlot, dialWs, dialSlot, name);
  sendToPlugin(settingsWs, settingsSlot, { addGlobalThreshold: name });
  dialWs.messages.length = 0;
  sendToPlugin(dialWs, dialSlot, { requestDialCatalog: true });
  const addedMsg = await waitForWsMessage(dialWs, (msg) => {
    const globals = msg && msg.payload && msg.payload.globalThresholds;
    return Array.isArray(globals) && globals.some((threshold) => threshold.name === name);
  }, "global threshold add broadcast");
  const added = addedMsg.payload.globalThresholds.find((threshold) => threshold.name === name);
  globalId = added.id;
  assert(globalId, "added global threshold has no id");

  try {
    sendToPlugin(settingsWs, settingsSlot, {
      updateGlobalThreshold: {
        id: globalId,
        field: "thresholdText",
        value: "E2E Global",
        checked: false,
      },
    });
    dialWs.messages.length = 0;
    sendToPlugin(dialWs, dialSlot, { requestDialCatalog: true });
    await waitForWsMessage(dialWs, (msg) => {
      const globals = msg && msg.payload && msg.payload.globalThresholds;
      return Array.isArray(globals) && globals.some((threshold) => threshold.id === globalId && threshold.text === "E2E Global");
    }, "global threshold update broadcast");

    const globalSettings = {
      activeIndex: 0,
      pages: [pageFromSource(sourcePage, 0)],
    };
    globalSettings.pages[0].thresholds = [];
    globalSettings.pages[0].snoozeDurations = [0];
    globalSettings.pages[0].suppressedGlobalIDs = [];
    globalSettings.pages[0].currentThresholdId = "";
    await sendDialSettings(dialWs, dialSlot, globalSettings);

    await waitForSlot(dialSlot.context, (updated) => {
      const page = (((updated.settings || {}).pages || [])[0] || {});
      return page.currentThresholdId === globalId;
    }, "global threshold applies to dial page");

    const suppressedSettings = clone(globalSettings);
    suppressedSettings.pages[0].suppressedGlobalIDs = [globalId];
    suppressedSettings.pages[0].currentThresholdId = "";
    await sendDialSettings(dialWs, dialSlot, suppressedSettings);
    await sleep(250);
    const suppressedState = await fetchJson("/api/state");
    const suppressedSlot = findSlot(suppressedState, dialSlot.context);
    const suppressedPage = (((suppressedSlot.settings || {}).pages || [])[0] || {});
    assert(Array.isArray(suppressedPage.suppressedGlobalIDs) && suppressedPage.suppressedGlobalIDs.includes(globalId), "global threshold suppression was not persisted");
    assert(suppressedPage.currentThresholdId !== globalId, "suppressed global threshold still active on dial page");
  } finally {
    await deleteGlobalThresholdsByName(settingsWs, settingsSlot, dialWs, dialSlot, name);
  }
}

async function main() {
  const initialState = await fetchJson("/api/state");
  const slot = dialSlots(initialState).find((candidate) => selectedSourcePage(candidate));
  assert(slot, "no configured dial slot with a readable page found in DeckBridge state");
  const settingsSlot = (initialState.slots || []).find((candidate) => candidate.actionId === "com.moeilijk.lhm.settings");
  assert(settingsSlot, "no configured settings slot found in DeckBridge state");
  const sourcePage = selectedSourcePage(slot);
  const originalSettings = clone(slot.settings || { activeIndex: 0, pages: [] });
  const deviceId = initialState.primaryDeviceId || ((initialState.devices || [])[0] || {}).id;
  assert(deviceId, "no DeckBridge device available");
  const dialIndex = slot.keyIndex - encoderBaseIndex;
  assert(dialIndex >= 0, "selected slot is not a dial slot");

  const ws = await openPropertyInspectorSocket(slot.context);
  const settingsWs = await openPropertyInspectorSocket(settingsSlot.context);
  try {
    await testPiAddFlow(ws, slot, sourcePage);
    await testPiViewOptionsFlow(ws, slot);
    await testPiBulkAddFlow(ws, slot, sourcePage);

    const controlled = controlledSettings(sourcePage);
    await sendDialSettings(ws, slot, controlled);
    await waitForSlot(slot.context, (updated) => {
      return updated.settings &&
        updated.settings.activeIndex === 0 &&
        Array.isArray(updated.settings.pages) &&
        updated.settings.pages.length === 3;
    }, "controlled dial settings");

    await testPiThresholdAddFlow(ws, slot);
    await testGlobalThresholdFlow(settingsWs, settingsSlot, ws, slot, sourcePage);

    await sendDialSettings(ws, slot, controlled);
    await waitForSlot(slot.context, (updated) => {
      return updated.settings &&
        updated.settings.activeIndex === 0 &&
        Array.isArray(updated.settings.pages) &&
        updated.settings.pages.length === 3 &&
        (((updated.settings.pages || [])[0] || {}).thresholds || []).length === 1;
    }, "controlled dial settings after global threshold test");

    const initialImage = await waitForImage(slot.keyIndex, Boolean, "dial feedback image");
    const initialPng = decodePngDataUrl(initialImage);
    const edgeY = Math.floor(initialPng.height / 2);
    assert(isSeparatorColor(initialPng.pixel(0, edgeY)), "left separator band pixel 0 missing from rendered PNG");
    assert(isSeparatorColor(initialPng.pixel(1, edgeY)), "left separator band pixel 1 missing from rendered PNG");
    assert(isSeparatorColor(initialPng.pixel(2, edgeY)), "left separator band pixel 2 missing from rendered PNG");
    assert(isSeparatorColor(initialPng.pixel(initialPng.width - 3, edgeY)), "right separator band pixel 2 missing from rendered PNG");
    assert(isSeparatorColor(initialPng.pixel(initialPng.width - 2, edgeY)), "right separator band pixel 1 missing from rendered PNG");
    assert(isSeparatorColor(initialPng.pixel(initialPng.width - 1, edgeY)), "right separator band pixel 0 missing from rendered PNG");
    assert(!isSeparatorColor(initialPng.pixel(3, edgeY)), "left separator band wider than default 3px");
    assert(!isSeparatorColor(initialPng.pixel(initialPng.width - 4, edgeY)), "right separator band wider than default 3px");

    await postJson("/api/dials/rotate", { deviceId, dialIndex, ticks: 1 });
    await waitForSlot(slot.context, (updated) => updated.settings && updated.settings.activeIndex === 1, "dial rotate activeIndex +1");
    const pageTwoImage = await waitForImage(slot.keyIndex, (data) => data !== initialImage, "dial page 2 feedback image");
    await waitForImage(slot.keyIndex, (data) => data !== pageTwoImage, "live graph movement on threshold-free page", 7000);
    await postJson("/api/dials/rotate", { deviceId, dialIndex, ticks: -1 });
    await waitForSlot(slot.context, (updated) => updated.settings && updated.settings.activeIndex === 0, "dial rotate activeIndex restore");

    let fullscreenImage = initialImage;
    if (hasPageIndicator(initialPng)) {
      await postJson("/api/dials/press", { deviceId, dialIndex });
      fullscreenImage = await waitForImage(slot.keyIndex, (data) => {
        return !hasPageIndicator(decodePngDataUrl(data));
      }, "dial press initial fullscreen image");
    }
    assert(!hasPageIndicator(decodePngDataUrl(fullscreenImage)), "fullscreen image still shows overview indicator");

    await postJson("/api/dials/press", { deviceId, dialIndex });
    const overviewImage = await waitForImage(slot.keyIndex, (data) => {
      return hasPageIndicator(decodePngDataUrl(data));
    }, "dial press overview image");
    assert(hasPageIndicator(decodePngDataUrl(overviewImage)), "overview page indicator missing from rendered PNG");

    await postJson("/api/dials/press", { deviceId, dialIndex });
    fullscreenImage = await waitForImage(slot.keyIndex, (data) => {
      return !hasPageIndicator(decodePngDataUrl(data));
    }, "dial press fullscreen image");
    assert(!hasPageIndicator(decodePngDataUrl(fullscreenImage)), "fullscreen image still shows overview indicator");

    const beforeTouch = fullscreenImage;
    await postJson("/api/dials/touch", { deviceId, dialIndex });
    await waitForImage(slot.keyIndex, (data) => data !== beforeTouch, "touch/snooze feedback image");

    console.log("deckbridge dial live e2e ok");
  } finally {
    await sendDialSettings(ws, slot, originalSettings);
    await waitForSlot(slot.context, (restored) => {
      const pages = (restored.settings && restored.settings.pages) || [];
      return pages.length === ((originalSettings.pages || []).length) &&
        restored.settings.activeIndex === (originalSettings.activeIndex || 0);
    }, "dial settings restore");
    ws.close();
    settingsWs.close();
  }
}

main().catch((err) => {
  console.error("deckbridge dial live e2e failed:", err.message);
  process.exit(1);
});
