package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WatchConfig defines settings for a single folder watch job.
type WatchConfig struct {
	Name            string
	Dir             string
	Pattern         string
	Dest            string
	ExtractArchives bool
	PollInterval    int // in seconds
	UsePolling      bool
}

// Config contains all configured watches.
type Config struct {
	Watches map[string]WatchConfig
}

// ExpandPath expands ~ and environment variables in path strings.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	return filepath.Clean(path)
}

// LoadConfig reads and parses an INI configuration file.
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	watches := make(map[string]WatchConfig)

	var currentWatch *WatchConfig

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[watch:") && strings.HasSuffix(line, "]") {
			if currentWatch != nil {
				if err := validateWatch(*currentWatch); err != nil {
					return nil, err
				}
				watches[currentWatch.Name] = *currentWatch
			}

			name := strings.TrimSuffix(strings.TrimPrefix(line, "[watch:"), "]")
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("empty watch section name")
			}

			currentWatch = &WatchConfig{
				Name:            name,
				PollInterval:    5,
				ExtractArchives: false,
				UsePolling:      false,
			}
			continue
		}

		if currentWatch != nil {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				switch key {
				case "dir":
					currentWatch.Dir = ExpandPath(val)
				case "pattern":
					currentWatch.Pattern = val
				case "dest":
					currentWatch.Dest = ExpandPath(val)
				case "extract-archives":
					currentWatch.ExtractArchives = parseBool(val)
				case "poll-interval":
					if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
						currentWatch.PollInterval = secs
					}
				case "use-polling":
					currentWatch.UsePolling = parseBool(val)
				}
			}
		}
	}

	if currentWatch != nil {
		if err := validateWatch(*currentWatch); err != nil {
			return nil, err
		}
		watches[currentWatch.Name] = *currentWatch
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return &Config{Watches: watches}, nil
}

func parseBool(val string) bool {
	lower := strings.ToLower(val)
	return lower == "true" || lower == "yes" || lower == "1"
}

func validateWatch(w WatchConfig) error {
	if w.Dir == "" {
		return fmt.Errorf("watch '%s' missing required 'dir'", w.Name)
	}
	if w.Pattern == "" {
		return fmt.Errorf("watch '%s' missing required 'pattern'", w.Name)
	}
	if w.Dest == "" {
		return fmt.Errorf("watch '%s' missing required 'dest'", w.Name)
	}
	return nil
}
