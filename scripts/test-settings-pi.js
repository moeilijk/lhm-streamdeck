#!/usr/bin/env node
"use strict";

const fs = require("fs");
const vm = require("vm");

function assert(condition, msg) {
  if (!condition) {
    throw new Error(msg);
  }
}

class FakeElement {
  constructor(initial = {}) {
    this.value = initial.value || "";
    this.checked = initial.checked || false;
    this.textContent = initial.textContent || "";
    this.style = initial.style || {};
    this.handlers = {};
  }
  addEventListener(evt, fn) {
    this.handlers[evt] = this.handlers[evt] || [];
    this.handlers[evt].push(fn);
  }
  trigger(evt) {
    const fns = this.handlers[evt] || [];
    fns.forEach((fn) => fn({ target: this }));
  }
}

function loadSandbox(opts = {}) {
  const elements = {
    pollInterval: new FakeElement({ value: "1000" }),
    currentRate: new FakeElement({ textContent: "" }),
    tileBackground: new FakeElement({ value: "#112233" }),
    tileTextColor: new FakeElement({ value: "#aabbcc" }),
    showLabel: new FakeElement({ checked: true }),
    connectionStatus: new FakeElement({ textContent: "", style: {} }),
  };
  const sent = [];
  let intervalFn = null;

  const sandbox = {
    console,
    JSON,
    URLSearchParams,
    setTimeout: (fn) => {
      fn();
      return 1;
    },
    setInterval: (fn) => {
      intervalFn = fn;
      return 2;
    },
    clearInterval: () => {},
    window: null,
    websocket: null,
    WebSocket: function () {
      return opts.mockSocket || { readyState: 1, send() {} };
    },
    document: {
      readyState: "complete",
      addEventListener() {},
      getElementById(id) {
        return elements[id] || null;
      },
    },
  };
  sandbox.window = sandbox;

  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync("com.moeilijk.lhm.sdPlugin/settings_pi.js", "utf8"), sandbox);

  sandbox.websocket = {
    readyState: 1,
    send(msg) {
      sent.push(JSON.parse(msg));
    },
  };

  return { sandbox, elements, sent, getIntervalFn: () => intervalFn };
}

function testShowLabelPayload() {
  const { sandbox, elements, sent } = loadSandbox();
  sandbox.context = "ctx-settings";
  sandbox.uuid = "ctx-fallback";

  sandbox.saveTileSettings("force");
  elements.showLabel.checked = false;
  sandbox.saveTileSettings("force");

  const updates = sent.filter(
    (m) => m.event === "sendToPlugin" && m.payload && m.payload.updateTileAppearance
  );
  assert(updates.length >= 2, "expected at least two updateTileAppearance events");
  assert(updates[0].payload.updateTileAppearance.showLabel === true, "first payload should be true");
  assert(updates[updates.length - 1].payload.updateTileAppearance.showLabel === false, "last payload should be false");
}

function testContextFanout() {
  const { sandbox, sent } = loadSandbox();
  sandbox.context = "ctx-action";
  sandbox.uuid = "ctx-pi";

  sandbox.saveTileSettings("force");
  const setSettings = sent.filter((m) => m.event === "setSettings");
  const updateMsgs = sent.filter((m) => m.event === "sendToPlugin" && m.payload && m.payload.updateTileAppearance);
  assert(setSettings.length === 1, "expected one setSettings message");
  assert(updateMsgs.length === 1, "expected one updateTileAppearance message");
  assert(setSettings[0].context === "ctx-pi", "setSettings should use PI uuid context");
  assert(updateMsgs[0].context === "ctx-pi", "sendToPlugin should use PI uuid context");
}

function testPollIntervalEvents() {
  const { sandbox, elements, sent } = loadSandbox();
  sandbox.context = "ctx-settings";
  sandbox.uuid = "ctx-fallback";
  elements.pollInterval.value = "2000";
  elements.pollInterval.trigger("change");

  const global = sent.find((m) => m.event === "setGlobalSettings");
  assert(global, "missing setGlobalSettings");
  assert(global.payload.pollInterval === 2000, "poll interval payload mismatch");

  const pollMsgs = sent.filter((m) => m.event === "sendToPlugin" && m.payload && m.payload.setPollInterval === 2000);
  assert(pollMsgs.length === 1, "expected one setPollInterval message");
}

function testDidReceiveSettingsAppliesUi() {
  const ws = {
    readyState: 1,
    send() {},
    onopen: null,
    onmessage: null,
  };
  const { sandbox, elements } = loadSandbox({ mockSocket: ws });
  sandbox.connectElgatoStreamDeckSocket("12345", "uuid-x", "registerPropertyInspector", "{}", JSON.stringify({
    action: "com.moeilijk.lhm.settings",
    context: "ctx-x",
  }));
  ws.onmessage({
    data: JSON.stringify({
      event: "didReceiveSettings",
      payload: {
        settings: {
          tileBackground: "#334455",
          tileTextColor: "#fedcba",
          showLabel: false,
        },
      },
    }),
  });
  assert(elements.tileBackground.value === "#334455", "tileBackground not applied");
  assert(elements.tileTextColor.value === "#fedcba", "tileTextColor not applied");
  assert(elements.showLabel.checked === false, "showLabel not applied");
}

function testMalformedInputsDoNotCrash() {
  const ws = {
    readyState: 1,
    send() {},
    onopen: null,
    onmessage: null,
  };
  const { sandbox } = loadSandbox({ mockSocket: ws });
  sandbox.connectElgatoStreamDeckSocket("12345", "uuid-x", "registerPropertyInspector", "{bad", "{bad");
  ws.onmessage({ data: "{bad" });
}

function testPollingFallbackSave() {
  const { sandbox, elements, sent, getIntervalFn } = loadSandbox();
  sandbox.context = "ctx-settings";
  sandbox.uuid = "ctx-fallback";

  const ws = {
    readyState: 1,
    send(msg) {
      sent.push(JSON.parse(msg));
    },
    onopen: null,
    onmessage: null,
  };
  sandbox.WebSocket = function () {
    return ws;
  };
  sandbox.connectElgatoStreamDeckSocket("12345", "uuid-x", "registerPropertyInspector", "{}", JSON.stringify({
    action: "com.moeilijk.lhm.settings",
    context: "ctx-settings",
  }));
  ws.onopen();

  elements.tileBackground.value = "#445566";
  const tick = getIntervalFn();
  assert(typeof tick === "function", "interval fallback function missing");
  tick();

  const updates = sent.filter((m) => m.event === "sendToPlugin" && m.payload && m.payload.updateTileAppearance);
  assert(updates.length >= 1, "polling fallback did not send update");
}

function main() {
  testShowLabelPayload();
  testContextFanout();
  testPollIntervalEvents();
  testDidReceiveSettingsAppliesUi();
  testMalformedInputsDoNotCrash();
  testPollingFallbackSave();
  process.stdout.write("settings-pi tests ok (6 cases)\n");
}

main();
