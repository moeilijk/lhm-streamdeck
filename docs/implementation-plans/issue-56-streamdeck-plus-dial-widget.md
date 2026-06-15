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

## Feedback Ledger

This ledger is the stable numbered source of truth for issue #56 follow-up. It
combines the original issue request, the reporter's hardware-test feedback, and
owner triage decisions made during follow-up planning.

### Raw Issue Extraction

This extraction lists each distinct point from the issue body and comments before
triage. It exists to prevent source feedback from disappearing into summaries.

1. Reporter uses LHM and the plugin successfully.
2. Reporter likes the Composite Dashboard action for combining related metrics.
3. Reporter has a Stream Deck+ profile page dedicated to system metrics.
4. Reporter sees the Stream Deck+ dial section as unused/wasted by the current
   plugin.
5. Requested widget: configure a list of metrics.
6. Requested widget: display one metric fullscreen instead of stacking metrics.
7. Requested widget: use a display style like the normal LHM button.
8. Requested widget: turn the dial to cycle through the configured metrics.
9. Example use case: cycle through all fan speeds on one dial slot.
10. Example use case: cycle through duty percentages for individual CPU cores.
11. Example use case: create a memory dial cycling RAM and VRAM metrics.
12. Perfect-world extension: allow each dial page to be a Composite Dashboard.
13. Perfect-world extension: allow each dial page to be a Derived Metric.
14. Reporter expects Composite/Derived dial pages to be more complicated.
15. Existing workaround: put normal LHM buttons inside Stream Deck Action Wheel.
16. Action Wheel problem: it shrinks the display.
17. Action Wheel problem: it shows neighboring pages darkened in the background.
18. Action Wheel problem: this makes readings hard to read.
19. Custom widget goal: avoid Action Wheel shrinking.
20. Custom widget goal: use the wider Stream Deck+ touch panel.
21. Reporter offered to test on real Stream Deck+ hardware.
22. Reporter has limited Go/JavaScript experience and does not expect to
    contribute much code.
23. Maintainer response: without hardware access, the feature needed external
    hardware validation.
24. Maintainer V1 proposal: one custom LHM dial action.
25. Maintainer V1 proposal: configurable list of normal readings.
26. Maintainer V1 proposal: turning the dial cycles the active reading.
27. Maintainer V1 proposal: touch display renders one reading fullscreen using
    existing normal tile style where possible.
28. Maintainer V1 non-goal: avoid Composite Dashboard pages in first version.
29. Maintainer V1 non-goal: avoid Derived Metric pages in first version.
30. Reporter suggested planning multiple versions from the start.
31. Reporter suggested a "Simple Metric Carousel" dial action.
32. Reporter suggested a later "Extended Carousel" if feasible and worthwhile.
33. Reporter noted a new action avoids backwards compatibility concerns.
34. Reporter noted a new action allows different internal architecture later.
35. Prototype branch existed before hardware validation.
36. Prototype still required real Stream Deck+ validation before being marked
    implemented.
37. Hardware test build was published as prerelease
    `issue-56-streamdeck-plus-test-20260614-0225`.
38. Hardware test artifact was
    `com.moeilijk.lhm.issue-56-hardware-test.streamDeckPlugin`.
39. Hardware test scope: validate first Stream Deck+ Dial Carousel on real
    hardware.
40. Hardware test scope: confirm dial press behavior.
41. Hardware test scope: confirm dial rotate behavior.
42. Hardware test scope: confirm touch behavior.
43. Hardware test scope: confirm overview carousel behavior.
44. Hardware test scope: confirm hidden-page updates.
45. Maintainer stated successful hardware test alone does not close #56.
46. Maintainer stated Derived Metric support should become separate follow-up.
47. Maintainer stated Composite Dashboard support should become separate
    follow-up.
48. Reporter created a new profile for hardware testing.
49. Reporter uninstalled the official release before installing the preview.
50. Reporter installed the preview build.
51. Hardware result: adding dial actions worked.
52. Hardware result: adding pages worked.
53. Hardware result: reordering pages worked.
54. Hardware result: configuring pages worked.
55. Hardware result: multiple dials did not interfere with each other.
56. Hardware result: hardware interaction had no obvious problems.
57. Hardware result: inputs responded promptly.
58. Hardware result: no lag/delay was observed.
59. Hardware result: no rendering artifacts were observed.
60. Hardware result: turning the dial cycles pages.
61. Hardware result: dial rotation direction felt intuitive to the tester.
62. Hardware result: pressing the knob switches view mode.
63. Hardware result: tapping the touchscreen did not appear to do anything.
64. Hardware result: swiping the touchscreen still switches Stream Deck pages.
65. Feedback: dial press view-mode switching is not advertised in the UI.
66. Feedback: two adjacent carousels have no visible border.
67. Feedback: missing border makes it difficult to see where one graph ends and
    the other begins.
