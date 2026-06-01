// Package sweeper runs the orphan-upload sweep on a fixed cadence.
//
// Per Req 2.5 / Property 1, pending uploads whose object never landed
// must be removed from object storage and the database within 24 hours.
// We host the cron in-process (single API instance is fine for v0.1);
// when we scale out, we move it to a dedicated worker.
package sweeper

import (
	"context"
	"log/slog"
	"time"

	"github.com/tomeku/doclens/services/ingestion/app"
)

// Run blocks until ctx is done, sweeping every interval. Errors are
// logged and swallowed so a transient DB hiccup never tears down the
// API.
func Run(ctx context.Context, svc *app.Service, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	logger.Info("upload sweeper running", "interval", interval)

	// Run once at startup so the first pass doesn't wait an interval.
	sweepOnce(ctx, svc, logger)

	for {
		select {
		case <-ctx.Done():
			logger.Info("upload sweeper stopping")
			return
		case <-tick.C:
			sweepOnce(ctx, svc, logger)
		}
	}
}

func sweepOnce(ctx context.Context, svc *app.Service, logger *slog.Logger) {
	res, err := svc.SweepExpiredUploads(ctx, 200)
	if err != nil {
		logger.Error("upload sweep failed", "err", err)
		return
	}
	if res.ExpiredCount > 0 {
		logger.Info("upload sweep tick",
			"expired", res.ExpiredCount,
			"deleted", res.DeletedCount,
		)
	}
}
