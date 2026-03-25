# File Watcher

A simple user-mode daemon (fish shell script) that watches a directory for files matching a pattern and automatically moves them to a destination.

## Features

- Watch any directory for new files
- Match files using glob patterns (e.g., `*.pdf`, `*.{jpg,png,gif}`, `report_*.csv`)
- Automatically move matching files to a destination directory
- Uses `inotifywait` for efficient watching (with polling fallback)
- Handles duplicate filenames by appending a counter
- Can run as a foreground process or daemon
- Optional logging to file

## Requirements

- Fish shell (version 3.0+)
- Optional: `inotify-tools` for efficient file system watching

```bash
# Install inotify-tools (recommended)
sudo apt install inotify-tools   # Debian/Ubuntu
sudo dnf install inotify-tools   # Fedora
sudo pacman -S inotify-tools     # Arch
```

## Usage

### Single Watch Mode

```bash
./file-watcher.fish <watch_dir> <pattern> <dest_dir> [options]
```

### Config File Mode (Multiple Watches)

```bash
./file-watcher.fish --config <config_file> [options]
```

### Arguments (Single Watch Mode)

| Argument    | Description                                          |
|-------------|------------------------------------------------------|
| `watch_dir` | Directory to watch for new files                     |
| `pattern`   | Glob pattern to match (e.g., `*.pdf`, `*.{jpg,png}`) |
| `dest_dir`  | Destination directory for matched files              |

### Options

| Option                      | Description                                |
|-----------------------------|--------------------------------------------|
| `--config <path>`           | Use config file for multiple watches       |
| `--poll-interval <sec>`     | Polling interval in seconds (default: 5)   |
| `--use-polling`             | Force polling mode instead of inotify      |
| `--daemon`                  | Run in background (daemonize)              |
| `--pid-file <path>`         | Write PID to file (default in daemon mode: `~/.local/state/file-watcher/file-watcher.pid`) |
| `--log-file <path>`         | Write logs to file (default in daemon mode: `~/.local/state/file-watcher/file-watcher.log`) |
| `--max-log-size <bytes>`    | Max log file size before rotation (default: 1048576 = 1MB). Keeps one backup (.log.1) |
| `--extract-archives`        | If archive contains only one file and it matches, extract it (supports zip, tar, tar.gz, tar.bz2, tar.xz) |
| `--help`                    | Show help message                          |
| `--version`                 | Show version                               |

## Config File Format

Create a config file with multiple `[watch:name]` sections:

```ini
# ~/.config/file-watcher/config

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
dir = /tmp
pattern = report_*.csv
dest = ~/Reports
poll-interval = 10
```

### Config Options Per Watch

| Option              | Description                              |
|---------------------|------------------------------------------|
| `dir`               | Directory to watch (required)            |
| `pattern`           | Glob pattern to match (required)         |
| `dest`              | Destination directory (required)         |
| `extract-archives`  | Extract from single-file archives (true/false) |
| `poll-interval`     | Polling interval in seconds              |
| `use-polling`       | Force polling mode (true/false)          |

## Examples

### Move PDFs from Downloads to Documents

```bash
./file-watcher.fish ~/Downloads "*.pdf" ~/Documents/PDFs
```

### Move images to Pictures folder

```bash
./file-watcher.fish ~/Downloads "*.{jpg,jpeg,png,gif,webp}" ~/Pictures
```

### Watch for CSV reports and run as daemon

```bash
# Simple - uses default pid/log files in ~/.local/state/file-watcher/
./file-watcher.fish /tmp "report_*.csv" ~/Reports --daemon

# With custom paths
./file-watcher.fish /tmp "report_*.csv" ~/Reports \
    --daemon \
    --pid-file /tmp/file-watcher.pid \
    --log-file ~/file-watcher.log
```

### Force polling mode with 10-second interval

```bash
./file-watcher.fish ~/Downloads "*.pdf" ~/Documents/PDFs \
    --use-polling \
    --poll-interval 10
```

### Extract PDFs from archives

If an archive (zip, tar.gz, etc.) contains only a single file and it matches the pattern, extract it:

```bash
./file-watcher.fish ~/Downloads "*.pdf" ~/Documents/PDFs --extract-archives
```

### Run multiple watches with config file

```bash
./file-watcher.fish --config ~/.config/file-watcher/config --daemon
```

## Stopping the Daemon

If running in foreground: Press `Ctrl+C`

If running as daemon:
```bash
kill $(cat ~/.local/state/file-watcher/file-watcher.pid)
```

## Autostart on Login (systemd)

1. **Create and edit your config file**:
   ```bash
   mkdir -p ~/.config/file-watcher
   cp file-watcher.conf ~/.config/file-watcher/config
   vim ~/.config/file-watcher/config
   ```

2. **Install the service**:
   ```bash
   mkdir -p ~/.config/systemd/user
   cp file-watcher.service ~/.config/systemd/user/
   ```

3. **Enable and start**:
   ```bash
   systemctl --user daemon-reload
   systemctl --user enable file-watcher
   systemctl --user start file-watcher
   ```

4. **Check status**:
   ```bash
   systemctl --user status file-watcher
   ```

5. **View logs**:
   ```bash
   journalctl --user -u file-watcher -f
   ```

To stop/disable:
```bash
systemctl --user stop file-watcher
systemctl --user disable file-watcher
```

To reload after config changes:
```bash
systemctl --user restart file-watcher
```

## How It Works

1. **inotify mode** (default): Uses Linux's inotify subsystem via `inotifywait` to efficiently detect when files are created or moved into the watched directory. This is instant and uses minimal CPU.

2. **Polling mode** (fallback): If `inotify-tools` is not installed, the script falls back to polling the directory at regular intervals.

When a matching file is detected:
- The script waits briefly (0.5s) to ensure the file is fully written
- Moves the file to the destination directory
- If a file with the same name exists, appends a counter (e.g., `file_1.pdf`, `file_2.pdf`)

## License

MIT
