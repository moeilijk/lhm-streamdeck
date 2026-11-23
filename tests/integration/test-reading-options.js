'use strict';
// Reading tile options that came in with #120.
// Covers: reset of the selected sensor's MIN/MAX on key press (PR #113).
//
// Key indices 14–15 (not used by the other suites).

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
  return { levels: levels.length, columns: tops.length, width: info.width };
}

async function run() {
  console.log('── reading option tests ──');

  await mockReset();
  const { wsPort, piPort } = await waitForDeckBridge(20000);
  console.log(`DeckBridge: wsPort=${wsPort} piPort=${piPort}`);

  for (const k of [KEY_SETTINGS, KEY_TILE]) await deleteSlot(piPort, k);
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

  // ── Cleanup ─────────────────────────────────────────────────────────────────
  await mockReset();
  for (const k of [KEY_SETTINGS, KEY_TILE]) await deleteSlot(piPort, k);

  summary();
}

run().catch(e => { console.error(e); process.exit(1); });
