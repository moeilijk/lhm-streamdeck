# Issue #56 — Dial Carousel: Werklijst V1

Korte, uitvoerbare werklijst voor #56. Het volledige triage-archief staat in
[issue-56-streamdeck-plus-dial-widget.md](issue-56-streamdeck-plus-dial-widget.md).
Dit bestand is de leidende werklijst; dat bestand is alleen referentie.

## Validatie-uitgangspunt

**Geen enkel feedback-punt geldt als opgelost tot het end-to-end is gevalideerd:**
lokale deploy (`scripts/deploy-local.sh`) + expliciet hardware-akkoord ná de
betreffende wijziging. Een gemergede commit of "code is aanwezig" telt **niet**
als opgelost.

**Scope-model:** de opdeling V1–Vxx zijn **emu-test blokken** — brokken die we
één voor één bouwen en in de emu (DeckBridge) valideren en laten goedkeuren. Het
is samen **één basis-deliverable**, geen losse hardware-releases.

**Build/source-marker discipline:** elke wijziging die in de emu getest moet
worden, moet zichtbaar traceerbaar zijn. Bij **DeckBridge**-wijzigingen altijd de
zichtbare `BUILD relurl-####` verhogen en met een DeckBridge-test borgen. Bij
**plugin**-wijzigingen altijd de zichtbare PI-rij `Build` (`pluginBuildRef`)
bijwerken en met de PI functional test borgen. Geen emu-test laten uitvoeren op
een build waarvan de zichtbare marker niet bij de actuele diff hoort.

**De hardware-test komt pas ná emu-akkoord op _alles_.** De HW-test is extern en
buiten onze controle, dus eenmalig: we sturen er één complete build heen zodra
álle emu-blokken zijn goedgekeurd. Geen per-block hardware-test.

**KEIHARDE EIS — emu = hardware.** De DeckBridge-emu moet zich identiek gedragen
en tonen als de echte Stream Deck+, anders is hij waardeloos voor validatie. Elke
divergentie tussen emu en hardware is een **V0-blocker**: eerst de emu kloppend
maken, daarna pas plugin-werk erin valideren. Concreet: geen emu-cosmetica die de
hardware verkeerd voorstelt (bv. randen die er op HW niet zijn) en identiek event-/
render-gedrag (bv. een net-toegewezen dial moet direct de plugin-render tonen,
net als op HW).

**Gevonden emu-divergenties (V0-blockers) — OPGELOST in `relurl-1767`:**

1. **Touch-strip-randen waren emu-cosmetica.** De emu tekende een 1px-rand per
   dial-segment plus een net-niet-zwarte segment-achtergrond (`#050606`); echte HW
   is één doorlopend 800×100-LCD zónder rand. **Fix (relurl-1768):**
   `border-left/right` op `.dial-display` verwijderd én achtergrond → `#000`, strip
   is nu volledig naadloos zoals HW. Enige resterende markering is de blauwe
   selectie-highlight (emu-only editing-affordance). (Het ontbreken-van-scheiding
   is nu zichtbaar in de emu — precies de reporter-klacht; bewijst de noodzaak van
   onze eigen 1px-kolommen, plugin-item 4.)
2. **Net-toegewezen dial rendert de plugin-state niet.** Root-cause gevonden: de
   1s-poll `refreshLiveState` verfriste `/api/state` (waarin de feedback-tekst
   title/value zit) **alleen als de PI dicht was**; tijdens configureren bleef de
   actie-naam-fallback staan tot een Refresh. **Fix:** poll ververst nu altijd
   state + `renderDeck()` (dial-feedback live, ook met open PI); alleen het
   inspector-formulier wordt overgeslagen bij open PI om edits niet te clobberen.
   **Gebruiker-bevestigd:** geen handmatige refresh meer nodig; render verschijnt
   vanzelf binnen ~1s (poll-cadans). Op HW ~direct → resterende ~1s is een kleine
   setup-timing-afwijking, geen gedrags-divergentie. Acceptabel voor V0; eventueel
   later te verkleinen door poll-interval of een render-trigger na assign.

**Buiten scope tot de basis HW-akkoord heeft:** Derived Metric- en Composite
Dashboard-pagina's. Die zijn een latere ronde, ná goedkeuring van de
basisfunctionaliteit op hardware.

Concreet voor deze branch:

- De hardware-test van 14 juni dekte alleen het **oude prototype**. Alle code-
  en DeckBridge-wijzigingen daarna zijn nog **onbevestigd**.
