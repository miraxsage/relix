package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// Version of the application
const AppVersion = "0.1.0"

// Global project directory (set via -d flag)
var projectDirectory string

func main() {
	// Define command-line flags
	var showHelp bool
	var showVersion bool
	var projectDir string

	flag.StringVar(&projectDir, "d", "", "Project root directory path")
	flag.StringVar(&projectDir, "project-directory", "", "Project root directory path")
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.BoolVar(&showVersion, "version", false, "Show version")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Relix - GitLab Release Manager\n\n")
		fmt.Fprintf(os.Stderr, "Usage: relix [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -d, --project-directory <path>  Project root directory path\n")
		fmt.Fprintf(os.Stderr, "  -h, --help                      Show this help message\n")
		fmt.Fprintf(os.Stderr, "  -v, --version                   Show version\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  relix                           Run in current directory\n")
		fmt.Fprintf(os.Stderr, "  relix -d /path/to/project       Run with specified project directory\n")
	}

	flag.Parse()

	// Handle help flag
	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	// Handle version flag
	if showVersion {
		fmt.Printf("Relix v%s\n", AppVersion)
		os.Exit(0)
	}

	// Validate and set project directory
	if projectDir != "" {
		// Convert to absolute path
		absPath, err := filepath.Abs(projectDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid project directory path: %v\n", err)
			os.Exit(1)
		}

		// Check if directory exists
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: project directory does not exist: %s\n", absPath)
			} else {
				fmt.Fprintf(os.Stderr, "Error: cannot access project directory: %v\n", err)
			}
			os.Exit(1)
		}

		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: specified path is not a directory: %s\n", absPath)
			os.Exit(1)
		}

		projectDirectory = absPath
	}

	// Load theme from config before creating the model (rebuilds all styles)
	loadThemeFromConfig()

	p := tea.NewProgram(NewModel(), tea.WithAltScreen())

	// Send program reference to model for async message sending
	go func() {
		p.Send(setProgramMsg{program: p})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
