#!/usr/bin/env node
"use strict";

// TRUE end-to-end test for the dial bulk flow against a RUNNING DeckBridge + plugin.
//
// "End to end" reaches the DIAL PIXELS, not the PI DOM:
//   pick reading -> the bulk helper offers a concrete choice ("'Read' on all 4 disks")
//   -> Add -> settings PERSISTED to the plugin -> plugin RENDERS the dial -> the dial
//   image changes from the empty placeholder to the bulk-added readings.
//
// Correctness is checked WITHOUT re-deriving the code's predicate: the chosen
// "across matching sensors" set is verified by independent PROPERTIES (same label,
// same category) and COMPLETENESS against the raw catalog (every same-category sensor
// that has that reading is present).
//
// Non-destructive: it mutates one originally-empty dial and RESETS it afterwards.
// Guarded: skips cleanly (exit 0) when DeckBridge/jsdom is not reachable.

const fs = require("fs");
const path = require("path");
const crypto = require("crypto");

const repoRoot = path.resolve(__dirname, "..");
const BASE = process.env.DECKBRIDGE_URL || "http://127.0.0.1:34075";
const PLUGIN_ID = "com.moeilijk.lhm";

function loadDep(name) {
  for (const c of [name, path.resolve(repoRoot, "node_modules", name), path.resolve(repoRoot, "../DeckBridge/node_modules", name)]) {
    try {
      return require(c);
    } catch (e) {
      /* next */
    }
  }
  return null;
}

const jsdomMod = loadDep("jsdom");
const WebSocket = loadDep("ws");
const piHtml = fs.readFileSync(path.join(repoRoot, "com.moeilijk.lhm.sdPlugin/dial_pi.html"), "utf8");
const piUtils = fs.readFileSync(path.join(repoRoot, "com.moeilijk.lhm.sdPlugin/pi_utils.js"), "utf8");
const dialPi = fs.readFileSync(path.join(repoRoot, "com.moeilijk.lhm.sdPlugin/dial_pi.js"), "utf8");

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const norm = (v) => String(v || "").trim().toLowerCase();
const idOf = (sensorUid, readingId) => sensorUid + ":" + String(readingId);
const md5 = (s) => crypto.createHash("md5").update(String(s || "")).digest("hex").slice(0, 12);

async function getState() {
  const res = await fetch(`${BASE}/api/state`);
  if (!res.ok) throw new Error("state " + res.status);
  return res.json();
}
const dialSlots = (s) => (s.slots || []).filter((x) => x && /dial/i.test(String(x.actionId || x.action || "")));
const slotByContext = (s, ctx) => dialSlots(s).find((x) => x.context === ctx);

// Pick the reading whose (label,type,unit) appears on the MOST sensors — the
// "across matching sensors" case (disks, NICs, multi-GPU).
function findMultiInstanceReading(readings) {
  const groups = new Map();
  for (const r of readings) {
    const key = [norm(r.label), norm(r.type), norm(r.unit)].join("|");
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  }
  let best = null;
  let bestN = 1;
  for (const arr of groups.values()) {
    const n = new Set(arr.map((r) => r.sensorUid)).size;
    if (n > bestN) {
      bestN = n;
      best = arr;
    }
  }
  return best ? best[0] : null;
}

async function bootLivePi(slot) {
  const { JSDOM } = jsdomMod;
  const ctx = slot.context;
  const url = `${BASE}/${PLUGIN_ID}/dial_pi.html?context=${ctx}&wsPort=34075`;
  const dom = new JSDOM(piHtml, { runScripts: "dangerously", pretendToBeVisual: true, url });
  const win = dom.window;
  win.WebSocket = WebSocket;
  win.eval(piUtils);
  win.eval(dialPi);
  win.document.dispatchEvent(new win.Event("DOMContentLoaded"));
  const actionInfo = { action: slot.actionId, context: ctx, device: slot.deviceId || "deckbridge-plus-0", payload: { settings: {} } };
  win.connectElgatoStreamDeckSocket("34075", ctx, "registerPropertyInspector", "{}", JSON.stringify(actionInfo));
  for (let i = 0; i < 50; i++) {
    if (((win.currentCatalog || {}).readings || []).length > 0) return win;
    await sleep(100);
  }
  throw new Error("catalog never arrived from the live plugin");
}

const $ = (win, id) => win.document.getElementById(id);
const fire = (win, el, t) => el.dispatchEvent(new win.Event(t, { bubbles: true }));

async function pollDial(ctx, predicate, tries = 40) {
  for (let i = 0; i < tries; i++) {
    const slot = slotByContext(await getState(), ctx);
    if (slot && predicate(slot)) return slot;
    await sleep(150);
  }
  return slotByContext(await getState(), ctx);
}

async function resetDial(win, ctx) {
  try {
    win.currentSettings = { activeIndex: 0, pages: [], sourceProfileId: win.currentSettings.sourceProfileId || "" };
    win.saveSettings();
    await pollDial(ctx, (s) => ((s.settings && s.settings.pages) || []).length === 0);
  } catch (e) {
    console.error("cleanup warning: " + (e && e.message ? e.message : e));
  }
}