- DeckBridge-fixes (`706c0c1` vorm, `8fb6320` persistence) bestaan in de
  DeckBridge-branch en de lokale server draait die build, **maar** dat de
  gerapporteerde distortie- en persistence-klacht daadwerkelijk verholpen is, is
  **niet bevestigd**.
- Daarom staat hieronder **alles** als open/te-valideren, inclusief de punten die
  eerder als "gedekt" of "geïmplementeerd" werden afgevinkt.

Basis-doel is de **metric-carousel** voor normale LHM-readings: fullscreen
leesbaar, rotatie cyclet, multi-preview overview. Geen Composite/Derived pagina's
in deze ronde.

## Kernprincipe: tile-pariteit = hergebruik, geen heruitvinding

**Harde eis (ook voor derived & composite later):** hergebruik de bestaande,
goedgekeurde/released **tile-controls verbatim** (zelfde markup, ranges, defaults,
gedeelde `pi_utils.js`-helpers). Codex' parallelle, afwijkende controls in
`dial_pi.*` (en straks `derived_pi`/`composite_pi`) zijn **hallucinaties die eruit
moeten**. **Alleen afwijken waar de dial/touch-hardware het echt vereist**
(pagina-carousel, rotate, touch, strip-canvas-formaat). Waar de dial-render een
veld nog niet honoreert, hergebruik de **bestaande** tile-render-logica (bv.
`plugin.go` EMA) i.p.v. iets nieuws te schrijven.

Voorbeeld (gat A, gedaan): font-control = tile-slider-patroon (range-wrap + step
0.5 + gedeelde `positionRangeVal`). Dial-aanpassingen (toegestaan, canvas is
breder/hoger dan een tile-key): range **8–30** i.p.v. tile 8–20, defaults 14/18
i.p.v. 10.5. Codex' number-input met `0=auto` is verwijderd.

Voorbeeld (gat B, gedaan): Smoothing = tile-control verbatim (0.1–1, 1 = uit) +
de **bestaande** EMA uit `plugin.go` toegepast in `dial.go` (keyed op `pageCtx`,
gedeelde `smoothedValues`). "Update every" weggelaten — de dial draait op één
tick, dus per-pagina-interval is een dial-eigenschap, geen parity-gat. Wiring
gemeten (smoothingAlpha round-trip PI→Go→persist→PI). → **Stap 3 (Display-
pariteit) compleet; nog niet gecommit (wacht op emu-check).**

## Kernprincipe: dial-pagina = tile, per pagina

Een dial-pagina hoort **dezelfde configuratie te hebben als een normale tile**
(de Reading-actie / `index_pi.html`), alleen herhaald per pagina met de dial-paging
eroverheen. Codex heeft i.p.v. dat een **styling-only** dial-PI gebouwd: alleen
Display + Scale, zonder de alert-/interactie-secties.

**Pariteitskloof (tile-secties die per dial-pagina ontbreken):**

| Tile-PI-sectie | Dial-pagina | Gevolg van ontbreken |
|----------------|-------------|----------------------|
| Source (sensor/reading) | ✅ aanwezig | — |
| Display (kleur/font/graph) | ✅ grotendeels | pariteit verifiëren |
| Scale (min/max, format, divisor, unit) | ✅ aanwezig | default-scale wijkt af (item 1) |
| Timing & Smoothing | ❌ | geen per-pagina update/EMA — te bepalen of per pagina of globaal |
| **Alert Snooze** (snooze-duren) | ❌ | **touch werkt niet** (item B) |
| **Thresholds** (per pagina) | ❌ | geen per-pagina alerts instelbaar |
| **Global Thresholds** (suppressie) | ❌ | `SuppressedGlobalIDs` bestaat in Go, geen UI |

Door dit principe **subsumeren** verschillende losse items: bij echte tile-pariteit
komen default-scale (1), font `0=auto` (6), touch/snooze (B) en thresholds vanzelf
mee uit het bestaande tile-gedrag, i.p.v. los te worden gepatcht. Per-pagina
default-kleur (5) en de dial-eigen onderdelen (paging, indicator, edge, overview)
blijven aanvullingen bovenop de tile-pariteit.

## Statusoverzicht (alles = te valideren tot bewezen)

