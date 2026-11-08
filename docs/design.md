# Ryoport design

Ryoport is the one place on a Ryoku desktop to reach the machines you work
with: virtual machines on this computer, a guest with the GPU passed through,
servers you SSH into, and serial, Telnet, RDP or VNC consoles. It starts them,
connects to them, and tells you whether they are healthy.

This page is the reasoning behind the layout and the look. The architecture is
in `architecture.md`.

## How it gets used

| Moment | How often | Time in the window | What it needs |
|---|---|---|---|
| Connect to a server, VM or console | several times a day | under 10 s | find it in two keystrokes, one action |
| Check that things are up | daily | 10 to 30 s | state at a glance, what needs you first |
| Start or stop a VM | a few times a week | seconds | the action on the row, progress you can see |
| Watch a machine install or work | now and then, sometimes on a second screen | minutes | live charts that cost almost nothing |
| Create a machine | monthly | minutes, mostly waiting | a guided flow, progress that survives navigation |
| Add or change a server | rarely | a minute or two | a short form, the advanced part folded away |

Most visits are short and purposeful. The window is a launcher first, a status
board second, and a place for long flows a distant third.

1. **The fleet list is home.** Everything you can reach is one search away and
   Enter does the obvious thing: connect to a server, start a stopped machine,
   open a running one.
2. **One primary action per resource.** The row shows it on hover and Enter
   runs it. Everything else lives in the work pane.
3. **State is a stone and a word together.** Never colour alone.
4. **Long jobs stay where they belong.** A download shows on its row, in the
   machine's work pane and in Activity, and keeps going while you look away.
5. **Nothing moves or polls when nobody is looking.** An idle Ryoport draws no
   frames.
6. **Creating is a flow you enter on purpose**, from the New plaque.
7. **Forms open in the work pane, not in dialogs.** Adding a server or a
   console, importing an ISO or making an instant machine is a draft in the
   work pane. Dialogs are only for decisions you cannot undo.

## Lithic

Ryoport's look is Lithic, a UI language made for Ryoku: Material 3's working
structure (an adaptive list and detail, a navigation rail, real controls,
tonal containers, motion that answers a cause) in cut stone. Its thesis is
"cut, not molded".

1. **Cut, not molded.** Corners are cut at 45 degrees or square, never
   rounded. The cut follows scale: 16 dp on panes and dialogs (top corners of
   panes), 7 dp on buttons and fields, 6 dp on small buttons, 3 to 4 dp on
   tokens. Only content (logos, charts) curves.
2. **Flat planes.** Every surface is one flat tone. Depth is only the order of
   tones: ground, list pane, work pane, raised, bone. No gradients, glows,
   inner shadows, inset wells or soft drop shadows. Overlays are a solid scrim
   and a slab one tone up with a hairline.
3. **Bone is where you are.** The selected row, the current rail destination
   and the current filter are bone slabs with ink type, Ryoku's inversion.
4. **Gold is what you can do.** Flat gold leaf on consequential actions
   (connect, start, stop, create, save, sign in); gold outline and gold type for
   secondary actions. Navigation and view controls are stone and bone.
   Nothing you can only watch is gold.
5. **Stones are states,** always beside their word: jade alive (running,
   online), lapis becoming (starting, creating, downloading), amber needs you,
   jasper failed or unreachable, slate idle.
   - **The notch:** a row's top-left corner is cut and filled with its state's
     stone; an idle row's cut stays empty.
   - **The mineral field:** the header of the selected thing (name, proof,
     actions) sits on a slab tinted with its state's stone.
   - **The flake:** the standalone token (services, map nodes), a small
     slanted bar in the stone's colour, outlined when idle, part filled while
     becoming.
6. **The blade marks progress and place.** A bar whose leading edge is cut at
   45 degrees: the current tab's underline, job progress, meters.
7. **Engraved data.** Charts are a fine line over a 45 degree hatch; numbers in
   Space Mono; no area fills in colour.
8. **One family, set monumentally.** The selected thing's name in Space
   Grotesk Bold at 52 dp is the one big line of a view; everything else at work
   sizes.
9. **Becoming is drawn.** A machine being made shows its name in outline,
   filling with the progress, and its future facts as construction lines.

## Layout

Three widths, in dp:

| Width | Rail | List | Work pane |
|---|---|---|---|
| expanded, 1280 and up | 88, icons with labels | 384 | the rest |
| medium, 760 to 1279 | 72, icons only | 320 | the rest |
| compact, under 760 | none, a bottom bar | full width | replaces the list, with back |

Default window 1280 x 800, minimum 640 x 520. When the compositor does not draw
decorations (GNOME), Ryoport draws its own window controls at the top right of
the work pane and lets the empty top strip of each pane drag the window.

**Rail.** The gold New plaque at the top, then Fleet, Activity and Settings.
The current destination is a bone tile that moves to its new place. An update
shows as a notch on the Settings tile and nowhere else.

**List pane.** The search field (Ctrl K opens the palette, / focuses it), one
line saying how the fleet is ("Three running. mika-server needs you."), the
filters with live counts (All, Machines, Servers, Consoles), then rows grouped
by state in the order Needs you, Running, Becoming, Stopped. A row is a flat
tablet, 56 dp (48 compact density): the notch, the engraved logo, the name, one
line of facts, and on the right the claim and its proof ("Online" over
"23 ms"). Hovering a row, or moving to it with the keyboard, swaps the claim
for its one gold action. A becoming row carries its progress as a blade along
its bottom edge. The selection is a bone slab that moves from row to row.

