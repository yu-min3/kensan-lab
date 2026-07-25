package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/application"
	"github.com/yu-min3/kensan-lab/apps/feed/internal/briefing"
	driveintegration "github.com/yu-min3/kensan-lab/apps/feed/internal/integrations/drive"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "validate":
		return validate(args[1:])
	case "run":
		return runImport(args[1:])
	default:
		return usageError()
	}
}

func validate(args []string) error {
	if len(args) != 1 {
		return usageError()
	}
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read report: %w", err)
	}
	doc, err := briefing.Validate(content, time.Time{})
	if err != nil {
		return err
	}
	fmt.Printf("valid report: date=%s generated_at=%s\n", doc.Metadata.Date, doc.Metadata.GeneratedAt.Format(time.RFC3339))
	return nil
}

func runImport(args []string) error {
	flags := flag.NewFlagSet("feed run", flag.ContinueOnError)
	dateValue := flags.String("date", "", "report date in YYYY-MM-DD (default: today in FEED_TIMEZONE)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError()
	}
	locationName := getenv("FEED_TIMEZONE", "Asia/Tokyo")
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return fmt.Errorf("load FEED_TIMEZONE: %w", err)
	}
	now := time.Now().In(location)
	date := now
	if *dateValue != "" {
		date, err = time.ParseInLocation("2006-01-02", *dateValue, location)
		if err != nil {
			return fmt.Errorf("parse --date: %w", err)
		}
	}
	root := os.Getenv("KENSAN_DATA_DIR")
	folderID := os.Getenv("GOOGLE_DRIVE_OUTPUT_FOLDER_ID")
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if root == "" || folderID == "" || credentialsFile == "" {
		return fmt.Errorf("KENSAN_DATA_DIR, GOOGLE_DRIVE_OUTPUT_FOLDER_ID, and GOOGLE_APPLICATION_CREDENTIALS are required")
	}
	inbox, err := driveintegration.New(context.Background(), folderID, credentialsFile)
	if err != nil {
		return err
	}
	result, err := application.Run(context.Background(), application.Config{
		WorkspaceRoot: root,
		Inbox:         inbox,
		Now:           func() time.Time { return now },
	}, date)
	if err != nil {
		return err
	}
	if result.NoOp {
		fmt.Printf("report already imported: %s\n", result.Path)
	} else {
		fmt.Printf("report imported: %s\n", result.Path)
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func usageError() error {
	return fmt.Errorf("usage: feed validate <path> | feed run [--date YYYY-MM-DD]")
}
