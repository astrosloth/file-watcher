# File Watcher (Go Edition)

A high-performance, cross-platform daemon and CLI tool written in **Go** that watches directories for matching files and automatically processes or moves them to destination folders.

Supports **Windows**, **Linux**, and **macOS**.

---

## Features

- **Cross-Platform**: Built with native Go system APIs (`fsnotify`), supporting Windows, macOS, and Linux out-of-the-box without requiring external tools.
- **Event-Driven & Polling Fallback**: Uses OS file system events (`inotify` / ReadDirectoryChangesW / FSEvents) with configurable polling fallback mode.
- **Brace Expansion Globbing**: Advanced pattern matching supporting brace expansion (e.g., `*.{jpg,jpeg,png,gif,webp}`).
- **Single-File Archive Extraction**: Pure Go stream extraction for `.zip`, `.tar`, `.tar.gz`, `.tgz`, `.tar.bz2`, `.tbz`, `.tar.xz`, `.txz`, `.7z`, `.rar` archives containing a single matching file.
- **Duplicate Filename Resolution**: Automatically appends counters (`file_1.pdf`, `file_2.pdf`) to avoid overwriting existing files.
- **File Write Stabilization / Debouncing**: Built-in debouncer to prevent moving incomplete/partial writes (browser downloads, large file transfers).
- **Structured Log Rotation (`log/slog`)**: Size-based automatic log file rotation.
- **Multi-Watch INI Configuration**: Manage multiple directory watches in a single config file.

---

## Installation & Autostart Setup

### Automated Installation (User Autostart)

`file-watcher` can be installed as an automatic background service for your user account (no Administrator/root privileges required).

**Windows:**
Right-click `install.ps1` and select **Run with PowerShell**, or run in PowerShell:
```powershell
.\install.ps1
```

**Linux / macOS / CLI:**
```bash
go build -o file-watcher ./cmd/file-watcher
./file-watcher install
```

This will:
- Copy the executable to `%LOCALAPPDATA%\file-watcher\` (Windows) or `~/.local/bin/` (Linux/macOS).
- Create a starter configuration file at `~/.config/file-watcher/file-watcher.conf` if one does not exist.
- Register user autostart (`HKCU Registry` on Windows, `systemd --user` on Linux, `launchd` on macOS).
- Launch the background service immediately.

---

### Updating `file-watcher`

To update `file-watcher` to a new version, simply re-run the installation step:

**Windows:**
```powershell
.\install.ps1
```

**Linux / macOS / CLI:**
```bash
./file-watcher install
```

The installer will automatically stop any running background instance, overwrite the executable, preserve your existing configuration file, and restart the background service.

---

### Service Management

| Action | Command |
|---|---|
| **Check Status** | `file-watcher status` |
| **Install / Update** | `file-watcher install` |
| **Uninstall** | `file-watcher uninstall` |

---

### Building Binary Manually

```powershell
go build -o file-watcher.exe ./cmd/file-watcher
```
On Linux/macOS:
```bash
go build -o file-watcher ./cmd/file-watcher
```

## Usage

### Single Watch Mode

```bash
file-watcher <watch_dir> <pattern> <dest_dir> [options]
```

### Config File Mode (Multiple Watches)

```bash
file-watcher --config <config_file> [options]
```

### Command Line Options

| Option                  | Description                                                                     |
|-------------------------|---------------------------------------------------------------------------------|
| `--config <path>`       | Use config file for multiple watches                                           |
| `--poll-interval <sec>` | Polling interval in seconds (default: 5)                                        |
| `--use-polling`         | Force polling mode instead of event-driven fsnotify                            |
| `--extract-archives`    | Extract matching file from single-file archives (.zip, .tar.gz, etc.)           |
| `--daemon`              | Run in background mode (creates PID and log files)                              |
| `--pid-file <path>`     | Write process PID to file                                                      |
| `--log-file <path>`     | Write structured logs to file                                                   |
| `--max-log-size <bytes>`| Max log size before size rotation (default: 1048576 = 1MB)                      |
| `--help`, `-h`          | Show help message                                                               |
| `--version`, `-v`       | Show version                                                                    |

---

## Configuration File Format

Create a configuration file (e.g. `file-watcher.conf`):

```ini
[watch:pdfs]
dir = ~/Downloads
pattern = *.pdf
dest = ~/Documents/PDFs
extract-archives = true

[watch:images]
dir = ~/Downloads
pattern = *.{jpg,jpeg,png,gif,webp}
dest = ~/Pictures

[watch:reports]
dir = C:/tmp
pattern = report_*.csv
dest = ~/Reports
poll-interval = 10
```

---

## Examples

### Watch Downloads for PDFs and move to Documents
```bash
file-watcher ~/Downloads "*.pdf" ~/Documents/PDFs
```

### Move Images with Brace Pattern
```bash
file-watcher ~/Downloads "*.{jpg,jpeg,png,gif}" ~/Pictures
```

### Auto-Extract Single-File Archives
```bash
file-watcher ~/Downloads "*.pdf" ~/Documents/PDFs --extract-archives
```

### Multi-Watch Configuration
```bash
file-watcher --config file-watcher.conf
```

---

## Development & Quality Checks

Run full static analysis, code formatting check, unit tests with coverage report, and compile the binary:

**Windows (PowerShell):**
```powershell
.\check.ps1
```

**Linux / macOS (Bash):**
```bash
./check.sh
```

**Fish Shell:**
```fish
./check.fish
```

Or run standard Go tests directly:
```bash
go test -v ./...
```

---

## License

MIT
