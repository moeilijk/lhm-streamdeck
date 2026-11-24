'use strict';
// Source-gone tests (issue #101): a Composite or Derived tile shows the same
// placeholder as a Reading tile when its source has no live data, instead of
// keeping its last values, and returns to live rendering when the data is
// back. An open Property Inspector is told when the source is gone.
//
// Taken over from hwinfo-streamdeck (tests/integration/test-source-gone.js),
// without the parts about remote sources inside one bridge.
//
// Key indices 4-7.

const http = require('http');
const {
  pass, fail, summary, sleep,
  waitForDeckBridge, connectPI, waitForMessage, sdpi,
  createSlot, deleteSlot,
  mockSet, mockReset, ensureMockSourceProfile,
  MOCK_PORT,
} = require('./helpers');

const SETTINGS_ACTION  = 'com.moeilijk.lhm.settings';
const READING_ACTION   = 'com.moeilijk.lhm.reading';
const COMPOSITE_ACTION = 'com.moeilijk.lhm.composite';
const DERIVED_ACTION   = 'com.moeilijk.lhm.derived';
const KEY_SETTINGS  = 4;
const KEY_READING   = 5;
const KEY_COMPOSITE = 6;
const KEY_DERIVED   = 7;
const STATE_BASE = process.env.DECKBRIDGE_URL || 'http://127.0.0.1:34075';

function mockSource(available) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({ available });
    const req = http.request({ host: '127.0.0.1', port: MOCK_PORT, path: '/source', method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(data) }
    }, res => {
      res.resume();
      res.on('end', () => res.statusCode === 200 ? resolve() : reject(new Error('mock /source ' + res.statusCode)));
    });
    req.on('error', reject);
    req.write(data);
    req.end();
  });
}

async function tileImage(context) {
  const res = await fetch(`${STATE_BASE}/api/state`);
  if (!res.ok) throw new Error('state ' + res.status);
  const state = await res.json();
  const slot = (state.slots || []).find(s => s.context === context);
  return slot ? (slot.imageDataUrl || '') : '';
}

// Poll until the tile image equals (want=true) or differs from (want=false)
// `ref`; the poll interval may have been raised by earlier tests.
async function waitForImage(context, ref, want, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let img = '';
  while (Date.now() < deadline) {
    await sleep(500);
    img = await tileImage(context);
    if (img && (img === ref) === want) return img;
  }
  return img;
}

async function configureSlot(wsPort, ctx, action, selectPrefix, slotIdx, sensorUID, readingLabel) {
  const { ws } = await connectPI(wsPort, ctx, action);
  const readingsP = waitForMessage(ws, msg => {
    if (msg.event !== 'sendToPropertyInspector') return undefined;
    const pl = msg.payload || {};
    if (!pl.readings || !Array.isArray(pl.readings)) return undefined;
    if (slotIdx === null || pl.slotIndex === slotIdx) return pl.readings;
  });
  sdpi(ws, ctx, action, `${selectPrefix}sensorSelect`, sensorUID);
  const readings = await readingsP;
  const r = readings.find(x => x.label === readingLabel);
  if (!r) { ws.close(); throw new Error(`reading "${readingLabel}" not found on ${sensorUID}`); }
  sdpi(ws, ctx, action, `${selectPrefix}readingSelect`, String(r.id));
  await sleep(800);
  ws.close();
}

// Wait for an evStatus payload on an open Property Inspector connection.
function waitForStatus(ws, wantError, timeoutMs = 20000) {
  return waitForMessage(ws, msg => {
    if (msg.event !== 'sendToPropertyInspector') return undefined;
    const pl = msg.payload || {};
    if (wantError && pl.error === true) return pl;
    if (!wantError && pl.message === 'show_ui') return pl;
  }, timeoutMs).catch(() => null);
}

