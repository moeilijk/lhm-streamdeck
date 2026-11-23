#!/usr/bin/env node
"use strict";

// Real-DOM render test for the option controls of the reading tile.
//
// Loads the ACTUAL index_pi.html with the ACTUAL sdpi.css/local.css into jsdom,
// runs the ACTUAL pi_utils.js and index_pi.js in that window, and asserts what a
// user gets: the control is in the right section, it is visible, it shows the
// saved setting, and changing it sends the setting to the plugin.
//
// sdpi.css hides every input[type=checkbox]; a checkbox is only visible when it
// uses the repo's show-label-checkbox pattern. The control case at the end
// proves that cascade is really applied.

const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const sdpiDir = path.join(repoRoot, "com.moeilijk.lhm.sdPlugin");

function loadJsdom() {
  for (const c of ["jsdom", path.resolve(repoRoot, "node_modules/jsdom"), path.resolve(repoRoot, "../DeckBridge/node_modules/jsdom")]) {
    try {
      return require(c).JSDOM;
    } catch (e) {
      /* next */
    }
  }
  throw new Error("jsdom not found (keep the sibling DeckBridge/node_modules/jsdom reachable).");
}

const JSDOM = loadJsdom();
const read = (f) => fs.readFileSync(path.join(sdpiDir, f), "utf8");
const html = read("index_pi.html");
const css = read("css/sdpi.css") + "\n" + read("css/local.css");

// Checkbox options: element id (= settings key) and the <summary> of its section.
const CHECKBOXES = [{ id: "resetMinMaxOnPress", section: "Scale", label: "Reset on press" }];

let failures = 0;
function check(cond, msg) {
  if (cond) {
    console.log("ok - " + msg);
  } else {
    failures++;
    console.error("FAIL - " + msg);
  }
}

function bootPi() {
  const body = html.replace(/^[\s\S]*?<body[^>]*>/i, "").replace(/<\/body>[\s\S]*$/i, "").replace(/<script[\s\S]*?<\/script>/gi, "");
  const dom = new JSDOM(`<!DOCTYPE html><html><head><style>${css}</style></head><body>${body}</body></html>`, {
    pretendToBeVisual: true,
    runScripts: "outside-only",
  });
  const win = dom.window;
  win.eval(read("pi_utils.js"));
  win.eval(read("index_pi.js"));
  const sent = [];
  win.eval("uuid = 'ctx-reading'; actionInfo = { action: 'com.moeilijk.lhm.reading' };");
  win.__sent = sent;
  win.eval("websocket = { readyState: 1, send: function (m) { window.__sent.push(JSON.parse(m)); } };");
  return { win, sent };
}

const { win, sent } = bootPi();
const doc = win.document;

CHECKBOXES.forEach(({ id, section, label }) => {
  const box = doc.getElementById(id);
  check(!!box && box.type === "checkbox", `${id}: checkbox exists in index_pi.html`);
  if (!box) return;

  const details = box.closest("details");
  const summary = details && details.querySelector("summary");
  check(!!summary && summary.textContent.trim() === section, `${id}: sits in the ${section} section`);

  const row = box.closest(".sdpi-item");
  check(!!row && row.classList.contains("show-label-row"), `${id}: uses the show-label-row pattern`);
  check(box.classList.contains("show-label-checkbox") && box.parentElement.classList.contains("show-label-cell"), `${id}: uses the show-label-checkbox pattern`);
  const rowLabel = row && row.querySelector(".sdpi-item-label");
  check(!!rowLabel && rowLabel.textContent.trim() === label, `${id}: row label is "${label}"`);

  const display = win.getComputedStyle(box).display;
  check(display !== "none", `${id}: is visible (computed display="${display}")`);

  check(box.checked === false, `${id}: unchecked by default`);
  win.applyDisplayOptionSettings({ [id]: true });
  check(box.checked === true, `${id}: shows a saved true setting`);
  win.applyDisplayOptionSettings({});
  check(box.checked === false, `${id}: shows a tile without the setting as off`);

  win.bindDisplayOptionControls();
  const before = sent.length;
  box.checked = true;
  box.dispatchEvent(new win.Event("change", { bubbles: true }));
  const msg = sent[sent.length - 1];
  const c = msg && msg.payload && msg.payload.sdpi_collection;
  check(sent.length === before + 1 && !!c && c.key === id && c.checked === true, `${id}: a user change sends { key: "${id}", checked: true }`);
  check(!!msg && msg.event === "sendToPlugin" && msg.context === "ctx-reading", `${id}: the message targets this tile`);
});

