'use strict';
// Source profiles tests: add, update host/port, set default, remove.
//
// Key index 70.

const {
  pass, fail, summary, sleep,
  waitForDeckBridge, connectPI, waitForMessage, sendToPlugin,
  createSlot, deleteSlot,
  mockReset, readGlobalSettings, ensureMockSourceProfile,
} = require('./helpers');

const SETTINGS_ACTION = 'com.moeilijk.lhm.settings';
const KEY_SETTINGS = 31;

// Wait for a sendToPropertyInspector message containing sourceProfiles
function waitForStatus(ws, timeoutMs = 8000) {
  return waitForMessage(ws, msg => {
    if (msg.event !== 'sendToPropertyInspector') return undefined;
    const pl = msg.payload || {};
    if (Array.isArray(pl.sourceProfiles)) return pl;
  }, timeoutMs);
}

async function run() {
  console.log('── source profiles tests ──');

  await mockReset();
  const { wsPort, piPort } = await waitForDeckBridge(20000);
  console.log(`DeckBridge: wsPort=${wsPort} piPort=${piPort}`);

  await deleteSlot(piPort, KEY_SETTINGS);
  await sleep(300);

  const settingsCtx = await createSlot(piPort, KEY_SETTINGS, SETTINGS_ACTION);
  await sleep(800);

  await ensureMockSourceProfile(wsPort, settingsCtx);
  await sleep(1500);

  const { ws, payload: initPayload } = await connectPI(wsPort, settingsCtx, SETTINGS_ACTION);
  const initialProfiles = initPayload.sourceProfiles || [];
  const initialDefault  = initPayload.defaultSourceProfileId || '';

  console.log(`initial profiles: ${initialProfiles.length}, default: ${initialDefault}`);

  // ── TEST 1: Add source profile ───────────────────────────────────────────────
  console.log('\n[test 1] add source profile');

  const addResultP = waitForStatus(ws);
  sendToPlugin(ws, settingsCtx, SETTINGS_ACTION, { addSourceProfile: true });
  const addResult = await addResultP;

  if (addResult.sourceProfiles.length === initialProfiles.length + 1) {
    pass('test 1a — profile count increased by 1');
  } else {
    fail(`test 1a — expected ${initialProfiles.length + 1}, got ${addResult.sourceProfiles.length}`);
  }

  const newProfile = addResult.sourceProfiles.find(p => !initialProfiles.find(e => e.id === p.id));
  if (!newProfile) { ws.close(); throw new Error('new profile not found in response'); }

  if (newProfile.name === 'New Source') {
    pass(`test 1b — new profile default name: ${newProfile.name}`);
  } else {
    fail(`test 1b — expected 'New Source', got: ${newProfile.name}`);
  }

  // ── TEST 2: Update profile host/port ─────────────────────────────────────────
  console.log('\n[test 2] update profile host/port');

  const updateResultP = waitForStatus(ws);
  sendToPlugin(ws, settingsCtx, SETTINGS_ACTION, {
    setSourceProfile: { id: newProfile.id, name: 'Test Profile', host: '192.168.1.99', port: 8090 }
  });
  const updateResult = await updateResultP;

  const updated = updateResult.sourceProfiles.find(p => p.id === newProfile.id);
  if (updated && updated.name === 'Test Profile' && updated.host === '192.168.1.99' && updated.port === 8090) {
    pass('test 2 — name, host, port updated correctly');
  } else {
    fail(`test 2 — expected {name:Test Profile, host:192.168.1.99, port:8090}, got: ${JSON.stringify(updated)}`);
  }

  // ── TEST 3: Set profile as default ───────────────────────────────────────────
  console.log('\n[test 3] set profile as default');

  const setDefaultP = waitForStatus(ws);
  sendToPlugin(ws, settingsCtx, SETTINGS_ACTION, { setDefaultSourceProfile: newProfile.id });
  const setDefaultResult = await setDefaultP;

  if (setDefaultResult.defaultSourceProfileId === newProfile.id) {
    pass('test 3a — defaultSourceProfileId updated');
  } else {
    fail(`test 3a — expected ${newProfile.id}, got: ${setDefaultResult.defaultSourceProfileId}`);
  }

  // Verify persisted in global settings file
  await sleep(300);
  const gs = readGlobalSettings();
  if (gs.defaultSourceProfileId === newProfile.id) {
    pass('test 3b — default persisted to global settings file');
  } else {
    fail(`test 3b — global settings defaultSourceProfileId: ${gs.defaultSourceProfileId}`);
  }

  // Restore original default
  const restoreP = waitForStatus(ws);
  sendToPlugin(ws, settingsCtx, SETTINGS_ACTION, { setDefaultSourceProfile: initialDefault });
  await restoreP;
  await sleep(300);

  // ── TEST 4: Delete profile ────────────────────────────────────────────────────
  console.log('\n[test 4] delete profile');

  const deleteResultP = waitForStatus(ws);
  sendToPlugin(ws, settingsCtx, SETTINGS_ACTION, { deleteSourceProfile: newProfile.id });
  const deleteResult = await deleteResultP;

  const stillThere = deleteResult.sourceProfiles.find(p => p.id === newProfile.id);
  if (!stillThere) {
    pass('test 4a — profile removed from list');
  } else {
    fail(`test 4a — profile still present after delete`);
  }

  if (deleteResult.sourceProfiles.length === initialProfiles.length) {
    pass('test 4b — profile count back to initial');
  } else {
    fail(`test 4b — expected ${initialProfiles.length}, got ${deleteResult.sourceProfiles.length}`);
  }

  ws.close();

  // ── Cleanup ─────────────────────────────────────────────────────────────────
  await mockReset();
  await deleteSlot(piPort, KEY_SETTINGS);

  summary();
}

run().catch(e => { console.error(e); process.exit(1); });
