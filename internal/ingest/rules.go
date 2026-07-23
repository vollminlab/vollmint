// Package ingest holds post-ingestion enrichment: category rules, transfer
// matching, and the sync orchestration.
package ingest

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/vollminlab/vollmint/internal/store"
)

type rule struct {
	matchType, pattern string
	lowerPattern       string
	categoryID         int
	re                 *regexp.Regexp
}

// ApplyRules assigns categories to uncategorized transactions. First matching
// rule wins (priority ASC, id ASC). Substring matches are case-insensitive
// against payee + description. Returns rows categorized.
func ApplyRules(ctx context.Context, s *store.Store) (int, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT match_type, pattern, category_id FROM category_rules ORDER BY priority, id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var rules []rule
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.matchType, &r.pattern, &r.categoryID); err != nil {
			return 0, err
		}
		if r.matchType == "regex" {
			re, err := regexp.Compile(r.pattern)
			if err != nil {
				return 0, fmt.Errorf("rule %q: %w", r.pattern, err)
			}
			r.re = re
		} else {
			r.lowerPattern = strings.ToLower(r.pattern)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	txRows, err := s.Pool.Query(ctx,
		`SELECT id, payee, description FROM transactions WHERE category_id IS NULL`)
	if err != nil {
		return 0, err
	}
	defer txRows.Close()
	type match struct {
		id  int64
		cat int
	}
	var matches []match
	for txRows.Next() {
		var id int64
		var payee, desc string
		if err := txRows.Scan(&id, &payee, &desc); err != nil {
			return 0, err
		}
		haystack := strings.ToLower(payee + " " + desc)
		for _, r := range rules {
			hit := false
			if r.re != nil {
				hit = r.re.MatchString(payee) || r.re.MatchString(desc)
			} else {
				hit = strings.Contains(haystack, r.lowerPattern)
			}
			if hit {
				matches = append(matches, match{id, r.categoryID})
				break
			}
		}
	}
	if err := txRows.Err(); err != nil {
		return 0, err
	}

	batch := &pgx.Batch{}
	for _, m := range matches {
		batch.Queue(`UPDATE transactions SET category_id=$1, updated_at=now() WHERE id=$2 AND category_id IS NULL`,
			m.cat, m.id)
	}
	results := s.Pool.SendBatch(ctx, batch)
	defer results.Close()

	var rowsAffected int
	for i := 0; i < len(matches); i++ {
		tag, err := results.Exec()
		if err != nil {
			return 0, err
		}
		rowsAffected += int(tag.RowsAffected())
	}
	return rowsAffected, nil
}
