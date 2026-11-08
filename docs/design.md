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
board second, and a place for long flows a distant third. Everything below
follows from that.

1. **The fleet list is home.** Everything you can reach is one search away and
   Enter does the obvious thing: connect to a server, start a stopped machine,
   open a running one.
2. **One primary action per resource, always visible.** The row carries it.
   Everything else lives in the detail pane, where there is room to explain it.
3. **State is shape, colour and a word together.** Never colour alone.
4. **Long jobs live in one place.** Downloads and builds show on the row they
   belong to and in Activity, and keep going while you look at something else.
5. **Nothing moves or polls when nobody is looking.** Ryoku laptops run on
   battery. An idle Ryoport draws no frames.
6. **Creating is a flow you enter on purpose.** It is not a permanent tab
   competing with the things you use every day.

## Layout

The window is a list-detail workspace, the canonical large-screen layout on
Android 17 and the shape of every good fleet console (Proxmox, Tailscale,
Cockpit). Three widths, measured in dp:

| Width | Rail | List | Detail |
|---|---|---|---|
| expanded, 1280 and up | 88, icons with labels | 384 | the rest, at least 560 |
| medium, 760 to 1279 | 72, icons only | 320 | the rest |
| compact, under 760 | bottom bar | full width | replaces the list, with back |

Default window 1280 x 800, minimum 640 x 520.

```
expanded
+------+--------------------------+--------------------------------------+
| seal | Search machines, servers | [seal] ally              ( Online )  |
|      | (All|Machines|Servers|..)|        neur0map@192.168.110.8       |
| (+)  | 2 running  1 needs you   | [ Connect | v ] [ Files ] [ ... ]    |
| New  |                          |                                      |
|      | Pinned                   |  Overview  Access  Network  Workloads|
| Fleet| [s] ally      23 ms  ~ > |                                      |
|      | [s] win11     Looking  > |  +--------+ +--------+ +--------+    |
| Activ| Machines                 |  | CPU    | | Memory | | Disk   |    |
|      | [s] ubuntu-24  Stopped > |  +--------+ +--------+ +--------+    |
| Sett | Servers                  |  Facts, services, recent activity    |
|      | [s] prowl     41 ms    > |                                      |
|      | [s] mika    Needs pass > |                                      |
| v1.0 | ~~ Ubuntu 24.04  62%  x  |                                      |
+------+--------------------------+--------------------------------------+
```

**Rail.** The Ryoport seal at the top, then the New button, then three
destinations: Fleet, Activity and Settings. The active destination sits on a
bone indicator that slides to its new place. An update, when there is one,
shows as a dot on Settings and nowhere else.

**List pane.** A search field, a connected button group to narrow by kind
(All, Machines, Servers, Consoles), and status chips that are filters, never
decoration: "2 running" shows the running ones. Rows are grouped (Pinned, then
by kind or by the server groups you set). A row is 64 dp: the seal, the name,
one line of facts, a small sparkline when the thing is live, and the primary
action. Jobs in progress dock at the bottom of the pane with a wavy progress
line, so a download stays visible wherever you are.

**Detail pane.** A header with the seal at 56 dp, the name, a state pill, one
line of facts and the actions: a split button for the primary action (Connect
with Terminal, Files, Run a command; Start with Window, SPICE, Headless or
Disposable), tonal buttons for the next two, an overflow menu for the rest.
Destructive actions only live in the overflow and always confirm with the
resource's name. Tabs under the header, at most five per kind.

