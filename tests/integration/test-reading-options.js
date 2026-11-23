'use strict';
// Reading tile options that came in with #120.
// Covers: reset of the selected sensor's MIN/MAX on key press (PR #113) and the
// Range modes Manual / Auto / Sensor Min / Max (PR #112).
//
// Key indices 14–16 (not used by the other suites).

const http = require('http');
const path = require('path');
const {
  pass, fail, summary, sleep,
  waitForDeckBridge, connectPI, waitForMessage, sdpi,
  httpPost, createSlot, deleteSlot,
  mockSet, mockReset,
  getTileSettings, ensureMockSourceProfile,
  MOCK_PORT,
} = require('./helpers');

const sharp = require(path.resolve(__dirname, '../../../DeckBridge/node_modules/sharp'));

const SETTINGS_ACTION = 'com.moeilijk.lhm.settings';
const READING_ACTION  = 'com.moeilijk.lhm.reading';
const DEVICE          = 'deckbridge-xl-0';
const POLL_MS         = 1000;

const KEY_SETTINGS = 14;
const KEY_TILE     = 15;
const KEY_RANGE    = 16;  // tile for the Range modes, reads "CPU Total"

const RESET_PATH = '/mockcpu/0/temperature/0'; // "CPU Package", the tile's reading
const OTHER_PATH = '/mockcpu/0/load/0';        // "CPU Total", must stay untouched

// Min/Value/Max of every mock reading, straight from the source the plugin polls.
function mockReadings() {
  return new Promise((resolve, reject) => {
    http.get({ host: '127.0.0.1', port: MOCK_PORT, path: '/data.json' }, res => {
      let out = '';
      res.on('data', c => out += c);
      res.on('end', () => {
        const found = {};
        const walk = n => {
          if (n.SensorId) found[n.SensorId] = { min: parseFloat(n.Min), value: parseFloat(n.Value), max: parseFloat(n.Max) };
          (n.Children || []).forEach(walk);
        };
        walk(JSON.parse(out));
        resolve(found);
      });
    }).on('error', reject);
  });
}

async function configureTile(wsPort, ctx, sensorUID, readingLabel) {
  const { ws, payload } = await connectPI(wsPort, ctx, READING_ACTION);
  const sensor = (payload.sensors || []).find(s => s.uid === sensorUID);
  if (!sensor) { ws.close(); throw new Error(`sensor ${sensorUID} not found`); }
  const readingsP = waitForMessage(ws, msg => {
    if (msg.event !== 'sendToPropertyInspector') return undefined;
    const pl = msg.payload || {};
    if (pl.readings && Array.isArray(pl.readings)) return pl.readings;
  });
  sdpi(ws, ctx, READING_ACTION, 'sensorSelect', sensorUID);
  const r = (await readingsP).find(x => x.label === readingLabel);
  if (!r) { ws.close(); throw new Error(`reading "${readingLabel}" not found`); }
  sdpi(ws, ctx, READING_ACTION, 'readingSelect', String(r.id));
  await sleep(800);
  ws.close();
}

async function setOption(wsPort, ctx, key, checked) {
  const { ws } = await connectPI(wsPort, ctx, READING_ACTION);
  sdpi(ws, ctx, READING_ACTION, key, '', { checked });
  await sleep(600);
  ws.close();
}

function pressKey(piPort, keyIndex) {
  return httpPost(piPort, '/api/keys/press', { deviceId: DEVICE, keyIndex });
}

