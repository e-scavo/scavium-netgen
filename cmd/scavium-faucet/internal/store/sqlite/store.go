// Package sqlite provides a SQLite-backed faucet persistence implementation.
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

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/migrations"

	"github.com/ethereum/go-ethereum/common"
	_ "modernc.org/sqlite"
)

var _ domain.ClaimStore = (*Store)(nil)
var _ domain.DailyBudgetStore = (*Store)(nil)
var _ domain.RateLimiter = (*Store)(nil)
var _ domain.AbuseSignalRecorder = (*Store)(nil)
var _ domain.AbuseSignalCounter = (*Store)(nil)
var _ domain.AbuseSignalDistinctCounter = (*Store)(nil)
var _ domain.AbuseSignalPruner = (*Store)(nil)
var _ domain.AbuseSignalReporter = (*Store)(nil)
var _ domain.QueueStore = (*Store)(nil)
var _ domain.RuntimePolicyStore = (*Store)(nil)
var _ domain.CampaignStore = (*Store)(nil)

// ErrNotFound reports that the requested record does not exist.
var ErrNotFound = domain.ErrNotFound

// Store persists faucet state in SQLite and implements the domain store contracts.
type Store struct {
	db *sql.DB
}

// Open opens a SQLite database file and applies embedded migrations.
// WAL mode and a 5-second busy timeout are enabled to allow concurrent access
// from the HTTP handler and the background worker without SQLITE_BUSY errors.
//
// The path argument may include SQLite URI query parameters. This is used by
// tests to opt into faster temporary-database pragmas without weakening the
// production default DSN.
func Open(path string) (*Store, error) {
	dsn := sqliteDSN(path)
	db, err := sql.Open("sqlite", dsn)
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

func sqliteDSN(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return "file:" + path + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

// New wraps an existing sql.DB as a faucet Store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping verifies that the database connection is usable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// PingQueue verifies that queue-related request columns are accessible.
func (s *Store) PingQueue(ctx context.Context) error {
	var count int
	return s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM requests
		WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		LIMIT 1
	`, string(domain.ClaimStatusQueued), formatTime(time.Now().UTC())).Scan(&count)
}

// Migrate applies pending embedded schema migrations in lexical order.
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

// RecordAbuseSignal persists a production-safe anti-abuse signal.  Signal
// writes are intentionally best-effort at the service layer, but this method
// returns errors so tests and future admin tooling can surface storage issues.
func (s *Store) RecordAbuseSignal(ctx context.Context, signal domain.AbuseSignal) error {
	if signal.Kind == "" {
		return errors.New("abuse signal kind is required")
	}
	if signal.CreatedAt.IsZero() {
		signal.CreatedAt = time.Now().UTC()
	}
	address := ""
	if signal.Address != (common.Address{}) {
		address = signal.Address.Hex()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO abuse_signals (
			kind, address, remote_ip, fingerprint, user_agent, claim_id, reason, score, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(signal.Kind),
		address,
		strings.TrimSpace(signal.RemoteIP),
		strings.TrimSpace(signal.Fingerprint),
		strings.TrimSpace(signal.UserAgent),
		strings.TrimSpace(signal.ClaimID),
		strings.TrimSpace(signal.Reason),
		signal.Score,
		formatTime(signal.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("record abuse signal: %w", err)
	}
	return nil
}

// CountRecentAbuseSignals returns a scoped count of abuse signals after the
// filter's Since timestamp.  Empty key fields are ignored so callers can count
// by IP, address, or fingerprint without exposing raw signal rows.
func (s *Store) CountRecentAbuseSignals(ctx context.Context, filter domain.AbuseSignalFilter) (int, error) {
	clauses, args, ok := abuseSignalWhere(filter)
	if !ok {
		return 0, nil
	}
	query := "SELECT COUNT(*) FROM abuse_signals WHERE " + strings.Join(clauses, " AND ")
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count recent abuse signals: %w", err)
	}
	return count, nil
}

func (s *Store) CountDistinctRecentAbuseSignalValues(ctx context.Context, filter domain.AbuseSignalFilter, field string) (int, error) {
	column := ""
	switch field {
	case "address":
		column = "address"
	case "remote_ip":
		column = "remote_ip"
	case "fingerprint":
		column = "fingerprint"
	default:
		return 0, fmt.Errorf("unsupported abuse signal distinct field %q", field)
	}
	clauses, args, ok := abuseSignalWhere(filter)
	if !ok {
		return 0, nil
	}
	clauses = append(clauses, column+" <> ''")
	query := "SELECT COUNT(DISTINCT " + column + ") FROM abuse_signals WHERE " + strings.Join(clauses, " AND ")
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count distinct recent abuse signal values: %w", err)
	}
	return count, nil
}

func abuseSignalWhere(filter domain.AbuseSignalFilter) ([]string, []any, bool) {
	if filter.Since.IsZero() || len(filter.Kinds) == 0 {
		return nil, nil, false
	}
	clauses := []string{"created_at >= ?"}
	args := []any{formatTime(filter.Since.UTC())}
	kindPlaceholders := make([]string, 0, len(filter.Kinds))
	for _, kind := range filter.Kinds {
		if kind == "" {
			continue
		}
		kindPlaceholders = append(kindPlaceholders, "?")
		args = append(args, string(kind))
	}
	if len(kindPlaceholders) == 0 {
		return nil, nil, false
	}
	clauses = append(clauses, "kind IN ("+strings.Join(kindPlaceholders, ",")+")")
	if remoteIP := strings.TrimSpace(filter.RemoteIP); remoteIP != "" {
		clauses = append(clauses, "remote_ip = ?")
		args = append(args, remoteIP)
	}
	if filter.Address != (common.Address{}) {
		clauses = append(clauses, "address = ?")
		args = append(args, filter.Address.Hex())
	}
	if fingerprint := strings.TrimSpace(filter.Fingerprint); fingerprint != "" {
		clauses = append(clauses, "fingerprint = ?")
		args = append(args, fingerprint)
	}
	if len(clauses) <= 2 {
		return nil, nil, false
	}
	return clauses, args, true
}

// ListAbuseSignals returns recent abuse signals for tests and future admin
// diagnostics.  It is deliberately not exposed on the public API.
func (s *Store) ListAbuseSignals(ctx context.Context, limit int) ([]domain.AbuseSignal, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, address, remote_ip, fingerprint, user_agent, claim_id, reason, score, created_at
		FROM abuse_signals
		ORDER BY id ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list abuse signals: %w", err)
	}
	defer rows.Close()

	var out []domain.AbuseSignal
	for rows.Next() {
		var signal domain.AbuseSignal
		var kind, address, createdAt string
		if err := rows.Scan(
			&signal.ID,
			&kind,
			&address,
			&signal.RemoteIP,
			&signal.Fingerprint,
			&signal.UserAgent,
			&signal.ClaimID,
			&signal.Reason,
			&signal.Score,
			&createdAt,
		); err != nil {
			return nil, err
		}
		signal.Kind = domain.AbuseSignalKind(kind)
		if address != "" {
			signal.Address = common.HexToAddress(address)
		}
		created, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("invalid abuse signal created_at: %w", err)
		}
		signal.CreatedAt = created
		out = append(out, signal)
	}
	return out, rows.Err()
}

// PruneAbuseSignals removes abuse signals older than olderThan.  Retention is
// intentionally keyed by created_at so operational cleanup never touches claim
// or transaction records.
func (s *Store) PruneAbuseSignals(ctx context.Context, olderThan time.Time) (int64, error) {
	if olderThan.IsZero() {
		return 0, errors.New("abuse signal prune cutoff is required")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM abuse_signals WHERE created_at < ?`, formatTime(olderThan.UTC()))
	if err != nil {
		return 0, fmt.Errorf("prune abuse signals: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune abuse signals rows affected: %w", err)
	}
	return rows, nil
}

// ListAbuseSignalSummaries returns internal aggregate counts grouped by signal
// kind.  It is designed for operator diagnostics and is not exposed publicly.
func (s *Store) ListAbuseSignalSummaries(ctx context.Context, since time.Time, limit int) ([]domain.AbuseSignalSummary, error) {
	if since.IsZero() {
		return nil, errors.New("abuse signal summary since timestamp is required")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, COUNT(*) AS count
		FROM abuse_signals
		WHERE created_at >= ?
		GROUP BY kind
		ORDER BY count DESC, kind ASC
		LIMIT ?
	`, formatTime(since.UTC()), limit)
	if err != nil {
		return nil, fmt.Errorf("list abuse signal summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.AbuseSignalSummary
	for rows.Next() {
		var summary domain.AbuseSignalSummary
		var kind string
		if err := rows.Scan(&kind, &summary.Count); err != nil {
			return nil, err
		}
		summary.Kind = domain.AbuseSignalKind(kind)
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *Store) CreateClaim(ctx context.Context, claim domain.Claim) (domain.Claim, error) {
	return s.CreateClaimWithIdempotency(ctx, claim, "")
}

func (s *Store) CreateClaimWithIdempotency(ctx context.Context, claim domain.Claim, idempotencyKey string) (domain.Claim, error) {
	if idempotencyKey != "" {
		existing, err := s.GetClaimByIdempotencyKey(ctx, idempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return domain.Claim{}, err
		}
	}

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
			id, address, token_id, token_symbol, token_type, token_address, token_decimals,
			amount_wei, status, reason, idempotency_key, campaign_id, invitation_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`,
		claim.ID,
		claim.Address.Hex(),
		claim.TokenID,
		claim.TokenSymbol,
		string(claim.TokenType),
		tokenAddressHex(claim.TokenAddress),
		claim.TokenDecimals,
		claim.AmountWei.String(),
		string(claim.Status),
		claim.Reason,
		idempotencyKey,
		strings.TrimSpace(claim.CampaignID),
		strings.TrimSpace(claim.InvitationCode),
		formatTime(claim.CreatedAt),
		formatTime(claim.UpdatedAt),
	)
	if err != nil {
		if idempotencyKey != "" {
			existing, lookupErr := s.GetClaimByIdempotencyKey(ctx, idempotencyKey)
			if lookupErr == nil {
				return existing, nil
			}
		}
		return domain.Claim{}, err
	}

	return claim, nil
}

// CreateClaimWithIdempotencyAndDailyBudget checks the UTC-day budget and inserts
// the claim in one SQLite write transaction.
func (s *Store) CreateClaimWithIdempotencyAndDailyBudget(ctx context.Context, claim domain.Claim, idempotencyKey string, dayStart, dayEnd time.Time, budgetWei *big.Int, statuses []domain.ClaimStatus) (domain.Claim, *big.Int, bool, error) {
	return s.createClaimWithBudget(ctx, claim, idempotencyKey, "", dayStart, dayEnd, budgetWei, statuses)
}

// CreateClaimWithIdempotencyAndDailyBudgetForToken checks a UTC-day budget scoped
// to the given token_id and inserts the claim in one SQLite write transaction.
func (s *Store) CreateClaimWithIdempotencyAndDailyBudgetForToken(ctx context.Context, claim domain.Claim, idempotencyKey string, tokenID string, dayStart, dayEnd time.Time, budgetWei *big.Int, statuses []domain.ClaimStatus) (domain.Claim, *big.Int, bool, error) {
	return s.createClaimWithBudget(ctx, claim, idempotencyKey, tokenID, dayStart, dayEnd, budgetWei, statuses)
}

func (s *Store) createClaimWithBudget(ctx context.Context, claim domain.Claim, idempotencyKey string, tokenID string, dayStart, dayEnd time.Time, budgetWei *big.Int, statuses []domain.ClaimStatus) (domain.Claim, *big.Int, bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return domain.Claim{}, nil, false, err
	}
	defer conn.Close() //nolint:errcheck

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return domain.Claim{}, nil, false, fmt.Errorf("begin daily budget tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if idempotencyKey != "" {
		row := conn.QueryRowContext(ctx, `
			SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
			FROM requests
			WHERE idempotency_key = NULLIF(?, '')
		`, idempotencyKey)
		existing, err := scanClaim(row)
		if err == nil {
			if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
				return domain.Claim{}, nil, false, fmt.Errorf("commit existing idempotent claim: %w", err)
			}
			committed = true
			return existing, big.NewInt(0), false, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return domain.Claim{}, nil, false, err
		}
	}

	used, err := claimAmountWeiForToken(ctx, conn, tokenID, dayStart, dayEnd, statuses)
	if err != nil {
		return domain.Claim{}, nil, false, err
	}
	if budgetWei != nil && budgetWei.Sign() > 0 {
		claimAmount := big.NewInt(0)
		if claim.AmountWei != nil {
			claimAmount = new(big.Int).Set(claim.AmountWei)
		}
		if new(big.Int).Add(used, claimAmount).Cmp(budgetWei) > 0 {
			if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
				return domain.Claim{}, nil, false, fmt.Errorf("commit exceeded daily budget check: %w", err)
			}
			committed = true
			return domain.Claim{}, used, true, nil
		}
	}

	if claim.AmountWei == nil {
		claim.AmountWei = big.NewInt(0)
	}
	if claim.CreatedAt.IsZero() {
		claim.CreatedAt = time.Now().UTC()
	}
	if claim.UpdatedAt.IsZero() {
		claim.UpdatedAt = claim.CreatedAt
	}

	_, err = conn.ExecContext(ctx, `
		INSERT INTO requests (
			id, address, token_id, token_symbol, token_type, token_address, token_decimals,
			amount_wei, status, reason, idempotency_key, campaign_id, invitation_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`,
		claim.ID,
		claim.Address.Hex(),
		claim.TokenID,
		claim.TokenSymbol,
		string(claim.TokenType),
		tokenAddressHex(claim.TokenAddress),
		claim.TokenDecimals,
		claim.AmountWei.String(),
		string(claim.Status),
		claim.Reason,
		idempotencyKey,
		strings.TrimSpace(claim.CampaignID),
		strings.TrimSpace(claim.InvitationCode),
		formatTime(claim.CreatedAt),
		formatTime(claim.UpdatedAt),
	)
	if err != nil {
		return domain.Claim{}, nil, false, err
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return domain.Claim{}, nil, false, fmt.Errorf("commit daily budget claim create: %w", err)
	}
	committed = true
	return claim, used, false, nil
}

