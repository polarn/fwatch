package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// version is set via ldflags during build
var version = "dev"

// Config represents the application configuration
type Config struct {
	WatchDir   string `yaml:"watch_dir"`
	Rules      []Rule `yaml:"rules"`
	CreateDirs bool   `yaml:"create_dirs"`
}

// Rule represents a file routing rule
type Rule struct {
	Extensions  []string `yaml:"extensions"`
	Destination string   `yaml:"destination"`
}

// getDefaultConfigPath returns the default configuration file path
// using platform-specific conventions
func getDefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		// Windows: use APPDATA or USERPROFILE
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "fwatch", "config.yaml")
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "fwatch", "config.yaml")
		}
	} else {
		// Unix-like: use XDG_CONFIG_HOME or ~/.config
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			return filepath.Join(configHome, "fwatch", "config.yaml")
		}
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".config", "fwatch", "config.yaml")
		}
	}

	// Last resort: current directory
	return "config.yaml"
}

func main() {
	defaultConfigPath := getDefaultConfigPath()
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version and exit if requested
	if *showVersion {
		fmt.Printf("fwatch %s\n", version)
		os.Exit(0)
	}

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate watch directory
	if _, err := os.Stat(config.WatchDir); os.IsNotExist(err) {
		log.Fatalf("Watch directory does not exist: %s", config.WatchDir)
	}

	// Create destination directories if needed
	if config.CreateDirs {
		for _, rule := range config.Rules {
			if err := os.MkdirAll(rule.Destination, 0755); err != nil {
				log.Printf("Warning: Failed to create directory %s: %v", rule.Destination, err)
			}
		}
	}

	// Start watching
	log.Printf("fwatch started - watching: %s", config.WatchDir)
	if err := watchDirectory(config); err != nil {
		log.Fatalf("Failed to watch directory: %v", err)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &config, nil
}

func watchDirectory(config *Config) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	// Add watch directory
	if err := watcher.Add(config.WatchDir); err != nil {
		return fmt.Errorf("adding watch directory: %w", err)
	}

	log.Printf("Watching directory: %s", config.WatchDir)

	// Map extensions to destinations for quick lookup
	extMap := buildExtensionMap(config.Rules)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher events channel closed")
			}

			// Only process create and write events
			if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write {
				// Small delay to ensure file is fully written
				time.Sleep(100 * time.Millisecond)

				// Wait for file to be ready (not locked by another process)
				if !waitForFile(event.Name, 5*time.Second) {
					log.Printf("File is locked or unavailable, skipping: %s", event.Name)
					continue
				}

				processFile(event.Name, extMap)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher errors channel closed")
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func buildExtensionMap(rules []Rule) map[string]string {
	extMap := make(map[string]string)
	for _, rule := range rules {
		for _, ext := range rule.Extensions {
			// Normalize extension to lowercase
			extMap[strings.ToLower(ext)] = rule.Destination
		}
	}
	return extMap
}

// waitForFile waits for a file to be ready (not locked) by attempting to open it
func waitForFile(filePath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Try to open the file exclusively to check if it's locked
		file, err := os.OpenFile(filePath, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			return true
		}

		// If file doesn't exist, it might have been moved already
		if os.IsNotExist(err) {
			return false
		}

		// Check if it's a permission/locking error
		if strings.Contains(err.Error(), "being used by another process") ||
			strings.Contains(err.Error(), "permission denied") {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// Other errors, give up
		return false
	}
	return false
}

func processFile(filePath string, extMap map[string]string) {
	// Skip if file doesn't exist (might have been moved already)
	info, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error stating file %s: %v", filePath, err)
		}
		return
	}

	// Skip directories
	if info.IsDir() {
		return
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return
	}

	// Check if we have a rule for this extension
	destination, exists := extMap[ext]
	if !exists {
		return
	}

	// Build destination path
	fileName := filepath.Base(filePath)
	destPath := filepath.Join(destination, fileName)

	// Check if destination file already exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists, add timestamp to make it unique
		timestamp := time.Now().Format("20060102-150405")
		nameWithoutExt := strings.TrimSuffix(fileName, ext)
		destPath = filepath.Join(destination, fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext))
		log.Printf("Destination file exists, using: %s", filepath.Base(destPath))
	}

	// Move the file
	if err := moveFile(filePath, destPath); err != nil {
		log.Printf("Error moving file %s to %s: %v", filePath, destPath, err)
		return
	}

	log.Printf("Moved: %s → %s", fileName, destination)
}

// moveFile moves a file from src to dst, handling cross-device moves
func moveFile(src, dst string) error {
	// Try rename first (fastest method)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Check if it's a cross-device/cross-filesystem error
	// Linux: "invalid cross-device link"
	// Windows: ERROR_NOT_SAME_DEVICE or "The system cannot move the file to a different disk drive"
	if isCrossDeviceError(err) {
		return copyAndDelete(src, dst)
	}

	// For other errors, return them
	return err
}

// isCrossDeviceError checks if the error indicates a cross-device/cross-filesystem operation
func isCrossDeviceError(err error) bool {
	errMsg := err.Error()
	// Linux error message
	if strings.Contains(errMsg, "invalid cross-device link") {
		return true
	}
	// Windows error messages
	if strings.Contains(errMsg, "not the same device") ||
		strings.Contains(errMsg, "different disk drive") {
		return true
	}
	return false
}

// copyAndDelete copies a file and then deletes the source
func copyAndDelete(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("getting source file info: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy the content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying file content: %w", err)
	}

	// Ensure data is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("syncing destination file: %w", err)
	}

	// Close files before attempting delete
	srcFile.Close()
	dstFile.Close()

	// Remove the source file with retries (for locked files on Windows)
	return removeFileWithRetry(src, 3, 500*time.Millisecond)
}

// removeFileWithRetry attempts to remove a file with retries for locked files
func removeFileWithRetry(path string, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := os.Remove(path)
		if err == nil {
			return nil
		}

		lastErr = err

		// If file doesn't exist, consider it success
		if os.IsNotExist(err) {
			return nil
		}

		// Check if it's a lock/permission error
		if strings.Contains(err.Error(), "being used by another process") ||
			strings.Contains(err.Error(), "permission denied") {
			if i < maxRetries-1 {
				time.Sleep(delay)
				continue
			}
		}

		// Other errors, return immediately
		return fmt.Errorf("removing source file: %w", err)
	}

	return fmt.Errorf("removing source file after %d retries: %w", maxRetries, lastErr)
}
