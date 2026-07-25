package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// TxnRow is a transaction as the API returns it. Amount is a decimal string.
type TxnRow struct {
	ID             int64   `json:"id"`
	Source         string  `json:"source"`
	AccountID      string  `json:"account_id"`
	AccountName    string  `json:"account_name"`
	Posted         string  `json:"posted"` // YYYY-MM-DD
	Amount         string  `json:"amount"`
	Description    string  `json:"description"`
	Payee          string  `json:"payee"`
	Pending        bool    `json:"pending"`
	CategoryID     *int    `json:"category_id"`
	CategoryName   *string `json:"category_name"`
	OwnerOverride  *string `json:"owner_override"`
	EffectiveOwner string  `json:"effective_owner"`
	TransferPeerID *int64  `json:"transfer_peer_id"`
}

// TxnFilter narrows a transaction listing. Zero values mean "no filter" for
// that field, except View and Month which are required by the API layer.
type TxnFilter struct {
	View          string // scott|nikki|joint|household
	Month         string // YYYY-MM ("" = no month filter)
	CategoryID    *int
	AccountID     string
	Query         string // substring match on payee/description
	Uncategorized bool
}

// ownerClause appends the effective-owner filter for a view. household → none.
// Returns the SQL fragment (may be empty) and any bind arg to append.
func ownerClause(view string, args *[]any) string {
	switch view {
	case "scott", "nikki", "joint":
		*args = append(*args, view)
		return fmt.Sprintf(" AND COALESCE(t.owner_override, a.owner) = $%d", len(*args))
	default: // household or unknown → no owner filter
		return ""
	}
}

// ListTransactions returns transactions matching the filter, newest first.
func (s *Store) ListTransactions(ctx context.Context, f TxnFilter) ([]TxnRow, error) {
	var sb strings.Builder
	args := []any{}
	sb.WriteString(`
		SELECT t.id, t.source, t.account_id, a.name, to_char(t.posted,'YYYY-MM-DD'),
		       t.amount::text, t.description, t.payee, t.pending,
		       t.category_id, c.name, t.owner_override,
		       COALESCE(t.owner_override, a.owner), t.transfer_peer_id
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE 1=1`)
	sb.WriteString(ownerClause(f.View, &args))
	if f.Month != "" {
		args = append(args, f.Month+"-01")
		sb.WriteString(fmt.Sprintf(
			" AND t.posted >= $%d::date AND t.posted < ($%d::date + interval '1 month')", len(args), len(args)))
	}
	if f.CategoryID != nil {
		args = append(args, *f.CategoryID)
		sb.WriteString(fmt.Sprintf(" AND t.category_id = $%d", len(args)))
	}
	if f.AccountID != "" {
		args = append(args, f.AccountID)
		sb.WriteString(fmt.Sprintf(" AND t.account_id = $%d", len(args)))
	}
	if f.Uncategorized {
		sb.WriteString(" AND t.category_id IS NULL")
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		sb.WriteString(fmt.Sprintf(" AND (t.payee ILIKE $%d OR t.description ILIKE $%d)", len(args), len(args)))
	}
	sb.WriteString(" ORDER BY t.posted DESC, t.id DESC")

	rows, err := s.Pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TxnRow, 0)
	for rows.Next() {
		var r TxnRow
		if err := rows.Scan(&r.ID, &r.Source, &r.AccountID, &r.AccountName, &r.Posted,
			&r.Amount, &r.Description, &r.Payee, &r.Pending,
			&r.CategoryID, &r.CategoryName, &r.OwnerOverride,
			&r.EffectiveOwner, &r.TransferPeerID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ErrNotFound is returned when an update targets a row that does not exist.
var ErrNotFound = errors.New("not found")

// TxnPatch is a partial update to a transaction. A nil field is left unchanged.
// For OwnerOverride, a non-nil pointer to "" clears the override to NULL;
// any other value sets it (validated by the DB CHECK constraint).
type TxnPatch struct {
	CategoryID    *int
	OwnerOverride *string
}

// UpdateTransaction applies a partial update. Returns ErrNotFound if no row
// with the given id exists. category_id and owner_override are the only
// user-editable fields (see spec API surface).
func (s *Store) UpdateTransaction(ctx context.Context, id int64, p TxnPatch) error {
	sets := []string{"updated_at=now()"}
	args := []any{}
	if p.CategoryID != nil {
		args = append(args, *p.CategoryID)
		sets = append(sets, fmt.Sprintf("category_id=$%d", len(args)))
	}
	if p.OwnerOverride != nil {
		if *p.OwnerOverride == "" {
			sets = append(sets, "owner_override=NULL")
		} else {
			args = append(args, *p.OwnerOverride)
			sets = append(sets, fmt.Sprintf("owner_override=$%d", len(args)))
		}
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE transactions SET %s WHERE id=$%d",
		strings.Join(sets, ", "), len(args))
	tag, err := s.Pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Category is a spending category.
type Category struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID *int   `json:"parent_id"`
	Kind     string `json:"kind"`
	IsVice   bool   `json:"is_vice"`
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, parent_id, kind, is_vice FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Kind, &c.IsVice); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory inserts a category and returns its id. kind must be one of
// spend|income|transfer|savings (enforced by the DB CHECK).
func (s *Store) CreateCategory(ctx context.Context, name, kind string, isVice bool) (int, error) {
	var id int
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO categories (name, kind, is_vice) VALUES ($1,$2,$3) RETURNING id`,
		name, kind, isVice).Scan(&id)
	return id, err
}

// CategoryPatch is a partial category update; nil fields are unchanged.
type CategoryPatch struct {
	Name   *string
	Kind   *string
	IsVice *bool
}

func (s *Store) UpdateCategory(ctx context.Context, id int, p CategoryPatch) error {
	sets := []string{}
	args := []any{}
	if p.Name != nil {
		args = append(args, *p.Name)
		sets = append(sets, fmt.Sprintf("name=$%d", len(args)))
	}
	if p.Kind != nil {
		args = append(args, *p.Kind)
		sets = append(sets, fmt.Sprintf("kind=$%d", len(args)))
	}
	if p.IsVice != nil {
		args = append(args, *p.IsVice)
		sets = append(sets, fmt.Sprintf("is_vice=$%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	tag, err := s.Pool.Exec(ctx,
		fmt.Sprintf("UPDATE categories SET %s WHERE id=$%d", strings.Join(sets, ", "), len(args)), args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Rule is a payee→category matcher.
type Rule struct {
	ID         int    `json:"id"`
	Priority   int    `json:"priority"`
	MatchType  string `json:"match_type"`
	Pattern    string `json:"pattern"`
	CategoryID int    `json:"category_id"`
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, priority, match_type, pattern, category_id FROM category_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Priority, &r.MatchType, &r.Pattern, &r.CategoryID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRule(ctx context.Context, priority int, matchType, pattern string, categoryID int) (int, error) {
	var id int
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO category_rules (priority, match_type, pattern, category_id)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		priority, matchType, pattern, categoryID).Scan(&id)
	return id, err
}

func (s *Store) DeleteRule(ctx context.Context, id int) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM category_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