// GetClaimByIdempotencyKey returns the claim previously created for idempotencyKey.
func (s *Store) GetClaimByIdempotencyKey(ctx context.Context, idempotencyKey string) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		WHERE idempotency_key = NULLIF(?, '')
	`, idempotencyKey)

	claim, err := scanClaim(row)
	if err != nil {
		return domain.Claim{}, err
	}
	return claim, nil
}

func (s *Store) GetClaim(ctx context.Context, id string) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
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
	return s.ListClaimsByAddressPage(ctx, address, limit, 0)
}

// ListClaimsByAddressPage returns persisted claims for one address with bounded
// offset pagination in reverse creation order.
func (s *Store) ListClaimsByAddressPage(ctx context.Context, address common.Address, limit, offset int) ([]domain.Claim, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		WHERE address = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, address.Hex(), limit, offset)
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

// DailyClaimAmountWeiForToken sums persisted claim amounts for one token in a UTC day window.
func (s *Store) DailyClaimAmountWeiForToken(ctx context.Context, tokenID string, dayStart, dayEnd time.Time, statuses []domain.ClaimStatus) (*big.Int, error) {
	return claimAmountWeiForToken(ctx, s.db, tokenID, dayStart, dayEnd, statuses)
}

// ListAdminClaims returns persisted claims for admin listing in reverse
// creation order.
func (s *Store) ListAdminClaims(ctx context.Context, limit, offset int) ([]domain.Claim, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list admin claims: %w", err)
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

// AdminClaimCounts returns persisted claim counts grouped by status for the
// admin dashboard and queue snapshot.
func (s *Store) AdminClaimCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM requests
		GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("admin claim counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// AdminQueueCounts summarizes persisted queue state without exposing sensitive
// claim fields.
func (s *Store) AdminQueueCounts(ctx context.Context, now time.Time) (map[string]int, int, int, int, int, int, error) {
	counts, err := s.AdminClaimCounts(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, err
	}
	nowStr := formatTime(now.UTC())

	var ready int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM requests
		WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
	`, string(domain.ClaimStatusQueued), nowStr).Scan(&ready); err != nil {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("admin ready queue count: %w", err)
	}

	var delayed int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM requests
		WHERE status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at > ?
	`, string(domain.ClaimStatusQueued), nowStr).Scan(&delayed); err != nil {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("admin delayed queue count: %w", err)
	}

	inFlight := counts[string(domain.ClaimStatusSending)]
	pendingTx := counts[string(domain.ClaimStatusSent)]
	terminal := counts[string(domain.ClaimStatusConfirmed)] + counts[string(domain.ClaimStatusFailed)] + counts[string(domain.ClaimStatusRejected)]
	return counts, ready, delayed, inFlight, pendingTx, terminal, nil
}

// ListAdminQueueClaims returns the bounded item slice used by the admin queue
// endpoint. Only queue-relevant persisted states are included.
func (s *Store) ListAdminQueueClaims(ctx context.Context, limit int) ([]domain.Claim, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		WHERE status IN (?, ?, ?, ?)
		ORDER BY updated_at DESC, id ASC
		LIMIT ?
	`,
		string(domain.ClaimStatusQueued),
		string(domain.ClaimStatusSending),
		string(domain.ClaimStatusSent),
		string(domain.ClaimStatusFailed),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list admin queue claims: %w", err)
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