68. Feedback: add a thin border on both sides.
69. Feedback: font sizes in the UI are set to 0 by default.
70. Feedback: rendered text appears around font size 14 despite UI value 0.
71. Feedback: if 0 means automatic size, that is not obvious.
72. Feedback: automatic font size should be communicated better.
73. Feedback: fullscreen view has no active page/page count indication.
74. Feedback: carousel view has no active page/page count indication.
75. Feedback: fullscreen gives no indication that more pages exist.
76. Feedback: suggested row of gray dots at the bottom.
77. Feedback: suggested active dot slightly brighter.
78. Feedback: suggested active dot stretched sideways.
79. Feedback: for many pages, replace dots with explicit `x / y` text.
80. Feedback: tester was unsure whether dot/text switching should be automatic
    or manual.
81. Feedback: Action Wheel normally shows 9 points before making outer ones
    smaller.
82. Feedback: tester would set dot/text threshold around 9 pages.
83. Feedback: carousel previews are distorted.
84. Feedback: distorted previews are barely readable.
85. Feedback: carousel view is probably intended as navigation.
86. Feedback: users might want carousel view to show 3 pages at the same time.
87. Feedback: reduce preview height slightly while keeping width.
88. Feedback: reducing preview height would bring aspect ratio closer to the
    original.
89. Feedback: maybe crop the sides of the preview image.
90. Feedback: cropping sides may reduce distortion and improve readability.
91. Feedback: add option to select carousel/overview as default view.
92. Feedback: default view option would promote overview from navigation tool to
    alternative view mode.
93. Feedback: graphs appear hardcoded to default scale 0-100.
94. Feedback: normal LHM action appears to calculate default scale from selected
    reading.
95. Feedback: maybe each page could get a different default graph color.
96. Feedback: suggested hardcoded list of around 5 colors with wrap-around.
97. Feedback: color variation could help navigating the list.
98. Feedback: page/page-list changes make the UI-selected page display on the
    device.
99. Feedback: tester found selected page display unexpected but understandable.
100. Feedback: page/page-list changes reset all graphs.
101. Feedback: user expectation is that graphs persist while cycling pages.
102. Feedback: graph reset is a minor annoyance while setting graph scale.
103. Feedback: adding many pages is tedious.
104. Example tedious case: frequency for all 8 cores.
105. Example tedious case: duty cycle for all 16 virtual cores.
106. Feedback: maybe presets or assistant can add pages by rule.
107. Suggested bulk rule: reading X of all cores of the selected processor.
108. Suggested bulk rule: all readings from the selected sensor.
109. Hardware environment: Windows 11.
110. Hardware environment: Stream Deck software 7.4.2 (22730).
111. Hardware environment: original Stream Deck+ hardware.
112. Maintainer follow-up: hardware validation confirmed core behavior.
113. Maintainer follow-up: V1 remains simple metric carousel for normal LHM
     readings.
114. Maintainer follow-up: Derived Metric pages remain out of V1.
115. Maintainer follow-up: Composite Dashboard pages remain out of V1.
116. Maintainer follow-up: hardware feedback must be triaged into V1 polish or
     follow-up issues.
117. Maintainer follow-up: no release/version changes until local deploy and
     explicit hardware approval.
118. DeckBridge follow-up: touch strip emulation needed correction to match real
     Stream Deck+ shape.
119. DeckBridge follow-up: dashboard persistence needed fixing because state was
     not remembered reliably during testing.
120. Owner triage: page indicator is a switchable wish, not a default fullscreen
     requirement.
121. Owner triage: one tester does not define the end goal.
122. Owner triage: default colors must not be invented or agreed without proof.
123. Owner triage: same measurement should keep the same color.
124. Owner triage: per-new-page color rotation is not an existing theme rule.
125. Owner triage: adjacent dials need one pixel column removed/reserved on both
     left and right edges.
126. Owner triage: bulk page creation belongs to V2 planning.
127. Owner triage: some unclear feedback items must be explained from the issue
     feedback before execution.
128. Owner triage: items 9 and 10 from the earlier working list are parked for
     V2.
129. Owner triage: item 11 from the earlier working list is V1.
130. Owner workflow: stop implementing before the feedback extraction and plan
     are complete and accepted.

### Triage Mapping

1. **Create one custom LHM Stream Deck+ dial action.**
   - Source: original issue request and first maintainer scope reply.
   - Scope: V1.
   - Plan: keep one `LHM Dial Carousel` action for V1.
   - Motivation: this avoids changing existing Reading, Composite Dashboard, or
     Derived Metric actions and avoids migration/backwards-compatibility work.

