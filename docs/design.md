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
6. **Creating is a flow you enter on purpose**, from New in the rail.
7. **Forms open in the work pane, not in dialogs.** Adding a server or a
   console, importing an ISO or making an instant machine is a draft in the
   work pane. Dialogs are only for decisions you cannot undo.

## Lithic

Ryoport's look is Lithic, a UI language made for Ryoku: Material 3's working
structure (an adaptive list and detail, a navigation rail, real controls,
tonal containers, motion that answers a cause) in cut stone. Its thesis is
"cut, not molded". The full specification, written for every Ryoku app to
adopt, with the design sheets and runnable web, QML and Go kits, lives in the
Ryoku repository under `docs/lithic/`.

1. **Cut, not molded.** Corners are cut at 45 degrees or square, never
   rounded. The cut follows scale: 16 dp on panes and dialogs (top corners of
   panes), 7 dp on buttons and fields, 6 dp on small buttons, 3 to 4 dp on
   tokens. Only content (logos, charts) curves.
2. **Flat planes.** Every surface is one flat tone. Depth is only the order of
   tones: ground, list pane, work pane, raised, bone. No gradients, glows,
   inner shadows, inset wells or soft drop shadows. Overlays are a solid scrim
   and a slab one tone up with a hairline.
3. **Bone is where you are.** The selected row, the current rail destination
   and the current filter are selection slabs with ink type: bone in Lithic's
   own colours, the palette's secondary container on a Ryoku desktop.
4. **Gold is what you can do.** A flat fill on consequential actions (connect,
   start, stop, create, save, sign in); an outline and coloured type for
   secondary actions. Navigation and view controls are stone and selection.
   Nothing you can only watch is filled. On a Ryoku desktop the gold is the
   palette's primary colour; elsewhere it is the host distro's own colour.
5. **Stones are states,** always beside their word: jade alive (running,
   online), lapis becoming (starting, creating, downloading), amber needs you,
   jasper failed or unreachable, slate idle.
   - **The notch:** a row's top-left corner is cut and filled with its state's
     stone; an idle row's cut stays empty.
   - **The mineral field:** the header of the selected thing (name, proof,
     actions) sits on a slab tinted with its state's stone.
   - **The flake:** the standalone token (services, fact lines), a small
     slanted bar in the stone's colour, outlined when idle, part filled while
     becoming.
6. **The blade marks progress and place.** A bar whose leading edge is cut at
   45 degrees: the current tab's underline, job progress, meters.
7. **Engraved data.** Charts are a fine line over a 45 degree hatch; numbers in
   Space Mono; no area fills in colour.
8. **One family, set monumentally.** The selected thing's name in Space
   Grotesk Bold at 52 dp is the one big line of a view; everything else at work
   sizes.
9. **Becoming is shown, not drawn.** A machine being made shows its name in
   outline, the stage and its progress on a blade, the four steps with the
   current one carrying the tool's latest line, and the tool's last lines.

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

**Rail.** The Ryoport mark at the top is Home: it returns to the view from
above. Then Fleet, New (Ctrl N) and Activity, with Settings at the bottom. The
current destination is a selection tile that moves to its new place. An
update shows as a notch on the Settings tile and nowhere else.

**List pane.** A "Fleet" title over one line saying how the fleet is ("Three
running. mika-server needs you."), the search field (Ctrl K opens the palette,
/ focuses it), the filters with live counts (All, Machines, Servers, Consoles),
then rows grouped in the order Needs you, Pinned, Running, Becoming, Stopped.
Pinned holds everything pinned whatever its state, except what needs you,
which stays in Needs you. Every kind can be pinned from its menu.
A row is a flat tablet, 56 dp (48 compact density): the notch, the engraved
logo, the name, one line of facts with the machine's marks after them
(Disposable in amber first, then Instant and Sealed), and on the right the
claim and its proof ("Online" over "23 ms"; a server with a failed systemd unit
stays under Running with "1 failed unit" in amber). Hovering a row, or moving
to it with the keyboard while the list has focus, swaps the claim for its one
action. A becoming row carries its progress as a blade along its bottom edge.
The selection is a slab that moves from row to row.

**Work pane.** The mineral field header: the engraved logo, the name, one
sentence of proof, and the act row (a gold split button for the primary
action, gold-outline plaques for the next ones, an overflow plaque for the
rest). Destructive actions live in the overflow and confirm in a dialog. Under
the header, tabs on a blade, at most five per kind, and the content on flat
stone.

**Nothing selected.** The work pane shows the Now board: the fleet sentence
large with one line of proof under it (when it was checked and how many are
up), pinned machines as tiles with their state and one action each, then two
columns from 860 dp: Needs you (each with the action that fixes it) and
Running now (live CPU, latency, uptime) on the left; This computer (CPU,
memory and the free space on the disk that holds the VMs as quiet meters that
turn amber when nearly full, the engine and passthrough lines) and Recent
activity on the right. Host figures are read only while the board is on
screen: every 3 s focused, 15 s unfocused, twice that on battery. No diagrams.