// Distinct heights of the graph in the rendered key image. The default graph is
// green on black, so the top-most green pixel of a column is the level drawn
// there. A graph with history shows several levels. A graph that was cleared is
// drawn flat at the current value, so it shows exactly one.
async function graphLevels(piPort, keyIndex) {
  const state = await new Promise((resolve, reject) => {
    http.get({ host: '127.0.0.1', port: piPort, path: '/api/state' }, res => {
      let out = '';
      res.on('data', c => out += c);
      res.on('end', () => { try { resolve(JSON.parse(out)); } catch (e) { reject(e); } });
    }).on('error', reject);
  });
  const slot = (state.slots || []).find(x => x.deviceId === DEVICE && x.keyIndex === keyIndex);
  if (!slot || !slot.imageDataUrl) throw new Error('no key image for index ' + keyIndex);
  const buf = Buffer.from(slot.imageDataUrl.split(',')[1], 'base64');
  const { data, info } = await sharp(buf).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  const green = (x, y) => {
    const o = (y * info.width + x) * 4;
    return data[o + 1] > data[o] + 30 && data[o + 1] > data[o + 2] + 30;
  };
  const tops = [];
  for (let x = 0; x < info.width; x++) {
    for (let y = 0; y < info.height; y++) {
      if (green(x, y)) { tops.push(y); break; }
    }
  }
  // Scaling the key image blurs an edge over a few pixels: heights that close
  // together are one level.
  const levels = [];
  tops.slice().sort((p, q) => p - q).forEach(y => {
    if (levels.length === 0 || y - levels[levels.length - 1] > 4) levels.push(y);
  });
  // span: distance between the highest and the lowest level, as a fraction of
  // the key height. last: height of the newest sample (right edge), 0 = bottom
  // of the key and 1 = top.
  const span = tops.length ? (Math.max(...tops) - Math.min(...tops)) / info.height : 0;
  const last = tops.length ? 1 - tops[tops.length - 1] / info.height : 0;
  return { levels: levels.length, columns: tops.length, width: info.width, span, last };
}

async function setValue(wsPort, ctx, key, value) {
  const { ws } = await connectPI(wsPort, ctx, READING_ACTION);
  sdpi(ws, ctx, READING_ACTION, key, value);
  await sleep(600);
  ws.close();
}

// Alternate the reading between two values so the graph shows both.
async function alternate(path, a, b, rounds) {
  for (let i = 0; i < rounds; i++) {
    await mockSet(path, a);
    await sleep(POLL_MS * 2);
    await mockSet(path, b);
    await sleep(POLL_MS * 2);
  }
}

