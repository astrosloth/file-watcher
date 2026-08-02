//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"file-watcher/pkg/config"
)

const Label = "com.filewatcher.daemon"

// Install copies binary to ~/.local/bin/file-watcher and writes Launchd plist.
func Install(customConfig string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}

	targetExe := config.ExpandPath("~/.local/bin/file-watcher")
	if err := copyFile(execPath, targetExe); err != nil {
		return err
	}
	_ = os.Chmod(targetExe, 0755)

	cfgPath, err := EnsureDefaultConfig(customConfig)
	if err != nil {
		return err
	}

	logFile := config.ExpandPath("~/.local/state/file-watcher/file-watcher.log")

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--config</string>
        <string>%s</string>
        <string>--log-file</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`, Label, targetExe, cfgPath, logFile)

	launchDir := config.ExpandPath("~/Library/LaunchAgents")
	if err := os.MkdirAll(launchDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	plistPath := filepath.Join(launchDir, Label+".plist")
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write launchd plist: %w", err)
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = exec.Command("launchctl", "load", plistPath).Run()

	fmt.Println("Successfully installed file-watcher macOS LaunchAgent!")
	fmt.Printf("  Binary: %s\n", targetExe)
	fmt.Printf("  Config: %s\n", cfgPath)
	fmt.Printf("  Plist:  %s\n", plistPath)
	return nil
}

// Uninstall stops and unloads launchd LaunchAgent.
func Uninstall() error {
	plistPath := config.ExpandPath("~/Library/LaunchAgents/" + Label + ".plist")
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = os.Remove(plistPath)

	fmt.Println("Successfully uninstalled file-watcher launchd agent.")
	return nil
}

// Status returns LaunchAgent status.
func Status() (string, error) {
	cmd := exec.Command("launchctl", "list", Label)
	out, err := cmd.Output()
	if err != nil {
		return "Not Loaded / Not Installed", nil
	}
	return fmt.Sprintf("launchd agent status: %s", out), nil
}
