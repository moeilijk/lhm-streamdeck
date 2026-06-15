# Implementation Plan: StreamDeck+ Dial Widget

**Issue:** [#56 - StreamDeck+ Dial Widget](https://github.com/moeilijk/lhm-streamdeck/issues/56)

**Status:** Prototype hardware test passed; release scope and follow-up work need to be separated before finalizing.

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
- DeckBridge/probe support for Stream Deck+ shape validation;
- unit and harness coverage for dial behavior and supporting protocol changes.

The current branch intentionally does not implement Composite Dashboard pages or
Derived Metric pages inside the dial carousel.

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

---

## Feedback Triage

### Candidate V1 Polish

These items directly improve the first release without changing the feature's
architecture:

- **Dial press discoverability:** expose that pressing the dial toggles overview,
  using the existing Property Inspector layout style.
- **Page position indicator:** add an unobtrusive indication of active page and
  total page count on the touch strip if it does not reduce readability.
- **Default graph scale:** make newly added dial pages use the same default
  min/max behavior as the normal LHM reading action.
- **Preview readability:** reduce overview preview distortion if it can be done
  in the existing overview render code without changing the primary fullscreen
  behavior.
- **Default page colors:** consider cycling through the same kind of small
  hardcoded palette already used by Composite Dashboard slots, if it remains
  predictable and easy to override.

Before implementation, each candidate must be checked against the existing UI
rules for this repository. The Property Inspector should keep the same
`sdpi-item`, `details`, input, and button patterns already used by comparable
screens.

### Follow-Up Work

These items are useful but should not block V1:

- Derived Metric support inside dial pages;
- Composite Dashboard support inside dial pages;
- a bulk page assistant or presets for adding many related metrics;
- making overview carousel the default display mode;
- configurable page indicator style;
- dedicated border/theming controls for adjacent dial displays;
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

### 2. Implement Only Approved V1 Polish

Apply selected polish in small, reviewable steps:

1. Dial press discoverability:
   - add a concise label or static value in the existing Display section;
   - do not add a new help block unless the same pattern already exists nearby.

2. Page position indicator:
   - add the indicator in render code, not as a separate Stream Deck feedback
     field;
   - prefer small dots for up to a modest page count;
   - prefer `x / y` for larger page counts if dots become unreadable;
   - verify it does not cover title/value text in common display modes.

3. Default graph scale:
   - reuse the existing `getDefaultMinMaxForReading` behavior;
   - expose required min/max metadata through the existing catalog payload only
     if the Property Inspector needs it while creating pages;
   - keep older settings compatible.

4. Overview preview readability:
   - keep fullscreen mode as the primary readable mode;
   - adjust only the overview preview scaling/cropping if this improves
     readability without changing the purpose of overview mode.

5. Default page colors:
   - only use a small deterministic palette;
   - keep every color user-editable with existing controls;
   - avoid adding new color controls.

### 3. Tests And Verification

Run the automated checks first:

- `go test ./...`;
- `scripts/verify-settings-pi.sh`;
- targeted Node syntax checks for touched Property Inspector files if not already
  covered by the script.

Add focused tests when logic is introduced:

- default min/max propagation for dial page creation or catalog payload;
- pure helper behavior for page indicator thresholds or overview crop selection;
- no regression to existing dial index wrapping tests.

### 4. Local Deploy

After automated checks pass, run:

- `scripts/deploy-local.sh`

This is the final local validation step before asking for a physical Stream Deck
test. No commit, push, release, or tag should happen before this deploy and the
explicit hardware test approval.

### 5. Hardware Approval

Ask the tester or maintainer to validate the locally deployed build on a real
Stream Deck+:

- install and assignment still work;
- rotation still cycles pages;
- dial press overview remains responsive;
- any new page indicator is readable and not distracting;
- default scales are sensible for non-percentage readings;
- existing non-dial actions still behave normally.

Only after explicit approval should release preparation continue.

### 6. Release Preparation

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
