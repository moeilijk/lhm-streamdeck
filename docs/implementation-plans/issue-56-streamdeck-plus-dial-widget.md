# Issue 56 Stream Deck+ Dial Widget Plan

## Requested Result

- Build a custom LHM Stream Deck+ dial widget.
- Configure a list of metrics.
- Show one metric at a time, large/readable, without Action Wheel neighboring-page previews.
- Rotate the dial to cycle metrics.
- Use the Stream Deck+ touch-panel area for the display.
- Keep existing non-dial LHM actions working.

## Current Implementation Target

- `com.moeilijk.lhm.dial` is the first dial widget action.
- It is assigned only to Stream Deck+ dial/encoder slots.
- The touch-panel segment displays the selected metric.
- The rotary control only changes the selected metric; it is not treated as a display.
- First version supports normal LHM readings.
- Composite Dashboard and Derived Metric pages are follow-up work unless hardware testing proves they are required immediately.

## Work Items

1. Keep DeckBridge emulation aligned with Stream Deck+ shape:
   - 4 x 2 key grid;
   - 4 dial/encoder slots;
   - separate touch-strip display row;
   - separate rotary input controls;
   - no visual display on the rotary itself.
2. Reject incompatible assignments:
   - Encoder actions only on valid dial slots;
   - Keypad actions only on keys;
   - stale invalid profile slots are removed or hidden.
3. Validate plugin behavior in DeckBridge:
   - dial widget can be assigned to each dial;
   - rotate cycles pages;
   - display stays readable and does not flicker to fallback text;
   - normal actions still work.
4. Hardware validation:
   - official Stream Deck software shows the action in the dial section;
   - assigning to a real Stream Deck+ dial works;
   - rotation direction and display readability are accepted by a hardware tester;
   - settings persist after Stream Deck restart.

## Done Criteria

The issue is not complete until real Stream Deck+ hardware confirms the widget is readable, assignable, and rotates through configured metrics as requested.
