// Settings Property Inspector for LHM Settings tile
var websocket = null,
  uuid = null,
  action = "com.moeilijk.lhm.settings",
  actionInfo = {},
  inInfo = {},
  context = null,
  appearanceSignature = null,
  appearancePollTimer = null,
  uiBound = false;
var allowedIntervals = [250, 500, 1000, 2000, 5000, 10000];

function parseJSONOrEmpty(raw) {
  if (!raw || typeof raw !== "string") {
    return {};
  }
  try {
    return JSON.parse(raw);
  } catch (_err) {
    return {};
  }
}

function byId(id) {
  return document.getElementById(id);
}

function normalizeInterval(value) {
  var v = parseInt(value, 10);
  if (isNaN(v)) {
    return 1000;
  }
  for (var i = 0; i < allowedIntervals.length; i++) {
    if (allowedIntervals[i] === v) {
      return v;
    }
  }
  // Map unknown values to the nearest supported option.
  var nearest = allowedIntervals[0];
  var nearestDiff = Math.abs(v - nearest);
  for (var j = 1; j < allowedIntervals.length; j++) {
    var diff = Math.abs(v - allowedIntervals[j]);
    if (diff < nearestDiff) {
      nearest = allowedIntervals[j];
      nearestDiff = diff;
    }
  }
  return nearest;
}

function sdkContext() {
  // Property Inspector must use its own registration UUID as context.
  // Using the action context causes Stream Deck to reject messages as "wrong context".
  return uuid || context;
}

function normalizeHex(value, fallback) {
  if (typeof value !== "string") {
    return fallback;
  }
  var v = value.trim().toLowerCase();
  if (!/^#[0-9a-f]{6}$/.test(v)) {
    return fallback;
  }
  return v;
}

function readTileSettingsFromUI() {
  var bg = byId("tileBackground");
  var text = byId("tileTextColor");
  var label = byId("showLabel");
  if (!bg || !text || !label) {
    return {
      tileBackground: "#000000",
      tileTextColor: "#ffffff",
      showLabel: true
    };
  }
  return {
    tileBackground: normalizeHex(bg.value, "#000000"),
    tileTextColor: normalizeHex(text.value, "#ffffff"),
    showLabel: label.checked === true
  };
}

function tileSettingsSignature(settings) {
  return [
    settings.tileBackground,
    settings.tileTextColor,
    settings.showLabel ? "1" : "0"
  ].join("|");
}

function applyTileSettingsToUI(settings) {
  var bgEl = byId("tileBackground");
  var textEl = byId("tileTextColor");
  var showEl = byId("showLabel");
  if (!bgEl || !textEl || !showEl) {
    return;
  }

  var bg = normalizeHex(settings.tileBackground, "#000000");
  var text = normalizeHex(settings.tileTextColor, "#ffffff");
  var show = settings.showLabel !== undefined ? settings.showLabel === true : true;

  if (bgEl.value !== bg) {
    bgEl.value = bg;
  }
  if (textEl.value !== text) {
    textEl.value = text;
  }
  if (showEl.checked !== show) {
    showEl.checked = show;
  }

  appearanceSignature = tileSettingsSignature(readTileSettingsFromUI());
}

function sendJson(payload) {
  if (!websocket || websocket.readyState !== 1) {
    return false;
  }
  websocket.send(JSON.stringify(payload));
  return true;
}

