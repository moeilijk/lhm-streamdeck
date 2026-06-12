# Implementation Plan: StreamDeck+ Dial Widget

**Issue:** [#56 - StreamDeck+ Dial Widget](https://github.com/moeilijk/lhm-streamdeck/issues/56)

**Status:** Prototype branch; real Stream Deck+ validation required.

---

## Issue Request

The requested feature is a custom LHM Stream Deck+ dial widget.

The reporter describes a Stream Deck+ profile page where normal LHM buttons work
well for system metrics, but the dial section is unused. The built-in Stream
Deck Action Wheel can approximate a metric carousel, but it shrinks the display
and shows neighboring pages darkened in the background, making metrics hard to
read.

The requested widget should:

- configure a list of metrics;
- display one metric at a time, fullscreen/readable, like a normal LHM button;
- configure the displayed metric with normal tile styling controls per page;
- cycle through metrics by turning the dial;
- avoid Action Wheel shrinking and neighboring-page previews;
- use the wider Stream Deck+ touch-panel area;
- support use cases such as cycling fan speeds, CPU core percentages, or
  RAM/VRAM metrics on one dial.

The issue mentions Composite Dashboard and Derived Metric pages as a possible
"perfect world" extension, while also noting that this would likely be more
complicated.

---

## V1 Scope

Version 1 should implement the simple metric carousel requested by the issue:

- one new custom LHM dial widget action;
- configurable list of normal LHM readings;
- full normal-reading tile presentation settings per page where applicable;
- one selected reading displayed large/readable on the Stream Deck+ touch-panel;
- rotary movement cycles the selected reading;
- no Action Wheel behavior;
- no neighboring page previews;
- existing Reading, Composite Dashboard, Derived Metric, and Settings actions
  remain unchanged.

Out of scope for V1 unless hardware testing shows it is required immediately:

- Composite Dashboard pages inside the dial carousel;
- Derived Metric pages inside the dial carousel;
- a general-purpose action wheel replacement;
- changing existing non-dial actions.

---

## Technical Implementation

The Stream Deck+ dial widget should be implemented as a Stream Deck+ dial/encoder
compatible action so it is assignable to the dial section in the official Stream
Deck software.

Required behavior:

- manifest declares the dial widget as Stream Deck+ dial/encoder compatible;
- Property Inspector manages the metric page list;
- Property Inspector manages normal tile styling per page;
- settings persist the page list, active page, and per-page styling;
- dial rotation changes the active page by the received tick count;
- display output renders the selected metric using the Stream Deck+ display area;
- rotary input is treated as input only, not as a display surface.

DeckBridge emulation must mirror the requested Stream Deck+ shape closely enough
for compatibility testing:

- 4 x 2 key grid;
- 4 dial slots;
- separate touch-strip display area;
- separate rotary input controls;
- no display rendered on the rotary itself;
- Encoder/dial actions only on valid dial slots;
- Keypad actions only on normal keys;
- stale invalid profile assignments are rejected, removed, or hidden.

---

## Validation Plan

Local validation:

- plugin builds;
- Stream Deck manifest validation passes;
- dial widget can be assigned in DeckBridge to valid dial slots only;
- rotating left/right cycles through configured readings;
- each page can be styled independently like a normal reading tile;
- display stays readable and does not flicker to fallback text;
- existing non-dial LHM actions still work.

Hardware validation:

- official Stream Deck software shows the widget in the Stream Deck+ dial section;
- assigning the action to a real dial works;
- rotation direction matches user expectation;
- multi-detent rotation behaves predictably;
- the displayed metric is readable and uses the touch-panel area better than
  Action Wheel;
- settings persist after restarting Stream Deck;
- existing non-dial LHM actions remain unchanged.

---

## Done Criteria

Issue #56 is complete only after real Stream Deck+ hardware confirms:

- install and assignment work in official Stream Deck software;
- rotating the dial cycles configured metrics;
- each metric page can be styled independently;
- the display is readable and avoids Action Wheel shrinking/neighbor previews;
- settings persist;
- existing LHM actions are not regressed.