// AdminRetryClaim re-queues an eligible failed/rejected claim for worker
// processing. next_attempt_at is cleared so the claim becomes immediately
// processable by DequeueBatch.
func (s *Store) AdminRetryClaim(ctx context.Context, id string) (bool, error) {
	now := formatTime(time.Now().UTC())
	result, err := s.db.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, reason = '', next_attempt_at = NULL, updated_at = ?
		WHERE id = ? AND status IN (?, ?)
	`,
		string(domain.ClaimStatusQueued),
		now,
		id,
		string(domain.ClaimStatusFailed),
		string(domain.ClaimStatusRejected),
	)
	if err != nil {
		return false, fmt.Errorf("admin retry claim: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("admin retry rows affected: %w", err)
	}
	return rows > 0, nil
}

// AdminCancelClaim rejects an eligible not-yet-sent claim.
func (s *Store) AdminCancelClaim(ctx context.Context, id, reason string) (bool, error) {
	now := formatTime(time.Now().UTC())
	result, err := s.db.ExecContext(ctx, `
		UPDATE requests
		SET status = ?, reason = ?, next_attempt_at = NULL, updated_at = ?
		WHERE id = ? AND status NOT IN (?, ?, ?)
	`,
		string(domain.ClaimStatusRejected),
		reason,
		now,
		id,
		string(domain.ClaimStatusSent),
		string(domain.ClaimStatusConfirmed),
		string(domain.ClaimStatusSending),
	)
	if err != nil {
		return false, fmt.Errorf("admin cancel claim: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("admin cancel rows affected: %w", err)
	}
	return rows > 0, nil
}

// AppendAdminAudit persists one admin audit entry.
func (s *Store) AppendAdminAudit(ctx context.Context, entry domain.AdminAuditEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_audit_logs (action, actor, target, detail, created_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		strings.TrimSpace(entry.Action),
		strings.TrimSpace(entry.Actor),
		strings.TrimSpace(entry.Target),
		strings.TrimSpace(entry.Detail),
		strings.TrimSpace(entry.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("append admin audit: %w", err)
	}
	return nil
}

// ListAdminAudit returns recent persisted admin audit entries in chronological
// order up to limit items.
func (s *Store) ListAdminAudit(ctx context.Context, limit int) ([]domain.AdminAuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT action, actor, target, detail, created_at
		FROM (
			SELECT id, action, actor, target, detail, created_at
			FROM admin_audit_logs
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, id ASC
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list admin audit: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.AdminAuditEntry, 0, limit)
	for rows.Next() {
		var entry domain.AdminAuditEntry
		if err := rows.Scan(&entry.Action, &entry.Actor, &entry.Target, &entry.Detail, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListAdminBlocklist returns persisted admin blocklist entries ordered by most
// recently blocked first.
func (s *Store) ListAdminBlocklist(ctx context.Context) ([]abuse.BlocklistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key_type, key_value, reason, blocked_at
		FROM admin_blocklist
		ORDER BY blocked_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list admin blocklist: %w", err)
	}
	defer rows.Close()

	entries := make([]abuse.BlocklistEntry, 0)
	for rows.Next() {
		var (
			entry   abuse.BlocklistEntry
			keyType string
			atText  string
		)
		if err := rows.Scan(&keyType, &entry.Key, &entry.Reason, &atText); err != nil {
			return nil, err
		}
		entry.KeyType = abuse.KeyType(keyType)
		blockedAt, err := parseTime(atText)
		if err != nil {
			return nil, fmt.Errorf("invalid admin blocklist blocked_at: %w", err)
		}
		entry.BlockedAt = blockedAt
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// AdminBlocklistAdd inserts or updates a persisted blocklist entry.
func (s *Store) AdminBlocklistAdd(ctx context.Context, keyType abuse.KeyType, value, reason string) error {
	if !abuse.ValidKeyType(keyType) {
		return errors.New("invalid blocklist key type")
	}
	canonical := abuse.CanonicalizeBlocklistValue(keyType, value)
	if canonical == "" {
		return errors.New("empty blocklist value")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_blocklist (key_type, key_value, reason, blocked_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key_type, key_value)
		DO UPDATE SET reason = excluded.reason, blocked_at = excluded.blocked_at
	`, string(keyType), canonical, strings.TrimSpace(reason), formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("add admin blocklist entry: %w", err)
	}
	return nil
}

