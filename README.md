# Ryoport

[![Release](https://img.shields.io/github/v/release/ryoku-dev/ryoport)](https://github.com/ryoku-dev/ryoport/releases/latest)
[![License](https://img.shields.io/badge/license-GPL--3.0--or--later-blue)](LICENSE)

Ryoport is Ryoku's fleet dashboard. It puts local quickemu machines, GPU-passthrough machines, SSH servers, and serial, Telnet, RDP and VNC consoles in one window.

It can create and operate machines, open remote sessions, show health and resource history, manage long-running jobs, and record activity. The native Go and Gio app follows the live Ryoku palette and does not need Quickshell.

## Install

### Ryoku

Ryoport comes with Ryoku from its v2 line, selected by default in the installer. If you skipped or removed it, run:

```sh
sudo ryoku-host pkg install ryoport
```

### Arch Linux and CachyOS

Download the `ryoport-<version>-1-x86_64.pkg.tar.zst` package and its checksum from the latest release, then run:

```sh
sha256sum -c ryoport-*-1-x86_64.pkg.tar.zst.sha256
sudo pacman -U ./ryoport-*-1-x86_64.pkg.tar.zst
```

### Other Linux distributions

Download `ryoport-<version>-linux-x86_64.tar.gz` and its checksum from the latest release, then run:

```sh
sha256sum -c ryoport-*-linux-x86_64.tar.gz.sha256
mkdir -p ~/.local
tar -C ~/.local -xzf ryoport-*-linux-x86_64.tar.gz
```

Ryoport updates this kind of installation itself. An install made from the 1.0.0 tarball can't, because that tarball has no install marker, so extract a newer tarball over it once.

Ryoku v2 also carries Fedora, Void Linux and Nix packages in its repositories. Those builds update through their system package manager.

## Build from source

Ryoport requires Go 1.26 and the development headers for Wayland, X11, EGL, Vulkan and libxkbcommon.

```sh
make build
sudo make SKIP_BUILD=1 install
```

The package installs `ryoport` and the compatibility command names `ryovm`, `ryossh`, `ryovm-mon` and `ryovm-fetch`.

## License

Ryoport is licensed under GPL-3.0-or-later.
