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
  websocket = new WebSocket("ws://" + location.hostname + ":" + inPort);

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
  if (typeof settings.activeIndex !== "number") settings.activeIndex = 0;
  if (settings.activeIndex < 0) settings.activeIndex = 0;
  if (settings.pages.length && settings.activeIndex >= settings.pages.length) {
    settings.activeIndex = settings.pages.length - 1;
  }
  return settings;
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

function addSelectedPage() {
  var sel = document.getElementById("readingSelect");
  var reading = filteredReadings[Number(sel.value)];
  if (!reading) return;
  currentSettings.pages.push({
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
    graphMode: "both"
  });
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
  });
});
