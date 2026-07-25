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
