//go:build windows

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"

	"file-watcher/pkg/config"
)

// RunKeyName is the Windows User Run Registry key identifier.
const RunKeyName = "FileWatcher"
const runRegistrySubkey = `Software\Microsoft\Windows\CurrentVersion\Run`

func openRunKey(access uint32) (registry.Key, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistrySubkey, access)
	if err != nil {
		return 0, fmt.Errorf("failed to open HKCU registry key: %w", err)
	}
	return k, nil
}

// Install installs the executable to %LOCALAPPDATA%\file-watcher\file-watcher.exe,
// creates default configuration if necessary, and configures user autostart via HKCU Registry (no elevation required).
func Install(customConfig string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}

	targetDir := config.ExpandPath("%LOCALAPPDATA%/file-watcher")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create app dir: %w", err)
	}

	targetExe := filepath.Join(targetDir, "file-watcher.exe")
	if strings.ToLower(filepath.Clean(execPath)) != strings.ToLower(filepath.Clean(targetExe)) {
		if err := copyFile(execPath, targetExe); err != nil {
			return err
		}
	}

	cfgPath, err := EnsureDefaultConfig(customConfig)
	if err != nil {
		return err
	}

	logFile := filepath.Join(targetDir, "file-watcher.log")
	commandVal := fmt.Sprintf(`"%s" --config "%s" --log-file "%s"`, targetExe, cfgPath, logFile)

	k, err := openRunKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.SetStringValue(RunKeyName, commandVal); err != nil {
		return fmt.Errorf("failed to set user autostart registry key: %w", err)
	}

	// Launch background process immediately for current session
	cmd := exec.Command(targetExe, "--config", cfgPath, "--log-file", logFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("autostart registry key set, but failed to launch background process: %w", err)
	}

	fmt.Println("Successfully installed file-watcher Windows User Autostart!")
	fmt.Printf("  Binary:   %s\n", targetExe)
	fmt.Printf("  Config:   %s\n", cfgPath)
	fmt.Printf("  Log:      %s\n", logFile)
	fmt.Printf("  Registry: HKCU:\\%s\\%s\n", runRegistrySubkey, RunKeyName)
	return nil
}

// Uninstall removes the HKCU registry run key and stops any running background file-watcher processes.
func Uninstall() error {
	k, err := openRunKey(registry.SET_VALUE)
	if err == nil {
		err = k.DeleteValue(RunKeyName)
		k.Close()
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("failed to remove user autostart registry key: %w", err)
		}
	}

	// Stop any running file-watcher background processes (excluding current process)
	currentPid := os.Getpid()
	stopCmd := exec.Command("taskkill", "/F", "/FI", fmt.Sprintf("PID ne %d", currentPid), "/IM", "file-watcher.exe")
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = stopCmd.Run()

	fmt.Println("Successfully uninstalled file-watcher Windows User Autostart.")
	return nil
}

// Status returns the current operational status of the HKCU User Autostart.
func Status() (string, error) {
	k, err := openRunKey(registry.QUERY_VALUE)
	if err != nil {
		return "Not Installed", nil
	}
	defer k.Close()

	val, _, err := k.GetStringValue(RunKeyName)
	if err != nil || val == "" {
		return "Not Installed", nil
	}

	currentPid := os.Getpid()
	checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID ne %d", currentPid), "/FI", "IMAGENAME eq file-watcher.exe", "/FO", "CSV", "/NH")
	checkCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := checkCmd.Output()
	if err == nil && strings.Contains(strings.ToLower(string(out)), "file-watcher.exe") {
		return "User Autostart Configured (Running)", nil
	}

	return "User Autostart Configured (Stopped)", nil
}
