# Implementation Plan: StreamDeck+ Dial Widget

**Issue:** [#56 - StreamDeck+ Dial Widget](https://github.com/moeilijk/lhm-streamdeck/issues/56)

**Status:** Prototype hardware test passed; release scope is being narrowed to
V1 fixes and clearly separated V2 work.

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

## Current Branch State

The prototype branch implements the first custom Stream Deck+ dial action:

- new `com.moeilijk.lhm.dial` action in the manifest;
- Encoder controller support for Stream Deck+ dial slots;
- `dial_pi.html` and `dial_pi.js` Property Inspector;
- configurable normal-reading pages;
- per-page normal tile styling controls where currently wired;
- active page persistence;
- dial rotation to cycle pages;
- dial press to toggle the overview carousel;
- touch tap mapped to threshold snooze/clear behavior for the active page;
- hidden page updates so graph history continues while pages are not active;
- DeckBridge/probe support for Stream Deck+ protocol validation;
- unit and harness coverage for dial behavior and supporting protocol changes.

The current branch intentionally does not implement Composite Dashboard pages or
Derived Metric pages inside the dial carousel.

DeckBridge still needs a visual correction before overview rendering can be
trusted locally: the emulated Stream Deck+ touch strip must match the real
hardware display shape closely enough to expose aspect-ratio problems before a
hardware tester sees them.

---

## Hardware Test Result

The reporter tested the prerelease build on real hardware:

- Windows 11;
- Stream Deck software 7.4.2 (22730);
- original Stream Deck+ hardware.

Validated behavior:

- preview build installs;
- dial actions can be added;
- pages can be added, reordered, configured, and removed;
- multiple dial actions do not interfere with each other;
- dial rotation cycles pages;
- rotation direction feels intuitive to the tester;
- dial press switches view mode;
- hardware response is prompt;
- no lag, delay, or rendering artifacts were reported;
- touch tap did not cause unexpected behavior in the tester's scenario;
- touchscreen swipe still switches Stream Deck pages as expected.

This satisfies the original hardware blocker for the first simple metric
carousel. Remaining feedback should be triaged as release polish versus follow-up
work.

---

## V1 Release Scope

V1 should remain the simple metric carousel requested by the issue:

- one new custom LHM dial widget action;
- configurable list of normal LHM readings;
- full normal-reading tile presentation settings per page where applicable;
- one selected reading displayed large/readable on the Stream Deck+ touch-panel;
- rotary movement cycles the selected reading;
- no Action Wheel behavior;
- no neighboring page previews in the primary fullscreen display;
- existing Reading, Composite Dashboard, Derived Metric, and Settings actions
  remain unchanged.

V1 may include small polish changes from the hardware feedback only when they are
low risk, localized, and do not introduce new Property Inspector patterns.

V1 must also correct behavior that deviates from the original tile contract:
new dial pages should inherit the same default scale and default appearance as a
normal reading tile unless the user overrides the page styling.

---

## Feedback Triage

### Candidate V1 Polish

These items directly improve the first release without changing the feature's
architecture:

- **Dial press discoverability:** expose that pressing the dial toggles overview,
  using existing Property Inspector or documentation patterns after checking how
  action behavior is documented for normal tiles.
- **Page position indicator:** add an optional indication of active page and
  total page count on the touch strip. This is a user preference, not a new
  default for fullscreen mode. Dial-press overview may show orientation as part
  of its navigation role.
- **Default graph scale:** make newly added dial pages use the same default
  min/max behavior as the normal LHM reading action.
- **DeckBridge touch-strip shape:** fix the local DeckBridge emulation first so
  preview rendering is evaluated against the same shape as a real Stream Deck+
  touch strip.
- **Preview readability:** after DeckBridge is corrected, reduce overview
  preview distortion by fitting the rendered page to the real touch-strip shape
  instead of scaling freely into the current near-square preview cards.
- **Adjacent dial separation:** reserve or draw one pixel column on the left and
  right edge of each dial canvas so adjacent dial actions have a visible
  boundary.
- **Graph history preservation:** avoid rebuilding every dial page graph on any
  page or page-list settings change. Preserve existing page graph history where
  possible, and rebuild only graphs whose visual settings or reading selection
  actually changed.

Before implementation, each candidate must be checked against the existing UI
rules for this repository. The Property Inspector should keep the same
`sdpi-item`, `details`, input, and button patterns already used by comparable
screens.

Default page color rotation is not a V1 candidate. Normal tiles use one default
appearance, while Composite Dashboard uses per-slot defaults only to distinguish
slots inside one composite tile. Dial pages should keep the normal tile defaults
or explicit user-selected styling.

### Follow-Up Work

These items are useful but should not block V1:

- Derived Metric support inside dial pages;
- Composite Dashboard support inside dial pages;
- a bulk page assistant or presets for adding many related metrics;
- making overview carousel the default display mode;
- configurable page indicator style;
- dedicated border/theming controls beyond the V1 one-pixel edge separation;
- broader touch interaction behavior beyond threshold snooze/clear;
- any larger redesign of the dial Property Inspector.

Follow-up issues should be created after the V1 release scope is agreed, so the
main issue can close cleanly when the simple metric carousel ships.

---

## Proposed Execution Plan

### 1. Freeze The V1 Scope

Confirm that issue #56 will ship as the simple metric carousel:

- normal readings only;
- one dial action;
- per-page styling;
- rotation cycles pages;
- dial press overview remains a navigation aid;
- no Derived or Composite pages in V1.

Record the scope decision in the issue before release work starts.

### 2. Correct DeckBridge Visual Emulation

Fix DeckBridge before changing overview rendering:

- identify the real Stream Deck+ touch-strip canvas shape used by the official
  software for encoder feedback;
- update the DeckBridge emulated touch strip to use that shape;
- verify that a local dial action preview shows the same display proportions as
  the real Stream Deck+;
- keep protocol-level dial behavior unchanged while correcting the visual
  geometry.

This step must happen before evaluating or changing carousel preview layout,
because the current emulation can hide the same distortion reported on hardware.

### 3. Implement Only Approved V1 Polish

Apply selected polish in small, reviewable steps:

1. Dial press discoverability:
   - inspect how normal tile action behavior is documented;
   - use the same pattern for dial press behavior, either in the PI or
     documentation;
   - avoid adding a new prominent help pattern for this one action.

2. Page position indicator:
   - add the indicator in render code, not as a separate Stream Deck feedback
     field;
   - add a setting so the fullscreen indicator can be turned on;
   - keep the fullscreen indicator off by default;
   - allow dial-press overview to show orientation as part of the overview
     navigation UI;
   - prefer small dots for low page counts;
   - prefer `x / y` when dots become unreadable;
   - verify it does not cover title/value text in common display modes.

3. Default graph scale:
   - reuse the existing `getDefaultMinMaxForReading` behavior;
   - ensure dial page creation follows the same selection path as the normal
     reading tile where practical;
   - keep older settings compatible.

4. Overview preview readability:
   - run this after DeckBridge visual geometry is fixed;
   - fit the page image to the real touch-strip aspect ratio;
   - avoid near-square preview cards for 2:1 dial canvases;
   - keep fullscreen mode as the primary readable mode.

5. Adjacent dial separation:
   - reserve or draw one pixel column at both horizontal edges;
   - keep it non-configurable for V1;
   - avoid introducing new theming controls.

6. Graph history preservation:
   - stop rebuilding all dial graphs on every settings save;
   - detect page identity and visual-setting changes;
   - preserve graph history for unchanged pages;
   - rebuild only the affected page graphs.

### 4. V2 Planning Notes

These should be tracked separately after V1 is stabilized:

1. Bulk page creation:
   - choose source profile;
   - choose sensor, device, or category;
   - choose rule such as all readings from this sensor, this reading across all
     matching sensors, all CPU cores, all fans, or all memory metrics;
   - preview the resulting pages before adding;
   - allow deselecting individual matches;
   - apply a naming template for generated page titles.

2. Overview as default display mode:
   - this comes from the tester's note that overview could become more than a
     navigation aid;
   - V2 should decide whether overview is a presentation mode or only a
     dial-press navigation mode;
   - do not include this in V1.

3. Derived Metric pages inside the dial carousel.

4. Composite Dashboard pages inside the dial carousel.

### 5. Tests And Verification

Run the automated checks first:

- `go test ./...`;
- `scripts/verify-settings-pi.sh`;
- targeted Node syntax checks for touched Property Inspector files if not already
  covered by the script.

Add focused tests when logic is introduced:

- default min/max propagation for dial page creation or catalog payload;
- pure helper behavior for page indicator thresholds or overview crop selection;
- no regression to existing dial index wrapping tests.

### 6. Local Deploy

After automated checks pass, run:

- `scripts/deploy-local.sh`

This is the final local validation step before asking for a physical Stream Deck
test. No commit, push, release, or tag should happen before this deploy and the
explicit hardware test approval.

### 7. Hardware Approval

Ask the tester or maintainer to validate the locally deployed build on a real
Stream Deck+:

- install and assignment still work;
- rotation still cycles pages;
- dial press overview remains responsive;
- an enabled page indicator is readable and not distracting;
- default scales are sensible for non-percentage readings;
- adjacent dial boundaries are visible but not intrusive;
- graph history is not reset by unrelated page-list edits;
- existing non-dial actions still behave normally.

Only after explicit approval should release preparation continue.

### 8. Release Preparation

Before changing `manifest.json`, tagging, or publishing:

- state the proposed next version;
- compare it with the current latest release;
- justify the version using semantic versioning and the feature scope;
- wait for explicit approval.

After approval:

- commit the final changes;
- push the branch;
- publish the release according to the approved version plan;
- update or close issue #56 depending on release outcome;
- create follow-up issues for all deferred feedback.

---

## Done Criteria

Issue #56 can be closed when:

- the simple metric carousel is released;
- real Stream Deck+ hardware validation has approved the release candidate;
- normal reading pages can be configured and styled independently;
- rotating the dial cycles configured readings;
- the selected reading is readable on the touch strip;
- settings persist;
- existing non-dial LHM actions are not regressed;
- out-of-scope enhancements are tracked separately.
