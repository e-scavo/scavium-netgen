package abuse

import (
	"context"
	"testing"
	"time"
)

type fakePruner struct {
	cutoff time.Time
	calls  int
}

func (f *fakePruner) PruneAbuseSignals(ctx context.Context, olderThan time.Time) (int64, error) {
	f.calls++
	f.cutoff = olderThan
	return 3, nil
}

func TestPruneSignalsByRetentionUsesRetentionWindow(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	pruner := &fakePruner{}

	removed, err := PruneSignalsByRetention(context.Background(), pruner, 30, now)
	if err != nil {
		t.Fatalf("prune signals: %v", err)
	}
	if removed != 3 {
		t.Fatalf("removed = %d, want 3", removed)
	}
	want := now.Add(-30 * 24 * time.Hour)
	if !pruner.cutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", pruner.cutoff, want)
	}
}

func TestPruneSignalsByRetentionDisabled(t *testing.T) {
	pruner := &fakePruner{}
	removed, err := PruneSignalsByRetention(context.Background(), pruner, 0, time.Now())
	if err != nil {
		t.Fatalf("prune signals: %v", err)
	}
	if removed != 0 || pruner.calls != 0 {
		t.Fatalf("removed=%d calls=%d, want disabled", removed, pruner.calls)
	}
}
