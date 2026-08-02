//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"file-watcher/pkg/config"
)

const UnitName = "file-watcher.service"

// Install copies executable to ~/.local/bin/file-watcher and writes a systemd --user service unit.
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

	unitContent := fmt.Sprintf(`[Unit]
Description=File Watcher User Service
After=network.target

[Service]
ExecStart=%s --config %s --log-file %s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, targetExe, cfgPath, logFile)

	systemdDir := config.ExpandPath("~/.config/systemd/user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user dir: %w", err)
	}

	unitPath := filepath.Join(systemdDir, UnitName)
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	_ = exec.Command("systemctl", "--user", "enable", "--now", UnitName).Run()

	fmt.Println("Successfully installed file-watcher systemd user service!")
	fmt.Printf("  Binary: %s\n", targetExe)
	fmt.Printf("  Config: %s\n", cfgPath)
	fmt.Printf("  Unit:   %s\n", unitPath)
	return nil
}

// Uninstall stops and disables the systemd user service.
func Uninstall() error {
	_ = exec.Command("systemctl", "--user", "stop", UnitName).Run()
	_ = exec.Command("systemctl", "--user", "disable", UnitName).Run()

	unitPath := config.ExpandPath("~/.config/systemd/user/" + UnitName)
	_ = os.Remove(unitPath)
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println("Successfully uninstalled file-watcher systemd user service.")
	return nil
}

// Status returns systemd user service status.
func Status() (string, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", UnitName)
	out, err := cmd.Output()
	if err != nil {
		return "Inactive / Not Installed", nil
	}
	return fmt.Sprintf("systemd service status: %s", out), nil
}
