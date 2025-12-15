# Libre Hardware Monitor Stream Deck Plugin

## ⚠⚠ Major refactor landed in pre-release v2.0.0, plugin code open sourced, remote monitoring infrastructure support ⚠⚠ 

---

>## Thank you & Looking for Maintainers
>
>Thank you everyone who has used and enjoyed this plugin. It started as a passion project and I continue to use it day to day. I am happy to finally release the full source on GitHub so the community can continue to shape it going forward.
>
>I haven't had the time to dedicate to this project in some time and appreciate everyone for hanging in there. I hope to work with some of you who are eager to take the project over. I am happy and ready to hand over the reigns. If there are development questions I'm happy to share my thoughts on the code and structure that exists.
>
>*-Shayne*

---

![alt text](images/demo.gif "Stream Deck Plugin Demo")

> The plugin now talks to [Libre Hardware Monitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor) over its HTTP bridge.

## Configuring Libre Hardware Monitor

1. Download the latest Libre Hardware Monitor release and unzip it somewhere permanent.
2. Launch `LibreHardwareMonitor.exe`.
3. Open `Options -> Remote Web Server...`.
4. Check **Active**, set the port to `8085`, and leave **Listen IP** at `127.0.0.1` (default settings).
5. Select **Start** to enable the server and close the dialog.
6. Verify things are working by opening [http://127.0.0.1:8085/data.json](http://127.0.0.1:8085/data.json) in a browser. The JSON response should look similar to [`example.json`](example.json). Keep Libre Hardware Monitor running while you use the Stream Deck action.

> Advanced: set the `LHM_ENDPOINT` environment variable before launching Stream Deck if you prefer another URL (e.g. a different port or host). The default endpoint is `http://127.0.0.1:8085/data.json`.


## Install and Setup the Plugin

1. Download the latest pre-compiled plugin

    [Plugin Releases](../../releases)

    > When upgrading, first uninstall: within the Stream Deck app choose "More Actions..." (bottom-right), locate "Libre Hardware Monitor" and choose "Uninstall". Your tiles and settings will be preserved.

2. Double-click to install the plugin

3. Choose "Install" went prompted by Stream Deck

    ![alt text](images/streamdeckinstall.png "Stream Deck Plugin Installation")

4. Locate "Libre Hardware Monitor" under "Custom" in the action list

    ![alt text](images/streamdeckactionlist.png "Stream Deck Action List")

5. Drag the "Libre Hardware Monitor" action from the list to a tile in the canvas area

    ![alt text](images/dragaction.gif "Drag Action")

6. Configure the action to display the sensor reading you wish

    ![alt text](images/configureaction.gif "Configure Action")

## Upstream diff (proof of origin)

Upstream is archived (read-only). Changes in this fork are visible here:
https://github.com/shayne/hwinfo-streamdeck/compare/main...moeilijk:lhm
