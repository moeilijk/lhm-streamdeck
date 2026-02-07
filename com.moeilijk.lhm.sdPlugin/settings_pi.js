// Settings Property Inspector for LHM Settings tile
var websocket = null,
  uuid = null,
  actionInfo = {},
  inInfo = {};

function connectElgatoStreamDeckSocket(inPort, inUUID, inRegisterEvent, inInfo, inActionInfo) {
  uuid = inUUID;
  actionInfo = JSON.parse(inActionInfo);
  inInfo = JSON.parse(inInfo);
  websocket = new WebSocket("ws://localhost:" + inPort);

  websocket.onopen = function () {
    // Register with Stream Deck
    websocket.send(JSON.stringify({
      event: inRegisterEvent,
      uuid: inUUID,
    }));

    // Request global settings
    websocket.send(JSON.stringify({
      event: "getGlobalSettings",
      context: inUUID,
    }));

    // Notify plugin that settings PI is connected
    websocket.send(JSON.stringify({
      action: actionInfo["action"],
      event: "sendToPlugin",
      context: uuid,
      payload: {
        settingsConnected: true
      },
    }));
  };

  websocket.onmessage = function (evt) {
    var jsonObj = JSON.parse(evt.data);
    var event = jsonObj["event"];

    // Handle global settings received
    if (event === "didReceiveGlobalSettings") {
      var settings = jsonObj.payload?.settings || {};
      var interval = settings.pollInterval || 1000;
      document.getElementById("pollInterval").value = interval;
      document.getElementById("currentRate").textContent = interval + "ms";
    }

    // Handle status updates from plugin
    if (event === "sendToPropertyInspector") {
      var payload = jsonObj.payload || {};
      if (payload.connectionStatus !== undefined) {
        var statusEl = document.getElementById("connectionStatus");
        statusEl.textContent = payload.connectionStatus;
        statusEl.style.color = payload.connectionStatus === "Connected" ? "#4a4" : "#a44";
      }
      if (payload.currentRate !== undefined) {
        document.getElementById("currentRate").textContent = payload.currentRate + "ms";
      }
    }
  };
}

// Handle poll interval change
document.getElementById("pollInterval").addEventListener("change", function(e) {
  var interval = parseInt(e.target.value);

  // Save to global settings (persisted by Stream Deck)
  websocket.send(JSON.stringify({
    event: "setGlobalSettings",
    context: uuid,
    payload: {
      pollInterval: interval
    }
  }));

  // Notify plugin to apply immediately
  websocket.send(JSON.stringify({
    action: actionInfo["action"],
    event: "sendToPlugin",
    context: uuid,
    payload: {
      setPollInterval: interval
    }
  }));

  // Update display immediately
  document.getElementById("currentRate").textContent = interval + "ms";
});