**Nothing selected.** The detail pane shows the harbour: one plain sentence
about the fleet in display type ("Everything is up" or "mika-server needs its
password"), then what needs you, what is running now with its live seal and
sparkline, jobs in progress, and recent activity.

**Activity.** Jobs in progress with cancel, things that need you with their
fix next to them (save a password, open the Hub's GPU page), and a history
grouped by day that you can filter by resource.

**Settings.** Cards for appearance, new machine defaults, the VM engine, GPU
passthrough, SSH keys, updates and about. Each control matches its value:
switches for on and off, button groups for a few choices, steppers for
numbers.

**New.** The New button opens a menu of what you can create: a machine from
the catalogue, an instant cloud machine, a machine from an ISO, a passthrough
machine (only when the host supports it), a server, a console. The catalogue
takes the content area with a searchable grid of systems. Everything else is a
sheet with only the fields that matter up front and an Advanced section folded
shut.

## Look

Ryoku's paper and ink, brought forward. The base is the live Ryoku palette, so
Ryoport always matches the desktop and changes with the wallpaper. Bone (the
inverse surface) marks the one primary action and the selection. Colour only
ever means state or data. From Material 3 Expressive, the design language of
Android 16 and 17, Ryoport takes the grammar and leaves the bubbles: tonal
surfaces instead of boxes drawn with hairlines, a real shape scale, connected
button groups, split buttons, a FAB menu, spring motion, and the wavy progress
line.

### The seal

The one bold thing. Every resource has a seal, a small shape holding its
system logo or icon. Its shape is its state, and it morphs when the state
changes, the way Android 17's expressive shapes do. It is also a nod to the
hanko seal that has always been part of Ryoku.

| State | Shape | Ink |
|---|---|---|
| stopped, offline, unknown | soft square | raised surface, dim icon |
| running, online | cookie, eight lobes | ok |
| starting, stopping, connecting, creating | soft burst, turning slowly | busy |
| needs a password, degraded | clover, four lobes | warn |
| failed, down when it should be up | soft square | bad |
| paused | circle | warn |

The burst is the only shape that moves on its own, and only while something is
actually happening. Reduced motion stops it.

### Tokens

Colour roles map onto the Material roles Ryoku writes to
`~/.cache/ryoku/colors.json`. When that file is missing Ryoport uses Ryoku's
signature palette: black paper, bone ink `#cdc4ba`, seal red `#e2342a`.

| Role | From | Used for |
|---|---|---|
| bg | surface | the window |
| panel | surfaceContainerLow | list pane, cards |
| raised | surfaceContainer | hover, idle seals |
| overlay | surfaceContainerHigh | menus, sheets, palette |
| selected | surfaceContainerHighest | selected row |
| ink, inkDim, inkFaint | onSurface, onSurfaceVariant | text, from strong to quiet |
| line | outlineVariant | the few hairlines left (tables, dividers) |
| bone, onBone | inverseSurface, inverseOnSurface | primary action, selection, rail indicator |
| accent | primary | focus ring, chart lines, links |
| ok, warn, bad, busy | fixed inks tuned for light and dark | state only |

State inks are muted, closer to printing inks than to traffic lights: sage
`#7fb59a`, ochre `#d9a65b`, seal red `#e2342a`, steel `#8ea6c9` on dark
paper, deeper versions of each on light paper.

Type is Space Grotesk for everything a person reads and Space Mono for
literal data (addresses, ports, paths, commands). Sizes in sp: display 40,
headline 26, title 18, row 15, body 14, label 13, caption 12, group 11. No
all-caps labels.

Spacing stays on Ryoku's scale: 4, 8, 12, 16, 24, 32, 48. Corner radii carry
hierarchy: 8 for chips and fields, 12 for menus, 16 for rows, cards and the
detail header, 24 for sheets and the palette, full for buttons, the search
field and status pills. Buttons press from full to 12, the small physical
squeeze of Material 3 Expressive.

Elevation is tone. Only things that float (menus, sheets, the palette, the
FAB) get a soft shadow.

## Motion and battery

Motion answers what you did. Indicators slide on a spring, a pressed button
squeezes, a seal morphs when its state changes, a pane slides 16 dp as it
swaps. Nothing animates on its own except the busy seal and the wavy progress
line, and only while their job runs. Reduced motion (Ryoku's
`theme.motion.reduce` or the app setting) makes every transition instant.

Polling follows what is on screen.

| Source | Focused | Visible, not focused | Hidden |
|---|---|---|---|
| machine list | on file change, 10 s safety | same | 60 s |
| stats of a running machine on screen | 2 s | 5 s | off |
| sparklines in the list | 5 s | 10 s | off |
| server reachability | 15 s | 30 s | off |
| server probes | 30 s, 10 s for the selected one | 60 s | off |
| web app checks | 60 s | off | off |
| passthrough status | 10 s | 30 s | off |

On battery every interval doubles. Jobs report their own progress whatever is
on screen.

## Copy

Sentence case, plain verbs, the same word through the whole flow: a button
that says Start produces "Starting" and then "Running". Errors say what
happened and what to do ("ally refused the key, save a password or deploy a
key"). Empty states offer the next step ("No machines yet. Create one from the
catalogue or import an ISO"). The harbour vocabulary from the old Ryoport
stays where it helps (the seal, the harbour view) and goes where it got in the
way.

## Keyboard

| Keys | Does |
|---|---|
| Ctrl K | command palette: every resource and action |
| / | search the list |
| Up, Down | move through the list |
| Enter | the row's primary action |
| Ctrl Enter | open the detail of the selected row |
| Ctrl N | the New menu |
| Ctrl 1, 2, 3 | Fleet, Activity, Settings |
| Ctrl R | refresh now |
| Esc | close the top overlay, then clear the selection |
| Ctrl Q | quit |
