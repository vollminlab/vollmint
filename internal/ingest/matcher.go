package ingest

import (
	"context"
	"regexp"

	"github.com/vollminlab/vollmint/internal/store"
)

// cardPaymentRe covers the payment descriptors of the household's issuers
// (Chase, Discover) plus generic autopay wording. Ordinary purchases never
// match these; extend the list if a new issuer joins.
var cardPaymentRe = regexp.MustCompile(`(?i)(E-PAYMENT|EPAYMENT|AUTOPAY|CARD ?PYMT|CRD PMT|PAYMENT THANK YOU|CHASE CREDIT CRD|DISCOVER +PAYMENT)`)

var venmoRe = regexp.MustCompile(`(?i)VENMO`)

// MatchTransfers pairs (a) bank-side VENMO debits with venmo_csv rows and
// (b) checking↔card payment legs. Runs inside one DB transaction; each row
// pairs at most once; returns the number of new pairs.
func MatchTransfers(ctx context.Context, s *store.Store) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var transferCat int
	if err := tx.QueryRow(ctx,
		`SELECT id FROM categories WHERE name='Transfer'`).Scan(&transferCat); err != nil {
		return 0, err
	}

	// User-assigned categories are sticky: pairing may only replace NULL or
	// the seed "Needs Venmo detail" placeholder with Transfer, never a
	// category a user set by hand.
	var needsVenmoCat int
	if err := tx.QueryRow(ctx,
		`SELECT id FROM categories WHERE name='Needs Venmo detail'`).Scan(&needsVenmoCat); err != nil {
		return 0, err
	}

	pairs := 0

	// (a) Venmo: bank debit ←→ venmo_csv row, equal amount, ±3 days.
	// Only the bank side becomes Transfer; the venmo side carries the spend.
	rows, err := tx.Query(ctx, `
		SELECT b.id, v.id FROM transactions b
		JOIN LATERAL (
		  SELECT id FROM transactions v
		  WHERE v.source='venmo_csv' AND v.transfer_peer_id IS NULL
		    AND v.amount = b.amount
		    AND v.posted BETWEEN b.posted - 3 AND b.posted + 3
		  ORDER BY abs(v.posted - b.posted), v.id LIMIT 1
		) v ON true
		WHERE b.source='simplefin' AND b.transfer_peer_id IS NULL
		  AND b.amount < 0 AND b.description ~* 'VENMO'
		ORDER BY b.id`)
	if err != nil {
		return 0, err
	}
	type pair struct{ a, b int64 }
	var venmoPairs []pair
	taken := map[int64]bool{}
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.a, &p.b); err != nil {
			rows.Close()
			return 0, err
		}
		if !taken[p.b] { // a venmo row can satisfy only one bank debit
			taken[p.b] = true
			venmoPairs = append(venmoPairs, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range venmoPairs {
		if _, err := tx.Exec(ctx, `UPDATE transactions
			SET transfer_peer_id=$1,
			    category_id = CASE WHEN category_id IS NULL OR category_id=$2 THEN $3 ELSE category_id END,
			    updated_at=now()
			WHERE id=$4`,
			p.b, needsVenmoCat, transferCat, p.a); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE transactions
			SET transfer_peer_id=$1, updated_at=now() WHERE id=$2`, p.a, p.b); err != nil {
			return 0, err
		}
		pairs++
	}

	// (b) Card payments: negative leg + positive leg, equal magnitude,
	// different simplefin accounts, ±5 days, payment-descriptor on either leg.
	rows, err = tx.Query(ctx, `
		SELECT o.id, i.id, o.description, i.description FROM transactions o
		JOIN LATERAL (
		  SELECT id, description FROM transactions i
		  WHERE i.source='simplefin' AND i.transfer_peer_id IS NULL
		    AND i.account_id <> o.account_id
		    AND i.amount = -o.amount AND i.amount > 0
		    AND i.posted BETWEEN o.posted - 5 AND o.posted + 5
		  ORDER BY abs(i.posted - o.posted), i.id LIMIT 1
		) i ON true
		WHERE o.source='simplefin' AND o.transfer_peer_id IS NULL AND o.amount < 0
		ORDER BY o.id`)
	if err != nil {
		return 0, err
	}
	var cardPairs []pair
	takenIn := map[int64]bool{}
	for rows.Next() {
		var p pair
		var descO, descI string
		if err := rows.Scan(&p.a, &p.b, &descO, &descI); err != nil {
			rows.Close()
			return 0, err
		}
		// Require a payment descriptor and exclude Venmo legs (handled above).
		if takenIn[p.b] || venmoRe.MatchString(descO) {
			continue
		}
		if cardPaymentRe.MatchString(descO) || cardPaymentRe.MatchString(descI) {
			takenIn[p.b] = true
			cardPairs = append(cardPairs, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range cardPairs {
		for _, upd := range []struct{ id, peer int64 }{{p.a, p.b}, {p.b, p.a}} {
			if _, err := tx.Exec(ctx, `UPDATE transactions
				SET transfer_peer_id=$1,
				    category_id = CASE WHEN category_id IS NULL OR category_id=$2 THEN $3 ELSE category_id END,
				    updated_at=now()
				WHERE id=$4`,
				upd.peer, needsVenmoCat, transferCat, upd.id); err != nil {
				return 0, err
			}
		}
		pairs++
	}

	return pairs, tx.Commit(ctx)
}
