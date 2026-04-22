package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/asheshgoplani/agent-deck/internal/calendar"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

func handleGoogleCalendar(args []string) {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printGoogleCalendarUsage()
		return
	}
	switch args[0] {
	case "auth":
		handleGoogleCalendarAuth()
	case "test":
		handleGoogleCalendarTest()
	default:
		fmt.Fprintf(os.Stderr, "Unknown google-calendar subcommand: %s\n", args[0])
		printGoogleCalendarUsage()
		os.Exit(1)
	}
}

func printGoogleCalendarUsage() {
	fmt.Println("Usage: agent-deck google-calendar <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  auth    Authorize Google Calendar access (opens browser)")
	fmt.Println("  test    Test the integration by fetching upcoming events")
}

func handleGoogleCalendarAuth() {
	cfg, err := session.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	credPath := cfg.GoogleCalendar.GetCredentialsPath()
	if credPath == "" {
		fmt.Fprintln(os.Stderr, "Error: cannot determine credentials path")
		os.Exit(1)
	}

	oauthCfg, err := calendar.ParseCredentials(credPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nPlace your Google Cloud credentials at:\n  %s\n", err, credPath)
		os.Exit(1)
	}

	tok, err := calendar.RunLoopbackAuth(context.Background(), oauthCfg, func(authURL string) {
		fmt.Printf("Opening browser for authorization...\n\n")
		openBrowser(authURL)
		fmt.Printf("If the browser didn't open, visit:\n  %s\n\nWaiting for authorization...\n", authURL)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authorization failed: %v\n", err)
		os.Exit(1)
	}

	tokenPath := cfg.GoogleCalendar.GetTokenPath()
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating token directory: %v\n", err)
		os.Exit(1)
	}
	if err := calendar.SaveToken(tokenPath, tok); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nAuthorization successful! Token saved to:\n  %s\n", tokenPath)
}

func handleGoogleCalendarTest() {
	cfg, err := session.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	collector, err := calendar.NewCollectorFromConfig(
		context.Background(),
		cfg.GoogleCalendar.GetCredentialsPath(),
		cfg.GoogleCalendar.GetTokenPath(),
		cfg.GoogleCalendar.CalendarIDs,
		cfg.GoogleCalendar.GetLookahead(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	events, err := collector.Collect(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching events: %v\n", err)
		os.Exit(1)
	}

	if len(events) == 0 {
		fmt.Println("No upcoming events found.")
		return
	}

	fmt.Printf("Found %d upcoming events:\n\n", len(events))
	for _, e := range events {
		fmt.Printf("  %s  %s\n", e.TimeUntilLabel(), e.Title)
	}
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start() //nolint:errcheck
	case "linux":
		exec.Command("xdg-open", url).Start() //nolint:errcheck
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() //nolint:errcheck
	default:
		fmt.Fprintf(os.Stderr, "Cannot open browser automatically on %s. Please visit the URL above manually.\n", runtime.GOOS)
	}
}