**Activity.** Jobs with cancel, things that need you with their fix, and the
history by day, filterable by resource. Jobs are drawn on one time scale, so a
long install looks long.

**Settings.** Appearance, new machine defaults, the VM engine, GPU
passthrough, SSH keys, updates and about. At full width a section index sits
beside one 760 dp column; at medium width the index becomes a row of chips.

**New.** New (Ctrl N) opens a menu: a machine from the catalogue, an instant
cloud machine, a machine from an ISO, a machine from a template, a passthrough
machine (when the host supports it), a server, a console. The catalogue takes
the work pane: a searchable grid of systems across its full width; choosing a
system (a click, or Enter in the search) opens it in a 720 dp dialog with its
releases, editions, the size it will get and Create. Its logo is fetched while
the tile is hovered, so the dialog opens complete. Everything else is a draft
in the work pane under the same header as a resource (its mark, the title at
the big size, one sentence, Make it or Save in gold): the form in a column of
at most 600 dp and, from 820 dp of width, an aside beside it that shows what
saving will make or write as you type. A server shows the exact stanza for
its SSH config, a console the command it runs, a machine its image, size,
key and folder. Narrower, the aside follows the form. The advanced part stays
folded shut, and no field complains before it was edited and left.

**Menus and dialogs.** A menu sits 8 dp from its button. A press outside a
menu or a dialog only closes it; it never also presses what was underneath.

**Escape** closes the topmost thing: an open menu or dialog, then the
palette, a draft, the catalogue, and finally the open detail, which returns
to the Now board.

## Tokens

On a Ryoku desktop every role but the state stones comes from the live Ryoku
palette (matugen), so Ryoport matches the desktop and changes with the
wallpaper. Forcing Dark or Light in Settings keeps Lithic's own neutrals for
that mode but still takes the accents from the palette, re-toned for it.

| Role | From the palette (dark) | Used for |
|---|---|---|
| ground | surfaceContainerLowest | behind the panes |
| stone 1 | surface | the list pane, the rail |
| stone 2 | surfaceContainerLow | the work pane, row tablets |
| stone 3 | surfaceContainer | hover, fields, tracks |
| stone 4 | surfaceContainerHigh | menus, dialogs |
| vein | surfaceContainerHighest | hairlines, quiet outlines |
| fg, fg 2, fg 3 | onSurface, onSurfaceVariant, outline | type from strong to faint |
| sel, on sel | secondaryContainer, onSecondaryContainer | where you are |
| gold, on gold | primary, onPrimary | what you can do |
| gold ink | primary, lightened or darkened to 4.5:1 on stone 2 | gold type and outlines |
| jasper | error | failed |

Light palettes map the same roles from the other end of the scale, so the
ground is the darkest stone and the work pane the lightest. Without a Ryoku
palette, Ryoport uses Lithic's signature stones: ground `#0e0e0e` to vein
`#393939` with bone `#e3e3e3`, or warm marble `#e2ddd4` to `#f4f1eb` with ink
`#1d1b18`.

On another distro, with no Ryoku palette, the gold is the host distro's own
colour from `/etc/os-release` (Arch `#1793d1`, CachyOS `#00b3a1`, Fedora
`#51a2da`, Void `#478061`, NixOS `#7ebae4`, Ubuntu `#e95420`, and so on), and
the selection stays bone. With neither, the signature gold below stands.

| Stone | Dark | Light | Means |
|---|---|---|---|
| jade | `#7ab8a0` | `#2f7a64` | alive |
| lapis | `#8aa2d6` | `#3d5f99` | becoming |
| amber | `#e0954a` | `#94520f` | needs you |
| jasper | `#e0553a` | `#b8321e` | failed |
| gold | `#c9a861` | `#c9a45a`, type `#85651f` | what you can do |

Jade, lapis and amber never follow the palette: they carry meaning.

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
| press: the corners chip deeper (7 to 11 dp) and the plaque sinks 1 dp; rows and menu items dip one tone | 90 ms | you pressed it |
| hover: the tone fades up, a row's claim gives way to its action | 160 ms in, 100 ms out | this is what Enter will do |
| select: the selection slab glides between rows and the filter slab between chips; the mineral field changes stone | 240 ms, 320 ms | you are here now |
| notch: a new state is cut into the corner | 200 ms | the state changed |
| navigate: the rail tile and the tab blade slide; the work pane's new content fades in rising 8 dp | 240 ms, 200 ms | where you went |
| open: a menu fades in closing the last 6 dp to its button, a dialog rises 12 dp over its scrim; both leave faster the way they came | 160 ms and 300 ms in, 100 ms and 160 ms out | it came from what you pressed |
| unfold: a folded section grows to its height and its fields fade in | 200 ms | there is more here |
| focus: the focus ring fades in | 120 ms | the keyboard is here |
| become: the outlined name fills and the blade's edge eases to each new value | with the job | it is being made |
| scroll: a wheel notch glides 120 dp; a trackpad follows the fingers exactly | 140 ms | the page went where you sent it |
| remove: the Undo placard drains | 5 s | you can still take it back |

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
| this computer, while the Now board is on screen | 3 s | 15 s | off |

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
