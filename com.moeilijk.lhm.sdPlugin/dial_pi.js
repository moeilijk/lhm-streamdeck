var websocket = null,
  uuid = null,
  actionInfo = {},
  currentSettings = { activeIndex: 0, pages: [] },
  currentCatalog = { sensors: [], readings: [], sourceProfiles: [] },
  filteredReadings = [];

function connectElgatoStreamDeckSocket(inPort, inUUID, inRegisterEvent, inInfo, inActionInfo) {
  uuid = inUUID;
  actionInfo = JSON.parse(inActionInfo);
  var info = JSON.parse(inInfo);
  websocket = new WebSocket("ws://" + ((typeof location !== "undefined" && location.hostname) ? location.hostname : "127.0.0.1") + ":" + inPort);

  websocket.onopen = function () {
    websocket.send(JSON.stringify({ event: inRegisterEvent, uuid: inUUID }));
    sendValueToPlugin("propertyInspectorConnected", "property_inspector");
  };

  websocket.onmessage = function (evt) {
    var msg = JSON.parse(evt.data);
    if (msg.event !== "sendToPropertyInspector") return;
    var payload = msg.payload || {};

    if (typeof payload.error === "boolean") {
      document.getElementById("ui").style.display = payload.error ? "none" : "";
      document.getElementById("error").style.display = payload.error ? "block" : "none";
      return;
    }

    if (payload.catalog) {
      currentCatalog = payload.catalog;
      populateProfiles();
      filterReadings();
    }
    if (payload.dialSettings) {
      currentSettings = normalizeSettings(payload.dialSettings);
      populateProfiles();
      renderPages();
    }
  };
}

function sendValueToPlugin(value, key) {
  if (!websocket || websocket.readyState !== 1) return;
  websocket.send(JSON.stringify({
    event: "sendToPlugin",
    context: uuid,
    action: actionInfo.action,
    payload: { [key]: value }
  }));
}

function normalizeSettings(settings) {
  settings = settings || {};
  if (!Array.isArray(settings.pages)) settings.pages = [];
  settings.pages = settings.pages.map(normalizePage);
  if (typeof settings.activeIndex !== "number") settings.activeIndex = 0;
  if (settings.activeIndex < 0) settings.activeIndex = 0;
  if (settings.pages.length && settings.activeIndex >= settings.pages.length) {
    settings.activeIndex = settings.pages.length - 1;
  }
  return settings;
}

function normalizePage(page) {
  page = page || {};
  if (page.min === undefined || page.min === null || page.min === "") page.min = 0;
  if (page.max === undefined || page.max === null || page.max === "") page.max = 100;
  if (!page.format) page.format = "";
  if (!page.divisor) page.divisor = "";
  if (!page.graphUnit) page.graphUnit = "";
  if (!page.titleColor) page.titleColor = "#b7b7b7";
  if (!page.foregroundColor) page.foregroundColor = "#005128";
  if (!page.backgroundColor) page.backgroundColor = "#000000";
  if (!page.highlightColor) page.highlightColor = "#009e00";
  if (!page.valueTextColor) page.valueTextColor = "#ffffff";
  if (!page.graphMode) page.graphMode = "both";
  if (!page.graphHeightPct) page.graphHeightPct = 100;
  if (!page.graphLineThickness) page.graphLineThickness = 1;
  if (!page.titleFontSize) page.titleFontSize = 0;
  if (!page.valueFontSize) page.valueFontSize = 0;
  if (!page.textStrokeColor) page.textStrokeColor = page.backgroundColor || "#000000";
  page.textStroke = !!page.textStroke;
  return page;
}

function saveSettings() {
  currentSettings = normalizeSettings(currentSettings);
  sendValueToPlugin(currentSettings, "dialSetSettings");
  renderPages();
}

function populateProfiles() {
  var sel = document.getElementById("sourceProfileSelect");
  if (!sel) return;
  var selected = currentSettings.sourceProfileId || "";
  sel.innerHTML = "";
  var defaultOpt = document.createElement("option");
  defaultOpt.value = "";
  defaultOpt.textContent = "Default";
  if (!selected) defaultOpt.selected = true;
  sel.appendChild(defaultOpt);
  (currentCatalog.sourceProfiles || []).forEach(function (profile) {
    var opt = document.createElement("option");
    opt.value = profile.id;
    opt.textContent = profile.name || profile.id;
    if (profile.id === selected) opt.selected = true;
    sel.appendChild(opt);
  });
  if (!sel.dataset.bound) {
    sel.dataset.bound = "1";
    sel.addEventListener("change", function (e) {
      currentSettings.sourceProfileId = e.target.value;
      currentSettings.pages.forEach(function (page) {
        page.sourceProfileId = e.target.value;
      });
      saveSettings();
      sendValueToPlugin(true, "requestDialCatalog");
    });
  }
}