// AdminBlocklistRemove removes a persisted blocklist entry. Missing entries are
// ignored to preserve existing admin API semantics.
func (s *Store) AdminBlocklistRemove(ctx context.Context, keyType abuse.KeyType, value string) error {
	if !abuse.ValidKeyType(keyType) {
		return errors.New("invalid blocklist key type")
	}
	canonical := abuse.CanonicalizeBlocklistValue(keyType, value)
	if canonical == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM admin_blocklist WHERE key_type = ? AND key_value = ?`, string(keyType), canonical); err != nil {
		return fmt.Errorf("remove admin blocklist entry: %w", err)
	}
	return nil
}

// IsBlocked checks whether a persisted blocklist entry exists for keyType and value.
func (s *Store) IsBlocked(ctx context.Context, keyType abuse.KeyType, value string) (bool, string, error) {
	if !abuse.ValidKeyType(keyType) {
		return false, "", errors.New("invalid blocklist key type")
	}
	canonical := abuse.CanonicalizeBlocklistValue(keyType, value)
	if canonical == "" {
		return false, "", nil
	}
	var reason string
	err := s.db.QueryRowContext(ctx, `SELECT reason FROM admin_blocklist WHERE key_type = ? AND key_value = ? LIMIT 1`, string(keyType), canonical).Scan(&reason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("check admin blocklist: %w", err)
	}
	return true, reason, nil
}

// LastClaimByAddressAndToken returns the latest persisted claim for one address and token_id.
func (s *Store) LastClaimByAddressAndToken(ctx context.Context, address common.Address, tokenID string) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		WHERE address = ? AND token_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, address.Hex(), strings.TrimSpace(tokenID))

	claim, err := scanClaim(row)
	if err != nil {
		return domain.Claim{}, err
	}
	return claim, nil
}

func (s *Store) LastClaimByAddress(ctx context.Context, address common.Address) (domain.Claim, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
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

// DailyClaimAmountWei sums persisted claim amounts in [dayStart, dayEnd) for the given statuses.
func (s *Store) DailyClaimAmountWei(ctx context.Context, dayStart, dayEnd time.Time, statuses []domain.ClaimStatus) (*big.Int, error) {
	return claimAmountWei(ctx, s.db, dayStart, dayEnd, statuses)
}

type claimAmountQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func claimAmountWei(ctx context.Context, q claimAmountQuerier, dayStart, dayEnd time.Time, statuses []domain.ClaimStatus) (*big.Int, error) {
	return claimAmountWeiForToken(ctx, q, "", dayStart, dayEnd, statuses)
}

func claimAmountWeiForToken(ctx context.Context, q claimAmountQuerier, tokenID string, dayStart, dayEnd time.Time, statuses []domain.ClaimStatus) (*big.Int, error) {
	if len(statuses) == 0 {
		return big.NewInt(0), nil
	}

	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+3)
	args = append(args, formatBudgetBound(dayStart), formatBudgetBound(dayEnd))
	whereToken := ""
	if strings.TrimSpace(tokenID) != "" {
		whereToken = " AND token_id = ?"
		args = append(args, strings.TrimSpace(tokenID))
	}
	for i, status := range statuses {
		placeholders[i] = "?"
		args = append(args, string(status))
	}

	rows, err := q.QueryContext(ctx, `
		SELECT amount_wei
		FROM requests
		WHERE created_at >= ? AND created_at < ?`+whereToken+`
			AND status IN (`+strings.Join(placeholders, ", ")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	total := big.NewInt(0)
	for rows.Next() {
		var amountWei string
		if err := rows.Scan(&amountWei); err != nil {
			return nil, err
		}
		amount, ok := new(big.Int).SetString(amountWei, 10)
		if !ok {
			return nil, fmt.Errorf("invalid stored amount wei: %s", amountWei)
		}
		total.Add(total, amount)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return total, nil
}

func formatBudgetBound(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
}

type claimScanner interface {
	Scan(dest ...any) error
}

func scanClaim(scanner claimScanner) (domain.Claim, error) {
	var (
		id             string
		address        string
		tokenID        string
		tokenSymbol    string
		tokenType      string
		tokenAddress   string
		tokenDecimals  int
		amountWei      string
		status         string
		reason         string
		retryCount     int
		nextAttemptAt  sql.NullString
		campaignID     string
		invitationCode string
		createdAt      string
		updatedAt      string
	)

	if err := scanner.Scan(&id, &address, &tokenID, &tokenSymbol, &tokenType, &tokenAddress, &tokenDecimals, &amountWei, &status, &reason, &retryCount, &nextAttemptAt, &campaignID, &invitationCode, &createdAt, &updatedAt); err != nil {
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
		ID:             id,
		Address:        common.HexToAddress(address),
		TokenID:        tokenID,
		TokenSymbol:    tokenSymbol,
		TokenType:      domain.TokenType(tokenType),
		TokenAddress:   common.HexToAddress(tokenAddress),
		TokenDecimals:  tokenDecimals,
		AmountWei:      amount,
		Status:         domain.ClaimStatus(status),
		Reason:         reason,
		RetryCount:     retryCount,
		NextAttemptAt:  nextAttempt,
		CampaignID:     campaignID,
		InvitationCode: invitationCode,
		CreatedAt:      created,
		UpdatedAt:      updated,
	}, nil
}

func tokenAddressHex(address common.Address) string {
	if address == (common.Address{}) {
		return ""
	}
	return address.Hex()
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
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
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
				(request_id, tx_hash, from_address, to_address, token_id, token_symbol, token_type,
				 token_address, token_decimals, value_wei, status, block_number, gas_used, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, claimID, tx.Hash.Hex(), tx.From.Hex(), tx.To.Hex(), tx.TokenID, tx.TokenSymbol,
			string(tx.TokenType), tokenAddressHex(tx.TokenAddress), tx.TokenDecimals, valueWei,
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

// ── WatcherStore ─────────────────────────────────────────────────────────────

var _ domain.WatcherStore = (*Store)(nil)

// ListPendingTransactions returns up to limit claims in 'sent' state that have
// an associated transaction row.
func (s *Store) ListPendingTransactions(ctx context.Context, limit int) ([]domain.PendingTx, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.request_id, t.tx_hash
		FROM transactions t
		JOIN requests r ON r.id = t.request_id
		WHERE r.status = ?
		LIMIT ?
	`, string(domain.ClaimStatusSent), limit)
	if err != nil {
		return nil, fmt.Errorf("list pending transactions: %w", err)
	}
	defer rows.Close()

	var out []domain.PendingTx
	for rows.Next() {
		var claimID, txHash string
		if err := rows.Scan(&claimID, &txHash); err != nil {
			return nil, err
		}
		out = append(out, domain.PendingTx{
			ClaimID: claimID,
			TxHash:  common.HexToHash(txHash),
		})
	}
	return out, rows.Err()
}

