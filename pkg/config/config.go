// Package config provides configuration parsing, path expansion, and validation
// for multi-watch INI files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// WatchConfig defines settings for a single folder watch job.
type WatchConfig struct {
	// Name is the section identifier for the watch job (e.g. "pdfs", "images").
	Name string

	// Dir is the directory path being monitored for incoming files.
	Dir string

	// Pattern is the glob pattern matched against incoming files.
	Pattern string

	// Dest is the destination directory for matched files.
	Dest string

	// ExtractArchives specifies whether single-file matching archives should be extracted.
	ExtractArchives bool

	// PollInterval is the polling frequency in seconds (default: 5).
	PollInterval int

	// UsePolling forces polling mode over event-driven (fsnotify) monitoring.
	UsePolling bool
}

// Config contains a map of all configured watch jobs indexed by watch name.
type Config struct {
	Watches map[string]WatchConfig
}

// ExpandPath expands home directory shortcuts (~/ and ~\) as well as environment variables ($VAR, %VAR%).
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	path = os.ExpandEnv(path)
	if strings.Contains(path, "%") {
		path = expandWinEnv(path)
	}
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

var winEnvRegex = regexp.MustCompile(`%([^%]+)%`)

func expandWinEnv(path string) string {
	return winEnvRegex.ReplaceAllStringFunc(path, func(match string) string {
		varName := match[1 : len(match)-1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

// LoadConfig reads and parses an INI configuration file containing [watch:<name>] sections.
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

// parseBool parses string boolean values ("true", "yes", "1").
func parseBool(val string) bool {
	lower := strings.ToLower(val)
	return lower == "true" || lower == "yes" || lower == "1"
}

// validateWatch ensures required WatchConfig parameters are populated.
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
