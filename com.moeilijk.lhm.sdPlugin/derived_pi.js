var websocket = null,
  uuid = null,
  actionInfo = {},
  allSensors = [],
  allPresets = [],
  allFavorites = [],
  currentSettings = {},
  sourceProfiles = [];

var onchangeevt = "onchange";

function connectElgatoStreamDeckSocket(inPort, inUUID, inRegisterEvent, inInfo, inActionInfo) {
  uuid = inUUID;
  actionInfo = JSON.parse(inActionInfo);
  websocket = new WebSocket("ws://localhost:" + inPort);

  websocket.onopen = function () {
    websocket.send(JSON.stringify({ event: inRegisterEvent, uuid: inUUID }));
    websocket.send(JSON.stringify({ event: "getGlobalSettings", context: inUUID }));
    sendValueToPlugin("propertyInspectorConnected", "property_inspector");
  };

  websocket.onmessage = function (evt) {
    var jsonObj = JSON.parse(evt.data);
    var event = jsonObj["event"];

    // Global settings (presets storage)
    if (event === "didReceiveGlobalSettings") {
      var gs = (jsonObj.payload && jsonObj.payload.settings) ? jsonObj.payload.settings : {};
      allPresets = Array.isArray(gs.derivedPresets) ? gs.derivedPresets : [];
      populatePresetSelect(allPresets);
      return;
    }

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

    // Favorites list
    if (Array.isArray(payload.favorites)) {
      allFavorites = payload.favorites;
      for (var i = 0; i < 8; i++) {
        populateFavoriteSelect(i, allFavorites);
      }
    }

    // Source profiles
    if (Array.isArray(payload.sourceProfiles)) {
      sourceProfiles = payload.sourceProfiles;
      rebuildSourceProfileDropdown(currentSettings.sourceProfileId || "");
    }

    // Sensor list
    if (Array.isArray(payload.sensors)) {
      allSensors = payload.sensors;
      for (var i = 0; i < 8; i++) {
        populateSensorSelect(i, allSensors);
      }
      populateAllSlotsSensorSelect(allSensors);
    }

    // Full settings object — must come before populateReadingSelect so currentSettings is current
    if (payload.derivedSettings) {
      currentSettings = payload.derivedSettings;
      applySettingsToUI(currentSettings);
      rebuildSourceProfileDropdown(currentSettings.sourceProfileId || "");
    }

    // Readings for a specific slot
    if (Array.isArray(payload.readings) && typeof payload.slotIndex === "number") {
      populateReadingSelect(payload.slotIndex, payload.readings);
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

// --- favorites ---

function populateFavoriteSelect(slotIdx, favorites) {
  var el = document.getElementById("slot" + slotIdx + "_favoriteSelect");
  if (!el) return;
  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.value = "";
  ph.text = favorites.length ? "— from favorite —" : "— no favorites saved —";
  ph.disabled = true;
  ph.selected = true;
  el.add(ph);
  favorites.forEach(function (fav) {
    var opt = document.createElement("option");
    opt.value = fav.id;
    var cat = fav.category ? fav.category.toUpperCase() + " — " : "";
    opt.text = cat + fav.sensorName + " — " + fav.readingLabel;
    el.add(opt);
  });
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

function populateAllSlotsSensorSelect(sensors) {
  var el = document.getElementById("allSlots_sensorSelect");
  if (!el) return;
  var sorted = sensors.slice().sort(function (a, b) {
    return a.name > b.name ? 1 : a.name < b.name ? -1 : 0;
  });

  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.value = "";
  ph.text = "Set all slots to…";
  ph.disabled = true;
  ph.selected = true;
  el.add(ph);

  sorted.forEach(function (sensor) {
    var opt = document.createElement("option");
    opt.text = sensor.name;
    opt.value = sensor.uid;
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

// --- presets ---

function populatePresetSelect(presets) {
  var el = document.getElementById("preset_load");
  if (!el) return;
  while (el.options.length) el.remove(0);
  var ph = document.createElement("option");
  ph.value = "";
  ph.text = "— select preset —";
  ph.disabled = true;
  ph.selected = true;
  el.add(ph);
  presets.forEach(function (p) {
    var opt = document.createElement("option");
    opt.text = p.name;
    opt.value = p.name;
    el.add(opt);
  });
}

function saveCurrentAsPreset(name) {
  if (!name) return;
  var slots = [];
  for (var i = 0; i < 8; i++) {
    var s = (currentSettings.slots && currentSettings.slots[i]) ? currentSettings.slots[i] : {};
    slots.push({
      sensorUid:    s.sensorUid    || "",
      readingId:    s.readingId    || 0,
      readingLabel: s.readingLabel || "",
      isValid:      s.isValid      || false,
      divisor:      s.divisor      || "",
      graphUnit:    s.graphUnit    || ""
    });
  }
  var preset = {
    name:      name,
    formula:   currentSettings.formula   || "sum",
    slotCount: currentSettings.slotCount || 2,
    slots:     slots
  };
  var updated = allPresets.filter(function (p) { return p.name !== name; });
  updated.push(preset);
  allPresets = updated;
  populatePresetSelect(allPresets);
  if (websocket && websocket.readyState === 1) {
    websocket.send(JSON.stringify({
      event: "setGlobalSettings",
      context: uuid,
      payload: { derivedPresets: allPresets }
    }));
  }
}

function loadPresetByName(name) {
  var preset = null;
  for (var i = 0; i < allPresets.length; i++) {
    if (allPresets[i].name === name) { preset = allPresets[i]; break; }
  }
  if (!preset) return;
  // Update local state so applySettingsToUI works when the backend responds
  currentSettings.formula   = preset.formula;
  currentSettings.slotCount = preset.slotCount;
  currentSettings.slots     = preset.slots;
  sendValueToPlugin({
    formula:   preset.formula,
    slotCount: preset.slotCount,
    slots:     preset.slots
  }, "loadDerivedPreset");
  // Reset the load select back to placeholder
  var el = document.getElementById("preset_load");
  if (el) el.selectedIndex = 0;
}

// --- populate all UI fields from settings ---

function applySettingsToUI(s) {
  setInputValue("titleFontSize", s.titleFontSize || 10.5);
  setInputValue("valueFontSize", s.valueFontSize || 10.5);
  setSelectValue("derived_formula", s.formula || "sum");
  setSelectValue("derived_slotCount", String(s.slotCount || 2));
  updateSlotCountForFormula(s.formula || "sum");
  updateSlotVisibility(s.slotCount || 2);

  setColorValue("derived_highlightColor", s.highlightColor);
  setColorValue("derived_foregroundColor", s.foregroundColor);
  setColorValue("derived_backgroundColor", s.backgroundColor);
  setColorValue("derived_valueTextColor", s.valueTextColor);
  setColorValue("derived_titleColor", s.titleColor);
  setInputValue("derived_min", s.min || "");
  setInputValue("derived_max", s.max || "");
  setInputValue("derived_format", s.format || "");
  setInputValue("derived_divisor", s.divisor || "");
  setSelectValue("derived_graphUnit", s.graphUnit || "");

  var slots = s.slots || [];
  for (var i = 0; i < 8; i++) {
    var slot = slots[i] || {};
    setInputValue("slot" + i + "_divisor", slot.divisor || "");
    setSelectValue("slot" + i + "_graphUnit", slot.graphUnit || "");

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

// --- slot count locking ---

function updateSlotCountForFormula(formula) {
  var el = document.getElementById("derived_slotCount");
  if (!el) return;
  if (formula === "delta") {
    el.value = "2";
    el.style.pointerEvents = "none";
    el.style.opacity = "0.5";
    updateSlotVisibility(2);
    sendSdpi("derived_slotCount", "2");
  } else {
    el.style.pointerEvents = "";
    el.style.opacity = "";
  }
}

// --- slot visibility ---

function updateSlotVisibility(count) {
  for (var i = 0; i < 8; i++) {
    var sec = document.getElementById("slot-section-" + i);
    if (sec) sec.style.display = i < count ? "" : "none";
  }
}

// --- wire up all events after DOM ready ---

document.addEventListener("DOMContentLoaded", function () {
  // Presets
  var presetLoad = document.getElementById("preset_load");
  if (presetLoad) {
    presetLoad[onchangeevt] = function () {
      if (presetLoad.value) loadPresetByName(presetLoad.value);
    };
  }
  var presetSaveas = document.getElementById("preset_saveas");
  if (presetSaveas) {
    presetSaveas.onchange = function () {
      var name = presetSaveas.value.trim();
      if (name) {
        saveCurrentAsPreset(name);
        presetSaveas.value = "";
      }
    };
  }

  // All-slots sensor select
  var allSlotsSel = document.getElementById("allSlots_sensorSelect");
  if (allSlotsSel) {
    allSlotsSel[onchangeevt] = function () {
      if (!allSlotsSel.value) return;
      sendSdpi("allSlots_sensorSelect", allSlotsSel.value);
      // Reset to placeholder after triggering
      allSlotsSel.selectedIndex = 0;
    };
  }

  // Tile-wide
  wireRange("titleFontSize");
  wireRange("valueFontSize");
  wireSelect("derived_formula", function (val) {
    updateSlotCountForFormula(val);
  });
  wireSelect("derived_slotCount", function (val) {
    updateSlotVisibility(parseInt(val, 10));
  });
  wireColor("derived_highlightColor");
  wireColor("derived_foregroundColor");
  wireColor("derived_backgroundColor");
  wireColor("derived_valueTextColor");
  wireColor("derived_titleColor");
  wireNumber("derived_min");
  wireNumber("derived_max");
  wireText("derived_format");
  wireText("derived_divisor");
  wireSelect("derived_graphUnit");

  for (var i = 0; i < 8; i++) {
    wireFavoriteSelect(i);
    wireSensorSelect(i);
    wireReadingSelect(i);
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

function wireFavoriteSelect(slotIdx) {
  var el = document.getElementById("slot" + slotIdx + "_favoriteSelect");
  if (!el) return;
  el[onchangeevt] = function () {
    if (!el.value) return;
    sendSdpi("slot" + slotIdx + "_applyFavorite", el.value);
    el.selectedIndex = 0;
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
