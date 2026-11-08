# Ryoport architecture

One Go binary, `ryoport`, holds the window and everything behind it. The UI is
Gio; the backend is plain Go packages the UI calls in-process. The old Ryoport
was a Quickshell app that shelled out to a Bash engine and three helper
binaries and parsed their JSON on every refresh. That loop is gone, but the
command names and their JSON stay, so scripts and Ryoku's command catalogue
keep working.

## Packages

| Package | Owns |
|---|---|
| `cmd/ryoport` | `main`: dispatch on `argv[0]` and the first argument, single instance, start the window |
| `internal/model` | shared types: resource IDs, kinds, states, jobs, events, alerts, metric series |
| `internal/qconf` | quickemu `.conf` files: read and write `key="value"` and `key=("a" "b")` lines without ever sourcing them |
| `internal/xdg` | every path Ryoport reads or writes |
| `internal/config` | app settings, and a one-time import of the old `ryoku/ryovm.json` defaults |
| `internal/vm` | quickemu machines: list, get, launch, stop, viewer, delete, config, snapshots, rename, resize, seal, templates, spawn, ssh, USB, port forwards, caps |
| `internal/qmp` | the QEMU monitor and guest agent: stats, power, balloon, CPU pinning |
| `internal/build` | making machines: quickget catalogue, create, import an ISO, instant cloud machines, the VirtIO driver CD, OS logos |
| `internal/fetch` | ranged parallel downloads with progress |
| `internal/passthrough` | Looking Glass machines through `ryoku-hub gpu vm` and `virsh` |
| `internal/remote` | SSH servers: ssh config with the managed include and sidecar, ping, probe, web app checks, Proxmox, connect, keys, tunnels |
| `internal/console` | serial, Telnet, RDP and VNC profiles |
| `internal/keyring` | Secret Service through `secret-tool` |
| `internal/launch` | terminals, viewers, file managers, URLs, detached processes |
| `internal/palette` | the live Ryoku palette: load, watch, fall back to the signature palette |
| `internal/update` | the release check and self-update |
| `internal/events` | the activity log, kept as JSON lines |
| `internal/store` | the app state the UI reads: one snapshot, pollers, actions, jobs, alerts |
| `internal/cli` | the `ryoport`, `ryovm`, `ryossh`, `ryovm-mon` and `ryovm-fetch` verbs and their JSON |
| `ui/theme` | tokens, embedded fonts, generated icons, motion |
| `ui/kit` | components |
| `ui/app` | window, rail, routing, overlays, keyboard, adaptive layout |
| `ui/pages/...` | fleet, harbour overview, machine, server, console, passthrough, create, activity, settings |

Backends never import `ui`. The UI never runs a backend function on its event
loop goroutine: it asks the store, the store does the work on a worker and
publishes a new snapshot, then wakes the window with `Invalidate`.

## Data flow

```
pollers / jobs  --update-->  store  --Snapshot()-->  frame
                                ^                      |
                                +------ actions -------+
```

The store keeps one immutable snapshot and swaps it whole after every change,
so a frame reads a consistent picture without locks. The UI tells the store
what is on screen (focused, visible, selected resource, page); the pollers use
that to pick their cadence, as in the table in `design.md`.

## Compatibility

Existing data keeps working untouched:

| What | Where |
|---|---|
| quickemu machines | `${RYOVM_HOME:-$XDG_DATA_HOME/ryoku/vms}/<name>.conf` and `<name>/` |
| shared images and templates | `<vm root>/.images/` |
| burn key | `$XDG_DATA_HOME/ryoku/keys/ryovm` |
| catalogue and logo cache | `$XDG_CACHE_HOME/ryoku/ryovm-catalog.json`, `ryovm-icons/` |
| SSH servers | `~/.ssh/config`, `~/.ssh/config.d/ryoport`, `$XDG_CONFIG_HOME/ryoku/ryoport/remotes.json` |
| consoles | `$XDG_CONFIG_HOME/ryoku/ryoport/consoles.json` |
| tunnels, askpass | `$XDG_STATE_HOME/ryoku/ryoport/` |
| saved passwords | Secret Service attributes `ryoport-alias` and `ryoport-console` |
| passthrough machines | owned by the Hub: `${RYOKU_VM_ROOT:-$XDG_DATA_HOME/ryoku/vm}`, libvirt domains `ryoku-<name>` |

New files live next to them: settings in
`$XDG_CONFIG_HOME/ryoku/ryoport/settings.json`, the activity log in
`$XDG_STATE_HOME/ryoku/ryoport/events.jsonl`.

Commands keep their names and output. The package installs `/usr/bin/ryoport`
and the links `ryovm`, `ryossh`, `ryovm-mon` and `ryovm-fetch` to it;
`internal/cli` picks the tool from `argv[0]`. `ryoport` with no arguments opens
the window, `ryoport <verb>` runs a verb, and `ryoport <name>` still starts
that passthrough machine and opens Looking Glass.

## Ryoku without Ryoku

Ryoport runs on any Linux desktop. What Ryoku adds is detected, never assumed:

| Ryoku piece | When present | When missing |
|---|---|---|
| `~/.cache/ryoku/colors.json` | follows the live palette | signature palette |
| `ryoku-hub gpu vm` | passthrough machines | the kind is hidden |
| `ryoku-host pkg add` | one-click engine install | the distro command to run |
| `ryoku-summon` | used by Ryoku's keybind to focus the window | not needed |

## Window identity

App ID `dev.ryoku.ryoport`, title `Ryoport`, desktop file
`dev.ryoku.ryoport.desktop`, icon `dev.ryoku.ryoport`. Terminals Ryoport opens
keep the classes `dev.ryoku.ryoport_ssh` and `dev.ryoku.ryoport_console`.
A second launch hands its arguments to the running instance over
`$XDG_RUNTIME_DIR/ryoport.sock` and exits.

## Releases

Every push to `main` releases, like Ryotunes. `scripts/next-version.sh`
reserves the next `v1.<minor>.<patch>` tag (patch is one digit, 1.0.9 rolls to
1.1.0), the version lives in `internal/version` next to an embedded copy of
`CHANGELOG.md`, and the release carries:

| Asset | For |
|---|---|
| `ryoport-<v>-1-x86_64.pkg.tar.zst` and `.sha256` | Arch and CachyOS, pacman identity `ryoport <v>-1` |
| `ryoport-<v>-linux-x86_64.tar.gz` and `.sha256` | any distro, a user install under `~/.local` |
| `ryoport-<v>.tar.gz` and `.sha256` | source with `vendor/`, built by the Fedora, Void and Nix lanes |

The tarball holds `bin/ryoport`, the four links,
`share/applications/dev.ryoku.ryoport.desktop`,
`share/icons/hicolor/scalable/apps/dev.ryoku.ryoport.svg`,
`share/metainfo/dev.ryoku.ryoport.metainfo.xml` and `share/licenses/ryoport/`.

## Updates

| Installed as | Who updates it |
|---|---|
| pacman package | the app: download, check the sha256 and the package identity, `pkexec pacman -U` the staged file; `ryoku update` does the same on Ryoku |
| user tarball | the app: download, check the sha256, swap `~/.local` in place |
| RPM, XBPS, Nix | the system package manager; the app says so and links the release |
| a source build | nobody; the app says it can't update itself |

The check reads `https://api.github.com/repos/ryoku-dev/ryoport/releases/latest`
at most once a day and when you ask. An install never restarts anything from a
package hook: the app relaunches itself after the transaction returns.
