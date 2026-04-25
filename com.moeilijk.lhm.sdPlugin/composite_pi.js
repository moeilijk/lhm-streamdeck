var websocket = null,
  uuid = null,
  actionInfo = {},
  allSensors = [],
  currentSettings = {},
  sourceProfiles = [];

var onchangeevt = "onchange";

function updateRangeDisplay(id) {
  var inp = document.getElementById(id);
  if (inp) positionRangeVal(inp);
}

function wireRangeOninput(id) {
  var inp = document.getElementById(id);
  if (inp) inp.oninput = function() { positionRangeVal(this); };
}

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

function sendSdpiChecked(key, checked) {
  sendValueToPlugin({ key: key, value: "", checked: checked }, "sdpi_collection");
}

function rebuildSourceProfileDropdown(selectedId) {
  var sel = byId("sourceProfileSelect");
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
  var el = byId("slot" + slotIdx + "_sensorSelect");
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
  var el = byId("slot" + slotIdx + "_readingSelect");
  if (!el) return;
  var slot = currentSettings.slots ? currentSettings.slots[slotIdx] : null;
  var currentRid = slot ? String(slot.readingId) : "";

  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.text = "Choose a reading";
  ph.disabled = true;
  if (!slot || !slot.isValid) ph.selected = true;
  el.add(ph);

  readings.slice().sort(compareReadings).forEach(function (r) {
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
  var slotCount = s.slotCount || 2;
  setSelectValue("composite_mode", s.mode || "both");
  setSelectValue("composite_slotCount", String(slotCount));
  setSelectValue("updateIntervalOverrideMs", String(s.updateIntervalOverrideMs || 0));
  updateSlotVisibility(slotCount);

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
    updateRangeDisplay("slot" + i + "_fillAlpha");
    setInputValue("slot" + i + "_min", slot.min != null ? slot.min : "");
    setInputValue("slot" + i + "_max", slot.max != null ? slot.max : "");
    setInputValue("slot" + i + "_titleFontSize", slot.titleFontSize || 9);
    updateRangeDisplay("slot" + i + "_titleFontSize");
    setInputValue("slot" + i + "_valueFontSize", slot.valueFontSize || 10.5);
    updateRangeDisplay("slot" + i + "_valueFontSize");
    setInputValue("slot" + i + "_graphHeightPct", slot.graphHeightPct || 100);
    updateRangeDisplay("slot" + i + "_graphHeightPct");
    setInputValue("slot" + i + "_graphLineThickness", slot.graphLineThickness || 1);
    updateRangeDisplay("slot" + i + "_graphLineThickness");
    var tsEl = byId("slot" + i + "_textStroke");
    if (tsEl) tsEl.checked = slot.textStroke === true;
    setColorValue("slot" + i + "_textStrokeColor", slot.textStrokeColor || "#000000");
    setInputValue("slot" + i + "_format", slot.format || "");
    setInputValue("slot" + i + "_divisor", slot.divisor || "");
    setSelectValue("slot" + i + "_graphUnit", slot.graphUnit || "");

    if (allSensors.length > 0) {
      setSelectValue("slot" + i + "_sensorSelect", slot.sensorUid || "");
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
  bindSdpiValue("composite_mode", sendSdpi, onchangeevt);
  bindSdpiValue("composite_slotCount", sendSdpi, onchangeevt, function (val) {
    updateSlotVisibility(parseInt(val, 10));
  });
  bindSdpiValue("updateIntervalOverrideMs", sendSdpi, onchangeevt);

  for (var i = 0; i < 4; i++) {
    wireSensorSelect(i);
    wireReadingSelect(i);
    bindSdpiValue("slot" + i + "_title", sendSdpi, "onchange");
    bindSdpiValue("slot" + i + "_highlightColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_foregroundColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_valueTextColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_titleColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_backgroundColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_fillAlpha", sendSdpi, onchangeevt);
    wireRangeOninput("slot" + i + "_fillAlpha");
    bindSdpiValue("slot" + i + "_min", sendSdpi, "onchange");
    bindSdpiValue("slot" + i + "_max", sendSdpi, "onchange");
    bindSdpiValue("slot" + i + "_titleFontSize", sendSdpi, onchangeevt);
    wireRangeOninput("slot" + i + "_titleFontSize");
    bindSdpiValue("slot" + i + "_valueFontSize", sendSdpi, onchangeevt);
    wireRangeOninput("slot" + i + "_valueFontSize");
    bindSdpiValue("slot" + i + "_graphHeightPct", sendSdpi, onchangeevt);
    wireRangeOninput("slot" + i + "_graphHeightPct");
    bindSdpiValue("slot" + i + "_graphLineThickness", sendSdpi, onchangeevt);
    wireRangeOninput("slot" + i + "_graphLineThickness");
    (function(idx) {
      var cb = byId("slot" + idx + "_textStroke");
      if (cb) cb.onchange = function() { sendSdpiChecked("slot" + idx + "_textStroke", this.checked); };
    })(i);
    bindSdpiValue("slot" + i + "_textStrokeColor", sendSdpi, onchangeevt);
    bindSdpiValue("slot" + i + "_format", sendSdpi, "onchange");
    bindSdpiValue("slot" + i + "_divisor", sendSdpi, "onchange");
    bindSdpiValue("slot" + i + "_graphUnit", sendSdpi, onchangeevt);
  }
});

function wireSensorSelect(slotIdx) {
  bindValueChange("slot" + slotIdx + "_sensorSelect", onchangeevt, function (value) {
    sendSdpi("slot" + slotIdx + "_sensorSelect", value);
  });
}

function wireReadingSelect(slotIdx) {
  bindValueChange("slot" + slotIdx + "_readingSelect", onchangeevt, function (value) {
    sendSdpi("slot" + slotIdx + "_readingSelect", value);
  });
}
