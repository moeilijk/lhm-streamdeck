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
  assert(p.foregroundColor === "#005128" && p.highlightColor === "#009e00", "first page uses first palette color");
  sb.addSelectedPage();
  const p2 = sb.currentSettings.pages[1];
  assert(p2.foregroundColor === "#003f73" && p2.highlightColor === "#00a2ff", "second page uses second palette color");
  assert(sb.currentSettings.activeIndex === 1, "active index = newest page");
}

function testDialPagePaletteCoversBulkAdd() {
  const { dialDefaultPageColors } = loadDialSandbox();
  const seen = new Set();
  for (let i = 0; i < 16; i++) {
    const colors = dialDefaultPageColors(i);
    const key = colors.foregroundColor + "|" + colors.highlightColor;
    assert(!seen.has(key), "first 16 bulk page colors are unique");
    seen.add(key);
  }
  const first = dialDefaultPageColors(0);
  const wrapped = dialDefaultPageColors(16);
  assert(wrapped.foregroundColor === first.foregroundColor && wrapped.highlightColor === first.highlightColor, "palette wraps after 16 colors");
}

function testAddPageButtonSendsSettings() {
  const listeners = {};
  const sent = [];
  const sb = loadDialSandbox({
    addPageBtn: {
      dataset: {},
      addEventListener(type, handler) { listeners[type] = handler; },
    },
    pageSensorSelect: { value: "cpu" },
    pageReadingSelect: { value: "2", disabled: false },
  });
  sb.websocket = {
    readyState: 1,
    send(payload) { sent.push(JSON.parse(payload)); },
  };
  sb.uuid = "ctx";
  sb.actionInfo = { action: "com.moeilijk.lhm.dial" };
  sb.renderPages = () => {};
  sb.currentCatalog = { readings: [
    { id: 1, sensorUid: "cpu", label: "CPU Core #1", unit: "%" },
    { id: 2, sensorUid: "cpu", label: "CPU Core #2", unit: "%" },
  ] };
  sb.currentSettings = { activeIndex: 0, pages: [{ title: "CPU Core #1" }] };
  sb.pageSelectionDraft = { sensorUid: "cpu", readingId: "2" };

  sb.bindAddPageControl();
  listeners.click({
    preventDefault() {},
    stopPropagation() {},
  });

  assert(sb.currentSettings.pages.length === 2, "button click adds a page");
  assert(sb.currentSettings.pages[1].title === "CPU Core #2", "button click uses selected reading");
  assert(sent.length === 1, "button click sends one plugin message");
  assert(sent[0].payload && sent[0].payload.dialSetSettings, "button click sends dialSetSettings");
  assert(sent[0].payload.dialSetSettings.pages.length === 2, "sent settings include added page");
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
  assert(html.includes("3c5e9b5 + V5-prep.19"), "plugin V5 build ref visible in PI");
  assert(html.includes('dial_pi.js?v=V5-prep.19'), "dial PI script is cache-busted with build ref");
  assert(html.includes("Dial press"), "dial-press row present");
  assert(html.includes("Toggle overview"), "dial-press behavior visible");
  assert(html.includes('id="globalThresholdsSection" hidden'), "global thresholds section starts hidden");
}