// ConfirmTransaction marks the claim as 'confirmed' and updates the transaction
// record with blockNumber and gasUsed.  Both writes are atomic.
func (s *Store) ConfirmTransaction(ctx context.Context, claimID string, blockNumber, gasUsed uint64) error {
	now := formatTime(time.Now().UTC())

	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin confirm tx: %w", err)
	}
	defer dbTx.Rollback() //nolint:errcheck

	result, err := dbTx.ExecContext(ctx, `
		UPDATE requests SET status = ?, updated_at = ? WHERE id = ? AND status = ?
	`, string(domain.ClaimStatusConfirmed), now, claimID, string(domain.ClaimStatusSent))
	if err != nil {
		return fmt.Errorf("confirm claim: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}

	if _, err := dbTx.ExecContext(ctx, `
		UPDATE transactions
		SET status = ?, block_number = ?, gas_used = ?, updated_at = ?
		WHERE request_id = ?
	`, string(domain.ClaimStatusConfirmed), blockNumber, gasUsed, now, claimID); err != nil {
		return fmt.Errorf("confirm transaction record: %w", err)
	}

	return dbTx.Commit()
}

// FailTransaction marks the claim and its transaction record as 'failed'.
func (s *Store) FailTransaction(ctx context.Context, claimID string, reason string) error {
	now := formatTime(time.Now().UTC())

	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail tx: %w", err)
	}
	defer dbTx.Rollback() //nolint:errcheck

	if _, err := dbTx.ExecContext(ctx, `
		UPDATE requests SET status = ?, reason = ?, updated_at = ? WHERE id = ?
	`, string(domain.ClaimStatusFailed), reason, now, claimID); err != nil {
		return fmt.Errorf("fail claim: %w", err)
	}

	// Update transaction record if one exists.
	if _, err := dbTx.ExecContext(ctx, `
		UPDATE transactions SET status = ?, updated_at = ? WHERE request_id = ?
	`, string(domain.ClaimStatusFailed), now, claimID); err != nil {
		return fmt.Errorf("fail transaction record: %w", err)
	}

	return dbTx.Commit()
}

