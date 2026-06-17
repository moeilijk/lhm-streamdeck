#!/usr/bin/env node
"use strict";

// Functional unit tests for the dial Property Inspector logic (dial_pi.js),
// following the same VM-sandbox pattern as test-reading-pi.js.

const fs = require("fs");
const vm = require("vm");

function assert(condition, msg) {
  if (!condition) {
    throw new Error(msg);
  }
}

function loadDialSandbox(elementsById = {}) {
  const sandbox = {
    console,
    JSON,
    setTimeout: (fn) => { fn(); return 1; },
    clearTimeout: () => {},
    navigator: { appVersion: "QtWebEngine" },
    location: { hostname: "127.0.0.1" },
    WebSocket: function WebSocket() {},
    document: {
      querySelector() { return null; },
      querySelectorAll() { return []; },
      getElementById(id) { return elementsById[id] || null; },
      createElement() { return {}; },
      addEventListener() {},
      body: { appendChild() {} },
    },
    window: null,
    websocket: null,
  };
  sandbox.window = sandbox;
  sandbox.addEventListener = () => {};
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync("com.moeilijk.lhm.sdPlugin/pi_utils.js", "utf8"), sandbox);
  vm.runInContext(fs.readFileSync("com.moeilijk.lhm.sdPlugin/dial_pi.js", "utf8"), sandbox);
  return sandbox;
}

function testNormalizePageDefaults() {
  const { normalizePage } = loadDialSandbox();
  const p = normalizePage({});
  assert(p.min === 0, "min default 0");
  assert(p.max === 100, "max default 100");
  assert(p.smoothingAlpha === 0, "smoothingAlpha default 0 (off)");
  assert(p.graphMode === "both", "graphMode default both");
  assert(p.graphHeightPct === 100, "graphHeightPct default 100");
  assert(p.graphLineThickness === 1, "graphLineThickness default 1");
  assert(Array.isArray(p.thresholds) && p.thresholds.length === 0, "thresholds default []");
  assert(Array.isArray(p.suppressedGlobalIDs) && p.suppressedGlobalIDs.length === 0, "suppressed globals default []");
  assert(Array.isArray(p.snoozeDurations) && p.snoozeDurations.length === 0, "snooze durations default []");
  assert(p.currentThresholdId === "", "current threshold default empty");
  assert(p.showTitleInGraph === true, "showTitleInGraph default true");
  assert(p.foregroundColor === "#005128", "foregroundColor default");
  const keep = normalizePage({ smoothingAlpha: 0.3, max: 0, snoozeDurations: [0, "900000", 123] });
  assert(keep.smoothingAlpha === 0.3, "explicit smoothingAlpha preserved");
  assert(keep.max === 0, "explicit max:0 sentinel preserved for derive");
  assert(keep.snoozeDurations.join(",") === "900000,0", "snooze durations normalized");
}

function testNormalizeSettingsClampsActiveIndex() {
  const { normalizeSettings } = loadDialSandbox();
  const clamped = normalizeSettings({ activeIndex: 5, pages: [{}, {}, {}] });
  assert(clamped.activeIndex === 2, "activeIndex clamped to last page");
  const neg = normalizeSettings({ activeIndex: -1, pages: [{}] });
  assert(neg.activeIndex === 0, "negative activeIndex -> 0");
  const empty = normalizeSettings({});
  assert(Array.isArray(empty.pages) && empty.pages.length === 0, "missing pages -> []");
  assert(clamped.pages[0].smoothingAlpha === 0, "pages run through normalizePage");
}

function testFieldInputRangeWrap() {
  const { fieldInput } = loadDialSandbox();
  assert(fieldInput(null) === null, "null host -> null");
  const input = { tagName: "INPUT", type: "range" };
  assert(fieldInput(input) === input, "plain input returned as-is");
  const nested = { type: "range" };
  const wrap = { tagName: "DIV", querySelector: (s) => (s === "input[type=range]" ? nested : null) };
  assert(fieldInput(wrap) === nested, "range-wrap host -> nested range input");
  const bare = { tagName: "DIV", querySelector: () => null };
  assert(fieldInput(bare) === bare, "div without range input -> itself");
}

function testPageTitle() {
  const { pageTitle } = loadDialSandbox();
  assert(pageTitle({ title: "X" }) === "X", "explicit title wins");
  assert(pageTitle({ readingLabel: "L" }) === "L", "falls back to readingLabel");
  assert(pageTitle({ sensorUid: "s" }) === "s", "falls back to sensorUid");
  assert(pageTitle({}) === "Reading", "default Reading");
}

function testSensorMatchesFilter() {
  const { sensorMatchesFilter } = loadDialSandbox();
  const s = { name: "CPU Package", category: "CPU", uid: "/cpu" };
  assert(sensorMatchesFilter(s, "", "") === true, "no filter matches");
  assert(sensorMatchesFilter(s, "package", "") === true, "term matches name");
  assert(sensorMatchesFilter(s, "gpu", "") === false, "non-matching term");
  assert(sensorMatchesFilter(s, "", "cpu") === true, "category matches");
  assert(sensorMatchesFilter(s, "", "gpu") === false, "non-matching category");
}

