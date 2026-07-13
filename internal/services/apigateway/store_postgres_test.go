package apigateway

import (
	"errors"
	"testing"
)

type fixedSQLResult struct {
	affected int64
	err      error
}

func (r fixedSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fixedSQLResult) RowsAffected() (int64, error) { return r.affected, r.err }

func TestRequireRowsAffected(t *testing.T) {
	notFound := errors.New("not found")

	if err := requireRowsAffected(fixedSQLResult{affected: 1}, notFound); err != nil {
		t.Fatalf("expected affected row to succeed, got %v", err)
	}
	if err := requireRowsAffected(fixedSQLResult{}, notFound); !errors.Is(err, notFound) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	resultErr := errors.New("rows affected unavailable")
	if err := requireRowsAffected(fixedSQLResult{err: resultErr}, notFound); !errors.Is(err, resultErr) {
		t.Fatalf("expected rows-affected error, got %v", err)
	}
}