function connectElgatoStreamDeckSocket(inPort, inUUID, inRegisterEvent, inInfo, inActionInfo) {
  uuid = inUUID;
  actionInfo = parseJSONOrEmpty(inActionInfo);
  if (actionInfo["action"]) {
    action = actionInfo["action"];
  }
  context = actionInfo["context"];
  if (!context && actionInfo["payload"]) {
    context = actionInfo["payload"]["context"];
  }

  // Fallback to query string values if PI host uses URL params.
  try {
    var qs = new URLSearchParams(window.location.search);
    if (!context) {
      context = qs.get("context");
    }
    if (!action) {
      action = qs.get("action") || "com.moeilijk.lhm.settings";
    }
  } catch (_err) {
    // Ignore URL parsing errors in constrained PI runtimes.
  }
  if (!context) {
    context = uuid;
  }
  inInfo = parseJSONOrEmpty(inInfo);
  bindUIHandlers();
  websocket = new WebSocket("ws://localhost:" + inPort);

  websocket.onopen = function () {
    // Register with Stream Deck
    sendJson({
      event: inRegisterEvent,
      uuid: inUUID,
    });

    // Request global settings
    sendJson({
      event: "getGlobalSettings",
      context: inUUID,
    });

    // Request action settings (for tile appearance).
    var ctx = sdkContext();
    if (ctx) {
      sendJson({
        event: "getSettings",
        context: ctx,
      });
    }

    // Notify plugin that settings PI is connected
    if (ctx) {
      sendJson({
        action: action,
        event: "sendToPlugin",
        context: ctx,
        payload: {
          settingsConnected: true
        },
      });
    }

    // Keep saving resilient even if some WebView color events are missed.
    if (appearancePollTimer !== null) {
      window.clearInterval(appearancePollTimer);
    }
    appearancePollTimer = window.setInterval(function() {
      var current = readTileSettingsFromUI();
      if (tileSettingsSignature(current) !== appearanceSignature) {
        saveTileSettings("poll");
      }
    }, 300);
  };

  websocket.onmessage = function (evt) {
    var jsonObj = parseJSONOrEmpty(evt.data);
    var event = jsonObj["event"];

    // Handle global settings received (poll interval)
    if (event === "didReceiveGlobalSettings") {
      var settings = {};
      if (jsonObj.payload && jsonObj.payload.settings) {
        settings = jsonObj.payload.settings;
      }
      var interval = normalizeInterval(settings.pollInterval || 1000);
      var pollEl = byId("pollInterval");
      var rateEl = byId("currentRate");
      if (pollEl) {
        pollEl.value = interval;
      }
      if (rateEl) {
        rateEl.textContent = interval + "ms";
      }
    }

    // Handle action settings received (tile appearance)
    if (event === "didReceiveSettings") {
      var settings = {};
      if (jsonObj.payload && jsonObj.payload.settings) {
        settings = jsonObj.payload.settings;
      }
      applyTileSettingsToUI(settings);
    }

    // Handle status updates from plugin
    if (event === "sendToPropertyInspector") {
      var payload = jsonObj.payload || {};
      if (payload.connectionStatus !== undefined) {
        var statusEl = byId("connectionStatus");
        if (statusEl) {
          statusEl.textContent = payload.connectionStatus;
          statusEl.style.color = payload.connectionStatus === "Connected" ? "#4a4" : "#a44";
        }
      }
      if (payload.currentRate !== undefined) {
        var currentRateEl = byId("currentRate");
        if (currentRateEl) {
          currentRateEl.textContent = payload.currentRate + "ms";
        }
      }
    }
  };

  websocket.onclose = function () {
    if (appearancePollTimer !== null) {
      window.clearInterval(appearancePollTimer);
      appearancePollTimer = null;
    }
  };
}

// Save tile appearance settings
function saveTileSettings(reason) {
  if (!websocket || websocket.readyState !== 1) {
    return;
  }
  var ctx = sdkContext();
  if (!ctx) {
    return;
  }

  var settings = readTileSettingsFromUI();
  var sig = tileSettingsSignature(settings);
  if (reason !== "force" && sig === appearanceSignature) {
    return;
  }
  appearanceSignature = sig;

  // Persist action settings.
  sendJson({
    event: "setSettings",
    context: ctx,
    payload: settings
  });

  // Trigger immediate redraw path.
  sendJson({
    action: action,
    event: "sendToPlugin",
    context: ctx,
    payload: {
      updateTileAppearance: settings
    }
  });
}

function scheduleTileSettingsSave() {
  window.setTimeout(function() {
    saveTileSettings("ui");
  }, 0);
}

// Expose handlers for inline PI control attributes.
window.saveTileSettings = saveTileSettings;
window.scheduleTileSettingsSave = scheduleTileSettingsSave;

function bindUIHandlers() {
  if (uiBound) {
    return;
  }

  var pollEl = byId("pollInterval");
  var bgEl = byId("tileBackground");
  var textEl = byId("tileTextColor");
  var showEl = byId("showLabel");
  if (!pollEl || !bgEl || !textEl || !showEl) {
    return;
  }

  pollEl.addEventListener("change", function(e) {
    if (!websocket || websocket.readyState !== 1) {
      return;
    }
    var interval = normalizeInterval(e.target.value);
    e.target.value = interval;

    sendJson({
      event: "setGlobalSettings",
      context: uuid,
      payload: {
        pollInterval: interval
      }
    });

    var rateEl = byId("currentRate");
    if (rateEl) {
      rateEl.textContent = interval + "ms";
    }

    sendJson({
      action: action,
      event: "sendToPlugin",
      context: sdkContext(),
      payload: {
        setPollInterval: interval
      }
    });
  });

  bgEl.addEventListener("change", scheduleTileSettingsSave);
  bgEl.addEventListener("input", scheduleTileSettingsSave);
  textEl.addEventListener("change", scheduleTileSettingsSave);
  textEl.addEventListener("input", scheduleTileSettingsSave);
  showEl.addEventListener("change", scheduleTileSettingsSave);
  showEl.addEventListener("input", scheduleTileSettingsSave);
  showEl.addEventListener("click", scheduleTileSettingsSave);

  appearanceSignature = tileSettingsSignature(readTileSettingsFromUI());
  uiBound = true;
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bindUIHandlers);
} else {
  bindUIHandlers();
}
