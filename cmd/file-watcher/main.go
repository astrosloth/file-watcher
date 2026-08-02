package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"file-watcher/pkg/config"
	"file-watcher/pkg/logging"
	"file-watcher/pkg/watcher"
)

const version = "1.0.0"

func main() {
	os.Exit(runMain())
}

func runMain() int {
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

	// Daemon mode defaults for PID and log files
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
	}

	// Configure logger
	var logger *slog.Logger
	if logFile != "" {
		expandedLogFile := config.ExpandPath(logFile)
		fileLogger, err := logging.NewFileLogger(expandedLogFile, maxLogSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to initialize file logger: %v\n", err)
			return 1
		}
		logger = fileLogger
	} else {
		logger = logging.NewConsoleLogger(os.Stdout)
	}

	// Setup PID file
	if pidFile != "" {
		expandedPidFile := config.ExpandPath(pidFile)
		if err := writePidFile(expandedPidFile); err != nil {
			logger.Error("Failed to write PID file", "path", expandedPidFile, "error", err)
		} else {
			defer os.Remove(expandedPidFile)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pollInterval := time.Duration(pollIntervalSec) * time.Second

	var wg sync.WaitGroup

	if configFile != "" {
		expandedConfig := config.ExpandPath(configFile)
		cfg, err := config.LoadConfig(expandedConfig)
		if err != nil {
			logger.Error("Failed to load config file", "path", expandedConfig, "error", err)
			return 1
		}

		logger.Info("Starting file-watcher with multi-watch config", "count", len(cfg.Watches), "config", expandedConfig)

		for _, wCfg := range cfg.Watches {
			intval := time.Duration(wCfg.PollInterval) * time.Second
			if isFlagPassed("poll-interval") {
				intval = pollInterval
			}
			isPoll := wCfg.UsePolling
			if isFlagPassed("use-polling") {
				isPoll = usePolling
			}
			isExtract := wCfg.ExtractArchives
			if isFlagPassed("extract-archives") {
				isExtract = extractArchives
			}

			w, err := watcher.New(watcher.Options{
				WatchDir:        wCfg.Dir,
				Pattern:         wCfg.Pattern,
				DestDir:         wCfg.Dest,
				ExtractArchives: isExtract,
				PollInterval:    intval,
				UsePolling:      isPoll,
				Logger:          logger.With("watch", wCfg.Name),
			})
			if err != nil {
				logger.Error("Failed to create watch", "watch", wCfg.Name, "error", err)
				continue
			}

			wg.Add(1)
			go func(w *watcher.Watcher, name string) {
				defer wg.Done()
				if err := w.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("Watcher stopped with error", "watch", name, "error", err)
				}
			}(w, wCfg.Name)
		}
	} else {
		args := flag.Args()
		if len(args) < 3 {
			printUsage()
			return 1
		}

		watchDir := config.ExpandPath(args[0])
		pattern := args[1]
		destDir := config.ExpandPath(args[2])

		w, err := watcher.New(watcher.Options{
			WatchDir:        watchDir,
			Pattern:         pattern,
			DestDir:         destDir,
			ExtractArchives: extractArchives,
			PollInterval:    pollInterval,
			UsePolling:      usePolling,
			Logger:          logger,
		})
		if err != nil {
			logger.Error("Failed to initialize watcher", "error", err)
			return 1
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Start(ctx); err != nil && err != context.Canceled {
				logger.Error("Watcher stopped with error", "error", err)
			}
		}()
	}

	logger.Info("file-watcher process running. Press Ctrl+C to stop.")
	<-ctx.Done()
	logger.Info("Shutting down file-watcher gracefully...")
	wg.Wait()
	logger.Info("Shutdown complete.")
	return 0
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func writePidFile(pidFile string) error {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func printUsage() {
	fmt.Printf("file-watcher v%s\n\n", version)
	fmt.Println("Usage: file-watcher [options] <watch_dir> <pattern> <dest_dir>")
	fmt.Println("       file-watcher [options] --config <config_file>")
	fmt.Println("\nArguments (single watch mode):")
	fmt.Println("  watch_dir    Directory to watch for new files")
	fmt.Println("  pattern      Glob pattern to match (e.g. '*.pdf', '*.{jpg,png}')")
	fmt.Println("  dest_dir     Destination directory for matched files")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
}