function testReadingsForSensor() {
  const sb = loadDialSandbox({
    pageList: { selectedIndex: 0 },
  });
  sb.currentCatalog = { readings: [
    { id: 2, sensorUid: "cpu", label: "B", unit: "%" },
    { id: 1, sensorUid: "cpu", label: "A", unit: "%" },
    { id: 3, sensorUid: "gpu", label: "C", unit: "%" },
  ] };
  const r = sb.readingsForSensor("cpu");
  assert(r.length === 2, "filters by sensor uid");
  assert(r[0].label === "A" && r[1].label === "B", "sorted by label");
}

function testResetPageSelectionDraft() {
  const sb = loadDialSandbox({
    pageList: { selectedIndex: 0 },
  });
  sb.resetPageSelectionDraft({ sensorUid: "cpu", readingId: 7 });
  assert(sb.pageSelectionDraft.sensorUid === "cpu", "draft sensor set");
  assert(sb.pageSelectionDraft.readingId === "7", "draft reading stringified");
  sb.resetPageSelectionDraft(null);
  assert(sb.pageSelectionDraft.sensorUid === "" && sb.pageSelectionDraft.readingId === "", "null clears draft");
}

function testAddSelectedPageSentinel() {
  const sb = loadDialSandbox({
    pageSensorSelect: { value: "cpu" },
    pageReadingSelect: { value: "1" },
  });
  sb.renderPages = () => {};
  sb.currentCatalog = { readings: [{ id: 1, sensorUid: "cpu", label: "CPU Core #1", unit: "%" }] };
  sb.pageSelectionDraft = { sensorUid: "cpu", readingId: "1" };
  sb.currentSettings = { activeIndex: 0, pages: [] };
  sb.addSelectedPage();
  assert(sb.currentSettings.pages.length === 1, "page added");
  const p = sb.currentSettings.pages[0];
  assert(p.min === 0 && p.max === 0, "new page uses min:0/max:0 derive sentinel");
  assert(p.sensorUid === "cpu" && String(p.readingId) === "1", "reading wired onto page");
  assert(sb.currentSettings.activeIndex === 0, "active index = new page");
}

function testMoveSelectedPage() {
  const sb = loadDialSandbox({ pageList: { selectedIndex: 0 } });
  sb.renderPages = () => {};
  sb.currentSettings = { activeIndex: 0, pages: [{ title: "A" }, { title: "B" }] };
  sb.moveSelectedPage(1);
  assert(sb.currentSettings.pages[0].title === "B" && sb.currentSettings.pages[1].title === "A", "pages swapped");
  assert(sb.currentSettings.activeIndex === 1, "active follows moved page");
}

function testRemoveSelectedPage() {
  const sb = loadDialSandbox({ pageList: { selectedIndex: 1 } });
  sb.renderPages = () => {};
  sb.currentSettings = { activeIndex: 1, pages: [{ title: "A" }, { title: "B" }] };
  sb.removeSelectedPage();
  assert(sb.currentSettings.pages.length === 1, "page removed");
  assert(sb.currentSettings.pages[0].title === "A", "correct page removed");
  assert(sb.currentSettings.activeIndex === 0, "active index clamped");
}

function testBuildRefVisibleInPi() {
  const html = fs.readFileSync("com.moeilijk.lhm.sdPlugin/dial_pi.html", "utf8");
  assert(html.includes('id="pluginBuildRef"'), "plugin build ref row present");
  assert(html.includes("c56a229"), "plugin build commit visible in PI");
  assert(html.includes('id="globalThresholdsSection" hidden'), "global thresholds section starts hidden");
}

function testSnoozeDurationNormalization() {
  const { normalizeSnoozeDurations } = loadDialSandbox();
  const got = normalizeSnoozeDurations(["0", 3600000, 300000, 300000, "bad", 900000]);
  assert(got.join(",") === "300000,900000,3600000,0", "snooze durations use tile order and de-dupe");
  assert(normalizeSnoozeDurations(null).length === 0, "non-array snooze durations -> []");
}

function testThresholdHelpers() {
  const sb = loadDialSandbox();
  const t = sb.createThreshold("Hot");
  assert(t.name === "Hot", "threshold name set");
  assert(t.enabled === true, "threshold enabled by default");
  assert(t.operator === ">=", "threshold operator default");
  sb.updateThresholdField(t, "value", "72.5");
  sb.updateThresholdField(t, "hysteresis", "");
  sb.updateThresholdField(t, "dwellMs", "1250");
  sb.updateThresholdField(t, "sticky", true);
  assert(t.value === 72.5, "threshold value parsed");
  assert(t.hysteresis === 0, "empty hysteresis -> 0");
  assert(t.dwellMs === 1250, "dwellMs parsed");
  assert(t.sticky === true, "sticky boolean parsed");
}

