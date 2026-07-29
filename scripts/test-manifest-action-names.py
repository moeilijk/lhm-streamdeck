#!/usr/bin/env python3
"""Tests for the action names in manifest.json (issue #80).

Elgato's naming guideline wants action names to describe their functionality
and stay concise; the Stream Deck action list already shows the plugin
category above its actions, so the "LHM " prefix only repeated it.

Fails (before the fix) on any action carrying that prefix, and whenever a
property inspector page's <title> drifts away from its action name.
"""
import json
import os
import re
import sys

sys.dont_write_bytecode = True  # keep scripts/__pycache__ out of the tree

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SDPLUGINDIR = os.path.join(REPO, "com.moeilijk.lhm.sdPlugin")
MANIFEST = os.path.join(SDPLUGINDIR, "manifest.json")

# The reading action keeps the plugin name (it is the plugin's primary tile);
# every other action is named after what it does.
EXPECTED_NAMES = {
    "com.moeilijk.lhm.reading": "Libre Hardware Monitor",
    "com.moeilijk.lhm.composite": "Composite Dashboard",
    "com.moeilijk.lhm.derived": "Derived Metric",
    "com.moeilijk.lhm.dial": "Dial Carousel",
    "com.moeilijk.lhm.settings": "Settings",
}

failures = []


def check(name, cond, detail=""):
    if cond:
        print(f"ok   {name}")
    else:
        failures.append(name)
        print(f"FAIL {name} {detail}")


with open(MANIFEST) as f:
    manifest = json.load(f)

actions = manifest["Actions"]
by_uuid = {a["UUID"]: a for a in actions}

# 1. The UUIDs are the identity of existing tiles and profiles: renaming must
#    never touch them (issue #80).
check(
    "action UUIDs unchanged",
    set(by_uuid) == set(EXPECTED_NAMES),
    repr(sorted(set(by_uuid) ^ set(EXPECTED_NAMES))),
)

# 2. No action repeats the plugin name as a prefix.
for action in actions:
    uuid, name = action["UUID"], action["Name"]
    check(f"{uuid}: no 'LHM ' prefix", not name.startswith("LHM "), repr(name))
    check(f"{uuid}: name is {EXPECTED_NAMES.get(uuid)!r}", name == EXPECTED_NAMES.get(uuid), repr(name))

# 3. Each property inspector page's <title> matches its action name.
for action in actions:
    pi = action.get("PropertyInspectorPath")
    if not pi:
        continue
    with open(os.path.join(SDPLUGINDIR, pi)) as f:
        html = f.read()
    match = re.search(r"<title>(.*?)</title>", html, re.S)
    check(f"{pi}: has a <title>", match is not None)
    if match:
        check(f"{pi}: <title> matches action name", match.group(1).strip() == action["Name"], repr(match.group(1)))

if failures:
    print(f"\n{len(failures)} failure(s)")
    sys.exit(1)
print("\nall checks passed")