| # | Onderwerp | Code-staat | Validatie-staat |
|---|-----------|-----------|-----------------|
| A | Kern: actie toevoegen, pagina's beheren, rotatie, dial-press | aanwezig, maar gewijzigd na 14-juni-test | **her-valideren** op hardware |
| B | Touch = tile-druk (snooze/clear) | aanwezig (`OnTouchTap`) + DeckBridge display-click → touch | **emu-akkoord in V2.4** |
| C | DeckBridge touch-strip vorm (8:1) | relurl-1767 | afmetingen kloppen (8:1, 200×100/dial); randen weg → doorlopend zoals HW. Visueel eindoordeel = gebruiker |
| D | DeckBridge persistence | commit aanwezig | **emu-geverifieerd**: restart-restore op 34075, profiel md5 identiek vóór/na, slots + activeDeviceId behouden, geen wipe |
| 1 | Default graph-scale uit reading | niet geïmplementeerd | open |
| 2 | Graph-historie behouden bij save | niet geïmplementeerd | open |
| 3 | Optionele page-indicator | niet geïmplementeerd | open |
| 4 | Edge-separatie aangrenzende dials | niet geïmplementeerd | open |
| 5 | Per-pagina default graph-kleur (palette, wrap-around) | niet geïmplementeerd | open |
| 6 | Font `0 = automatisch` communiceren | auto-logica aanwezig, UI-tekst niet | open |
| 7 | Dial-press discoverability | niet gedocumenteerd in PI | open |
| 8 | Overview-preview-distortie | niet geïmplementeerd | open (na C) |
| 9 | Overview: multi-preview (~3 pagina's) | niet geïmplementeerd | open (V1, na C) |
| 10 | UI-geselecteerde pagina toont op device | aanwezig (gedrag) | **te bevestigen** als bedoeld gedrag |
| 11 | Swipe blijft SD-pagina's wisselen | mag niet geconsumeerd worden | **regressie-bewaking** ontbreekt |
| 12 | Bulk-pagina-aanmaak (V4) | niet geïmplementeerd | open |
| 13 | Overview als default-modus (V4) | niet geïmplementeerd | open |
| 14 | Configureerbare indicator-stijl (V4) | niet geïmplementeerd | open |

## Voortgang (live)

**V0 — klaar, gecommit/gepusht** (DeckBridge `ac3c188`): naadloze 8:1-strip,
persistence (restart-restore), live render-na-assign.

**V1 — in uitvoering, in emu gedeployed, nog NIET gecommit:**

- **Item 1 (default-scale uit reading)** — KLAAR + getest. Go: `deriveDialPageScales`
  in `dial.go` (sentinel `max<=min` → `getDefaultMinMaxForReading`, echo terug naar
  PI). PI: `dial_pi.js` add-page stuurt `min:0,max:0`. Tests: `TestDialGraphScale`.
- **Item 2 (graph-historie behouden)** — KLAAR + getest. `dial.go`:
  `applyDialGraphSettings` (in-place update) + `buildDialGraphs` (hergebruik per
  index bij gelijke reading, rebuild alleen bij reading-wissel) + overview-state
  behouden in `handleDialSendToPlugin`. Test: `TestBuildDialGraphsPreservesHistory`.
- **Rotate→PI-sync (gebruiker-bug 1)** — `OnDialRotate` pusht nu `dialSettings`
  naar de PI zodat de PI de actieve pagina volgt. Gedeployed, nog te valideren.
- **DeckBridge flicker-fix** — `relurl-1769`, nog NIET gecommit. Poll herbouwt de
  deck niet meer elke seconde; dial-tekst/-image worden in-place gepatcht
  (`patchDeckImages` + `/api/images` met `feedbackTitle/Value`). Loste de
  "LHM Dial Carousel"-flits op.

**Gemeten verificatie (niet aangenomen):**
- Dial rendert live + wisselt per pagina: **volledige-image md5** van
  `/api/images` verandert over tijd (live) én verschilt per actieve pagina na
  rotate. ✅ (LET OP: vergelijk altijd de hele image-hash, niet len+tail — de
  PNG-IEND-tail `…SUVORK5CYII=` is gedeeld en gaf eerder een vals "identiek".)
- Bug 1 (rotate→PI-sync): WS-probe als PI → na `/api/dials/rotate` ontvangt de PI
  een `sendToPropertyInspector` met `dialSettings` (nieuwe activeIndex). ✅
- Item 1 (default-scale): WS-probe met **draaiende companion** → pagina met
  sentinel `min:0,max:0` voor "Used Memory" (GB) kreeg afgeleid 2/4, niet 0-100. ✅
- Item 2: unittest `TestBuildDialGraphsPreservesHistory`. ✅

**Bug 2 (PI page-switch):** onderliggend mechanisme werkt (pagina → grafiek
wisselt, gemeten). Oorzaak van de klacht was vrijwel zeker de rotate-desync (nu
gefixt) + flicker (nu gefixt). Letterlijke lijst-klik = user-check.

**Bug 3 (na tile-assign blijft oude dial-PI staan) — gefixt (relurl-1771):**
`assignAction` (en `move`) zetten wel `selectedKeyIndex` maar niet
`selectedContext`, terwijl `selectedSlot()` `selectedContext` prefereert → na
assign opende `openSelectedPI()` de vorige slot (open dial) i.p.v. de nieuwe tile.
Fix: `assignAction` zet `selectedContext` op de zojuist toegewezen slot. Code-zeker;
iframe-paneel-gedrag = user-check (anders `?debugPi=1` → server-log).
(Let op: het `move`-pad heeft dezelfde latente bug — nog te fixen.)

**ENV-valkuil (opgelost):** handmatige DeckBridge hard-restarts (om de
shutdown-hang te omzeilen) killen de **companion** → plugin geeft "LHM
Unavailable" en de emu heeft geen data. Companion altijd mee herstarten:
`setsid ~/projects/GitHub/lhm-companion/build/lhm-companion -port 8085 &` (bind op
WSL-IP, hier 172.18.175.238). Zie [[project_companion_wsl_source]].

**Open gebruiker-bug 2 (PI page-switch):** "in PI kan ik bij pages niet wisselen
van geselecteerde pagina/grafiek." De change-handler bestaat (`dial_pi.js:411`:
zet `activeIndex` + save + `renderPageSettings`). Hypothese: werd veroorzaakt/
vertroebeld door de rotate-desync (bug 1) — opnieuw valideren na de sync-fix. Als
het blijft: check of `#pageList` een zichtbare listbox is (size) en of een
re-render de selectie reset; eventueel "bewerkte pagina" loskoppelen van
"actieve pagina op device".