// Select options: element id (= settings key), section, row label, the option
// values in order and the value shown for a tile without the setting.
const SELECTS = [{ id: "scaleMode", section: "Scale", label: "Range", values: ["manual", "auto", "lhm"], fallback: "manual" }];

win.prepareDOMElements(doc);
win.bindDisplayOptionControls();

SELECTS.forEach(({ id, section, label, values, fallback }) => {
  const sel = doc.getElementById(id);
  check(!!sel && sel.tagName === "SELECT", `${id}: select exists in index_pi.html`);
  if (!sel) return;

  const summary = sel.closest("details") && sel.closest("details").querySelector("summary");
  check(!!summary && summary.textContent.trim() === section, `${id}: sits in the ${section} section`);
  const rowLabel = sel.closest(".sdpi-item").querySelector(".sdpi-item-label");
  check(!!rowLabel && rowLabel.textContent.trim() === label, `${id}: row label is "${label}"`);
  check(sel.classList.contains("sdpi-item-value") && sel.classList.contains("select"), `${id}: uses the sdpi select pattern`);
  check(JSON.stringify(Array.from(sel.options).map((o) => o.value)) === JSON.stringify(values), `${id}: offers ${values.join(", ")}`);
  check(win.getComputedStyle(sel).display !== "none", `${id}: is visible`);

  win.applyDisplayOptionSettings({});
  check(sel.value === fallback, `${id}: a tile without the setting shows "${fallback}"`);
  values.forEach((v) => {
    win.applyDisplayOptionSettings({ [id]: v });
    check(sel.value === v, `${id}: shows a saved "${v}" setting`);
  });

  win.applyDisplayOptionSettings({});
  const before = sent.length;
  sel.value = values[values.length - 1];
  sel.dispatchEvent(new win.Event("change", { bubbles: true }));
  const msg = sent[sent.length - 1];
  const c = msg && msg.payload && msg.payload.sdpi_collection;
  check(sent.length === before + 1 && !!c && c.key === id && c.value === values[values.length - 1], `${id}: a user change sends { key: "${id}", value: "${values[values.length - 1]}" }`);
});

// Range: the manual Min/Max inputs are only shown in Manual. Reset on press
// stays available in every mode.
{
  const manual = doc.getElementById("paramsgroup");
  const resetRow = doc.getElementById("resetMinMaxOnPressRow");
  const shown = (el) => win.getComputedStyle(el).display !== "none";
  [["manual", true], ["auto", false], ["lhm", false]].forEach(([mode, want]) => {
    win.applyDisplayOptionSettings({ scaleMode: mode });
    check(shown(manual) === want, `scaleMode ${mode} (saved): Min/Max inputs ${want ? "shown" : "hidden"}`);
    check(shown(resetRow), `scaleMode ${mode} (saved): Reset on press stays shown`);
  });
  const sel = doc.getElementById("scaleMode");
  [["auto", false], ["manual", true], ["lhm", false]].forEach(([mode, want]) => {
    sel.value = mode;
    sel.dispatchEvent(new win.Event("change", { bubbles: true }));
    check(shown(manual) === want, `scaleMode ${mode} (picked by the user): Min/Max inputs ${want ? "shown" : "hidden"}`);
  });
}

// Control: a bare checkbox computes to display:none, so the visibility checks
// above fail if a control ever regresses to that markup.
const ctrl = new JSDOM(`<!DOCTYPE html><html><head><style>${css}</style></head><body><input class="sdpi-item-value" id="bareBox" type="checkbox" /></body></html>`, { pretendToBeVisual: true }).window;
check(ctrl.getComputedStyle(ctrl.document.getElementById("bareBox")).display === "none", "control: a bare sdpi checkbox computes display:none (cascade is applied)");

if (failures) {
  console.error(`test-reading-options-render: ${failures} failure(s)`);
  process.exit(1);
}
console.log("test-reading-options-render: ok");