async function main() {
  if (!jsdomMod || !WebSocket) {
    console.log("skip: dial bulk live e2e (jsdom/ws not reachable)");
    return 0;
  }
  let state;
  try {
    state = await getState();
  } catch (e) {
    console.log("skip: dial bulk live e2e (DeckBridge not reachable at " + BASE + ")");
    return 0;
  }
  const target = dialSlots(state).find((s) => (((s.settings || {}).pages) || []).length === 0);
  if (!target) {
    console.log("skip: dial bulk live e2e (no empty dial to use as a non-destructive target)");
    return 0;
  }

  const win = await bootLivePi(target);
  const ctx = target.context;
  const readings = win.currentCatalog.readings || [];
  const seed = findMultiInstanceReading(readings);
  if (!seed) {
    console.log("skip: dial bulk live e2e (no multi-instance reading in the live catalog)");
    return 0;
  }
  const seedCat = norm(seed.category);
  // Independent completeness reference: every same-category sensor carrying this reading.
  const expectedSensors = new Set(
    readings.filter((r) => norm(r.category) === seedCat && norm(r.label) === norm(seed.label)).map((r) => r.sensorUid)
  );
  if (expectedSensors.size < 2) {
    console.log("skip: dial bulk live e2e (the multi-instance reading is not cross-sensor in one category)");
    return 0;
  }

  let failures = 0;
  const assert = require("assert");
  const check = (name, fn) => {
    try {
      fn();
      console.log("ok   - " + name);
    } catch (e) {
      failures++;
      console.error("FAIL - " + name + "\n       " + (e && e.message ? e.message : e));
    }
  };

  try {
    const before = slotByContext(state, ctx);
    const beforeImg = md5((before.feedback || {}).imageDataUrl);

    // Drive the REAL DOM: select the seed reading; the bulk helper auto-populates.
    win.currentSettings = { activeIndex: 0, pages: [], sourceProfileId: win.currentSettings.sourceProfileId || "" };
    win.pageSelectionDraft = { sensorUid: seed.sensorUid, readingId: String(seed.id) };
    $(win, "pageSensorSelect").value = seed.sensorUid;
    fire(win, $(win, "pageSensorSelect"), "change");
    $(win, "pageReadingSelect").value = String(seed.id);
    fire(win, $(win, "pageReadingSelect"), "change");

    // Choose the "This reading on the other matching sensors" rule and Preview.
    const ruleSel = $(win, "bulkRule");
    ruleSel.value = "matching-category";
    fire(win, ruleSel, "change");
    const chosenLabel = Array.from(ruleSel.options).find((o) => o.value === "matching-category").textContent;
    $(win, "bulkPreviewBtn").click();

    const cands = win.bulkPreviewCandidates || [];
    check("the matching-sensors rule yields the cross-sensor candidates", () => {
      assert.ok(cands.length >= 2, "expected >=2 candidates, got " + cands.length);
    });
    const candSensors = new Set(cands.map((c) => c.sensorUid));

    // CORRECTNESS by independent properties + completeness (not by re-deriving code):
    check("every chosen page is the SAME reading (label) as the seed", () => {
      for (const c of cands) assert.strictEqual(norm(c.reading.label), norm(seed.label), "wrong label: " + c.reading.label);
    });
    check("every chosen page is in the seed's category (not a different kind)", () => {
      for (const c of cands) assert.strictEqual(norm(c.reading.category), seedCat, "wrong category: " + c.reading.category);
    });
    check("the chosen set covers exactly the matching sensors in that category (complete)", () => {
      assert.deepStrictEqual([...candSensors].sort(), [...expectedSensors].sort(),
        "covered " + JSON.stringify([...candSensors]) + " vs catalog " + JSON.stringify([...expectedSensors]));
    });

    // Persist for real and let the plugin render the dial.
    $(win, "bulkAddBtn").click();
    const applied = await pollDial(ctx, (s) => (((s.settings || {}).pages) || []).length === cands.length);
    const pages = ((applied.settings || {}).pages) || [];

    check("plugin persisted one page per chosen match", () => {
      assert.strictEqual(pages.length, cands.length, "persisted " + pages.length + " of " + cands.length);
    });
    check("persisted pages have distinguishable, sensor-qualified titles", () => {
      const titles = pages.map((p) => p.title);
      assert.strictEqual(new Set(titles).size, titles.length, "titles not distinct: " + JSON.stringify(titles));
    });

    // THE FINISH LINE: the dial actually re-rendered.
    const rendered = await pollDial(ctx, (s) => {
      const img = ((s.feedback || {}).imageDataUrl) || "";
      return img && md5(img) !== beforeImg;
    });
    check("the dial image changed from the empty placeholder (plugin rendered the pages)", () => {
      const img = ((rendered.feedback || {}).imageDataUrl) || "";
      assert.ok(img.length > 0 && md5(img) !== beforeImg, "dial image is still the empty placeholder");
    });

    console.log("\nchoice: " + JSON.stringify(chosenLabel));
    console.log("seed " + seed.sensorUid + "/" + JSON.stringify(seed.label) + " -> " + pages.length + " pages -> dial re-rendered (img " + beforeImg + " -> " + md5(((rendered.feedback || {}).imageDataUrl)) + ")");
  } finally {
    await resetDial(win, ctx);
    const cleaned = slotByContext(await getState(), ctx);
    const n = (((cleaned || {}).settings || {}).pages || []).length;
    console.log(n === 0 ? "cleanup: dial reset to empty" : "cleanup WARNING: dial still has " + n + " pages");
    if (n !== 0) failures++;
  }

  return failures > 0 ? 1 : 0;
}

main()
  .then((code) => process.exit(code))
  .catch((e) => {
    console.error("dial bulk live e2e error: " + (e && e.stack ? e.stack : e));
    process.exit(1);
  });
