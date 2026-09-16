# fwatch

A lightweight, configurable file organizer that automatically moves files to designated folders based on their extensions.

## Features

- 🔍 Real-time file system monitoring using fsnotify
- ⚙️ YAML-based configuration
- 📁 Multiple file type routing rules
- 🔄 Automatic directory creation
- 🏷️ Handles duplicate filenames with timestamps
- 💾 Cross-filesystem move support (automatically handles moves between different devices/partitions)

## Installation

### Arch Linux (AUR)

```bash
# Using yay
yay -S fwatch-bin

# Or using paru
paru -S fwatch-bin
```

### Debian / Ubuntu

Download the `.deb` for your architecture from the [releases page](https://github.com/polarn/fwatch/releases) and install it:

```bash
sudo dpkg -i fwatch_*_amd64.deb
```

The package installs the binary to `/usr/bin/fwatch` and a systemd **user** unit to
`/usr/lib/systemd/user/fwatch.service`. Configure fwatch first, then enable the service:

```bash
mkdir -p ~/.config/fwatch
cp /usr/share/doc/fwatch/config.example.yaml ~/.config/fwatch/config.yaml
# edit ~/.config/fwatch/config.yaml, then:
systemctl --user enable --now fwatch.service
```

On a headless machine, also enable lingering so the service starts at boot and keeps running
after you log out — see [Run as Systemd Service](#run-as-systemd-service).

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/polarn/fwatch/releases).

```bash
# Example for Linux x86_64
wget https://github.com/polarn/fwatch/releases/latest/download/fwatch_Linux_x86_64.tar.gz
tar -xzf fwatch_Linux_x86_64.tar.gz
sudo mv fwatch /usr/local/bin/
```

### From Source

```bash
# Build the binary
go build -o fwatch

# Optional: Install to your PATH
sudo cp fwatch /usr/local/bin/
```

## Configuration

By default, fwatch looks for its configuration file at `~/.config/fwatch/config.yaml` (or `$XDG_CONFIG_HOME/fwatch/config.yaml` if set).

1. Create the config directory and copy the example configuration (installed from the deb or
   AUR package, the example is at `/usr/share/doc/fwatch/config.example.yaml`):
```bash
mkdir -p ~/.config/fwatch
cp config.example.yaml ~/.config/fwatch/config.yaml
```

2. Edit `~/.config/fwatch/config.yaml` to suit your needs:

```yaml
watch_dir: "/home/your_username/Downloads"
create_dirs: true

rules:
  - extensions: [".zip"]
    destination: "/home/your_username/zip-archives"
  - extensions: [".deb"]
    destination: "/home/your_username/debian"
```

## Usage

Run with default config location (`~/.config/fwatch/config.yaml`):
```bash
./fwatch
```

Use a custom config file:
```bash
./fwatch -config /path/to/config.yaml
```

## Run as Systemd Service

An example systemd service file (`fwatch.service`) is included. To install it:
```bash
# Copy binary and service file
sudo cp fwatch /usr/local/bin/
mkdir -p ~/.config/systemd/user
cp fwatch.service ~/.config/systemd/user/

# Reload systemd and enable service
systemctl --user daemon-reload
systemctl --user enable fwatch.service
systemctl --user start fwatch.service

# Check status
systemctl --user status fwatch.service

# View logs
journalctl --user -u fwatch.service -f
```

**Note:** The unit's `ExecStart` is `/usr/bin/fwatch`, where the deb and AUR packages install
the binary. If you installed manually to `/usr/local/bin`, edit `ExecStart` to match.

**Note:** User services only run while you have an active login session. On a server, or any
machine you log into rarely, enable lingering so fwatch starts at boot and survives logout:

```bash
sudo loginctl enable-linger $USER
```

**Note:** Make sure you've already configured fwatch (see [Configuration](#configuration) section above) before starting the service. fwatch exits if the config file is missing or invalid, and the unit restarts it every 10 seconds — `systemctl --user status fwatch.service` and the journal will show it flapping.

## Configuration Options

| Option | Type | Description |
|--------|------|-------------|
| `watch_dir` | string | Directory to monitor for new files |
| `rules` | array | List of file routing rules |
| `create_dirs` | bool | Auto-create destination directories |

## Example Use Cases

**For Downloads:**
```yaml
watch_dir: "/home/user/Downloads"
rules:
  - extensions: [".zip"]
    destination: "/home/user/zip-archives"
  - extensions: [".deb"]
    destination: "/home/user/debian"
```

**For Document Organization:**
```yaml
watch_dir: "/home/user/Documents/Inbox"
rules:
  - extensions: [".pdf"]
    destination: "/home/user/Documents/PDFs"
  - extensions: [".docx", ".doc"]
    destination: "/home/user/Documents/Word"
  - extensions: [".xlsx", ".xls"]
    destination: "/home/user/Documents/Spreadsheets"
```

## License

Apache 2.0