function testBulkPreviewListUsesDarkPiSelectStyle() {
  const html = fs.readFileSync("com.moeilijk.lhm.sdPlugin/dial_pi.html", "utf8");
  assert(html.includes("#pageList, #bulkPreviewList"), "bulk preview list shares dark list styling");
  assert(html.includes("#pageList option, #bulkPreviewList option"), "bulk preview options use dark option styling");
  assert(html.includes("#pageList option:checked, #bulkPreviewList option:checked"), "bulk preview selected option remains readable");
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

function testGlobalThresholdToggleClickSavesPageSuppression() {
  const buttons = [];
  const section = { hidden: false };
  const container = {
    innerHTML: "",
    appendChild() {},
  };
  const sb = loadDialSandbox({
    pageList: { selectedIndex: 0 },
  });
  sb.document.querySelector = (selector) => {
    if (selector === "#globalRefsContainer") return container;
    if (selector === "#globalThresholdsSection") return section;
    return null;
  };
  sb.document.createElement = (tag) => {
    const el = {
      className: "",
      textContent: "",
      title: "",
      style: {},
      appendChild() {},
      addEventListener(type, handler) {
        if (tag === "button" && type === "click") {
          buttons.push({ el, handler });
        }
      },
    };
    return el;
  };
  sb.renderPages = () => {};
  sb.currentSettings = {
    activeIndex: 0,
    pages: [{
      sensorUid: "cpu",
      readingId: "1",
      thresholds: null,
      currentThresholdId: "usage",
    }],
  };
  sb.currentCatalog = { readings: [{ id: "1", sensorUid: "cpu", type: "Usage" }] };
  sb.globalThresholds = [{ id: "usage", name: "Usage", readingType: "Usage" }];

  sb.renderActiveGlobals();
  assert(buttons.length === 1, "global threshold toggle rendered");
  buttons[0].handler();
  assert(sb.currentSettings.pages[0].suppressedGlobalIDs.join(",") === "usage", "global threshold suppression saved from click");

  buttons[0].handler();
  assert(sb.currentSettings.pages[0].suppressedGlobalIDs.length === 0, "global threshold unsuppression saved from click");
}

function testSnoozeClickSavesPerPageDurations() {
  // Drives the real bindSnoozeControls click handler against a stubbed DOM and
  // asserts the toggle writes per-page snoozeDurations and persists them.
  function makeButton(value) {
    const classes = {};
    return {
      dataset: { value: String(value) },
      _h: {},
      classList: {
        contains(cls) { return classes[cls] === true; },
        toggle(cls, force) {
          const next = force === undefined ? !classes[cls] : force === true;
          classes[cls] = next;
          return next;
        },
      },
      addEventListener(type, handler) { this._h[type] = handler; },
    };
  }
  const fifteenMin = makeButton(900000);
  const buttons = [fifteenMin, makeButton(300000), makeButton(0)];
  const sent = [];
  const sb = loadDialSandbox({ pageList: { selectedIndex: 0 } });
  sb.document.querySelectorAll = (selector) =>
    selector === ".snooze-duration" ? buttons : [];
  sb.renderPages = () => {};
  sb.websocket = {
    readyState: 1,
    send(payload) { sent.push(JSON.parse(payload)); },
  };
  sb.uuid = "ctx";
  sb.currentSettings = { activeIndex: 0, pages: [{ title: "A", snoozeDurations: [] }] };

  sb.bindSnoozeControls();

  // Selecting the 15-minute preset persists it on the active page.
  fifteenMin._h.click();
  assert(
    sb.currentSettings.pages[0].snoozeDurations.join(",") === "900000",
    "snooze toggle saves the selected duration on the active page"
  );
  assert(
    sent.length === 1 && sent[0].payload && sent[0].payload.dialSetSettings,
    "snooze toggle persists via dialSetSettings"
  );

  // Toggling the same preset off clears it again.
  fifteenMin._h.click();
  assert(
    sb.currentSettings.pages[0].snoozeDurations.length === 0,
    "snooze toggle off clears the per-page duration"
  );
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

function testSeparatorIsActionLevelWithCurrentDefault() {
  function rangeHost(input) {
    return { tagName: "DIV", querySelector: (s) => (s === "input[type=range]" ? input : null) };
  }
  const widthInput = { type: "range", value: "3", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const colorInput = { tagName: "INPUT", type: "color", value: "#000000", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const sb = loadDialSandbox({ separatorWidth: rangeHost(widthInput), separatorColor: colorInput });
  sb.renderPages = () => {};

  // Default (unset) must show the current separator look: width 3, #363e46.
  sb.currentSettings = { activeIndex: 0, pages: [] };
  sb.renderDialSettings();
  assert(widthInput.value === "3", "default separator width 3 (current look)");
  assert(colorInput.value === "#363e46", "default separator color #363e46 (current look)");

  // The controls are action-level: they write to currentSettings, not a page.
  sb.bindDialSettings();
  widthInput.value = "7";
  widthInput._h.input();
  assert(sb.currentSettings.separatorWidth === 7, "width writes to action settings");
  colorInput.value = "#ff0000";
  colorInput._h.change();
  assert(sb.currentSettings.separatorColor === "#ff0000", "color writes to action settings");
  assert(!("separatorWidth" in (sb.currentSettings.pages[0] || {})), "separator is not stored per page");
}

function testDialViewOptionsAreActionLevel() {
  const defaultView = { tagName: "SELECT", type: "select-one", value: "fullscreen", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const indicatorStyle = { tagName: "SELECT", type: "select-one", value: "auto", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const indicatorFullscreen = { tagName: "INPUT", type: "checkbox", checked: false, dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const indicatorColor = { tagName: "INPUT", type: "color", value: "#000000", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const indicatorSize = { tagName: "INPUT", type: "range", value: "1", dataset: {}, _h: {}, addEventListener(t, h) { this._h[t] = h; } };
  const sb = loadDialSandbox({ defaultView, indicatorStyle, indicatorFullscreen, indicatorColor, indicatorSize });
  sb.renderPages = () => {};
  sb.currentSettings = { activeIndex: 0, pages: [{}] };

  sb.renderDialSettings();
  assert(defaultView.value === "fullscreen", "default view defaults to fullscreen");
  assert(indicatorStyle.value === "auto", "indicator style defaults to auto");
  assert(indicatorFullscreen.checked === false, "indicator-in-fullscreen defaults off");
  assert(indicatorColor.value === "#bec6ce", "indicator color defaults to light grey");
  assert(indicatorSize.value === "6", "indicator size defaults to 6");

  sb.bindDialSettings();
  defaultView.value = "overview";
  defaultView._h.change();
  indicatorStyle.value = "count";
  indicatorStyle._h.change();
  indicatorFullscreen.checked = true;
  indicatorFullscreen._h.change();
  indicatorColor.value = "#ff0000";
  indicatorColor._h.change();
  indicatorSize.value = "7.5";
  indicatorSize._h.change();
  assert(sb.currentSettings.defaultView === "overview", "default view writes to action settings");
  assert(sb.currentSettings.indicatorStyle === "count", "indicator style writes to action settings");
  assert(sb.currentSettings.indicatorFullscreen === true, "indicator-in-fullscreen writes to action settings");
  assert(sb.currentSettings.indicatorColor === "#ff0000", "indicator color writes to action settings");
  assert(sb.currentSettings.indicatorSize === 7.5, "indicator size writes to action settings");
  assert(!("defaultView" in (sb.currentSettings.pages[0] || {})), "default view is not stored per page");
  assert(!("indicatorFullscreen" in (sb.currentSettings.pages[0] || {})), "indicator-in-fullscreen is not stored per page");
}

const tests = [
  ["normalizePage defaults", testNormalizePageDefaults],
  ["separator is action-level with current default", testSeparatorIsActionLevelWithCurrentDefault],
  ["dial view options are action-level", testDialViewOptionsAreActionLevel],
  // Bulk behaviour is covered end-to-end against the real DOM in
  // scripts/test-dial-bulk-render.js (and live in test-dial-bulk-live-e2e.js).
  ["normalizeSettings clamps activeIndex", testNormalizeSettingsClampsActiveIndex],
  ["fieldInput range-wrap handling", testFieldInputRangeWrap],
  ["pageTitle fallbacks", testPageTitle],
  ["sensorMatchesFilter", testSensorMatchesFilter],
  ["readingsForSensor filter+sort", testReadingsForSensor],
  ["resetPageSelectionDraft", testResetPageSelectionDraft],
  ["addSelectedPage derive sentinel", testAddSelectedPageSentinel],
  ["dial page palette covers bulk add", testDialPagePaletteCoversBulkAdd],
  ["add page button sends settings", testAddPageButtonSendsSettings],
  ["moveSelectedPage", testMoveSelectedPage],
  ["removeSelectedPage", testRemoveSelectedPage],
  ["build ref visible in PI", testBuildRefVisibleInPi],
  ["bulk preview list uses dark PI select style", testBulkPreviewListUsesDarkPiSelectStyle],
  ["snooze duration normalization", testSnoozeDurationNormalization],
  ["threshold helpers", testThresholdHelpers],
  ["global threshold helpers per page", testGlobalThresholdHelpersPerPage],
  ["global threshold section only when active", testGlobalThresholdSectionOnlyWhenActive],
  ["global threshold toggle click saves page suppression", testGlobalThresholdToggleClickSavesPageSuppression],
  ["snooze click saves per-page durations", testSnoozeClickSavesPerPageDurations],
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
