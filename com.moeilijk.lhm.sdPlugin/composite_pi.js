var websocket = null,
  uuid = null,
  actionInfo = {},
  allSensors = [],
  currentSettings = {},
  sourceProfiles = [];

var onchangeevt = "onchange";

function connectElgatoStreamDeckSocket(inPort, inUUID, inRegisterEvent, inInfo, inActionInfo) {
  uuid = inUUID;
  actionInfo = JSON.parse(inActionInfo);
  websocket = new WebSocket("ws://localhost:" + inPort);

  websocket.onopen = function () {
    websocket.send(JSON.stringify({ event: inRegisterEvent, uuid: inUUID }));
    sendValueToPlugin("propertyInspectorConnected", "property_inspector");
  };

  websocket.onmessage = function (evt) {
    var jsonObj = JSON.parse(evt.data);
    var event = jsonObj["event"];
    if (event !== "sendToPropertyInspector") return;
    var payload = jsonObj.payload || {};

    // Error state
    if (typeof payload.error === "boolean") {
      document.querySelector("#ui").style.display = payload.error ? "none" : "";
      document.querySelector("#error").style.display = payload.error ? "block" : "none";
      if (!payload.error && payload.message === "show_ui") {
        sendValueToPlugin("propertyInspectorConnected", "property_inspector");
      }
      return;
    }

    // Source profiles
    if (Array.isArray(payload.sourceProfiles)) {
      sourceProfiles = payload.sourceProfiles;
      rebuildSourceProfileDropdown(currentSettings.sourceProfileId || "");
    }

    // Sensor list
    if (Array.isArray(payload.sensors)) {
      allSensors = payload.sensors;
      for (var i = 0; i < 4; i++) {
        populateSensorSelect(i, allSensors);
      }
    }

    // Readings for a specific slot
    if (Array.isArray(payload.readings) && typeof payload.slotIndex === "number") {
      populateReadingSelect(payload.slotIndex, payload.readings);
    }

    // Full settings object
    if (payload.compositeSettings) {
      currentSettings = payload.compositeSettings;
      applySettingsToUI(currentSettings);
      rebuildSourceProfileDropdown(currentSettings.sourceProfileId || "");
    }
  };
}

function sendValueToPlugin(value, event) {
  if (!websocket || websocket.readyState !== 1) return;
  websocket.send(JSON.stringify({
    event: "sendToPlugin",
    context: uuid,
    action: actionInfo.action,
    payload: { [event]: value }
  }));
}

function sendSdpi(key, value) {
  sendValueToPlugin({ key: key, value: String(value) }, "sdpi_collection");
}

function rebuildSourceProfileDropdown(selectedId) {
  var sel = document.getElementById("sourceProfileSelect");
  if (!sel) return;
  sel.innerHTML = "";
  for (var i = 0; i < sourceProfiles.length; i++) {
    var opt = document.createElement("option");
    opt.value = sourceProfiles[i].id;
    opt.textContent = sourceProfiles[i].name || sourceProfiles[i].id;
    if (sourceProfiles[i].id === selectedId) opt.selected = true;
    sel.appendChild(opt);
  }
  if (!sel.dataset.bound) {
    sel.dataset.bound = "1";
    sel.addEventListener("change", function(e) {
      sendValueToPlugin(e.target.value, "sourceProfileId");
      sendValueToPlugin("propertyInspectorConnected", "property_inspector");
    });
  }
}

// --- sensor / reading dropdowns ---

function populateSensorSelect(slotIdx, sensors) {
  var el = document.getElementById("slot" + slotIdx + "_sensorSelect");
  if (!el) return;
  var currentUid = (currentSettings.slots && currentSettings.slots[slotIdx])
    ? currentSettings.slots[slotIdx].sensorUid : "";
  var sorted = sensors.slice().sort(function (a, b) {
    return a.name > b.name ? 1 : a.name < b.name ? -1 : 0;
  });

  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.text = "Choose a sensor";
  ph.disabled = true;
  if (!currentUid) ph.selected = true;
  el.add(ph);

  sorted.forEach(function (sensor) {
    var opt = document.createElement("option");
    opt.text = sensor.name;
    opt.value = sensor.uid;
    if (sensor.uid === currentUid) opt.selected = true;
    el.add(opt);
  });
  el.removeAttribute("disabled");
}

function populateReadingSelect(slotIdx, readings) {
  var el = document.getElementById("slot" + slotIdx + "_readingSelect");
  if (!el) return;
  var slot = currentSettings.slots ? currentSettings.slots[slotIdx] : null;
  var currentRid = slot ? String(slot.readingId) : "";

  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.text = "Choose a reading";
  ph.disabled = true;
  if (!slot || !slot.isValid) ph.selected = true;
  el.add(ph);

  readings.forEach(function (r) {
    var opt = document.createElement("option");
    opt.text = r.label + (r.unit ? " (" + r.unit + ")" : "");
    opt.value = String(r.id);
    if (String(r.id) === currentRid) opt.selected = true;
    el.add(opt);
  });
  el.removeAttribute("disabled");
}

// --- populate all UI fields from settings ---

