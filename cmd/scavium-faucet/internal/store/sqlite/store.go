package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"sort"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/migrations"

	"github.com/ethereum/go-ethereum/common"
	_ "modernc.org/sqlite"
)

var _ domain.ClaimStore = (*Store)(nil)
var _ domain.RateLimiter = (*Store)(nil)
var _ domain.QueueStore = (*Store)(nil)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	// Ensure migration-tracking table exists before running any migrations.
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue // already applied
		}

		sqlText, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`, name, formatTime(time.Now().UTC())); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}

func (s *Store) CreateClaim(ctx context.Context, claim domain.Claim) (domain.Claim, error) {
	return s.CreateClaimWithIdempotency(ctx, claim, "")
}

func (s *Store) CreateClaimWithIdempotency(ctx context.Context, claim domain.Claim, idempotencyKey string) (domain.Claim, error) {
	if claim.AmountWei == nil {
		claim.AmountWei = big.NewInt(0)
	}
	if claim.CreatedAt.IsZero() {
		claim.CreatedAt = time.Now().UTC()
	}
	if claim.UpdatedAt.IsZero() {
		claim.UpdatedAt = claim.CreatedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO requests (
			id, address, amount_wei, status, reason, idempotency_key, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)
	`,
		claim.ID,
		claim.Address.Hex(),
		claim.AmountWei.String(),
		string(claim.Status),
		claim.Reason,
		idempotencyKey,
		formatTime(claim.CreatedAt),
		formatTime(claim.UpdatedAt),
	)
	if err != nil {
		return domain.Claim{}, err
	}

	return claim, nil
}

func (s *Store) GetClaim(ctx context.Context, id string) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, amount_wei, status, reason, retry_count, next_attempt_at, created_at, updated_at
		FROM requests
		WHERE id = ?
	`, id)

	claim, err := scanClaim(row)
	if err != nil {
		return domain.Claim{}, err
	}
	return claim, nil
}

func (s *Store) UpdateClaimStatus(ctx context.Context, id string, status domain.ClaimStatus, reason string) (domain.Claim, error) {
	updatedAt := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, reason = ?, updated_at = ?
		WHERE id = ?
	`, string(status), reason, formatTime(updatedAt), id)
	if err != nil {
		return domain.Claim{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.Claim{}, err
	}
	if rowsAffected == 0 {
		return domain.Claim{}, ErrNotFound
	}

	return s.GetClaim(ctx, id)
}

func (s *Store) ListClaimsByAddress(ctx context.Context, address common.Address, limit int) ([]domain.Claim, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, address, amount_wei, status, reason, retry_count, next_attempt_at, created_at, updated_at
		FROM requests
		WHERE address = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, address.Hex(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []domain.Claim
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *Store) LastClaimByAddress(ctx context.Context, address common.Address) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, amount_wei, status, reason, retry_count, next_attempt_at, created_at, updated_at
		FROM requests
		WHERE address = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, address.Hex())

	claim, err := scanClaim(row)
	if err != nil {
		return domain.Claim{}, err
	}
	return claim, nil
}

type claimScanner interface {
	Scan(dest ...any) error
}

func scanClaim(scanner claimScanner) (domain.Claim, error) {
	var (
		id            string
		address       string
		amountWei     string
		status        string
		reason        string
		retryCount    int
		nextAttemptAt sql.NullString
		createdAt     string
		updatedAt     string
	)

	if err := scanner.Scan(&id, &address, &amountWei, &status, &reason, &retryCount, &nextAttemptAt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Claim{}, ErrNotFound
		}
		return domain.Claim{}, err
	}

	amount, ok := new(big.Int).SetString(amountWei, 10)
	if !ok {
		return domain.Claim{}, fmt.Errorf("invalid stored amount wei: %s", amountWei)
	}

	created, err := parseTime(createdAt)
	if err != nil {
		return domain.Claim{}, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return domain.Claim{}, err
	}

	var nextAttempt *time.Time
	if nextAttemptAt.Valid {
		t, err := parseTime(nextAttemptAt.String)
		if err != nil {
			return domain.Claim{}, fmt.Errorf("invalid next_attempt_at: %w", err)
		}
		nextAttempt = &t
	}

	return domain.Claim{
		ID:            id,
		Address:       common.HexToAddress(address),
		AmountWei:     amount,
		Status:        domain.ClaimStatus(status),
		Reason:        reason,
		RetryCount:    retryCount,
		NextAttemptAt: nextAttempt,
		CreatedAt:     created,
		UpdatedAt:     updated,
	}, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

// ── QueueStore ───────────────────────────────────────────────────────────────