function testGlobalThresholdHelpersPerPage() {
  const sb = loadDialSandbox();
  const page = { sensorUid: "cpu", readingId: "1", suppressedGlobalIDs: [] };
  sb.currentCatalog = { readings: [
    { id: "1", sensorUid: "cpu", type: "Temp" },
    { id: "2", sensorUid: "gpu", type: "Usage" },
  ] };
  sb.globalThresholds = [
    { id: "temp", readingType: "Temp" },
    { id: "all", readingType: "" },
    { id: "usage", readingType: "Usage" },
  ];
  const active = sb.activeGlobalThresholdsForPage(page).map((gt) => gt.id).join(",");
  assert(active === "temp,all", "globals filtered by selected page reading type");
  sb.setGlobalSuppressed(page, "temp", true);
  sb.setGlobalSuppressed(page, "temp", true);
  assert(page.suppressedGlobalIDs.join(",") === "temp", "suppression added once");
  sb.setGlobalSuppressed(page, "temp", false);
  assert(page.suppressedGlobalIDs.length === 0, "suppression removed");
}

function testGlobalThresholdSectionOnlyWhenActive() {
  const section = { hidden: false };
  const container = { innerHTML: "", appendChild() {} };
  const sb = loadDialSandbox({
    pageList: { selectedIndex: 0 },
  });
  sb.document.querySelector = (selector) => {
    if (selector === "#globalRefsContainer") return container;
    if (selector === "#globalThresholdsSection") return section;
    return null;
  };
  sb.document.createElement = () => ({
    style: {},
    appendChild() {},
  });
  sb.currentSettings = { activeIndex: 0, pages: [{ sensorUid: "cpu", readingId: "1", suppressedGlobalIDs: [] }] };
  sb.currentCatalog = { readings: [{ id: "1", sensorUid: "cpu", type: "Temp" }] };
  sb.globalThresholds = [];
  sb.renderActiveGlobals();
  assert(section.hidden === true, "section hidden with no globals");
  sb.globalThresholds = [{ id: "usage", readingType: "Usage" }];
  sb.renderActiveGlobals();
  assert(section.hidden === true, "section hidden when globals do not match page type");
}

function testAddThresholdClickAddsOneThreshold() {
  const listeners = {};
  const addBtn = {
    dataset: {},
    addEventListener(type, handler) { listeners[type] = handler; },
  };
  const nameInput = {
    dataset: {},
    value: "Click Test",
    addEventListener() {},
  };
  const sb = loadDialSandbox({
    pageList: { selectedIndex: 0 },
  });
  sb.document.querySelector = (selector) => {
    if (selector === "#addThresholdBtn") return addBtn;
    if (selector === "#newThresholdName") return nameInput;
    return null;
  };
  sb.currentSettings = { activeIndex: 0, pages: [{ title: "A", thresholds: [] }] };
  sb.renderPages = () => {};
  sb.bindThresholdControls();
  listeners.click({
    preventDefault() {},
    stopPropagation() {},
  });
  assert(sb.currentSettings.pages[0].thresholds.length === 1, "one click adds exactly one threshold");
  assert(sb.currentSettings.pages[0].thresholds[0].name === "Click Test", "threshold name comes from input");
}

const tests = [
  ["normalizePage defaults", testNormalizePageDefaults],
  ["normalizeSettings clamps activeIndex", testNormalizeSettingsClampsActiveIndex],
  ["fieldInput range-wrap handling", testFieldInputRangeWrap],
  ["pageTitle fallbacks", testPageTitle],
  ["sensorMatchesFilter", testSensorMatchesFilter],
  ["readingsForSensor filter+sort", testReadingsForSensor],
  ["resetPageSelectionDraft", testResetPageSelectionDraft],
  ["addSelectedPage derive sentinel", testAddSelectedPageSentinel],
  ["moveSelectedPage", testMoveSelectedPage],
  ["removeSelectedPage", testRemoveSelectedPage],
  ["build ref visible in PI", testBuildRefVisibleInPi],
  ["snooze duration normalization", testSnoozeDurationNormalization],
  ["threshold helpers", testThresholdHelpers],
  ["global threshold helpers per page", testGlobalThresholdHelpersPerPage],
  ["global threshold section only when active", testGlobalThresholdSectionOnlyWhenActive],
  ["add threshold click adds one threshold", testAddThresholdClickAddsOneThreshold],
];

let failed = 0;
for (const [name, fn] of tests) {
  try {
    fn();
    console.log("ok - " + name);
  } catch (err) {
    failed += 1;
    console.error("FAIL - " + name + ": " + err.message);
  }
}
if (failed > 0) {
  console.error(failed + " dial PI test(s) failed");
  process.exit(1);
}
console.log("test-dial-pi: ok");