2. **Support a configurable list of normal LHM readings.**
   - Source: original issue request.
   - Scope: V1.
   - Plan: pages in the dial action are normal reading pages only.
   - Motivation: this is the smallest implementation that satisfies the primary
     request and was the hardware-test target.

3. **Display one selected reading fullscreen/readable on the touch strip.**
   - Source: original issue request; Action Wheel comparison.
   - Scope: V1.
   - Plan: fullscreen remains the primary display mode.
   - Motivation: the issue exists because Action Wheel shrinks readings and
     makes them hard to read.

4. **Cycle readings by rotating the dial.**
   - Source: original issue request and hardware validation feedback.
   - Scope: V1.
   - Plan: dial rotation cycles the active page.
   - Motivation: this is core hardware behavior and was reported as intuitive in
     testing.

5. **Keep Composite Dashboard pages out of V1.**
   - Source: original issue "perfect world" note and maintainer scope replies.
   - Scope: V2.
   - Plan: create a follow-up issue after V1 if still desired.
   - Motivation: Composite pages would expand the data model and rendering
     contract beyond the simple metric carousel.

6. **Keep Derived Metric pages out of V1.**
   - Source: original issue "perfect world" note and maintainer scope replies.
   - Scope: V2.
   - Plan: create a follow-up issue after V1 if still desired.
   - Motivation: Derived pages require separate selection, formula, and graph
     state handling.

7. **Plan versions explicitly: simple first, extended later.**
   - Source: reporter follow-up suggesting a "Simple Metric Carousel" and later
     "Extended Carousel".
   - Scope: planning rule.
   - Plan: V1 is the simple metric carousel; V2 tracks larger extensions.
   - Motivation: separate actions/versions avoid compatibility and migration
     risk.

8. **Use real Stream Deck+ hardware validation before release.**
   - Source: reporter offer to test and maintainer validation blocker.
   - Scope: release gate.
   - Plan: do not mark implemented or release final without hardware approval.
   - Motivation: the maintainer does not have the hardware locally.

9. **Dial press toggles view mode.**
   - Source: hardware test result confirmed pressing the knob switches
     view-mode.
   - Scope: V1 behavior.
   - Plan: keep dial press as the overview toggle.
   - Motivation: hardware validation reported the behavior working.

10. **Dial press discoverability is missing.**
    - Source: hardware feedback: dial press switches view mode but is not
      advertised in the UI.
    - Scope: V1.
    - Plan: inspect existing PI/documentation patterns, then document the press
      behavior using the same pattern. Do not invent a new default-open help
      block for this action.
    - Motivation: users cannot discover a hidden press action from the current
      UI.

11. **Touch tap should not become an unsolicited default goal.**
    - Source: hardware feedback observed touch tap did nothing; owner triage
      says user-requested extras may be supported but must be scoped by the
      maintainer.
    - Scope: V1 only if kept as explicit/safe behavior.
    - Plan: keep any touch/snooze behavior controlled by existing threshold
      semantics and document or gate it if it is exposed.
    - Motivation: one tester's preference must not redefine the release goal.

12. **Touchscreen swipe must continue switching Stream Deck pages.**
    - Source: hardware test result.
    - Scope: regression guard.
    - Plan: do not consume swipe gestures in the plugin.
    - Motivation: hardware test confirmed expected Stream Deck page navigation.

13. **Adjacent dial carousels need a visible boundary.**
    - Source: hardware feedback: two carousels next to each other have no
      visible border.
    - Scope: V1.
    - Plan: reserve or draw one pixel column on the left and right edge of each
      dial canvas. Keep it non-configurable in V1.
    - Motivation: owner triage selected the exact V1 shape: one pixel column on
      both sides, not a broader theme system.

14. **Font-size controls showing `0` are confusing.**
    - Source: hardware feedback: UI shows font size 0 while text appears around
      14 px.
    - Scope: V1.
    - Plan: make automatic font size clear using an existing PI pattern.
    - Motivation: `0` is currently an implementation sentinel for automatic size
      but reads as an actual size to users.

15. **Page position indicator is missing.**
    - Source: hardware feedback: fullscreen and carousel view do not show page
      position or page count.
    - Scope: V1 as optional behavior only.
    - Plan: add a switchable indicator. Fullscreen indicator stays off by
      default. Overview may show orientation as part of its navigation role.
    - Motivation: owner triage rejected making one tester's preference the
      default end state while still allowing the requested affordance.

16. **Page indicator visual form should start from dots, then `x / y` when
    dots become unreadable.**
    - Source: hardware feedback suggested Action Wheel-like dots and `x / y`
      for many pages.
    - Scope: V1 if page indicator is implemented.
    - Plan: use compact dots for low page counts and text when dots would not
      fit/read clearly.
    - Motivation: indicator must not cover title/value text or become clutter.