**V2 — emu-akkoord (17 juni 2026, build `c56a229 + V2.4`):**

- **Thresholds per dial-pagina** — akkoord. Add-knop-regressie gefixt
  (`type="button"` + test: één click voegt exact één threshold toe).
- **Alert Snooze per dial-pagina** — akkoord. Touch start/cyclet/cleart snooze op
  de dial-pagina; regressies getest voor threshold-flap en lege
  `CurrentThresholdID`.
- **Global Thresholds per dial-pagina** — akkoord. Lege global-sectie wordt in de
  dial-PI verborgen; Settings-PI toont nieuw toegevoegde globals direct/open en
  dial-PI toont alleen matchende globals voor de geselecteerde pagina.
- **DeckBridge touch-emulatie** — akkoord voor V2: klik op de zichtbare
  dial-strip (`dial-display`) verstuurt nu ook touch; de losse `Touch`-knop blijft
  werken.

**Ongecommit (na akkoord committen):**
- lhm-repo: V2.4 alert/snooze/global-threshold PI + runtime + tests.
- DeckBridge-repo: zichtbare dial-strip click stuurt touch-event + test.

**Resterend V1:** Display-pariteit (dial-PI Display-sectie = tile: font-sliders
8–20 i.p.v. `0=auto`-number, graphmode/height/thickness/stroke); item 6; A (kern)
mee-valideren; dan V1 emu-akkoord.

**Open infra-punt:** DeckBridge graceful shutdown (SIGTERM) **hangt** → steeds
SIGKILL nodig, deploy-restart onbetrouwbaar. Kandidaat-fix vóór verder iteratief
deployen.

**V2.4 emu-test buildreferentie (plugin):** basiscommit
`c56a229b8d745fef3776cbf49c5b01db016855fe` met lokale V2-diff in
`com.moeilijk.lhm.sdPlugin/dial_pi.html`,
`com.moeilijk.lhm.sdPlugin/dial_pi.js`,
`com.moeilijk.lhm.sdPlugin/settings_pi.html`,
`com.moeilijk.lhm.sdPlugin/settings_pi.js`,
`internal/app/lhmstreamdeckplugin/dial.go`,
`internal/app/lhmstreamdeckplugin/dial_test.go`,
`internal/app/lhmstreamdeckplugin/threshold_state.go`,
`internal/app/lhmstreamdeckplugin/threshold_state_test.go`,
`scripts/test-dial-pi.js` en `scripts/test-settings-pi.js`.
DeckBridge lokaal: touch op de zichtbare dial-strip (`dial-display`) stuurt nu
ook `/api/dials/touch`; de losse `Touch`-knop blijft werken.

## Emu-test blokken (Vx)

Met het tile-pariteitsprincipe wordt de kern groter en splitst in data/display en
alerts/interactie. Voorstel = **zes blokken**, elk apart in de emu te valideren en
goed te keuren. Pas als ze allemaal emu-akkoord hebben, gaat er één build naar de
externe HW-test.

