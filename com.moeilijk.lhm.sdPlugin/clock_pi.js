var websocket, clockContext, settings = {};
function connectElgatoStreamDeckSocket(port, uuid, registerEvent, info, actionInfo) {
  var action = JSON.parse(actionInfo);
  clockContext = action.context;
  settings = action.payload.settings || {};
  showSettings();
  websocket = new WebSocket("ws://127.0.0.1:" + port);
  websocket.onopen = function() {
    websocket.send(JSON.stringify({event: registerEvent, uuid: uuid}));
    websocket.send(JSON.stringify({event: "getSettings", context: clockContext}));
  };
  websocket.onmessage = function(event) {
    var data = JSON.parse(event.data);
    if (data.event === "didReceiveSettings") { settings = data.payload.settings || {}; showSettings(); }
  };
}
function showSettings() {
  document.getElementById("language").value = settings.language === "ru" ? "ru" : "en";
  ["twelveHour", "staticColon"].forEach(function(id) { document.getElementById(id).value = String(settings[id] === true); });
}
["language", "twelveHour", "staticColon"].forEach(function(id) {
  document.getElementById(id).onchange = function() {
    settings[id] = id === "language" ? this.value : this.value === "true";
    if (websocket && websocket.readyState === WebSocket.OPEN) websocket.send(JSON.stringify({event: "setSettings", context: clockContext, payload: settings}));
  };
});
