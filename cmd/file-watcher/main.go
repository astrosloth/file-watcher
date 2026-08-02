// Package main provides the command-line interface entry point for file-watcher,
// supporting CLI arguments, multi-watch INI configuration files with live hot-reloading,
// daemon defaults, service auto-installation, and signal-based graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"file-watcher/pkg/config"
	"file-watcher/pkg/logger"
	"file-watcher/pkg/service"
	"file-watcher/pkg/watcher"

	"github.com/fsnotify/fsnotify"
)

const version = "1.0.0"

// main is the application entry point, delegating logic to runMain and exiting with its status code.
func main() {
	os.Exit(runMain())
}

// runMain parses command-line flags and subcommands, initializes loggers and PID files, starts watch instances,
// monitors configuration files for hot-reloading, and handles graceful shutdown upon receiving OS termination signals.
func runMain() int {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install":
			installFlags := flag.NewFlagSet("install", flag.ExitOnError)
			var cfgPath string
			installFlags.StringVar(&cfgPath, "config", "", "Path to custom configuration file")
			_ = installFlags.Parse(os.Args[2:])
			if err := service.Install(cfgPath); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Service installation failed: %v\n", err)
				return 1
			}
			return 0
		case "uninstall":
			if err := service.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Service uninstallation failed: %v\n", err)
				return 1
			}
			return 0
		case "status":
			st, err := service.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to retrieve status: %v\n", err)
				return 1
			}
			fmt.Println(st)
			return 0
		}
	}

	var (
		configFile      string
		pollIntervalSec int
		usePolling      bool
		extractArchives bool
		daemon          bool
		pidFile         string
		logFile         string
		maxLogSize      int64
		showHelp        bool
		showVersion     bool
	)

	flag.StringVar(&configFile, "config", "", "Use config file for multiple watches")
	flag.IntVar(&pollIntervalSec, "poll-interval", 5, "Polling interval in seconds (default: 5)")
	flag.BoolVar(&usePolling, "use-polling", false, "Force polling mode instead of inotify/fsnotify")
	flag.BoolVar(&extractArchives, "extract-archives", false, "Extract single-file archives that match pattern")
	flag.BoolVar(&daemon, "daemon", false, "Run in background mode")
	flag.StringVar(&pidFile, "pid-file", "", "Write PID to specified file")
	flag.StringVar(&logFile, "log-file", "", "Write logs to specified file")
	flag.Int64Var(&maxLogSize, "max-log-size", 1048576, "Max log file size before rotation in bytes (default: 1MB)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (short)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (short)")

	flag.Usage = printUsage
	flag.Parse()

	if showVersion {
		fmt.Printf("file-watcher v%s\n", version)
		return 0
	}

	if showHelp {
		printUsage()
		return 0
	}

	// Daemon mode background process detachment & default configuration
	if daemon {
		stateDir := config.ExpandPath("~/.local/state/file-watcher")
		if pidFile == "" {
			_ = os.MkdirAll(stateDir, 0755)
			pidFile = filepath.Join(stateDir, "file-watcher.pid")
		}
		if logFile == "" {
			_ = os.MkdirAll(stateDir, 0755)
			logFile = filepath.Join(stateDir, "file-watcher.log")
		}

		if os.Getenv("FILE_WATCHER_DAEMON_CHILD") != "1" {
			cmd := exec.Command(os.Args[0], os.Args[1:]...)
			cmd.Env = append(os.Environ(), "FILE_WATCHER_DAEMON_CHILD=1")

			if err := cmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to start daemon process: %v\n", err)
				return 1
			}

			fmt.Printf("Daemon started with PID: %d\n", cmd.Process.Pid)
			fmt.Printf("  PID file: %s\n", pidFile)
			fmt.Printf("  Log file: %s\n", logFile)
			return 0
		}
	}

	// Configure logger
	var appLogger *slog.Logger
	if logFile != "" {
		expandedLogFile := config.ExpandPath(logFile)
		fileLogger, err := logger.NewFileLogger(expandedLogFile, maxLogSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to initialize file logger: %v\n", err)
			return 1
		}
		appLogger = fileLogger
	} else {
		appLogger = logger.NewConsoleLogger(os.Stdout)
	}

	// Setup PID file
	if pidFile != "" {
		expandedPidFile := config.ExpandPath(pidFile)
		if err := writePidFile(expandedPidFile); err != nil {
			appLogger.Error("Failed to write PID file", "path", expandedPidFile, "error", err)
		} else {
			defer os.Remove(expandedPidFile)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pollInterval := time.Duration(pollIntervalSec) * time.Second
	_ = pollInterval

	var cfg *config.Config
	var expandedConfig string

	if configFile != "" {
		expandedConfig = config.ExpandPath(configFile)
		var err error
		cfg, err = config.LoadConfig(expandedConfig)
		if err != nil {
			appLogger.Error("Failed to load config file", "path", expandedConfig, "error", err)
			return 1
		}
	} else {
		args := flag.Args()
		if len(args) < 3 {
			printUsage()
			return 1
		}
		cfg = &config.Config{
			Watches: map[string]config.WatchConfig{
				"default": {
					Name:            "default",
					Dir:             config.ExpandPath(args[0]),
					Pattern:         args[1],
					Dest:            config.ExpandPath(args[2]),
					ExtractArchives: extractArchives,
					PollInterval:    pollIntervalSec,
					UsePolling:      usePolling,
				},
			},
		}
	}

	applyFlagOverrides(cfg, pollIntervalSec, usePolling, extractArchives)

	startWatches := func(parentCtx context.Context, c *config.Config) (context.CancelFunc, *sync.WaitGroup) {
		watchCtx, cancelFn := context.WithCancel(parentCtx)
		var wg sync.WaitGroup

		appLogger.Info("Starting file-watcher", "count", len(c.Watches))

		for _, wCfg := range c.Watches {
			w, err := watcher.New(watcher.Options{
				WatchDir:        wCfg.Dir,
				Pattern:         wCfg.Pattern,
				DestDir:         wCfg.Dest,
				ExtractArchives: wCfg.ExtractArchives,
				PollInterval:    time.Duration(wCfg.PollInterval) * time.Second,
				UsePolling:      wCfg.UsePolling,
				Logger:          appLogger.With("watch", wCfg.Name),
			})
			if err != nil {
				appLogger.Error("Failed to create watch", "watch", wCfg.Name, "error", err)
				continue
			}

			wg.Add(1)
			go func(w *watcher.Watcher, name string) {
				defer wg.Done()
				if err := w.Start(watchCtx); err != nil && err != context.Canceled {
					appLogger.Error("Watcher stopped with error", "watch", name, "error", err)
				}
			}(w, wCfg.Name)
		}
		return cancelFn, &wg
	}

	var mu sync.Mutex
	cancelCurrent, currentWg := startWatches(ctx, cfg)

	if expandedConfig != "" {
		cfgWatcher, err := fsnotify.NewWatcher()
		if err != nil {
			appLogger.Warn("Failed to create config file watcher, hot-reloading disabled", "error", err)
		} else {
			defer cfgWatcher.Close()
			if err := cfgWatcher.Add(expandedConfig); err != nil {
				appLogger.Warn("Failed to watch config file for changes", "path", expandedConfig, "error", err)
			} else {
				appLogger.Info("Config file hot-reloading active", "path", expandedConfig)

				go func() {
					var debounceTimer *time.Timer
					for {
						select {
						case <-ctx.Done():
							return
						case event, ok := <-cfgWatcher.Events:
							if !ok {
								return
							}
							if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
								if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
									_ = cfgWatcher.Add(expandedConfig)
								}
								if debounceTimer != nil {
									debounceTimer.Stop()
								}
								debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
									appLogger.Info("Config file modification detected, validating...", "path", expandedConfig)
									newCfg, err := config.LoadConfig(expandedConfig)
									if err != nil {
										appLogger.Error("Failed to reload config file (keeping existing configuration active)", "error", err)
										return
									}

									applyFlagOverrides(newCfg, pollIntervalSec, usePolling, extractArchives)

									appLogger.Info("Configuration reloaded successfully. Updating watch jobs...")
									mu.Lock()
									oldCancel := cancelCurrent
									oldWg := currentWg
									mu.Unlock()

									oldCancel()
									oldWg.Wait()

									mu.Lock()
									cancelCurrent, currentWg = startWatches(ctx, newCfg)
									mu.Unlock()
								})
							}
						case err, ok := <-cfgWatcher.Errors:
							if !ok {
								return
							}
							appLogger.Error("Config file watcher error", "error", err)
						}
					}
				}()
			}
		}
	}

	appLogger.Info("file-watcher process running. Press Ctrl+C to stop.")
	<-ctx.Done()
	appLogger.Info("Shutting down file-watcher gracefully...")
	mu.Lock()
	cancelCurrent()
	currentWg.Wait()
	mu.Unlock()
	appLogger.Info("Shutdown complete.")
	return 0
}

