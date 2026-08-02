#!/usr/bin/env fish
#
# file-watcher.fish - A simple user-mode daemon that watches a directory
# for files matching a pattern and moves them to a destination.
#
# Usage:
#   ./file-watcher.fish <watch_dir> <pattern> <dest_dir> [--poll-interval <seconds>]
#
# Examples:
#   ./file-watcher.fish ~/Downloads "*.pdf" ~/Documents/PDFs
#   ./file-watcher.fish ~/Downloads "*.{jpg,png,gif}" ~/Pictures
#   ./file-watcher.fish /tmp "report_*.csv" ~/Reports --poll-interval 10
#

set -l VERSION "1.0.0"
set -g SCRIPT_PATH (realpath (status filename))

function print_usage
    echo "file-watcher v$VERSION"
    echo ""
    echo "Usage: file-watcher.fish [options] <watch_dir> <pattern> <dest_dir>"
    echo "       file-watcher.fish [options] --config <config_file>"
    echo ""
    echo "Arguments (single watch mode):"
    echo "  watch_dir    Directory to watch for new files"
    echo "  pattern      Glob pattern to match (e.g., '*.pdf', '*.{jpg,png}')"
    echo "  dest_dir     Destination directory for matched files"
    echo ""
    echo "Options:"
    echo "  --config <path>        Use config file for multiple watches"
    echo "  --poll-interval <sec>  Polling interval in seconds (default: 5)"
    echo "  --use-polling          Force polling mode instead of inotify"
    echo "  --daemon               Run in background (daemonize)"
    echo "  --pid-file <path>      Write PID to file (default in daemon mode:"
    echo "                         ~/.local/state/file-watcher/file-watcher.pid)"
    echo "  --log-file <path>      Write logs to file (default in daemon mode:"
    echo "                         ~/.local/state/file-watcher/file-watcher.log)"
    echo "  --max-log-size <bytes> Max log file size before rotation (default: 1048576 = 1MB)"
    echo "  --extract-archives     Extract if archive has exactly one file and it matches"
    echo "  --help                 Show this help message"
    echo "  --version              Show version"
end

function rotate_log
    if not set -q LOG_FILE; or test -z "$LOG_FILE"
        return
    end
    
    if not test -f "$LOG_FILE"
        return
    end
    
    set -l size (stat -c %s "$LOG_FILE" 2>/dev/null; or echo 0)
    if test $size -ge $MAX_LOG_SIZE
        # Keep one backup
        mv "$LOG_FILE" "$LOG_FILE.1" 2>/dev/null
    end
end

function log_msg
    set -l timestamp (date "+%Y-%m-%d %H:%M:%S")
    set -l msg "[$timestamp] $argv"
    
    if set -q LOG_FILE; and test -n "$LOG_FILE"
        rotate_log
        echo $msg >> $LOG_FILE
    else
        echo $msg
    end
end

function cleanup
    log_msg "Shutting down file-watcher..."
    if set -q PID_FILE; and test -f $PID_FILE
        rm -f $PID_FILE
    end
    exit 0
end

function move_file
    set -l src_file $argv[1]
    set -l dest_dir $argv[2]
    
    if not test -f "$src_file"
        return 1
    end
    
    set -l basename (basename "$src_file")
    set -l dest_file "$dest_dir/$basename"
    
    # Handle duplicate filenames
    if test -f "$dest_file"
        set -l name (string replace -r '\.[^.]*$' '' "$basename")
        set -l ext (string match -r '\.[^.]*$' "$basename")
        set -l counter 1
        
        while test -f "$dest_file"
            set dest_file "$dest_dir/"$name"_"$counter$ext
            set counter (math $counter + 1)
        end
    end
    
    if mv "$src_file" "$dest_file" 2>/dev/null
        log_msg "Moved: $basename -> $dest_file"
        return 0
    else
        log_msg "ERROR: Failed to move $basename"
        return 1
    end
end

function is_archive
    set -l file $argv[1]
    set -l ext (string lower (string match -r '\.[^.]+$' "$file"))
    
    switch $ext
        case .zip .tar .tgz .tbz .tbz2 .txz .7z .rar
            return 0
        case .gz .bz2 .xz
            # Check if it's a .tar.gz, .tar.bz2, etc.
            if string match -q '*.tar.*' "$file"
                return 0
            end
            return 1
        case '*'
            return 1
    end
end