// ListStuckSending returns claims that have been in 'sending' state for longer
// than stuckAfter.
func (s *Store) ListStuckSending(ctx context.Context, stuckAfter time.Duration, limit int) ([]domain.Claim, error) {
	if limit <= 0 {
		return nil, nil
	}
	cutoff := formatTime(time.Now().UTC().Add(-stuckAfter))
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, address, token_id, token_symbol, token_type, token_address, token_decimals, amount_wei, status, reason, retry_count, next_attempt_at, campaign_id, invitation_code, created_at, updated_at
		FROM requests
		WHERE status = ? AND updated_at <= ?
		ORDER BY updated_at ASC
		LIMIT ?
	`, string(domain.ClaimStatusSending), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list stuck sending: %w", err)
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
	return claims, rows.Err()
}

const (
	runtimePolicyCooldownSeconds     = "cooldown_seconds"
	runtimePolicyRateLimitIPPerHour  = "rate_limit_ip_per_hour"
	runtimePolicyRateLimitAddrPerDay = "rate_limit_addr_per_day"
	runtimePolicyDailyBudgetWei      = "daily_budget_wei"
	runtimePolicyTokenBudgetPrefix   = "token_daily_budget_wei:"
)

// GetRuntimePolicy returns persisted runtime policy overrides. Unknown or
// malformed values are ignored so env/default configuration remains the fallback.
func (s *Store) GetRuntimePolicy(ctx context.Context) (domain.RuntimePolicy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM runtime_policy`)
	if err != nil {
		return domain.RuntimePolicy{}, fmt.Errorf("get runtime policy: %w", err)
	}
	defer rows.Close()
	policy := domain.RuntimePolicy{TokenDailyBudgetWei: make(map[string]*big.Int)}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return domain.RuntimePolicy{}, err
		}
		value = strings.TrimSpace(value)
		switch key {
		case runtimePolicyCooldownSeconds:
			policy.CooldownSeconds = parseRuntimePolicyInt(value)
		case runtimePolicyRateLimitIPPerHour:
			policy.RateLimitIPPerHour = parseRuntimePolicyInt(value)
		case runtimePolicyRateLimitAddrPerDay:
			policy.RateLimitAddrPerDay = parseRuntimePolicyInt(value)
		case runtimePolicyDailyBudgetWei:
			policy.DailyBudgetWei = parseRuntimePolicyBig(value)
		default:
			if strings.HasPrefix(key, runtimePolicyTokenBudgetPrefix) {
				tokenID := strings.TrimSpace(strings.TrimPrefix(key, runtimePolicyTokenBudgetPrefix))
				if tokenID != "" {
					if parsed := parseRuntimePolicyBig(value); parsed != nil {
						policy.TokenDailyBudgetWei[tokenID] = parsed
					}
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return domain.RuntimePolicy{}, err
	}
	if len(policy.TokenDailyBudgetWei) == 0 {
		policy.TokenDailyBudgetWei = nil
	}
	return policy, nil
}

// SetRuntimePolicy atomically replaces persisted runtime policy overrides.
// Invalid negative values are rejected at the persistence boundary as a final
// defense even when callers normally validate through the admin/domain layer.
func (s *Store) SetRuntimePolicy(ctx context.Context, policy domain.RuntimePolicy) error {
	if err := validateRuntimePolicyForPersistence(policy); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin runtime policy update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM runtime_policy`); err != nil {
		return fmt.Errorf("clear runtime policy: %w", err)
	}
	now := formatTime(time.Now().UTC())
	insert := func(key, value string) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO runtime_policy (key, value, updated_at) VALUES (?, ?, ?)`, key, value, now)
		return err
	}
	if policy.CooldownSeconds > 0 {
		if err := insert(runtimePolicyCooldownSeconds, fmt.Sprintf("%d", policy.CooldownSeconds)); err != nil {
			return fmt.Errorf("set runtime cooldown: %w", err)
		}
	}
	if policy.RateLimitIPPerHour > 0 {
		if err := insert(runtimePolicyRateLimitIPPerHour, fmt.Sprintf("%d", policy.RateLimitIPPerHour)); err != nil {
			return fmt.Errorf("set runtime ip limit: %w", err)
		}
	}
	if policy.RateLimitAddrPerDay > 0 {
		if err := insert(runtimePolicyRateLimitAddrPerDay, fmt.Sprintf("%d", policy.RateLimitAddrPerDay)); err != nil {
			return fmt.Errorf("set runtime addr limit: %w", err)
		}
	}
	if policy.DailyBudgetWei != nil {
		if err := insert(runtimePolicyDailyBudgetWei, policy.DailyBudgetWei.String()); err != nil {
			return fmt.Errorf("set runtime daily budget: %w", err)
		}
	}
	for tokenID, budget := range policy.TokenDailyBudgetWei {
		tokenID = strings.TrimSpace(tokenID)
		if tokenID == "" || budget == nil {
			continue
		}
		if err := insert(runtimePolicyTokenBudgetPrefix+tokenID, budget.String()); err != nil {
			return fmt.Errorf("set runtime token budget: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit runtime policy update: %w", err)
	}
	return nil
}