**Work pane.** The mineral field header: the engraved logo, the name, one
sentence of proof, and the act row (a gold split button for the primary
action, gold-outline plaques for the next ones, an overflow plaque for the
rest). Destructive actions live in the overflow and confirm in a dialog. Under
the header, tabs on a blade, at most five per kind, and the content on flat
stone.

**Nothing selected.** The work pane shows the view from above: the fleet
sentence large, what needs you with the gold action that fixes it, what is
becoming with its blade, and the map: this computer at the centre, octagonal
rings at 1, 10 and 100 ms of round trip, every machine a flake placed by how
far away it is, local machines on the innermost ring.

**Activity.** Jobs with cancel, things that need you with their fix, and the
history by day, filterable by resource. Jobs are drawn on one time scale, so a
long install looks long.

**Settings.** Appearance, new machine defaults, the VM engine, GPU
passthrough, SSH keys, updates and about, as plain sections on flat stone.

**New.** The New plaque (Ctrl N) opens a menu: a machine from the catalogue,
an instant cloud machine, a machine from an ISO, a machine from a template, a
passthrough machine (when the host supports it), a server, a console. The
catalogue takes the work pane with a searchable grid of systems; everything
else is a draft in the work pane with the advanced part folded shut.

## Tokens

The neutrals come from the live Ryoku palette, so Ryoport matches the desktop
and changes with the wallpaper; the stones and the gold are Lithic's own and
do not change.

| Role | From the palette (dark) | Used for |
|---|---|---|
| ground | surfaceContainerLowest | behind the panes |
| stone 1 | surface | the list pane, the rail |
| stone 2 | surfaceContainerLow | the work pane, row tablets |
| stone 3 | surfaceContainer | hover, fields, tracks |
| stone 4 | surfaceContainerHigh | menus, dialogs |
| vein | surfaceContainerHighest | hairlines, quiet outlines |
| fg, fg 2, fg 3 | onSurface, onSurfaceVariant, outline | type from strong to faint |
| sel, on sel | inverseSurface, inverseOnSurface | bone: where you are |

Light palettes map the same roles from the other end of the scale, so the
ground is the darkest stone and the work pane the lightest. Without a Ryoku
palette, Ryoport uses Lithic's signature stones: ground `#0e0e0e` to vein
`#393939` with bone `#e3e3e3`, or warm marble `#e2ddd4` to `#f4f1eb` with ink
`#1d1b18`.

| Stone | Dark | Light | Means |
|---|---|---|---|
| jade | `#7ab8a0` | `#2f7a64` | alive |
| lapis | `#8aa2d6` | `#3d5f99` | becoming |
| amber | `#e0954a` | `#94520f` | needs you |
| jasper | `#e0553a` | `#b8321e` | failed |
| gold | `#c9a861` | `#c9a45a`, type `#85651f` | what you can do |

Mineral fields mix the stone into the work pane tone (17 percent for jade, 15
for the others); slate fields are stone 3.

Type is Space Grotesk for everything a person reads and Space Mono for literal
data. Sizes in sp: name 52, sentence 34, section 21, row 15, body 14, label 13,
caption 12, group 12, mono 12 and 11. Sentence case, no all-caps.

Spacing runs 4, 8, 13, 21, 34, 55.

## Motion and battery

Knapping: one curve, cubic-bezier(.2, 0, 0, 1), no bounce, nothing moving at
rest.

| Motion | Duration | What it says |
|---|---|---|
| press: the corners chip deeper (7 to 11 dp) and the plaque sinks 1 dp | 90 ms | you pressed it |
| hover: the row steps up a tone, its claim gives way to its action | 160 ms | this is what Enter will do |
| select: the bone slab moves; the mineral field changes stone | 240 ms, 320 ms | you are here now |
| notch: a new state is cut into the corner | 200 ms | the state changed |
| navigate: the rail tile and the tab blade slide | 240 ms | where you went |
| become: construction lines draw, the outlined name fills, the blade advances | with the job | it is being made |
| remove: a hairline strikes the name, the Undo placard drains | 5 s | you can still take it back |
| delete: the slab breaks along a diagonal and falls away | 420 ms | it is gone |

Reduced motion (Ryoku's setting or the app's) turns every movement into an
instant change.

Polling follows what is on screen.

| Source | Focused | Visible, not focused | Hidden |
|---|---|---|---|
| machine list | on file change, 10 s safety | same | 60 s |
| stats of a running machine on screen | 2 s | 5 s | off |
| row metrics in the list | 5 s | 10 s | off |
| server reachability | 15 s | 30 s | off |
| server probes | 30 s, 10 s for the selected one | 60 s | off |
| web app checks | 60 s | off | off |
| passthrough status | 10 s | 30 s | off |

On battery every interval doubles. Jobs report their own progress whatever is
on screen.

## Copy

Sentence case, plain verbs, the same word through the whole flow: a button
that says Start produces "Starting" and then "Running". A state always comes
with its proof ("Online, 23 ms", "Needs you, sign-in failed 2 min ago").
Errors say what happened and what to do. Empty states offer the next step
("No machines yet. Make one from the catalogue or import an ISO").

## Keyboard

| Keys | Does |
|---|---|
| Ctrl K | command palette: every resource and action |
| / | search the list |
| Up, Down | move through the list |
| Enter | the row's primary action |
| Ctrl Enter | open the selected row's work pane |
| Ctrl N | the New menu |
| Ctrl 1, 2, 3 | Fleet, Activity, Settings |
| Ctrl R | refresh now |
| Esc | close the top overlay, then the draft, then clear the selection |
| Ctrl Q | quit |
