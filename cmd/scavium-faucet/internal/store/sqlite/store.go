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
		sqlText, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
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
		SELECT id, address, amount_wei, status, reason, created_at, updated_at
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
		SELECT id, address, amount_wei, status, reason, created_at, updated_at
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
		SELECT id, address, amount_wei, status, reason, created_at, updated_at
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
		id        string
		address   string
		amountWei string
		status    string
		reason    string
		createdAt string
		updatedAt string
	)

	if err := scanner.Scan(&id, &address, &amountWei, &status, &reason, &createdAt, &updatedAt); err != nil {
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

	return domain.Claim{
		ID:        id,
		Address:   common.HexToAddress(address),
		AmountWei: amount,
		Status:    domain.ClaimStatus(status),
		Reason:    reason,
		CreatedAt: created,
		UpdatedAt: updated,
	}, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