function filterReadings() {
  var q = (document.getElementById("sensorSearch").value || "").toLowerCase().trim();
  filteredReadings = (currentCatalog.readings || []).filter(function (reading) {
    if (!q) return true;
    var text = [
      reading.sensorName,
      reading.label,
      reading.unit,
      reading.type,
      reading.searchText
    ].join(" ").toLowerCase();
    return text.indexOf(q) !== -1;
  }).sort(function (a, b) {
    var an = (a.sensorName || "") + " " + (a.label || "");
    var bn = (b.sensorName || "") + " " + (b.label || "");
    return an > bn ? 1 : an < bn ? -1 : 0;
  });
  populateReadings();
}

function populateReadings() {
  var sel = document.getElementById("readingSelect");
  sel.innerHTML = "";
  filteredReadings.slice(0, 300).forEach(function (reading, index) {
    var opt = document.createElement("option");
    opt.value = String(index);
    opt.textContent = (reading.sensorName || reading.sensorUid || "") + " / " +
      reading.label + (reading.unit ? " (" + reading.unit + ")" : "");
    sel.appendChild(opt);
  });
  sel.disabled = filteredReadings.length === 0;
  document.getElementById("addPageBtn").disabled = filteredReadings.length === 0;
}

function pageTitle(page) {
  return page.title || page.readingLabel || page.sensorUid || "Reading";
}

function renderPages() {
  var list = document.getElementById("pageList");
  list.innerHTML = "";
  currentSettings.pages.forEach(function (page, index) {
    var opt = document.createElement("option");
    opt.value = String(index);
    opt.textContent = String(index + 1) + ". " + pageTitle(page);
    if (index === currentSettings.activeIndex) opt.selected = true;
    list.appendChild(opt);
  });
  updatePageButtons();
  renderPageSettings();
}

function selectedPageIndex() {
  var list = document.getElementById("pageList");
  return list.selectedIndex >= 0 ? list.selectedIndex : currentSettings.activeIndex || 0;
}

function updatePageButtons() {
  var idx = selectedPageIndex();
  var count = currentSettings.pages.length;
  document.getElementById("removePageBtn").disabled = count === 0;
  document.getElementById("moveUpBtn").disabled = count === 0 || idx <= 0;
  document.getElementById("moveDownBtn").disabled = count === 0 || idx >= count - 1;
}

function selectedPage() {
  var idx = selectedPageIndex();
  if (idx < 0 || idx >= currentSettings.pages.length) return null;
  currentSettings.pages[idx] = normalizePage(currentSettings.pages[idx]);
  return currentSettings.pages[idx];
}

function setValue(id, value) {
  var el = document.getElementById(id);
  if (!el) return;
  if (el.type === "checkbox") el.checked = !!value;
  else el.value = value === undefined || value === null ? "" : String(value);
}

function renderPageSettings() {
  var panel = document.getElementById("pageSettings");
  var page = selectedPage();
  panel.hidden = !page;
  if (!page) return;
  setValue("pageTitle", page.title || "");
  setValue("graphMode", page.graphMode || "both");
  setValue("minValue", page.min);
  setValue("maxValue", page.max);
  setValue("formatValue", page.format || "");
  setValue("divisorValue", page.divisor || "");
  setValue("graphUnit", page.graphUnit || "");
  setValue("titleFontSize", page.titleFontSize || 0);
  setValue("valueFontSize", page.valueFontSize || 0);
  setValue("graphHeightPct", page.graphHeightPct || 100);
  setValue("graphLineThickness", page.graphLineThickness || 1);
  setValue("titleColor", page.titleColor || "#b7b7b7");
  setValue("valueTextColor", page.valueTextColor || "#ffffff");
  setValue("backgroundColor", page.backgroundColor || "#000000");
  setValue("foregroundColor", page.foregroundColor || "#005128");
  setValue("highlightColor", page.highlightColor || "#009e00");
  setValue("textStroke", page.textStroke);
  setValue("textStrokeColor", page.textStrokeColor || page.backgroundColor || "#000000");
}

