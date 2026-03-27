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
    this.checked = initial.checked === true;
    this.disabled = initial.disabled === true;
    this.dataset = initial.dataset || {};
    this.style = initial.style || {};
    this.handlers = {};
    this.classList = {
      values: new Set(initial.classNames || []),
      add: (...names) => names.forEach((name) => this.classList.values.add(name)),
      remove: (...names) => names.forEach((name) => this.classList.values.delete(name)),
      contains: (name) => this.classList.values.has(name),
      toggle: (name, force) => {
        if (force === undefined) {
          if (this.classList.values.has(name)) {
            this.classList.values.delete(name);
            return false;
          }
          this.classList.values.add(name);
          return true;
        }
        if (force) {
          this.classList.values.add(name);
          return true;
        }
        this.classList.values.delete(name);
        return false;
      },
    };
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

function loadSandbox() {
  const snoozeButtons = [
    new FakeElement({ dataset: { value: "300000" } }),
    new FakeElement({ dataset: { value: "900000" } }),
    new FakeElement({ dataset: { value: "3600000" } }),
    new FakeElement({ dataset: { value: "0" } }),
  ];

  const selectors = {
    "#snooze5m": snoozeButtons[0],
    "#snooze15m": snoozeButtons[1],
    "#snooze1h": snoozeButtons[2],
    "#snoozeUntilResumed": snoozeButtons[3],
  };

  const sent = [];
  const sandbox = {
    console,
    JSON,
    setTimeout: (fn) => {
      fn();
      return 1;
    },
    clearTimeout: () => {},
    navigator: {
      appVersion: "QtWebEngine",
    },
    document: {
      querySelector(selector) {
        return selectors[selector] || null;
      },
      querySelectorAll(selector) {
        if (selector === ".snooze-duration") {
          return snoozeButtons;
        }
        return [];
      },
      addEventListener() {},
      body: {
        appendChild() {},
      },
    },
    window: null,
    websocket: null,
    uuid: "ctx-reading",
    actionInfo: {
      action: "com.moeilijk.lhm.reading",
    },
    Event: function Event(type) {
      this.type = type;
    },
  };
  sandbox.window = sandbox;
  sandbox.addEventListener = () => {};

  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync("com.moeilijk.lhm.sdPlugin/index_pi.js", "utf8"), sandbox);

  sandbox.uuid = "ctx-reading";
  sandbox.actionInfo = {
    action: "com.moeilijk.lhm.reading",
  };
  sandbox.websocket = {
    readyState: 1,
    send(msg) {
      sent.push(JSON.parse(msg));
    },
  };

  return { sandbox, sent, snoozeButtons };
}

function testNormalizeSnoozeDurations() {
  const { sandbox } = loadSandbox();
  const result = sandbox.normalizeSnoozeDurations([900000, 123, 0, 300000, 900000, 3600000]);
  assert(Array.isArray(result), "normalizeSnoozeDurations should return an array");
  assert(result.join(",") === "300000,900000,3600000,0", `unexpected normalized durations: ${result.join(",")}`);
}

function testApplySnoozeDurationsToUI() {
  const { sandbox, snoozeButtons } = loadSandbox();
  sandbox.applySnoozeDurationsToUI({
    snoozeDurations: [900000, 0],
  });

  assert(snoozeButtons[0].classList.contains("is-selected") === false, "5m should stay unselected");
  assert(snoozeButtons[1].classList.contains("is-selected") === true, "15m should be selected");
  assert(snoozeButtons[2].classList.contains("is-selected") === false, "1h should stay unselected");
  assert(snoozeButtons[3].classList.contains("is-selected") === true, "until resumed should be selected");
}

function testReadSnoozeDurationsFromUI() {
  const { sandbox, snoozeButtons } = loadSandbox();
  snoozeButtons[2].classList.add("is-selected");
  snoozeButtons[0].classList.add("is-selected");

  const result = sandbox.readSnoozeDurationsFromUI();
  assert(result.join(",") === "300000,3600000", `unexpected selected durations: ${result.join(",")}`);
}

function testBindSnoozeControlsSendsPayload() {
  const { sandbox, sent, snoozeButtons } = loadSandbox();
  sandbox.bindSnoozeControls();

  snoozeButtons[3].classList.add("is-selected");
  snoozeButtons[0].trigger("click");

  assert(sent.length >= 1, "expected snooze control change to send a plugin message");
  const msg = sent[sent.length - 1];
  assert(msg.event === "sendToPlugin", "expected sendToPlugin event");
  assert(msg.action === "com.moeilijk.lhm.reading", "unexpected action in plugin payload");
  assert(msg.context === "ctx-reading", "unexpected context in plugin payload");
  assert(msg.payload && msg.payload.sdpi_collection, "missing sdpi_collection payload");
  assert(msg.payload.sdpi_collection.key === "snoozeDurations", "unexpected payload key");
  assert(
    Array.isArray(msg.payload.sdpi_collection.selection) &&
      msg.payload.sdpi_collection.selection.join(",") === "300000,0",
    `unexpected snooze selection payload: ${JSON.stringify(msg.payload.sdpi_collection.selection)}`
  );
}

function main() {
  testNormalizeSnoozeDurations();
  testApplySnoozeDurationsToUI();
  testReadSnoozeDurationsFromUI();
  testBindSnoozeControlsSendsPayload();
  process.stdout.write("reading-pi tests ok (4 cases)\n");
}

main();
