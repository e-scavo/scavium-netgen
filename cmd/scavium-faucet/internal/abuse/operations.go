package abuse

import (
	"context"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
)

// PruneSignalsByRetention removes abuse signals older than retentionDays.
// A zero retention disables pruning, allowing operators to opt out explicitly
// without changing schema or public API behavior.
func PruneSignalsByRetention(ctx context.Context, pruner domain.AbuseSignalPruner, retentionDays int, now time.Time) (int64, error) {
	if pruner == nil || retentionDays <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	return pruner.PruneAbuseSignals(ctx, cutoff)
}