// applyFlagOverrides applies explicit CLI flag overrides to watch configurations.
func applyFlagOverrides(cfg *config.Config, pollIntervalSec int, usePolling, extractArchives bool) {
	if !isFlagPassed("poll-interval") && !isFlagPassed("use-polling") && !isFlagPassed("extract-archives") {
		return
	}
	for name, wCfg := range cfg.Watches {
		if isFlagPassed("poll-interval") {
			wCfg.PollInterval = pollIntervalSec
		}
		if isFlagPassed("use-polling") {
			wCfg.UsePolling = usePolling
		}
		if isFlagPassed("extract-archives") {
			wCfg.ExtractArchives = extractArchives
		}
		cfg.Watches[name] = wCfg
	}
}

// isFlagPassed checks if a specific flag name was explicitly passed in command-line arguments.
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// writePidFile creates any missing parent directories and writes the current process ID to pidFile.
func writePidFile(pidFile string) error {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// printUsage prints application usage instructions and flag defaults to stdout.
func printUsage() {
	fmt.Printf("file-watcher v%s\n\n", version)
	fmt.Println("Usage: file-watcher [options] <watch_dir> <pattern> <dest_dir>")
	fmt.Println("       file-watcher [options] --config <config_file>")
	fmt.Println("       file-watcher <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  install [--config <path>]  Install autostart background task/service")
	fmt.Println("  uninstall                  Remove autostart background task/service")
	fmt.Println("  status                     Show autostart task/service status")
	fmt.Println("\nArguments (single watch mode):")
	fmt.Println("  watch_dir    Directory to watch for new files")
	fmt.Println("  pattern      Glob pattern to match (e.g. '*.pdf', '*.{jpg,png}')")
	fmt.Println("  dest_dir     Destination directory for matched files")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
}