17. **DeckBridge must emulate the real Stream Deck+ touch-strip shape before
    judging preview rendering.**
    - Source: owner triage and hardware distortion feedback.
    - Scope: V1 support work outside this repo.
    - Plan: use the DeckBridge Stream Deck+ shape fix as the local validation
      baseline before tuning overview rendering.
    - Motivation: the old emulation could hide real hardware aspect-ratio
      problems.

18. **Overview preview distortion must be reduced after DeckBridge shape is
    corrected.**
    - Source: hardware feedback: carousel previews are distorted and barely
      readable.
    - Scope: V1.
    - Plan: fit/crop page previews against the real touch-strip aspect ratio
      instead of freely scaling near-square cards.
    - Motivation: overview is a navigation aid, but it should not make previews
      misleading or unreadable.

19. **Overview as default display mode is not V1.**
    - Source: hardware feedback suggested an option to make carousel view the
      default; owner triage moved this to V2.
    - Scope: V2.
    - Plan: track separately after V1.
    - Motivation: default overview changes the primary display contract.

20. **Default graph scale must be derived from the original normal tile logic.**
    - Source: hardware feedback: dial graphs look hardcoded to 0-100 while the
      normal LHM action derives the default value from the selected reading.
    - Scope: V1.
    - Plan: reuse `getDefaultMinMaxForReading` for newly added dial pages and
      reading changes.
    - Motivation: V1 must preserve the normal tile contract for the same
      reading instead of hardcoding dial-specific defaults.

21. **Do not add rotating hardcoded default colors per dial page in V1.**
    - Source: hardware feedback suggested different default graph colors; owner
      triage rejected hardcoded color rotation.
    - Scope: not V1.
    - Plan: keep normal tile defaults unless the user explicitly changes page
      styling.
    - Motivation: the existing theme rule is that the same measurement keeps
      the same color; Composite per-slot colors are not a precedent for dial
      page color rotation.

22. **Verify exact default colors rather than assuming them.**
    - Source: owner triage asked whether defaults actually exist and rejected
      agreeing without proof.
    - Scope: V1 verification.
    - Plan: compare dial page defaults against the normal Reading action defaults
      and align any differences.
    - Motivation: V1 should inherit the original tile appearance, not a copied
      approximation.

23. **Changing page selection may update the device display.**
    - Source: hardware feedback: selecting a page in the UI becomes the page
      displayed on-device; tester called it unexpected but understandable.
    - Scope: acceptable V1 behavior unless it causes another issue.
    - Plan: keep current behavior unless implementation work reveals a
      regression.
    - Motivation: direct preview of selected page is useful during setup and was
      not raised as a blocker.

24. **Graph history must not reset on every page or settings change.**
    - Source: hardware feedback: all graphs reset when changing a page/page
      list.
    - Scope: V1.
    - Plan: preserve graph state for unchanged pages and rebuild only pages
      whose reading identity or visual graph settings changed.
    - Motivation: users expect graph history to persist while cycling through
      configured pages.

25. **Bulk page creation is tedious for many cores/fans.**
    - Source: hardware feedback: adding 8/16+ pages is tedious.
    - Scope: V2.
    - Plan: design a bulk helper separately.
    - Motivation: useful but larger than release polish.

26. **Bulk helper should support rule-based selection.**
    - Source: hardware feedback examples: all cores, all readings from selected
      sensor, all matching readings.
    - Scope: V2.
    - Plan: include rule selection, preview, deselection, and naming template in
      V2 design.
    - Motivation: bulk creation must be controllable to avoid generating noisy
      pages.

27. **DeckBridge persistence must be reliable for testing.**
    - Source: owner feedback during follow-up testing.
    - Scope: V1 support work outside this repo.
    - Plan: use the DeckBridge persistence fix before further local dashboard
      validation.
    - Motivation: if DeckBridge forgets state, local validation cannot be
      trusted.

28. **No release/version change before local deploy and hardware approval.**
    - Source: repository instructions and issue plan.
    - Scope: release gate.
    - Plan: run automated checks, then `scripts/deploy-local.sh`, then wait for
      explicit hardware-test approval before commit/push/release code changes.
    - Motivation: this repo's release flow requires physical Stream Deck
      validation for code changes.

29. **V1 implementation must stay within existing PI patterns.**
    - Source: repository instructions and plan.
    - Scope: implementation constraint.
    - Plan: use existing `sdpi-item`, `details`, input, checkbox, range, and
      button patterns only.
    - Motivation: avoid introducing new UI conventions in this repo for one
      action.

30. **No unapproved code work before the feedback ledger and V1 plan are
    accepted.**
    - Source: owner instruction during follow-up.
    - Scope: workflow rule.
    - Plan: treat this ledger as the stop point before more implementation.
    - Motivation: previous work skipped the requested planning checkpoint.

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