async function run() {
  console.log('── source-gone tests ──');

  await mockReset();
  const { wsPort, piPort } = await waitForDeckBridge(20000);
  console.log(`DeckBridge: wsPort=${wsPort} piPort=${piPort}`);

  const ALL_KEYS = [KEY_SETTINGS, KEY_READING, KEY_COMPOSITE, KEY_DERIVED];
  for (const k of ALL_KEYS) await deleteSlot(piPort, k);
  await sleep(300);
  const ctxSettings  = await createSlot(piPort, KEY_SETTINGS,  SETTINGS_ACTION);
  const ctxReading   = await createSlot(piPort, KEY_READING,   READING_ACTION);
  const ctxComposite = await createSlot(piPort, KEY_COMPOSITE, COMPOSITE_ACTION);
  const ctxDerived   = await createSlot(piPort, KEY_DERIVED,   DERIVED_ACTION);
  await sleep(1500);
  await ensureMockSourceProfile(wsPort, ctxSettings);
  await sleep(1500);

  await configureSlot(wsPort, ctxReading, READING_ACTION, '', null, '/mockcpu/0', 'CPU Package');
  await configureSlot(wsPort, ctxComposite, COMPOSITE_ACTION, 'slot0_', 0, '/mockcpu/0', 'CPU Package');
  {
    const { ws } = await connectPI(wsPort, ctxDerived, DERIVED_ACTION);
    sdpi(ws, ctxDerived, DERIVED_ACTION, 'derived_slotCount', '2');
    await sleep(300);
    sdpi(ws, ctxDerived, DERIVED_ACTION, 'derived_formula', 'average');
    await sleep(600);
    ws.close();
  }
  await configureSlot(wsPort, ctxDerived, DERIVED_ACTION, 'slot0_', 0, '/mockcpu/0', 'CPU Package');
  await configureSlot(wsPort, ctxDerived, DERIVED_ACTION, 'slot1_', 1, '/mockcpu/0', 'CPU Total');
  await sleep(2500);

  // ── TEST 1: all three tiles render live ──
  console.log('\n[test 1] tiles render while the source polls');
  const live = {
    reading:   await tileImage(ctxReading),
    composite: await tileImage(ctxComposite),
    derived:   await tileImage(ctxDerived),
  };
  for (const [name, img] of Object.entries(live)) {
    if (img.length > 0) pass(`test 1 — ${name} tile rendered an image`);
    else fail(`test 1 — ${name} tile has no image`);
  }
  if (live.composite !== live.reading && live.derived !== live.reading) pass('test 1 — the three tiles show different images');
  else fail('test 1 — composite or derived shows the same image as the reading tile while live');

  // ── TEST 2: source gone → all three show the same placeholder ──
  console.log('\n[test 2] source stops answering');
  const { ws: piWs } = await connectPI(wsPort, ctxComposite, COMPOSITE_ACTION);
  const errorP = waitForStatus(piWs, true);
  await mockSource(false);
  const placeholder = await waitForImage(ctxReading, live.reading, false);
  if (placeholder && placeholder !== live.reading) pass('test 2a — reading tile leaves live rendering');
  else fail('test 2a — reading tile image unchanged after the source went away');
  const compositeDown = await waitForImage(ctxComposite, placeholder, true);
  if (compositeDown === placeholder) pass('test 2b — composite tile shows the reading tile placeholder');
  else fail('test 2b — composite tile does not show the placeholder');
  const derivedDown = await waitForImage(ctxDerived, placeholder, true);
  if (derivedDown === placeholder) pass('test 2c — derived tile shows the reading tile placeholder');
  else fail('test 2c — derived tile does not show the placeholder');
  const errorStatus = await errorP;
  if (errorStatus && /unavailable/i.test(errorStatus.message || '')) pass(`test 2d — the open composite inspector is told: "${errorStatus.message}"`);
  else fail('test 2d — the open composite inspector got no error status');

  // ── TEST 3: source back → composite and derived render live again ──
  console.log('\n[test 3] source answers again');
  const showP = waitForStatus(piWs, false);
  await mockSource(true);
  await mockSet('/mockcpu/0/temperature/0', 66);
  const compositeUp = await waitForImage(ctxComposite, placeholder, false);
  if (compositeUp && compositeUp !== placeholder) pass('test 3a — composite tile resumes rendering');
  else fail('test 3a — composite tile stayed on the placeholder');
  const derivedUp = await waitForImage(ctxDerived, placeholder, false);
  if (derivedUp && derivedUp !== placeholder) pass('test 3b — derived tile resumes rendering');
  else fail('test 3b — derived tile stayed on the placeholder');
  if (await showP) pass('test 3c — the open composite inspector gets its UI back');
  else fail('test 3c — the open composite inspector got no show_ui');
  piWs.close();

  // ── TEST 4: a second outage shows the placeholder again ──
  console.log('\n[test 4] source stops answering a second time');
  await mockSource(false);
  const compositeDown2 = await waitForImage(ctxComposite, placeholder, true);
  if (compositeDown2 === placeholder) pass('test 4a — composite tile shows the placeholder again');
  else fail('test 4a — composite tile does not show the placeholder on the second outage');
  const derivedDown2 = await waitForImage(ctxDerived, placeholder, true);
  if (derivedDown2 === placeholder) pass('test 4b — derived tile shows the placeholder again');
  else fail('test 4b — derived tile does not show the placeholder on the second outage');

  // ── Cleanup ──
  await mockSource(true);
  await mockReset();
  for (const k of ALL_KEYS) await deleteSlot(piPort, k);

  summary();
}

run().catch(async e => { console.error(e); try { await mockSource(true); } catch (_) {} process.exit(1); });