function bindPageField(id, key, parser) {
  var el = document.getElementById(id);
  if (!el || el.dataset.bound) return;
  el.dataset.bound = "1";
  var handler = function () {
    var page = selectedPage();
    if (!page) return;
    var raw = el.type === "checkbox" ? el.checked : el.value;
    page[key] = parser ? parser(raw) : raw;
    saveSettings();
  };
  el.addEventListener("input", handler);
  el.addEventListener("change", handler);
}

function bindPageSettings() {
  bindPageField("pageTitle", "title");
  bindPageField("graphMode", "graphMode");
  bindPageField("minValue", "min", function (v) { return Number(v) || 0; });
  bindPageField("maxValue", "max", function (v) { return Number(v) || 100; });
  bindPageField("formatValue", "format");
  bindPageField("divisorValue", "divisor");
  bindPageField("graphUnit", "graphUnit");
  bindPageField("titleFontSize", "titleFontSize", function (v) { return Number(v) || 0; });
  bindPageField("valueFontSize", "valueFontSize", function (v) { return Number(v) || 0; });
  bindPageField("graphHeightPct", "graphHeightPct", function (v) { return Number(v) || 100; });
  bindPageField("graphLineThickness", "graphLineThickness", function (v) { return Number(v) || 1; });
  bindPageField("titleColor", "titleColor");
  bindPageField("valueTextColor", "valueTextColor");
  bindPageField("backgroundColor", "backgroundColor");
  bindPageField("foregroundColor", "foregroundColor");
  bindPageField("highlightColor", "highlightColor");
  bindPageField("textStroke", "textStroke", function (v) { return !!v; });
  bindPageField("textStrokeColor", "textStrokeColor");
}

function addSelectedPage() {
  var sel = document.getElementById("readingSelect");
  var reading = filteredReadings[Number(sel.value)];
  if (!reading) return;
  currentSettings.pages.push(normalizePage({
    sourceProfileId: currentSettings.sourceProfileId || "",
    sensorUid: reading.sensorUid,
    readingId: String(reading.id),
    readingLabel: reading.label,
    title: reading.label,
    min: 0,
    max: 100,
    format: "",
    divisor: "",
    graphUnit: "",
    isValid: true,
    titleColor: "#b7b7b7",
    foregroundColor: "#005128",
    backgroundColor: "#000000",
    highlightColor: "#009e00",
    valueTextColor: "#ffffff",
    graphMode: "both",
    graphHeightPct: 100,
    graphLineThickness: 1,
    textStroke: false,
    textStrokeColor: "#000000"
  }));
  currentSettings.activeIndex = currentSettings.pages.length - 1;
  saveSettings();
}

function removeSelectedPage() {
  var idx = selectedPageIndex();
  if (idx < 0 || idx >= currentSettings.pages.length) return;
  currentSettings.pages.splice(idx, 1);
  if (currentSettings.activeIndex >= currentSettings.pages.length) {
    currentSettings.activeIndex = Math.max(0, currentSettings.pages.length - 1);
  }
  saveSettings();
}

function moveSelectedPage(delta) {
  var idx = selectedPageIndex();
  var next = idx + delta;
  if (idx < 0 || next < 0 || next >= currentSettings.pages.length) return;
  var page = currentSettings.pages[idx];
  currentSettings.pages[idx] = currentSettings.pages[next];
  currentSettings.pages[next] = page;
  currentSettings.activeIndex = next;
  saveSettings();
}

document.addEventListener("DOMContentLoaded", function () {
  document.getElementById("sensorSearch").addEventListener("input", filterReadings);
  document.getElementById("addPageBtn").addEventListener("click", addSelectedPage);
  document.getElementById("removePageBtn").addEventListener("click", removeSelectedPage);
  document.getElementById("moveUpBtn").addEventListener("click", function () { moveSelectedPage(-1); });
  document.getElementById("moveDownBtn").addEventListener("click", function () { moveSelectedPage(1); });
  document.getElementById("pageList").addEventListener("change", function () {
    currentSettings.activeIndex = selectedPageIndex();
    saveSettings();
    renderPageSettings();
  });
  bindPageSettings();
});
