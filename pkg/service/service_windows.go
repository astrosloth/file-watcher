//go:build windows

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"file-watcher/pkg/config"
)

// TaskName is the Windows Scheduled Task identifier.
const TaskName = "FileWatcher"

// Install installs the executable to %LOCALAPPDATA%\file-watcher\file-watcher.exe,
// creates default configuration if necessary, and registers/starts a Windows Scheduled Task.
func Install(customConfig string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}

	targetDir := config.ExpandPath("~/.local/state/file-watcher")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create app state dir: %w", err)
	}

	targetExe := filepath.Join(config.ExpandPath("%LOCALAPPDATA%"), "file-watcher", "file-watcher.exe")
	if strings.ToLower(filepath.Clean(execPath)) != strings.ToLower(filepath.Clean(targetExe)) {
		if err := copyFile(execPath, targetExe); err != nil {
			return err
		}
	}

	cfgPath, err := EnsureDefaultConfig(customConfig)
	if err != nil {
		return err
	}

	logFile := filepath.Join(config.ExpandPath("%LOCALAPPDATA%"), "file-watcher", "file-watcher.log")

	psCmd := fmt.Sprintf(
		`$action = New-ScheduledTaskAction -Execute "%s" -Argument "--config `+"`"+`"%s`+"`"+`" --log-file `+"`"+`"%s`+"`"+`""; `+
			`$trigger = New-ScheduledTaskTrigger -AtLogOn; `+
			`$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0; `+
			`Register-ScheduledTask -TaskName "%s" -Action $action -Trigger $trigger -Settings $settings -Force; `+
			`Start-ScheduledTask -TaskName "%s"`,
		targetExe, cfgPath, logFile, TaskName, TaskName,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to register scheduled task: %v (details: %s)", err, stderr.String())
	}

	fmt.Println("Successfully installed file-watcher Windows Scheduled Task!")
	fmt.Printf("  Binary: %s\n", targetExe)
	fmt.Printf("  Config: %s\n", cfgPath)
	fmt.Printf("  Log:    %s\n", logFile)
	return nil
}

// Uninstall stops and unregisters the Windows Scheduled Task for file-watcher.
func Uninstall() error {
	psCmd := fmt.Sprintf(
		`Stop-ScheduledTask -TaskName "%s" -ErrorAction SilentlyContinue; `+
			`Unregister-ScheduledTask -TaskName "%s" -Confirm:$false -ErrorAction SilentlyContinue`,
		TaskName, TaskName,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unregister scheduled task: %v (details: %s)", err, stderr.String())
	}

	fmt.Println("Successfully un-registered file-watcher Windows Scheduled Task.")
	return nil
}

// Status returns the current operational status of the Windows Scheduled Task.
func Status() (string, error) {
	psCmd := fmt.Sprintf(`(Get-ScheduledTask -TaskName "%s" -ErrorAction SilentlyContinue).State`, TaskName)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	out, err := cmd.Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return "Not Installed", nil
	}
	return fmt.Sprintf("Scheduled Task Status: %s", strings.TrimSpace(string(out))), nil
}
