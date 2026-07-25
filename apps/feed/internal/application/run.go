package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/briefing"
)

type Config struct {
	WorkspaceRoot string
	Inbox         briefing.ReportInbox
	Metrics       MetricsUpdater
	Now           func() time.Time
}

type MetricsUpdater interface {
	UpdateAll(context.Context) (MetricsResult, error)
}

type MetricsResult struct {
	Projects int
	Updated  int
}

type Result struct {
	Briefing briefing.ImportResult
	Metrics  MetricsResult
}

func Run(ctx context.Context, config Config, date time.Time) (Result, error) {
	if config.WorkspaceRoot == "" {
		return Result{}, fmt.Errorf("workspace root is required")
	}
	if config.Inbox == nil {
		return Result{}, fmt.Errorf("report inbox is required")
	}
	if config.Metrics == nil {
		return Result{}, fmt.Errorf("metrics updater is required")
	}
	briefingResult, briefingErr := (briefing.Importer{
		Root:  config.WorkspaceRoot,
		Inbox: config.Inbox,
		Now:   config.Now,
	}).Import(ctx, date)
	metricsResult, metricsErr := config.Metrics.UpdateAll(ctx)
	return Result{Briefing: briefingResult, Metrics: metricsResult}, errors.Join(briefingErr, metricsErr)
}