// ClearRuntimePolicy removes all runtime overrides so env/default config takes effect.
func (s *Store) ClearRuntimePolicy(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM runtime_policy`)
	if err != nil {
		return fmt.Errorf("clear runtime policy: %w", err)
	}
	return nil
}

func validateRuntimePolicyForPersistence(policy domain.RuntimePolicy) error {
	if policy.CooldownSeconds < 0 || policy.RateLimitIPPerHour < 0 || policy.RateLimitAddrPerDay < 0 {
		return fmt.Errorf("invalid runtime policy: integer overrides must be non-negative")
	}
	if policy.DailyBudgetWei != nil && policy.DailyBudgetWei.Sign() < 0 {
		return fmt.Errorf("invalid runtime policy: daily budget must be non-negative")
	}
	for tokenID, budget := range policy.TokenDailyBudgetWei {
		if strings.TrimSpace(tokenID) == "" {
			return fmt.Errorf("invalid runtime policy: token budget id is required")
		}
		if budget != nil && budget.Sign() < 0 {
			return fmt.Errorf("invalid runtime policy: token budget must be non-negative")
		}
	}
	return nil
}

func parseRuntimePolicyInt(raw string) int {
	v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || !v.IsInt64() || v.Sign() <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if v.Int64() > maxInt {
		return 0
	}
	return int(v.Int64())
}

func parseRuntimePolicyBig(raw string) *big.Int {
	v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || v.Sign() < 0 {
		return nil
	}
	return v
}

// CreateCampaign inserts or replaces no existing campaign; ids are stable and unique.
func (s *Store) CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	campaign.ID = strings.TrimSpace(campaign.ID)
	campaign.Name = strings.TrimSpace(campaign.Name)
	campaign.TokenID = strings.TrimSpace(campaign.TokenID)
	if campaign.ID == "" || campaign.Name == "" || !domain.IsValidCampaignScope(campaign.Scope) {
		return domain.Campaign{}, fmt.Errorf("invalid campaign")
	}
	now := time.Now().UTC()
	if campaign.CreatedAt.IsZero() {
		campaign.CreatedAt = now
	}
	if campaign.UpdatedAt.IsZero() {
		campaign.UpdatedAt = campaign.CreatedAt
	}
	budget := ""
	if campaign.BudgetWei != nil {
		if campaign.BudgetWei.Sign() < 0 {
			return domain.Campaign{}, fmt.Errorf("invalid campaign budget")
		}
		budget = campaign.BudgetWei.String()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO campaigns (id, name, token_id, scope, budget_wei, enabled, starts_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, campaign.ID, campaign.Name, campaign.TokenID, string(campaign.Scope), budget, boolInt(campaign.Enabled), nullableTime(campaign.StartsAt), nullableTime(campaign.EndsAt), formatTime(campaign.CreatedAt), formatTime(campaign.UpdatedAt))
	if err != nil {
		return domain.Campaign{}, err
	}
	return campaign, nil
}

func (s *Store) GetCampaign(ctx context.Context, id string) (domain.Campaign, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, token_id, scope, budget_wei, enabled, starts_at, ends_at, created_at, updated_at FROM campaigns WHERE id = ?`, strings.TrimSpace(id))
	return scanCampaign(row)
}