async function run() {
  console.log('── reading option tests ──');

  await mockReset();
  const { wsPort, piPort } = await waitForDeckBridge(20000);
  console.log(`DeckBridge: wsPort=${wsPort} piPort=${piPort}`);

  for (const k of [KEY_SETTINGS, KEY_TILE, KEY_RANGE]) await deleteSlot(piPort, k);
  await sleep(500);

  const settingsCtx = await createSlot(piPort, KEY_SETTINGS, SETTINGS_ACTION);
  const ctx         = await createSlot(piPort, KEY_TILE, READING_ACTION);
  await sleep(1000);

  await ensureMockSourceProfile(wsPort, settingsCtx);
  await sleep(1500);
  await configureTile(wsPort, ctx, '/mockcpu/0', 'CPU Package');

  // Give the source a recorded range and the tile a visible history.
  await mockSet(RESET_PATH, 30);
  await sleep(POLL_MS * 2);
  await mockSet(RESET_PATH, 90);
  await sleep(POLL_MS * 2);
  await mockSet(RESET_PATH, 60);
  await sleep(POLL_MS * 8);
  console.log('setup complete — tile configured, history recorded');

  // ── TEST 1: option is off by default, a key press changes nothing ───────────
  console.log('\n[test 1] default: a key press does not reset MIN/MAX');
  let s = await getTileSettings(wsPort, ctx, READING_ACTION);
  if (s.resetMinMaxOnPress) fail('test 1 — resetMinMaxOnPress is on for a new tile');
  const before = await mockReadings();
  const histBefore = await graphLevels(piPort, KEY_TILE);
  await pressKey(piPort, KEY_TILE);
  await sleep(POLL_MS * 3);
  let now = await mockReadings();
  if (now[RESET_PATH].min === before[RESET_PATH].min && now[RESET_PATH].max === before[RESET_PATH].max) {
    pass(`test 1 — source keeps its range ${before[RESET_PATH].min}..${before[RESET_PATH].max} with the option off`);
  } else {
    fail(`test 1 — source range changed with the option off: ${JSON.stringify(now[RESET_PATH])}`);
  }
  const histStill = await graphLevels(piPort, KEY_TILE);
  if (histBefore.levels >= 2 && histStill.levels >= 2) {
    pass(`test 1 — the key keeps its graph history (${histStill.levels} levels) with the option off`);
  } else {
    fail(`test 1 — expected a graph with history before and after the press, got ${histBefore.levels} and ${histStill.levels} levels`);
  }

  // ── TEST 2: option persists ─────────────────────────────────────────────────
  console.log('\n[test 2] the option is stored on the tile');
  await setOption(wsPort, ctx, 'resetMinMaxOnPress', true);
  s = await getTileSettings(wsPort, ctx, READING_ACTION);
  if (s.resetMinMaxOnPress === true) pass('test 2 — resetMinMaxOnPress=true persisted');
  else fail(`test 2 — resetMinMaxOnPress not persisted: ${JSON.stringify(s.resetMinMaxOnPress)}`);

  // ── TEST 3: key press resets the selected sensor only ───────────────────────
  console.log('\n[test 3] key press resets MIN/MAX of the selected sensor');
  const histFull = await graphLevels(piPort, KEY_TILE);
  await pressKey(piPort, KEY_TILE);
  await sleep(POLL_MS * 2 + 500);
  now = await mockReadings();
  const r = now[RESET_PATH];
  if (r.min === r.value && r.max === r.value) pass(`test 3 — source MIN/MAX collapsed to the current value (${r.value})`);
  else fail(`test 3 — source MIN/MAX not reset: ${JSON.stringify(r)}`);
  const o = now[OTHER_PATH];
  if (o.min === before[OTHER_PATH].min && o.max === before[OTHER_PATH].max) pass('test 3 — another sensor keeps its MIN/MAX');
  else fail(`test 3 — another sensor changed: ${JSON.stringify(o)}`);

  // ── TEST 4: the rendered key lost its history ───────────────────────────────
  console.log('\n[test 4] the key image starts a fresh graph');
  const histAfter = await graphLevels(piPort, KEY_TILE);
  if (histFull.levels >= 2 && histAfter.levels === 1 && histAfter.columns === histAfter.width) {
    pass(`test 4 — graph history cleared on the key (${histFull.levels} levels before, flat after)`);
  } else {
    fail(`test 4 — graph history not cleared: ${histFull.levels} levels before, ${histAfter.levels} after`);
  }
  await mockSet(RESET_PATH, 85);
  await sleep(POLL_MS * 4);
  const histLater = await graphLevels(piPort, KEY_TILE);
  if (histLater.levels >= 2) pass(`test 4 — the graph records again after the reset (${histLater.levels} levels)`);
  else fail(`test 4 — the graph stopped after the reset (${histLater.levels} levels)`);

  // ── Range modes (PR #112) ───────────────────────────────────────────────────
  // "CPU Total" starts at the mock's default of 20 % and then moves between 50
  // and 54 %. Each mode has to draw the newest sample at the height that follows
  // from its range.
  await mockReset();
  const rangeCtx = await createSlot(piPort, KEY_RANGE, READING_ACTION);
  await sleep(1000);
  await configureTile(wsPort, rangeCtx, '/mockcpu/0', 'CPU Total');

  console.log('\n[test 5] Range defaults to Manual');
  s = await getTileSettings(wsPort, rangeCtx, READING_ACTION);
  if (!s.scaleMode) pass('test 5 — a new tile has no scaleMode (Manual)');
  else fail(`test 5 — a new tile has scaleMode=${JSON.stringify(s.scaleMode)}`);
  await alternate(OTHER_PATH, 50, 54, 3);
  const manual = await graphLevels(piPort, KEY_RANGE);
  if (manual.last > 0.4 && manual.last < 0.65) {
    pass(`test 5 — Manual 0..100 draws 54 % at ${(manual.last * 100).toFixed(0)} % of the key height`);
  } else {
    fail(`test 5 — Manual 0..100 draws 54 % at ${(manual.last * 100).toFixed(0)} % of the key height, expected about 54 %`);
  }

  console.log('\n[test 6] Auto fits the range to the recent readings');
  await setValue(wsPort, rangeCtx, 'scaleMode', 'auto');
  s = await getTileSettings(wsPort, rangeCtx, READING_ACTION);
  if (s.scaleMode === 'auto') pass('test 6 — scaleMode=auto persisted');
  else fail(`test 6 — scaleMode not persisted: ${JSON.stringify(s.scaleMode)}`);
  if (s.min === 0 && s.max === 100) pass('test 6 — the manual limits 0..100 are kept');
  else fail(`test 6 — manual limits changed: ${s.min}..${s.max}`);
  await alternate(OTHER_PATH, 50, 54, 2);
  const auto = await graphLevels(piPort, KEY_RANGE);
  // The history holds the mock's start value 20 and the 50/54 swing. Auto pads
  // 20..54 and rounds outward to 10..60, so 54 sits at 88 % of the height
  // instead of the 54 % it has on the manual scale.
  if (Math.abs(auto.last - 0.88) < 0.08) {
    pass(`test 6 — Auto draws 54 % at ${(auto.last * 100).toFixed(0)} % of the key height (range 10..60)`);
  } else {
    fail(`test 6 — Auto draws 54 % at ${(auto.last * 100).toFixed(0)} % of the key height, expected about 88 %`);
  }

  console.log('\n[test 7] Sensor Min / Max follows the range recorded by the source');
  await setValue(wsPort, rangeCtx, 'scaleMode', 'lhm');
  s = await getTileSettings(wsPort, rangeCtx, READING_ACTION);
  if (s.scaleMode === 'lhm') pass('test 7 — scaleMode=lhm persisted');
  else fail(`test 7 — scaleMode not persisted: ${JSON.stringify(s.scaleMode)}`);
  // Narrow the recorded range to the current value, then widen it to 40..60.
  await new Promise((resolve, reject) => {
    const req = http.request({ host: '127.0.0.1', port: MOCK_PORT, method: 'POST', path: '/Sensor?action=ResetMinMax&id=' + encodeURIComponent(OTHER_PATH) }, res => { res.resume(); res.on('end', resolve); });
    req.on('error', reject);
    req.end();
  });
  await mockSet(OTHER_PATH, 40);
  await sleep(POLL_MS * 2);
  await mockSet(OTHER_PATH, 60);
  await sleep(POLL_MS * 2);
  await mockSet(OTHER_PATH, 45);
  await sleep(POLL_MS * 3);
  const src = (await mockReadings())[OTHER_PATH];
  const want = (45 - Math.floor(src.min)) / (Math.ceil(src.max) - Math.floor(src.min));
  const lhm = await graphLevels(piPort, KEY_RANGE);
  if (src.min === 40 && src.max === 60) pass('test 7 — the source recorded 40..60');
  else fail(`test 7 — the source recorded ${src.min}..${src.max}, expected 40..60`);
  if (Math.abs(lhm.last - want) < 0.12) {
    pass(`test 7 — 45 % is drawn at ${(lhm.last * 100).toFixed(0)} % of the key height (range ${src.min}..${src.max})`);
  } else {
    fail(`test 7 — 45 % is drawn at ${(lhm.last * 100).toFixed(0)} % of the key height, expected about ${(want * 100).toFixed(0)} %`);
  }

  console.log('\n[test 8] back to Manual restores the manual limits');
  await setValue(wsPort, rangeCtx, 'scaleMode', 'manual');
  await sleep(POLL_MS * 2);
  const back = await graphLevels(piPort, KEY_RANGE);
  if (Math.abs(back.last - 0.45) < 0.12) pass(`test 8 — 45 % is drawn at ${(back.last * 100).toFixed(0)} % of the key height on 0..100`);
  else fail(`test 8 — 45 % is drawn at ${(back.last * 100).toFixed(0)} % of the key height, expected about 45 %`);

  // ── Cleanup ─────────────────────────────────────────────────────────────────
  await mockReset();
  for (const k of [KEY_SETTINGS, KEY_TILE, KEY_RANGE]) await deleteSlot(piPort, k);

  summary();
}

run().catch(e => { console.error(e); process.exit(1); });
