var websocket = null,
  uuid = null,
  actionInfo = {},
  allSensors = [],
  currentSettings = {},
  sourceProfiles = [],
  slotThresholdAdvancedOpen = {};

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

    // Per-slot threshold updates from plugin (after add/remove/reorder)
    if (payload.slotThresholds && typeof payload.slotThresholds.slotIndex === "number") {
      var st = payload.slotThresholds;
      renderSlotThresholds(st.slotIndex, st.thresholds || []);
      if (currentSettings.slots) currentSettings.slots[st.slotIndex].thresholds = st.thresholds || [];
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
  var saInp = byId("smoothingAlpha") && byId("smoothingAlpha").querySelector("input[type=range]");
  if (saInp) { saInp.value = s.smoothingAlpha > 0 ? s.smoothingAlpha : 1; positionRangeVal(saInp); }
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

    renderSlotThresholds(i, slot.thresholds || []);
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
  (function() {
    var inp = document.querySelector("#smoothingAlpha input[type=range]");
    if (inp) {
      inp.oninput = function() { positionRangeVal(this); };
      inp.onchange = function() { sendSdpi("smoothingAlpha", this.value); };
    }
  })();

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
    wireSlotAddThreshold(i);
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

// --- composite slot threshold helpers ---

function sendCompositeThresholdUpdate(slotIdx, key, thresholdId, value, checked) {
  sendValueToPlugin({
    key: "slot" + slotIdx + "_" + key,
    thresholdId: thresholdId,
    value: value !== undefined ? String(value) : "",
    checked: checked !== undefined ? checked : false
  }, "sdpi_collection");
}

function wireSlotAddThreshold(slotIdx) {
  var btn = byId("slot" + slotIdx + "_addThresholdBtn");
  var nameInput = byId("slot" + slotIdx + "_newThresholdName");
  if (!btn || !nameInput) return;
  btn.addEventListener("click", function() {
    var name = nameInput.value.trim() || "New Threshold";
    sendValueToPlugin({ key: "slot" + slotIdx + "_addThreshold", value: name }, "sdpi_collection");
    nameInput.value = "";
  });
  nameInput.addEventListener("keypress", function(e) {
    if (e.key === "Enter") btn.click();
  });
}

function renderSlotThresholds(slotIdx, thresholds) {
  var container = byId("slot" + slotIdx + "_thresholdsContainer");
  if (!container) return;
  while (container.firstChild) container.removeChild(container.firstChild);
  if (!thresholds || !thresholds.length) return;
  thresholds.forEach(function(threshold, index) {
    var el = createSlotThresholdElement(slotIdx, threshold, index, thresholds.length);
    container.appendChild(el);
  });
}

function createSlotThresholdElement(slotIdx, threshold, index, total) {
  var template = document.querySelector("#compositeThresholdTemplate");
  var clone = template.content.cloneNode(true);
  var wrapper = clone.querySelector(".threshold-item");
  wrapper.dataset.thresholdId = threshold.id;

  var nameInput = clone.querySelector(".threshold-name");
  nameInput.value = threshold.name || "";
  var textInput = clone.querySelector(".threshold-text");
  textInput.value = threshold.text || "";
  var operatorSelect = clone.querySelector(".threshold-operator");
  operatorSelect.value = threshold.operator || ">=";
  var valueInput = clone.querySelector(".threshold-value");
  valueInput.value = threshold.value !== undefined && threshold.value !== null ? threshold.value : "";
  var hysteresisInput = clone.querySelector(".threshold-hysteresis");
  hysteresisInput.value = threshold.hysteresis !== undefined && threshold.hysteresis !== null ? threshold.hysteresis : "";
  var dwellInput = clone.querySelector(".threshold-dwell");
  dwellInput.value = threshold.dwellMs !== undefined && threshold.dwellMs !== null ? threshold.dwellMs : "";
  var cooldownInput = clone.querySelector(".threshold-cooldown");
  cooldownInput.value = threshold.cooldownMs !== undefined && threshold.cooldownMs !== null ? threshold.cooldownMs : "";

  var stickyBtn = clone.querySelector(".threshold-sticky-toggle");
  var advancedToggleBtn = clone.querySelector(".threshold-advanced-toggle");
  var advancedPanel = clone.querySelector(".threshold-advanced-panel");

  var bgInput = clone.querySelector(".threshold-bg");
  bgInput.value = threshold.backgroundColor || "#333300";
  var fgInput = clone.querySelector(".threshold-fg");
  fgInput.value = threshold.foregroundColor || "#999900";
  var hlInput = clone.querySelector(".threshold-hl");
  hlInput.value = threshold.highlightColor || "#ffff00";
  var vtInput = clone.querySelector(".threshold-vt");
  vtInput.value = threshold.valueTextColor || "#ffff00";
  var tcInput = clone.querySelector(".threshold-tc");
  if (tcInput) tcInput.value = threshold.textColor || "#ffffff";

  var moveUpBtn = clone.querySelector(".threshold-move-up");
  var moveDownBtn = clone.querySelector(".threshold-move-down");
  if (moveUpBtn) moveUpBtn.disabled = index === 0;
  if (moveDownBtn) moveDownBtn.disabled = index === total - 1;

  var toggleBtn = clone.querySelector(".threshold-toggle");
  var settingsDiv = clone.querySelector(".threshold-settings");
  var isEnabled = threshold.enabled;
  var isSticky = threshold.sticky === true;
  var advKey = slotIdx + ":" + threshold.id;
  var isAdvancedOpen = slotThresholdAdvancedOpen[advKey] === true;

  function updateToggleState() {
    toggleBtn.textContent = isEnabled ? "on" : "off";
    toggleBtn.style.background = isEnabled ? "#4a4" : "#a44";
    settingsDiv.style.display = isEnabled ? "block" : "none";
  }
  function updateStickyState() {
    if (!stickyBtn) return;
    stickyBtn.textContent = isSticky ? "on" : "off";
    stickyBtn.style.background = isSticky ? "#4a4" : "#a44";
    stickyBtn.style.color = "#fff";
  }
  function updateAdvancedState() {
    if (!advancedToggleBtn || !advancedPanel) return;
    advancedToggleBtn.textContent = isAdvancedOpen ? "Advanced ▼" : "Advanced ►";
    advancedPanel.style.display = isAdvancedOpen ? "block" : "none";
  }

  updateToggleState();
  updateStickyState();
  updateAdvancedState();

  var thresholdId = threshold.id;

  toggleBtn.addEventListener("click", function() {
    isEnabled = !isEnabled;
    updateToggleState();
    sendCompositeThresholdUpdate(slotIdx, "thresholdEnabled", thresholdId, isEnabled ? "true" : "false", isEnabled);
  });

  var nameTimeout;
  nameInput.addEventListener("input", function(e) {
    clearTimeout(nameTimeout);
    nameTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdName", thresholdId, e.target.value);
    }, 300);
  });

  var textTimeout;
  textInput.addEventListener("input", function(e) {
    clearTimeout(textTimeout);
    textTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdText", thresholdId, e.target.value);
    }, 300);
  });

  operatorSelect.addEventListener("change", function(e) {
    sendCompositeThresholdUpdate(slotIdx, "thresholdOperator", thresholdId, e.target.value);
  });

  var valueTimeout;
  valueInput.addEventListener("input", function(e) {
    clearTimeout(valueTimeout);
    valueTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdValue", thresholdId, e.target.value);
    }, 300);
  });

  var hystTimeout;
  hysteresisInput.addEventListener("input", function(e) {
    clearTimeout(hystTimeout);
    hystTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdHysteresis", thresholdId, e.target.value);
    }, 300);
  });

  var dwellTimeout;
  dwellInput.addEventListener("input", function(e) {
    clearTimeout(dwellTimeout);
    dwellTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdDwellMs", thresholdId, e.target.value);
    }, 300);
  });

  var cooldownTimeout;
  cooldownInput.addEventListener("input", function(e) {
    clearTimeout(cooldownTimeout);
    cooldownTimeout = setTimeout(function() {
      sendCompositeThresholdUpdate(slotIdx, "thresholdCooldownMs", thresholdId, e.target.value);
    }, 300);
  });

  if (stickyBtn) {
    stickyBtn.addEventListener("click", function() {
      isSticky = !isSticky;
      updateStickyState();
      sendCompositeThresholdUpdate(slotIdx, "thresholdSticky", thresholdId, isSticky ? "true" : "false", isSticky);
    });
  }

  if (advancedToggleBtn) {
    advancedToggleBtn.addEventListener("click", function() {
      isAdvancedOpen = !isAdvancedOpen;
      slotThresholdAdvancedOpen[advKey] = isAdvancedOpen;
      updateAdvancedState();
    });
  }

  bgInput.addEventListener("change", function(e) {
    sendCompositeThresholdUpdate(slotIdx, "thresholdBackgroundColor", thresholdId, e.target.value);
  });
  fgInput.addEventListener("change", function(e) {
    sendCompositeThresholdUpdate(slotIdx, "thresholdForegroundColor", thresholdId, e.target.value);
  });
  hlInput.addEventListener("change", function(e) {
    sendCompositeThresholdUpdate(slotIdx, "thresholdHighlightColor", thresholdId, e.target.value);
  });
  vtInput.addEventListener("change", function(e) {
    sendCompositeThresholdUpdate(slotIdx, "thresholdValueTextColor", thresholdId, e.target.value);
  });
  if (tcInput) {
    tcInput.addEventListener("change", function(e) {
      sendCompositeThresholdUpdate(slotIdx, "thresholdTextColor", thresholdId, e.target.value);
    });
  }

  if (moveUpBtn) {
    moveUpBtn.addEventListener("click", function() {
      sendValueToPlugin({ key: "slot" + slotIdx + "_reorderThreshold", thresholdId: thresholdId, value: "up" }, "sdpi_collection");
    });
  }
  if (moveDownBtn) {
    moveDownBtn.addEventListener("click", function() {
      sendValueToPlugin({ key: "slot" + slotIdx + "_reorderThreshold", thresholdId: thresholdId, value: "down" }, "sdpi_collection");
    });
  }

  var removeBtn = clone.querySelector(".threshold-remove");
  removeBtn.addEventListener("click", function() {
    delete slotThresholdAdvancedOpen[advKey];
    sendValueToPlugin({ key: "slot" + slotIdx + "_removeThreshold", thresholdId: thresholdId }, "sdpi_collection");
    var item = removeBtn.closest(".threshold-item");
    if (item && item.parentNode) item.parentNode.removeChild(item);
  });

  return clone;
}