function applySettingsToUI(s) {
  setSelectValue("composite_mode", s.mode || "both");
  setSelectValue("composite_slotCount", String(s.slotCount || 2));
  updateSlotVisibility(s.slotCount || 2);

  var slots = s.slots || [];
  for (var i = 0; i < 4; i++) {
    var slot = slots[i] || {};
    setInputValue("slot" + i + "_title", slot.title || "");
    setColorValue("slot" + i + "_highlightColor", slot.highlightColor);
    setColorValue("slot" + i + "_foregroundColor", slot.foregroundColor);
    setColorValue("slot" + i + "_valueTextColor", slot.valueTextColor);
    setColorValue("slot" + i + "_titleColor", slot.titleColor);
    setColorValue("slot" + i + "_backgroundColor", slot.backgroundColor);
    setInputValue("slot" + i + "_fillAlpha", slot.fillAlpha != null ? slot.fillAlpha : 55);
    if (slot.min || slot.max) {
      setInputValue("slot" + i + "_min", slot.min || "");
      setInputValue("slot" + i + "_max", slot.max || "");
    }
    setInputValue("slot" + i + "_titleFontSize", slot.titleFontSize || 9);
    setInputValue("slot" + i + "_valueFontSize", slot.valueFontSize || 10.5);
    setInputValue("slot" + i + "_format", slot.format || "");
    setInputValue("slot" + i + "_divisor", slot.divisor || "");
    setSelectValue("slot" + i + "_graphUnit", slot.graphUnit || "");

    // Re-populate sensor dropdown selection if sensors already loaded
    if (allSensors.length > 0 && slot.sensorUid) {
      var sensorSel = document.getElementById("slot" + i + "_sensorSelect");
      if (sensorSel) {
        for (var j = 0; j < sensorSel.options.length; j++) {
          if (sensorSel.options[j].value === slot.sensorUid) {
            sensorSel.selectedIndex = j;
            break;
          }
        }
      }
    }
  }
}

function setInputValue(id, val) {
  var el = document.getElementById(id);
  if (el && val != null) el.value = val;
}

function setColorValue(id, hex) {
  if (!hex) return;
  // Ensure 6-digit hex for color inputs
  if (hex.length === 4) {
    hex = "#" + hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3];
  }
  var el = document.getElementById(id);
  if (el) el.value = hex;
}

function setSelectValue(id, val) {
  var el = document.getElementById(id);
  if (!el || val == null) return;
  for (var i = 0; i < el.options.length; i++) {
    if (el.options[i].value === String(val)) {
      el.selectedIndex = i;
      return;
    }
  }
}

// --- slot visibility ---

function updateSlotVisibility(count) {
  for (var i = 0; i < 4; i++) {
    var sec = document.getElementById("slot-section-" + i);
    if (sec) sec.style.display = i < count ? "" : "none";
  }
}

// --- wire up all events after DOM ready ---

document.addEventListener("DOMContentLoaded", function () {
  // Tile-wide
  wireSelect("composite_mode");
  wireSelect("composite_slotCount", function (val) {
    updateSlotVisibility(parseInt(val, 10));
  });

  for (var i = 0; i < 4; i++) {
    wireSensorSelect(i);
    wireReadingSelect(i);
    wireText("slot" + i + "_title");
    wireColor("slot" + i + "_highlightColor");
    wireColor("slot" + i + "_foregroundColor");
    wireColor("slot" + i + "_valueTextColor");
    wireColor("slot" + i + "_titleColor");
    wireColor("slot" + i + "_backgroundColor");
    wireRange("slot" + i + "_fillAlpha");
    wireNumber("slot" + i + "_min");
    wireNumber("slot" + i + "_max");
    wireRange("slot" + i + "_titleFontSize");
    wireRange("slot" + i + "_valueFontSize");
    wireText("slot" + i + "_format");
    wireText("slot" + i + "_divisor");
    wireSelect("slot" + i + "_graphUnit");
  }
});

function wireSelect(id, extra) {
  var el = document.getElementById(id);
  if (!el) return;
  el[onchangeevt] = function () {
    sendSdpi(id, el.value);
    if (extra) extra(el.value);
  };
}

function wireSensorSelect(slotIdx) {
  var id = "slot" + slotIdx + "_sensorSelect";
  var el = document.getElementById(id);
  if (!el) return;
  el[onchangeevt] = function () {
    sendSdpi("slot" + slotIdx + "_sensorSelect", el.value);
  };
}

function wireReadingSelect(slotIdx) {
  var id = "slot" + slotIdx + "_readingSelect";
  var el = document.getElementById(id);
  if (!el) return;
  el[onchangeevt] = function () {
    sendSdpi("slot" + slotIdx + "_readingSelect", el.value);
  };
}

function wireText(id) {
  var el = document.getElementById(id);
  if (!el) return;
  el.onchange = function () { sendSdpi(id, el.value); };
}

function wireColor(id) {
  var el = document.getElementById(id);
  if (!el) return;
  el[onchangeevt] = function () { sendSdpi(id, el.value); };
}

function wireRange(id) {
  var el = document.getElementById(id);
  if (!el) return;
  el[onchangeevt] = function () { sendSdpi(id, el.value); };
}

function wireNumber(id) {
  var el = document.getElementById(id);
  if (!el) return;
  el.onchange = function () { sendSdpi(id, el.value); };
}
