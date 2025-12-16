// SFTP Sync Service - A bidirectional SFTP file synchronization service.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"sftp-sync/internal/config"
	"sftp-sync/internal/service"
	"sftp-sync/pkg/logger"
)

var (
	version = "1.0.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	// Define flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	installFlag := flag.Bool("install", false, "Install as Windows service")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Windows service")
	statusFlag := flag.Bool("status", false, "Show service status")
	validateFlag := flag.Bool("validate", false, "Validate configuration file")
	versionFlag := flag.Bool("version", false, "Show version information")
	encodeFlag := flag.String("encode", "", "Encode a password to base64 format")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "SFTP Sync Service v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --config config.yaml              Run the service\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --config config.yaml --validate   Validate config\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --install --config C:\\path\\config.yaml  Install as service\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --uninstall                       Uninstall service\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --status                          Show service status\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --encode \"mypassword\"             Encode password\n", os.Args[0])
	}

	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("SFTP Sync Service\n")
		fmt.Printf("  Version: %s\n", version)
		fmt.Printf("  Commit:  %s\n", commit)
		fmt.Printf("  Built:   %s\n", date)
		os.Exit(0)
	}

	// Handle encode flag
	if *encodeFlag != "" {
		encoded := config.EncodeSecret(*encodeFlag)
		fmt.Printf("Encoded password: %s\n", encoded)
		os.Exit(0)
	}

	// Handle status flag
	if *statusFlag {
		status, err := service.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service status: %s\n", status)
		os.Exit(0)
	}

	// Handle install flag
	if *installFlag {
		absPath, err := getAbsolutePath(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving config path: %v\n", err)
			os.Exit(1)
		}

		// Validate config first
		if err := validateConfig(absPath); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration validation failed:\n%v\n", err)
			os.Exit(1)
		}

		if err := service.Install(absPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing service: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Service installed successfully.")
		fmt.Println("Use 'sc start sftp-sync' to start the service.")
		os.Exit(0)
	}

	// Handle uninstall flag
	if *uninstallFlag {
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error uninstalling service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service uninstalled successfully.")
		os.Exit(0)
	}

	// Handle validate flag
	if *validateFlag {
		if err := validateConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration validation failed:\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration is valid.")
		os.Exit(0)
	}

	// Run the service
	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getAbsolutePath(path string) (string, error) {
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") || (len(path) > 1 && path[1] == ':') {
		return path, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return cwd + string(os.PathSeparator) + path, nil
}

func validateConfig(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	errors := config.Validate(cfg)
	if len(errors) > 0 {
		var sb strings.Builder
		for _, e := range errors {
			sb.WriteString("  - ")
			sb.WriteString(e.Error())
			sb.WriteString("\n")
		}
		return fmt.Errorf("validation errors:\n%s", sb.String())
	}

	return nil
}

func run(configPath string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if errors := config.Validate(cfg); len(errors) > 0 {
		var sb strings.Builder
		sb.WriteString("configuration validation failed:\n")
		for _, e := range errors {
			sb.WriteString("  - ")
			sb.WriteString(e.Error())
			sb.WriteString("\n")
		}
		return fmt.Errorf(sb.String())
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Close()

	// Set version for service
	service.Version = version

	// Create service
	svc, err := service.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Check if running as Windows service
	if service.IsWindowsService() {
		log.Info().Msg("Running as Windows service")
		return svc.Run()
	}

	// Run in foreground
	log.Info().Msg("Running in foreground mode")
	return svc.RunForeground()
}