func (s *Store) ListCampaigns(ctx context.Context, limit, offset int) ([]domain.Campaign, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, token_id, scope, budget_wei, enabled, starts_at, ends_at, created_at, updated_at FROM campaigns ORDER BY created_at DESC, id ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DisableCampaign(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE campaigns SET enabled = 0, updated_at = ? WHERE id = ?`, formatTime(time.Now().UTC()), strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetCampaignEnabled updates only the enabled flag. It is used by the
// admin service as a best-effort rollback primitive if durable audit writing
// fails after a disable operation.
func (s *Store) SetCampaignEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE campaigns SET enabled = ?, updated_at = ? WHERE id = ?`, boolInt(enabled), formatTime(time.Now().UTC()), strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCampaign removes a campaign that has no dependent rows. It is intended
// for admin rollback after an audit failure immediately following creation.
func (s *Store) DeleteCampaign(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM campaigns WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateInvitationCode(ctx context.Context, code domain.InvitationCode) (domain.InvitationCode, error) {
	code.Code = strings.TrimSpace(code.Code)
	code.CampaignID = strings.TrimSpace(code.CampaignID)
	if code.Code == "" || code.CampaignID == "" || code.MaxUses < 0 || code.Uses < 0 || (code.MaxUses > 0 && code.Uses > code.MaxUses) {
		return domain.InvitationCode{}, fmt.Errorf("invalid invitation code")
	}
	if _, err := s.GetCampaign(ctx, code.CampaignID); err != nil {
		return domain.InvitationCode{}, err
	}
	now := time.Now().UTC()
	if code.CreatedAt.IsZero() {
		code.CreatedAt = now
	}
	if code.UpdatedAt.IsZero() {
		code.UpdatedAt = code.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO invitation_codes (code, campaign_id, max_uses, uses, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, code.Code, code.CampaignID, code.MaxUses, code.Uses, boolInt(code.Enabled), formatTime(code.CreatedAt), formatTime(code.UpdatedAt))
	if err != nil {
		return domain.InvitationCode{}, err
	}
	return code, nil
}

func (s *Store) GetInvitationCode(ctx context.Context, code string) (domain.InvitationCode, error) {
	row := s.db.QueryRowContext(ctx, `SELECT code, campaign_id, max_uses, uses, enabled, created_at, updated_at FROM invitation_codes WHERE code = ?`, strings.TrimSpace(code))
	return scanInvitationCode(row)
}

func (s *Store) ConsumeInvitationCode(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE invitation_codes SET uses = uses + 1, updated_at = ? WHERE code = ? AND enabled = 1 AND (max_uses = 0 OR uses < max_uses)`, formatTime(time.Now().UTC()), strings.TrimSpace(code))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteInvitationCode removes an invitation created by an admin operation that
// could not be durably audited.
func (s *Store) DeleteInvitationCode(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM invitation_codes WHERE code = ?`, strings.TrimSpace(code))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddCampaignAllowlistEntry(ctx context.Context, entry domain.CampaignAllowlistEntry) error {
	entry.CampaignID = strings.TrimSpace(entry.CampaignID)
	if entry.CampaignID == "" || entry.Address == (common.Address{}) {
		return fmt.Errorf("invalid allowlist entry")
	}
	if _, err := s.GetCampaign(ctx, entry.CampaignID); err != nil {
		return err
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO campaign_allowlist (campaign_id, address, note, created_at) VALUES (?, ?, ?, ?)`, entry.CampaignID, entry.Address.Hex(), strings.TrimSpace(entry.Note), formatTime(entry.CreatedAt))
	return err
}

func (s *Store) IsAddressAllowlisted(ctx context.Context, campaignID string, address common.Address) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_allowlist WHERE campaign_id = ? AND address = ?`, strings.TrimSpace(campaignID), address.Hex()).Scan(&count)
	return count > 0, err
}

// RemoveCampaignAllowlistEntry removes one allowlist entry for admin rollback.
func (s *Store) RemoveCampaignAllowlistEntry(ctx context.Context, campaignID string, address common.Address) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM campaign_allowlist WHERE campaign_id = ? AND address = ?`, strings.TrimSpace(campaignID), address.Hex())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CampaignUsage(ctx context.Context, campaignID string, statuses []domain.ClaimStatus) (domain.CampaignUsage, error) {
	campaignID = strings.TrimSpace(campaignID)
	usage := domain.CampaignUsage{CampaignID: campaignID, UsedWei: big.NewInt(0)}
	if campaignID == "" || len(statuses) == 0 {
		return usage, nil
	}
	placeholders := make([]string, 0, len(statuses))
	args := []any{campaignID}
	for _, st := range statuses {
		placeholders = append(placeholders, "?")
		args = append(args, string(st))
	}
	rows, err := s.db.QueryContext(ctx, `SELECT amount_wei FROM requests WHERE campaign_id = ? AND status IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return usage, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return usage, err
		}
		v, ok := new(big.Int).SetString(raw, 10)
		if !ok {
			return usage, fmt.Errorf("invalid campaign amount wei: %s", raw)
		}
		usage.ClaimCount++
		usage.UsedWei.Add(usage.UsedWei, v)
	}
	return usage, rows.Err()
}

type campaignScanner interface{ Scan(dest ...any) error }

func scanCampaign(scanner campaignScanner) (domain.Campaign, error) {
	var id, name, tokenID, scope, budget, createdAt, updatedAt string
	var enabled int
	var startsAt, endsAt sql.NullString
	if err := scanner.Scan(&id, &name, &tokenID, &scope, &budget, &enabled, &startsAt, &endsAt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Campaign{}, ErrNotFound
		}
		return domain.Campaign{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return domain.Campaign{}, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return domain.Campaign{}, err
	}
	var starts, ends *time.Time
	if startsAt.Valid {
		t, err := parseTime(startsAt.String)
		if err != nil {
			return domain.Campaign{}, err
		}
		starts = &t
	}
	if endsAt.Valid {
		t, err := parseTime(endsAt.String)
		if err != nil {
			return domain.Campaign{}, err
		}
		ends = &t
	}
	var b *big.Int
	if strings.TrimSpace(budget) != "" {
		v, ok := new(big.Int).SetString(budget, 10)
		if !ok {
			return domain.Campaign{}, fmt.Errorf("invalid campaign budget wei: %s", budget)
		}
		b = v
	}
	return domain.Campaign{ID: id, Name: name, TokenID: tokenID, Scope: domain.CampaignScope(scope), BudgetWei: b, Enabled: enabled != 0, StartsAt: starts, EndsAt: ends, CreatedAt: created, UpdatedAt: updated}, nil
}

func scanInvitationCode(scanner campaignScanner) (domain.InvitationCode, error) {
	var c domain.InvitationCode
	var createdAt, updatedAt string
	var enabled int
	if err := scanner.Scan(&c.Code, &c.CampaignID, &c.MaxUses, &c.Uses, &enabled, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InvitationCode{}, ErrNotFound
		}
		return domain.InvitationCode{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return domain.InvitationCode{}, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return domain.InvitationCode{}, err
	}
	c.Enabled = enabled != 0
	c.CreatedAt = created
	c.UpdatedAt = updated
	return c, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return formatTime(*t)
}
