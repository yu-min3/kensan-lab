package application

import (
	"context"
	"fmt"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/briefing"
)

type Config struct {
	WorkspaceRoot string
	Inbox         briefing.ReportInbox
	Now           func() time.Time
}

func Run(ctx context.Context, config Config, date time.Time) (briefing.ImportResult, error) {
	if config.WorkspaceRoot == "" {
		return briefing.ImportResult{}, fmt.Errorf("workspace root is required")
	}
	if config.Inbox == nil {
		return briefing.ImportResult{}, fmt.Errorf("report inbox is required")
	}
	return briefing.Importer{
		Root:  config.WorkspaceRoot,
		Inbox: config.Inbox,
		Now:   config.Now,
	}.Import(ctx, date)
}