function list_archive_contents
    set -l file $argv[1]
    set -l ext (string lower "$file")
    
    if string match -q '*.zip' "$ext"
        unzip -Z1 "$file" 2>/dev/null
    else if string match -q '*.7z' "$ext"
        7z l "$file" 2>/dev/null
    else if string match -q '*.rar' "$ext"
        unrar l "$file" 2>/dev/null
    else if string match -q '*.tar' "$ext"
        tar -tf "$file" 2>/dev/null
    else if string match -q '*.tar.gz' "$ext"; or string match -q '*.tgz' "$ext"
        tar -tzf "$file" 2>/dev/null
    else if string match -q '*.tar.bz2' "$ext"; or string match -q '*.tbz' "$ext"; or string match -q '*.tbz2' "$ext"
        tar -tjf "$file" 2>/dev/null
    else if string match -q '*.tar.xz' "$ext"; or string match -q '*.txz' "$ext"
        tar -tJf "$file" 2>/dev/null
    end
end

function extract_file_from_archive
    set -l archive $argv[1]
    set -l target_file $argv[2]
    set -l dest_dir $argv[3]
    set -l ext (string lower "$archive")
    
    if string match -q '*.zip' "$ext"
        unzip -j -o "$archive" "$target_file" -d "$dest_dir" 2>/dev/null
    else if string match -q '*.7z' "$ext"
        7z e -o"$dest_dir" -y "$archive" "$target_file" 2>/dev/null
    else if string match -q '*.rar' "$ext"
        unrar e -y "$archive" "$target_file" "$dest_dir" 2>/dev/null
    else if string match -q '*.tar' "$ext"
        tar -xf "$archive" -C "$dest_dir" --strip-components=(string split '/' "$target_file" | count; math $x - 1) "$target_file" 2>/dev/null
        # Simpler approach: extract to temp and move
    else if string match -q '*.tar.gz' "$ext"; or string match -q '*.tgz' "$ext"
        tar -xzf "$archive" -C "$dest_dir" "$target_file" --strip-components=(math (string split '/' "$target_file" | count) - 1) 2>/dev/null
    else if string match -q '*.tar.bz2' "$ext"; or string match -q '*.tbz' "$ext"; or string match -q '*.tbz2' "$ext"
        tar -xjf "$archive" -C "$dest_dir" "$target_file" --strip-components=(math (string split '/' "$target_file" | count) - 1) 2>/dev/null
    else if string match -q '*.tar.xz' "$ext"; or string match -q '*.txz' "$ext"
        tar -xJf "$archive" -C "$dest_dir" "$target_file" --strip-components=(math (string split '/' "$target_file" | count) - 1) 2>/dev/null
    end
    
    return $status
end

function process_archive
    set -l archive $argv[1]
    set -l pattern $argv[2]
    set -l dest_dir $argv[3]
    
    if not is_archive "$archive"
        return 1
    end
    
    # List archive contents (files only, not directories)
    set -l contents (list_archive_contents "$archive")
    set -l files
    
    for entry in $contents
        # Skip directories (entries ending with /)
        if string match -q '*/' "$entry"
            continue
        end
        set -a files $entry
    end
    
    # Only process if archive contains exactly one file
    if test (count $files) -ne 1
        if test (count $files) -gt 1
            log_msg "Archive "(basename "$archive")" contains "(count $files)" files, skipping (need exactly 1)"
        end
        return 1
    end
    
    set -l single_file $files[1]
    set -l file_basename (basename "$single_file")
    
    # Check if the single file matches the pattern
    if not string match -q -- $pattern "$file_basename"
        return 1
    end
    
    log_msg "Archive "(basename "$archive")" contains single matching file: $file_basename"
    
    # Create temp dir for extraction
    set -l tmpdir (mktemp -d)
    
    if extract_file_from_archive "$archive" "$single_file" "$tmpdir"
        set -l extracted_file "$tmpdir/$file_basename"
        
        if test -f "$extracted_file"
            move_file "$extracted_file" "$dest_dir"
            rm -rf "$tmpdir"
            
            # Remove the archive after successful extraction
            rm -f "$archive"
            log_msg "Removed archive: "(basename "$archive")
            return 0
        end
    end
    
    rm -rf "$tmpdir"
    log_msg "ERROR: Failed to extract $file_basename from archive"
    return 1
end