// Enqueue transitions a claim to the 'queued' state.
func (s *Store) Enqueue(ctx context.Context, claimID string) error {
	now := formatTime(time.Now().UTC())
	result, err := s.db.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, updated_at = ?
		WHERE id = ?
	`, string(domain.ClaimStatusQueued), now, claimID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DequeueBatch picks up to n queued claims (skipping those whose next_attempt_at
// is still in the future), transitions them to 'sending', and returns them.
func (s *Store) DequeueBatch(ctx context.Context, n int) ([]domain.Claim, error) {
	if n <= 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	nowStr := formatTime(now)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin dequeue tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx, `
		SELECT id, address, amount_wei, status, reason, retry_count, next_attempt_at, created_at, updated_at
		FROM requests
		WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?
	`, string(domain.ClaimStatusQueued), nowStr, n)
	if err != nil {
		return nil, fmt.Errorf("query queued claims: %w", err)
	}

	var claims []domain.Claim
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		claims = append(claims, claim)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, claim := range claims {
		if _, err := tx.ExecContext(ctx, `
			UPDATE requests
			SET status = ?, updated_at = ?
			WHERE id = ?
		`, string(domain.ClaimStatusSending), nowStr, claim.ID); err != nil {
			return nil, fmt.Errorf("transition to sending %s: %w", claim.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit dequeue tx: %w", err)
	}
	return claims, nil
}

// Ack transitions a claim from 'sending' to 'sent' and persists the transaction
// record when tx contains a non-zero hash.  Both writes happen in a single
// database transaction.
func (s *Store) Ack(ctx context.Context, claimID string, tx domain.Transaction) error {
	now := formatTime(time.Now().UTC())

	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ack tx: %w", err)
	}
	defer dbTx.Rollback() //nolint:errcheck

	result, err := dbTx.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, string(domain.ClaimStatusSent), now, claimID, string(domain.ClaimStatusSending))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}

	// Only record a transaction row when a real hash is present (skip dry-run).
	if tx.Hash != (common.Hash{}) {
		valueWei := "0"
		if tx.ValueWei != nil {
			valueWei = tx.ValueWei.String()
		}
		if _, err := dbTx.ExecContext(ctx, `
			INSERT INTO transactions
				(request_id, tx_hash, from_address, to_address, value_wei, status,
				 block_number, gas_used, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, claimID, tx.Hash.Hex(), tx.From.Hex(), tx.To.Hex(), valueWei,
			string(tx.Status), tx.BlockNumber, tx.GasUsed,
			formatTime(tx.CreatedAt), formatTime(tx.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert transaction record: %w", err)
		}
	}

	return dbTx.Commit()
}

// Fail increments retry_count.  When retry_count reaches maxRetries the claim is
// dead-lettered (status = 'failed').  Otherwise it is re-queued with a linear
// backoff: next_attempt_at = now + retry_count*30s.
func (s *Store) Fail(ctx context.Context, claimID string, reason string, maxRetries int) error {
	now := time.Now().UTC()
	nowStr := formatTime(now)

	var retryCount int
	if err := s.db.QueryRowContext(ctx, `SELECT retry_count FROM requests WHERE id = ?`, claimID).Scan(&retryCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	newCount := retryCount + 1
	if newCount >= maxRetries {
		_, err := s.db.ExecContext(ctx, `
			UPDATE requests
			SET status = ?, reason = ?, retry_count = ?, updated_at = ?
			WHERE id = ?
		`, string(domain.ClaimStatusFailed), reason, newCount, nowStr, claimID)
		return err
	}

	backoff := time.Duration(newCount) * 30 * time.Second
	nextAttempt := formatTime(now.Add(backoff))
	_, err := s.db.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, reason = ?, retry_count = ?, next_attempt_at = ?, updated_at = ?
		WHERE id = ?
	`, string(domain.ClaimStatusQueued), reason, newCount, nextAttempt, nowStr, claimID)
	return err
}

// ── RateLimiter ──────────────────────────────────────────────────────────────

// Allow implements domain.RateLimiter using the rate_limits table.
// It counts requests for a given key within a fixed time window.
// Window boundaries are aligned to the epoch (e.g. for 1h windows: 00:00, 01:00, ...).
func (s *Store) Allow(ctx context.Context, key string, limit int, window time.Duration) (domain.RateLimitDecision, error) {
	now := time.Now().UTC()
	windowSecs := int64(window.Seconds())

	// Align window start to epoch boundary.
	windowStartUnix := (now.Unix() / windowSecs) * windowSecs
	windowStart := time.Unix(windowStartUnix, 0).UTC()
	windowStartStr := formatTime(windowStart)
	nowStr := formatTime(now)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RateLimitDecision{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Ensure the row exists with count=0.
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO rate_limits
			(limit_key, window_start, window_seconds, count, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
	`, key, windowStartStr, windowSecs, nowStr, nowStr)
	if err != nil {
		return domain.RateLimitDecision{}, fmt.Errorf("insert rate limit row: %w", err)
	}

	// Increment count.
	_, err = tx.ExecContext(ctx, `
		UPDATE rate_limits
		SET count = count + 1, updated_at = ?
		WHERE limit_key = ? AND window_start = ? AND window_seconds = ?
	`, nowStr, key, windowStartStr, windowSecs)
	if err != nil {
		return domain.RateLimitDecision{}, fmt.Errorf("increment rate limit: %w", err)
	}

	// Read current count.
	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT count FROM rate_limits
		WHERE limit_key = ? AND window_start = ? AND window_seconds = ?
	`, key, windowStartStr, windowSecs).Scan(&count)
	if err != nil {
		return domain.RateLimitDecision{}, fmt.Errorf("read rate limit count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.RateLimitDecision{}, fmt.Errorf("commit rate limit tx: %w", err)
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	windowEnd := windowStart.Add(window)
	retryAfter := windowEnd.Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}

	allowed := count <= limit
	decision := domain.RateLimitDecision{
		Allowed:   allowed,
		Remaining: remaining,
	}
	if !allowed {
		decision.RetryAfter = retryAfter
		decision.Reason = fmt.Sprintf("rate limit exceeded for key %q: %d/%d in window", key, count, limit)
	}
	return decision, nil
}