| Blok | Thema | Bevat | Afhankelijk van |
|------|-------|-------|-----------------|
| **V0** | DeckBridge-fundament (DeckBridge-repo) | C touch-strip-vorm, D persistence (restart-restore); + **emu=HW divergenties fixen** (randloze strip, render-na-assign) | — (eerst) |
| **V1** | Tile-pariteit — data/display per pagina | dial-pagina hergebruikt tile Display + Scale + Timing&Smoothing → default-scale (1), font `0=auto` (6) komen mee; graph-historie (2); A mee-valideren | V0 |
| **V2** | Tile-pariteit — alerts/interactie per pagina | Alert Snooze + Thresholds + Global-threshold-suppressie per pagina → maakt touch=tile-druk echt (B) | V1 |
| **V3** | Dial-eigen UI | 4 edge-separatie, 3 page-indicator, 7 dial-press uitleg, 5 per-pagina default-kleur | V0 |
| **V4** | Overview multi-preview | 8+9 multi-preview (~3 pagina's), distortie weg | V0 (vorm) |
| **V5** | Bulk & extra's (basis-scope) | bulk-pagina-aanmaak, overview-als-default-toggle, configureerbare indicator-stijl | V1–V4 |

Per blok meelopen: 11 swipe-regressie (mag nooit breken) en 10 UI-pagina-op-device
(bevestigen als bedoeld gedrag). V1/V2 zijn het zwaarst (PI-pariteit); splits verder
als een blok per sessie te groot wordt.

## Vx — uitvoerplan (afvinkbaar)

**"Klaar wanneer" geldt voor ELK blok** (definition of done, naast de blok-eigen
voorwaarden):
- [ ] Functionaliteit reuse de released tile-componenten; alleen afwijken waar
  dial/touch het echt vereist (zie kernprincipes hierboven).
- [ ] Tests meegeleverd **in dezelfde wijziging**, volgens AGENTS.md "Testing":
  gedragstest (geen string-match van logica), state-condities gedekt (PI open/dicht,
  hidden pages, `thresholds:null`, met/zonder smoothing), cross-component-invariant
  gepind op de consumer.
- [ ] Check-suite groen: `make verify` / `scripts/verify-settings-pi.sh` + DeckBridge `npm test`.
- [ ] Build-marker opgehoogd (relurl / pluginBuildRef) + asserttest.
- [ ] Emu-validatie + gebruiker-akkoord vóór het blok als af geldt.

**Status:**
- **V0** ✅ klaar + gecommit (DeckBridge-fundament).
- **V1** ✅ klaar + gecommit + getest (tile-pariteit data/display).
- **V2** ✅ emu-akkoord (17 juni) + testlat dicht: DeckBridge freeze-fix
  (`d9250eb`, rotary overgeslagen in `patchDeckImages`) en plugin-gedragstests op
  niveau. Go dekt nu alle drie de touch-takken (snooze-cycle/clear, sticky-clear
  zónder snooze-duren, no-op); PI voert snooze-toggle + threshold-add +
  global-suppress echt uit met per-pagina opslag-assert (incl. `thresholds:null`).
  `make verify` + `scripts/verify-settings-pi.sh` groen.
- **V3** ✅ emu-akkoord, klaar voor commit: dynamische separator, page-indicator,
  dial-press uitleg, per-pagina default-kleur en graph-label sanitize met Go-, PI-
  en DeckBridge-live-e2e-tests.
- **V4** ✅ emu-akkoord (`3c5e9b5 + V4-prep`): overview multi-preview zonder
  overlap, tests groen.
- **V5** ✅ emu-akkoord (`V5-prep.16`): bulk-apply UX (kort "No matching
  readings", uitgegrijsd/niet-aanklikbaar), tests groen, gecommit + gepusht
  (`0bf49a2`).

### V2 — alerts/interactie per pagina (valideren + testen)
- [x] Alert Snooze, Thresholds, Global-threshold-suppressie per pagina werken,
  gebonden aan `selectedPage()`, opgeslagen via `dialSetSettings`.
- [x] Touch op de strip = tile-druk (snooze cyclen/clearen) op de actieve pagina,
  óok via globale thresholds. Gemeten op device/emu.
- **Getest:** Go `TestHandleDialPageTouch*` dekt de drie touch-takken
  (snooze-cycle/clear, sticky-clear zónder snooze-duren, no-op) +
  `TestUpdateDialPageKeepsSnoozeWhenThresholdDrops`; PI `test-dial-pi.js` voert
  snooze-toggle, threshold-add en global-suppress echt uit en assert per-pagina
  opslag + `dialSetSettings`-verzending (incl. `thresholds:null`). `make verify` +
  `scripts/verify-settings-pi.sh` groen.

### V3 — dial-eigen UI (valideren + testen)
- [x] Dynamische separator per dial: breedte 0-10, kleur, default breedte 3 /
  `#363e46`, actie-niveau opgeslagen.
- [x] Page-indicator (dots ≤9, `x/y` daarboven), off in fullscreen.
- [x] Per-pagina default-kleur uit palette (wrap-around) bij toevoegen.
- [x] Dial-press-uitleg in PI (bestaand patroon).
- [x] Graph-label sanitize tegen onveilige runes.
- **Getest:** Go pixel/resolve-tests voor separator + indicator, graph sanitize-test,
  PI-tests voor separator/palette/build-marker en DeckBridge live e2e voor separator,
  page indicator, dial press, touch/snooze, global-threshold cleanup en live movement.

### V4 — overview multi-preview
- [x] Overview toont ~3 pagina's (actieve gecentreerd), tegen echte touch-strip-aspect.
- **Getest:** Go-tests voor preview-rect aspect/centrering + centered crop zonder
  vervormende schaal; `scripts/verify-settings-pi.sh` met DeckBridge live e2e;
  `make verify`.
- **Emu-akkoord:** gebruiker bevestigde "functioneel prima".

### V5 — bulk & extra's
- [x] Bulk-pagina-aanmaak via regel: alle readings van sensor, of geselecteerde
  reading over sensors; preview + deselect via multi-select.
- [x] Overview-als-default-toggle; configureerbare indicator-stijl.
- **Getest:** PI-test voor bulk-generatie + deselect + pagina-aanmaak; PI-test
  voor action-level default view/indicatorstijl; Go-tests voor renderer-resolve;
  DeckBridge live e2e voor view-opties en bulk-add; `scripts/verify-settings-pi.sh`;
  `make verify`.
- **Emu-akkoord (`V5-prep.16`):** bulk-apply melding kort + uitgegrijsd; gecommit
  + gepusht (`0bf49a2`).

## Te (her)valideren — niet-gaten

Deze stonden eerder als "done/gedekt", maar zijn **niet** als opgelost te
beschouwen tot opnieuw bevestigd.

### A. Kern-dialgedrag her-valideren
De 14-juni-hardware-test gold het prototype. Sindsdien is code gewijzigd. Bij de
volgende hardware-ronde opnieuw bevestigen: actie toevoegen, pagina's
toevoegen/herordenen/configureren/verwijderen, meerdere dials zonder
interferentie, rotatie cyclet pagina's, dial-press wisselt weergave.

### B. Touch = tile-druk
Een tap op de touch-strip heeft **dezelfde functie als een druk op een tile** —
niets meer. Dit is de volledige definitie van het touch-gedrag; er is geen aparte
"bredere touch-interactie".

Exact gedrag van de tile-druk (`OnKeyDown`,
[delegate.go:347](../../internal/app/lhmstreamdeckplugin/delegate.go#L347)),
toegepast op de actieve pagina:

1. Geen actieve threshold (`CurrentThresholdID == ""`) → tap doet niets.
2. Wel actieve threshold **met** geconfigureerde snooze-duren → cycle naar de
   volgende snooze-duur; na de laatste → snooze clearen.
3. Wel actieve threshold **zonder** snooze-duren → sticky threshold clearen.

`OnTouchTap` ([dial.go:586](../../internal/app/lhmstreamdeckplugin/dial.go#L586))
implementeert deze drietraps-logica al 1:1 per actieve pagina (`pageCtx`).

**Bevinding (codex-werk gecontroleerd):** codex bouwde alleen het Go-gedrag. De
dial-PI bevat **geen** uitleg én **geen** config: `normalizePage`
([dial_pi.js:65](../../com.moeilijk.lhm.sdPlugin/dial_pi.js#L65)) kent geen
`snoozeDurations`/`thresholds`-veld. Daardoor:

- Een dial-pagina krijgt alleen een actieve threshold via **globale thresholds**
  (Settings, #41); per-pagina thresholds zijn niet instelbaar.
- Snooze-duren zijn per pagina niet instelbaar → touch valt terug op de
  "sticky clearen"-tak; de snooze-cyclus van een normale tile is niet op te zetten.
- Dit verklaart waarom de tester "tap doet niks" zag.

Bestaand patroon om te hergebruiken bij communiceren/config: het normale-tile blok
"Alert Snooze" met label "On Key Press" en uitleg-`<p>`
([index_pi.html:330-345](../../com.moeilijk.lhm.sdPlugin/index_pi.html#L330-L345)).

**Beslist:** dit valt onder het tile-pariteitsprincipe — de dial-pagina krijgt de
tile Alert Snooze- + Thresholds-secties (blok **V2**), waarmee touch=tile-druk
volledig echt en configureerbaar wordt. Geen losse touch-patch; het komt mee met
de PI-pariteit.

### C. DeckBridge touch-strip vorm
Commit `706c0c1` zet de emulatie op 8:1 en de lokale server draait die build.
Nog **niet** bevestigd dat de gerapporteerde preview-distortie hiermee verdwijnt
of dat de emulatie de echte hardware-vorm matcht. Valideren vóór item 8.

### D. DeckBridge persistence
Commit `8fb6320` claimt persistente dashboard-state. **Restart-restore-test
opnieuw draaien** en bevestigen dat opgeslagen slots/device na herstart bewaard
blijven; anders is lokale validatie niet betrouwbaar.

## Open V1-werk

Volgorde = aanbevolen uitvoervolgorde (code-items eerst, daarna tekst/PI, daarna
validatie). Elk item is pas klaar ná lokale deploy + hardware-bevestiging.

### 1. Default graph-scale uit de reading afleiden — code
- Probleem: [dial.go:56-60](../../internal/app/lhmstreamdeckplugin/dial.go#L56-L60)
  valt terug op 0–100 als `Min/Max` niet gezet zijn.
- Fix: gebruik `getDefaultMinMaxForReading`
  ([handlers.go:68](../../internal/app/lhmstreamdeckplugin/handlers.go#L68)) bij
  het toevoegen van een pagina / wisselen van reading, net als
  [composite.go:687](../../internal/app/lhmstreamdeckplugin/composite.go#L687).
- Let op: bestaande opgeslagen settings moeten blijven werken (alleen lege/
  niet-gezette scale afleiden, expliciete waarden respecteren).

### 2. Graph-historie behouden bij settings-save — code
- Probleem: `dialSetSettings` roept `initDialState` aan
  ([dial.go:552](../../internal/app/lhmstreamdeckplugin/dial.go#L552)) en
  herbouwt álle graphs → alle historie weg bij elke wijziging.
- Fix: bestaande graphs hergebruiken; alleen pagina's herbouwen waarvan de
  reading-identiteit of visuele graph-instellingen daadwerkelijk veranderden.

### 3. Optionele page-indicator op de touch-strip — code
- Off by default in fullscreen (geen nieuwe default-weergave forceren).
- Kleine grijze dots bij weinig pagina's; actieve dot iets feller én zijwaarts
  uitgerekt (zoals de issue-feedback vroeg); `x / y`-tekst zodra dots onleesbaar
  worden (richtdrempel ~9, zoals de Action Wheel).
- In render-code tekenen, niet via een apart Stream Deck feedback-veld.
- Mag titel/waarde niet overlappen.

### 4. Edge-separatie tussen aangrenzende dials — code
- **Bron (reporter, expliciet):** "two carousels right next to each other have no
  visible border ... maybe it would be a good investment to add a thin border on
  both sides."
- Eén pixelkolom links én rechts **getekend in de gerenderde dial-PNG** (niet in
  emu-CSS), zodat de scheiding óók op echte HW verschijnt.
- Niet-configureerbaar in V1; geen nieuwe theming-controls.
- **Emu-verifieerbaar sinds relurl-1768:** de emu-strip is nu naadloos (= HW), dus
  onze getekende 1px-kolommen zijn in de emu zichtbaar exact zoals op HW.

### 5. Per-pagina default graph-kleur — code (feature, NIET afgewezen)
- Door de reporter gevraagd en **in scope**: elke pagina krijgt een andere default
  graph-kleur uit een kleine hardcoded palette (~5 kleuren, wrap-around). Helpt
  bij het navigeren door de lijst.
- Eerdere "afgewezen"-markering was een onterechte codex-triage en is teruggedraaid.
- Default per pagina; de gebruiker kan de kleur per pagina nog steeds overschrijven.
- Verifieer daarbij dat de overige dial-defaults (buiten kleur) overeenkomen met de
  normale Reading-actie, zodat alleen kleur bewust afwijkt.

### 6. Font-size `0 = automatisch` duidelijk maken — PI-tekst
- Auto-logica bestaat al in code (`defaultDialTitleFontSize` /
  `defaultDialValueFontSize`, [dial.go:103-110](../../internal/app/lhmstreamdeckplugin/dial.go#L103-L110)).
- Alleen in de Property Inspector communiceren dat `0` = automatisch, binnen
  bestaande PI-patronen (geen nieuw default-open helpblok).

### 7. Dial-press discoverability — PI/doc-tekst
- Documenteer dat indrukken de overview-weergave toggelt, via een bestaand
  PI/documentatie-patroon. Geen nieuw prominent helpblok voor deze ene actie.

### 8 + 9. Overview als multi-preview (~3 pagina's) — code (na C)
- **Beslist:** overview wordt in V1 een multi-preview die ~3 pagina's tegelijk
  toont (de actieve gecentreerd, buren ernaast), zoals de reporter opperde — geen
  1-pagina-navigatie-aid.
- Pas ná validatie van de DeckBridge 8:1-vorm uitwerken: previews tegen de echte
  touch-strip-aspect-ratio fitten/croppen i.p.v. vrij schalen in bijna-vierkante
  kaarten, zodat de distortie-klacht verdwijnt.
- Fullscreen blijft de primaire leesbare modus; multi-preview is de overview-/
  navigatiemodus (dial-press).

### 10. UI-geselecteerde pagina toont op device — gedrag bevestigen
- Een pagina selecteren in de PI maakt die de actieve pagina op het device.
- Tester vond dit "onverwacht maar begrijpelijk". Behouden als bedoeld V1-gedrag
  tenzij het een regressie veroorzaakt; expliciet bevestigen, niet stilzwijgend.

### 11. Swipe-regressiebewaking — test/gedrag
- Het plugin mag touchscreen-swipe-gestures **niet** consumeren; swipe moet
  Stream Deck-pagina's blijven wisselen.
- Borgen: geen swipe-handler toevoegen die dit breekt, en bij hardware-validatie
  expliciet als checkpunt opnemen.

## Validatie & gates (volgorde)

**Gate 1 — automatische checks**
1. `go test ./...`
2. `scripts/verify-settings-pi.sh` (+ Node-syntaxcheck op gewijzigde PI-bestanden)
3. Gerichte tests voor nieuwe logica: default min/max-propagatie, page-indicator-
   drempel, graph-behoud bij save. Geen regressie op dial-index-wrapping.

**Gate 2 — emu-validatie (DeckBridge) — eerstvolgende gate, vóór hardware**
4. DeckBridge eerst betrouwbaar maken: restart-restore-test (item D) en
   preview-vorm-check tegen echte touch-strip-aspect-ratio (item C).
5. `scripts/deploy-local.sh` en de volledige basis-scope in de emu doorlopen:
   kern-gedrag (A), touch=tile (B), default-scales (1), graph-historie (2),
   page-indicator (3), edge-grens (4), per-pagina-kleur (5), multi-preview
   overview (8+9), UI-pagina-op-device (10), swipe (11), én V4: bulk-aanmaak,
   overview-als-default, configureerbare indicator-stijl.
6. **Akkoord van gebruiker in de emu** dat de basis-scope compleet en goed genoeg
   is. Geen hardware-test vóór dit akkoord.

**Gate 3 — hardware-test (extern, eenmalig)**
7. Pas na emu-akkoord én als **alle** basisfunctionaliteit-brokken af zijn:
   hardware-validatie op echte Stream Deck+ met dezelfde checkpunten. De
   hardware-test is extern en buiten onze controle, dus we sturen er één
   complete build heen, niet per chunk.

**Gate 4 — release**
8. Release pas na expliciete versie-motivatie en goedkeuring (zie CLAUDE.md
   release-stappenplan). Daarna #56 sluiten/updaten en V2-issues aanmaken.

## Scope-grenzen

### Buiten scope tot de basis HW-akkoord heeft
Deze komen pas ná goedkeuring van de basisfunctionaliteit:

- Derived Metric-pagina's in de dial.
- Composite Dashboard-pagina's in de dial.

### V4 — toelichting (basis-scope, vóór de HW-test)

- **Bulk-pagina-aanmaak / preset-assistent** — bron: reporter ("tedious" om 8/16
  pagina's los toe te voegen). Concreet: een PI-control om in één keer veel
  pagina's te genereren via een regel ("reading X voor alle cores", "alle readings
  van sensor Y"), met preview, individueel deselecteren en een naam-template.
  Groot maar goed afgebakend; lost een echte klacht op.
- **Overview als default-modus** — bron: reporter. Concreet: een per-dial toggle
  die de multi-preview overview de startweergave maakt i.p.v. fullscreen. Klein:
  PI-toggle + persist + honoreren bij WillAppear.
- **Configureerbare indicator-stijl** — bron: reporter (twijfelde manueel vs
  automatisch). Concreet: gebruiker kiest dots / `x/y` / uit i.p.v. de automatische
  drempel uit V2-item 3. Klein.

Bredere touch-interactie is **geen** apart V4-item: touch = tile-druk (zie blok B).