function process_existing_files
    set -l watch_dir $argv[1]
    set -l pattern $argv[2]
    set -l dest_dir $argv[3]
    
    # Use find with the pattern converted to a regex-friendly format
    for file in $watch_dir/$pattern
        if test -f "$file"
            move_file "$file" "$dest_dir"
        end
    end
    
    # Check archives if enabled
    if test "$EXTRACT_ARCHIVES" = true
        for archive in $watch_dir/*.{zip,tar,tar.gz,tgz,tar.bz2,tbz,tbz2,tar.xz,txz,7z,rar}
            if test -f "$archive"
                process_archive "$archive" $pattern "$dest_dir"
            end
        end
    end
end

function watch_with_inotify
    set -l watch_dir $argv[1]
    set -l pattern $argv[2]
    set -l dest_dir $argv[3]
    
    log_msg "Starting inotify-based watcher..."
    log_msg "Watching: $watch_dir"
    log_msg "Pattern: $pattern"
    log_msg "Destination: $dest_dir"
    
    # Process any existing files first
    process_existing_files $watch_dir $pattern $dest_dir
    
    # Watch for new files using inotifywait
    inotifywait -m -e close_write -e moved_to --format '%f' "$watch_dir" 2>/dev/null | while read filename
        # Check if file matches pattern
        set -l full_path "$watch_dir/$filename"
        
        if test -f "$full_path"
            # Use fish's string match for glob pattern matching
            if string match -q -- $pattern "$filename"
                # Small delay to ensure file is fully written
                sleep 0.5
                move_file "$full_path" "$dest_dir"
            else if test "$EXTRACT_ARCHIVES" = true; and is_archive "$full_path"
                # Check if archive contains a single matching file
                sleep 0.5
                process_archive "$full_path" $pattern "$dest_dir"
            end
        end
    end
end

function watch_with_polling
    set -l watch_dir $argv[1]
    set -l pattern $argv[2]
    set -l dest_dir $argv[3]
    set -l interval $argv[4]
    
    log_msg "Starting polling-based watcher (interval: "$interval"s)..."
    log_msg "Watching: $watch_dir"
    log_msg "Pattern: $pattern"
    log_msg "Destination: $dest_dir"
    
    # Keep track of files we've seen
    set -l seen_files
    
    while true
        for file in $watch_dir/$pattern
            if test -f "$file"
                # Check if we've already processed this file
                if not contains "$file" $seen_files
                    set -a seen_files "$file"
                    move_file "$file" "$dest_dir"
                end
            end
        end
        
        # Check archives if enabled
        if test "$EXTRACT_ARCHIVES" = true
            for archive in $watch_dir/*.{zip,tar,tar.gz,tgz,tar.bz2,tbz,tbz2,tar.xz,txz,7z,rar}
                if test -f "$archive"
                    if not contains "$archive" $seen_files
                        set -a seen_files "$archive"
                        process_archive "$archive" $pattern "$dest_dir"
                    end
                end
            end
        end
        
        # Clean up seen_files list (remove entries for files that no longer exist)
        set -l new_seen
        for f in $seen_files
            if test -f "$f"
                set -a new_seen "$f"
            end
        end
        set seen_files $new_seen
        
        sleep $interval
    end
end

function parse_config
    set -l config_file $argv[1]
    
    if not test -f "$config_file"
        echo "ERROR: Config file not found: $config_file"
        return 1
    end
    
    set -l current_section ""
    set -l watches
    
    while read -l line
        # Skip empty lines and comments
        set line (string trim "$line")
        if test -z "$line"; or string match -q '#*' "$line"
            continue
        end
        
        # Check for section header [watch:name]
        if string match -qr '^\[watch:(.+)\]$' "$line"
            set current_section (string match -r '^\[watch:(.+)\]$' "$line")[2]
            set -a watches "$current_section"
            # Initialize defaults for this watch
            set -g "watch_"$current_section"_dir" ""
            set -g "watch_"$current_section"_pattern" ""
            set -g "watch_"$current_section"_dest" ""
            set -g "watch_"$current_section"_extract_archives" false
            set -g "watch_"$current_section"_poll_interval" 5
            set -g "watch_"$current_section"_use_polling" false
            continue
        end
        
        # Parse key = value
        if test -n "$current_section"
            set -l parts (string split -m 1 '=' "$line")
            if test (count $parts) -eq 2
                set -l key (string trim $parts[1])
                set -l value (string trim $parts[2])
                
                # Expand ~ to home directory
                set value (string replace '~' "$HOME" "$value")
                
                switch $key
                    case dir
                        set -g "watch_"$current_section"_dir" "$value"
                    case pattern
                        set -g "watch_"$current_section"_pattern" "$value"
                    case dest
                        set -g "watch_"$current_section"_dest" "$value"
                    case extract-archives
                        if test "$value" = true -o "$value" = yes -o "$value" = 1
                            set -g "watch_"$current_section"_extract_archives" true
                        end
                    case poll-interval
                        set -g "watch_"$current_section"_poll_interval" "$value"
                    case use-polling
                        if test "$value" = true -o "$value" = yes -o "$value" = 1
                            set -g "watch_"$current_section"_use_polling" true
                        end
                end
            end
        end
    end < "$config_file"
    
    # Return the list of watch names
    for w in $watches
        echo $w
    end
end

function watch_single
    # Run a single watch with its own settings
    set -l name $argv[1]
    set -l watch_dir $argv[2]
    set -l pattern $argv[3]
    set -l dest_dir $argv[4]
    set -l extract_archives $argv[5]
    set -l poll_interval $argv[6]
    set -l use_polling $argv[7]
    
    # Set global for this watch's extract setting
    set -g EXTRACT_ARCHIVES $extract_archives
    
    log_msg "[$name] Starting watch: $watch_dir -> $dest_dir (pattern: $pattern)"
    
    if test "$use_polling" = true; or not command -q inotifywait
        watch_with_polling "$watch_dir" "$pattern" "$dest_dir" $poll_interval
    else
        watch_with_inotify "$watch_dir" "$pattern" "$dest_dir"
    end
end

function run_config_watches
    set -l config_file $argv[1]
    set -l poll_interval_override $argv[2]
    set -l use_polling_override $argv[3]
    
    set -l watches (parse_config "$config_file")
    
    if test (count $watches) -eq 0
        echo "ERROR: No watches defined in config file"
        return 1
    end
    
    log_msg "Starting "(count $watches)" watch(es) from config: $config_file"
    
    set -l pids
    
    for watch_name in $watches
        set -l w_dir (eval echo \$watch_"$watch_name"_dir)
        set -l w_pattern (eval echo \$watch_"$watch_name"_pattern)
        set -l w_dest (eval echo \$watch_"$watch_name"_dest)
        set -l w_extract (eval echo \$watch_"$watch_name"_extract_archives)
        set -l w_poll_int (eval echo \$watch_"$watch_name"_poll_interval)
        set -l w_use_poll (eval echo \$watch_"$watch_name"_use_polling)
        
        # Apply overrides if set
        if test -n "$poll_interval_override"
            set w_poll_int $poll_interval_override
        end
        if test "$use_polling_override" = true
            set w_use_poll true
        end
        
        # Validate
        if test -z "$w_dir" -o -z "$w_pattern" -o -z "$w_dest"
            log_msg "ERROR: Watch '$watch_name' missing required fields (dir, pattern, dest)"
            continue
        end
        
        if not test -d "$w_dir"
            log_msg "ERROR: Watch '$watch_name' - directory does not exist: $w_dir"
            continue
        end
        
        # Create destination if needed
        if not test -d "$w_dest"
            mkdir -p "$w_dest"
            log_msg "[$watch_name] Created destination: $w_dest"
        end
        
        # Launch watch in background
        fish -c "
            set -g LOG_FILE '$LOG_FILE'
            set -g MAX_LOG_SIZE $MAX_LOG_SIZE
            set -g EXTRACT_ARCHIVES $w_extract
            source '$SCRIPT_PATH'
            watch_single '$watch_name' '$w_dir' '$w_pattern' '$w_dest' $w_extract $w_poll_int $w_use_poll
        " &
        set -a pids $last_pid
        log_msg "[$watch_name] Launched with PID: $last_pid"
    end
    
    # Wait for all watches (they run forever until killed)
    for pid in $pids
        wait $pid 2>/dev/null
    end
end

# Parse arguments
set -l watch_dir ""
set -l pattern ""
set -l dest_dir ""
set -l config_file ""
set -l poll_interval 5
set -l use_polling false
set -l daemonize false
set -g PID_FILE ""
set -g LOG_FILE ""
set -g MAX_LOG_SIZE 1048576  # 1MB default
set -g EXTRACT_ARCHIVES false

set -l i 1
while test $i -le (count $argv)
    switch $argv[$i]
        case --help -h
            print_usage
            exit 0
        case --version -v
            echo "file-watcher v$VERSION"
            exit 0
        case --poll-interval
            set i (math $i + 1)
            set poll_interval $argv[$i]
        case --use-polling
            set use_polling true
        case --daemon
            set daemonize true
        case --pid-file
            set i (math $i + 1)
            set PID_FILE $argv[$i]
        case --log-file
            set i (math $i + 1)
            set LOG_FILE $argv[$i]
        case --max-log-size
            set i (math $i + 1)
            set MAX_LOG_SIZE $argv[$i]
        case --config
            set i (math $i + 1)
            set config_file $argv[$i]
        case --extract-archives
            set EXTRACT_ARCHIVES true
        case '--*'
            echo "Unknown option: $argv[$i]"
            exit 1
        case '*'
            if test -z "$watch_dir"
                set watch_dir $argv[$i]
            else if test -z "$pattern"
                set pattern $argv[$i]
            else if test -z "$dest_dir"
                set dest_dir $argv[$i]
            end
    end
    set i (math $i + 1)
end

# Determine mode: config file or single watch
set -l use_config false
if test -n "$config_file"
    set use_config true
    if not test -f "$config_file"
        echo "ERROR: Config file not found: $config_file"
        exit 1
    end
else
    # Validate arguments for single watch mode
    if test -z "$watch_dir" -o -z "$pattern" -o -z "$dest_dir"
        print_usage
        exit 1
    end
    
    # Expand paths
    set watch_dir (realpath -m "$watch_dir" 2>/dev/null; or echo "$watch_dir")
    set dest_dir (realpath -m "$dest_dir" 2>/dev/null; or echo "$dest_dir")
    
    # Validate directories
    if not test -d "$watch_dir"
        echo "ERROR: Watch directory does not exist: $watch_dir"
        exit 1
    end
    
    # Create destination directory if it doesn't exist
    if not test -d "$dest_dir"
        if not mkdir -p "$dest_dir"
            echo "ERROR: Cannot create destination directory: $dest_dir"
            exit 1
        end
        echo "Created destination directory: $dest_dir"
    end
end

# Set default PID and log files for daemon mode
if test "$daemonize" = true
    set -l state_dir ~/.local/state/file-watcher
    
    if test -z "$PID_FILE"
        mkdir -p $state_dir
        set PID_FILE "$state_dir/file-watcher.pid"
    end
    
    if test -z "$LOG_FILE"
        mkdir -p $state_dir
        set LOG_FILE "$state_dir/file-watcher.log"
    end
end

# Set up signal handlers
trap cleanup SIGINT SIGTERM

# Daemonize if requested
if test "$daemonize" = true
    echo "Starting file-watcher daemon..."
    echo "  PID file: $PID_FILE"
    echo "  Log file: $LOG_FILE (max size: $MAX_LOG_SIZE bytes)"
    if test "$use_config" = true
        echo "  Config: $config_file"
    else if test "$EXTRACT_ARCHIVES" = true
        echo "  Archive extraction: enabled"
    end
    
    # Fork to background
    if test "$use_config" = true
        fish -c "
            set -g LOG_FILE '$LOG_FILE'
            set -g PID_FILE '$PID_FILE'
            set -g MAX_LOG_SIZE $MAX_LOG_SIZE
            
            if test -n '$PID_FILE'
                echo %self > '$PID_FILE'
            end
            
            source '$SCRIPT_PATH'
            run_config_watches '$config_file' '$poll_interval' '$use_polling'
        " &
    else
        fish -c "
            set -g LOG_FILE '$LOG_FILE'
            set -g PID_FILE '$PID_FILE'
            set -g MAX_LOG_SIZE $MAX_LOG_SIZE
            set -g EXTRACT_ARCHIVES $EXTRACT_ARCHIVES
            
            if test -n '$PID_FILE'
                echo %self > '$PID_FILE'
            end
            
            source '$SCRIPT_PATH'
            
            if test '$use_polling' = true; or not command -q inotifywait
                watch_with_polling '$watch_dir' '$pattern' '$dest_dir' $poll_interval
            else
                watch_with_inotify '$watch_dir' '$pattern' '$dest_dir'
            end
        " &
    end
    
    set -l daemon_pid $last_pid
    echo "Daemon started with PID: $daemon_pid"
    
    if test -n "$PID_FILE"
        echo $daemon_pid > $PID_FILE
    end
    
    exit 0
end

# Write PID file if specified
if test -n "$PID_FILE"
    echo %self > $PID_FILE
end

# Start watching
if test "$use_config" = true
    run_config_watches "$config_file" "$poll_interval" "$use_polling"
else if test "$use_polling" = true; or not command -q inotifywait
    if not command -q inotifywait; and test "$use_polling" = false
        log_msg "inotifywait not found, falling back to polling mode"
        log_msg "Install inotify-tools for better performance: sudo apt install inotify-tools"
    end
    watch_with_polling $watch_dir $pattern $dest_dir $poll_interval
else
    watch_with_inotify $watch_dir $pattern $dest_dir
end
