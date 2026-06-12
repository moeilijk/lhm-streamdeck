# Issue 56: StreamDeck+ Dial Widget

**Issue:** [#56 - StreamDeck+ Dial Widget](https://github.com/moeilijk/lhm-streamdeck/issues/56)

**Status:** Prototype branch only. This file tracks the requested behavior,
acceptance criteria, and hardware validation checklist for the feature branch.

## Source Request

The reporter has a Stream Deck+ profile page dedicated to system metrics where
the dial section is unused. They ask for an LHM custom dial widget because the
built-in Stream Deck Action Wheel can already approximate the workflow, but it
shrinks the display and shows neighboring pages darkened in the background,
making the metrics hard to read.

The requested widget behavior is:

- configure a list of metrics, similar to Composite Dashboard configuration;
- display only one metric at a time, fullscreen, like a normal LHM button;
- turn the dial to cycle through the configured metrics;
- use the wider Stream Deck+ touch-panel space better than Action Wheel does;
- allow use cases such as cycling fan speeds, CPU core duty percentages, or
  RAM/VRAM memory metrics on one dial.

The issue also mentions a "perfect world" extension where each page could be a
Composite Dashboard or Derived Metric, but explicitly notes that this is probably
more complicated.

## Acceptance Criteria From The Issue

- The feature must be a custom LHM Dial Widget, not a wrapper around Action Wheel.
- It must provide a configurable metric list.
- It must show one selected metric clearly and fullscreen in the dial display
  area, without neighboring pages or Action Wheel-style shrinking.
- Rotating the dial must cycle the selected metric.
- The first implementation may limit pages to normal LHM readings.
- Composite Dashboard and Derived Metric pages remain a later extension unless
  the issue owner confirms they are required for the first version.
- Real Stream Deck+ hardware validation is required before the issue can be
  treated as implemented.

## Technical Mapping To Verify

This section is not scope by itself. It is the current technical interpretation
that must be validated against official Stream Deck software and real
Stream Deck+ hardware.

- Represent the widget as a Stream Deck+ `Encoder` action so it can be assigned
  to the dial section.
- Declare the action with `Controllers: ["Encoder"]`.
- Use dial input events for cycling:
  - `dialRotate` for rotary movement;
  - `dialDown` and `dialUp` for press input if needed;
  - `touchTap` only if a touch workflow is added.
- Use encoder feedback APIs for display output:
  - `setFeedbackLayout`;
  - `setFeedback`.
- The current prototype uses `$A0.full-canvas` to render one metric image. This
  is acceptable only if real hardware shows it as a readable fullscreen metric
  in the Stream Deck+ dial display area.

## Prototype State

Implemented on the feature branch:

- new action `com.moeilijk.lhm.dial`;
- Encoder-only manifest entry;
- configurable normal-reading pages in `dial_pi.html` / `dial_pi.js`;
- dial rotation updates the active page by signed tick count;
- rendering sends a metric image through encoder feedback;
- DeckBridge can emulate the Stream Deck+ profile for local compatibility
  testing.

This is not final acceptance. It only means there is a build ready for hardware
validation.

## Hardware Validation Checklist

Ask the issue reporter or another Stream Deck+ owner to verify:

- plugin installs and the action appears in the official Stream Deck software;
- the action can be assigned to the Stream Deck+ dial section;
- it does not appear as a normal keypad tile action on incompatible hardware;
- the metric list can be configured and persists after restart;
- rotating left/right cycles through metrics in the expected direction;
- multi-detent rotation behaves predictably;
- the displayed metric is readable and uses the touch-panel area better than
  Action Wheel;
- the implementation does not break existing LHM button, Composite Dashboard,
  Derived Metric, or Settings actions.

## Open Risks

- The `$A0.full-canvas` mapping may not match the visual result expected by the
  issue reporter on real Stream Deck+ hardware.
- The exact Stream Deck+ UI affordance in the official software may differ from
  DeckBridge's emulation.
- Composite Dashboard and Derived Metric page support may become required after
  the reporter tests the simple carousel.
