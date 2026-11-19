package dbtest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
)

// ErrInjectedFault is returned by the statement a Faults counter is armed for.
var ErrInjectedFault = errors.New("dbtest: injected database fault")

// Faults counts database round trips (begin, exec, query, commit) made through
// a FaultInjectingDB and can fail exactly the Nth one, so a test can reach every
// error branch of a store method without a misbehaving database.
type Faults struct {
	mu     sync.Mutex
	count  int
	failAt int
	// Row faults count rows delivered by Next across all queries.
	rows       int
	rowFailAt  int
	rowCorrupt bool
	// firstRows holds the index of each result set's first row seen since
	// the last Arm: row errors are handled per query, not per row.
	firstRows []int
	// Result faults count RowsAffected reads on exec results.
	results      int
	resultFailAt int
	// hook runs just before round trip hookAt, to interleave a concurrent change.
	hookAt int
	hook   func()
}

// reset clears every counter and disarms every kind of fault.
func (f *Faults) reset() {
	f.count, f.failAt = 0, 0
	f.rows, f.rowFailAt, f.firstRows = 0, 0, nil
	f.results, f.resultFailAt = 0, 0
	f.hookAt, f.hook = 0, nil
}

// Arm resets the counters and makes the nth subsequent round trip fail; n <= 0
// disables injection.
func (f *Faults) Arm(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reset()
	f.failAt = n
}

// ArmRow resets the counters and makes the nth row read fail: with corrupt
// false Next returns an error (surfacing through rows.Err or Row.Scan); with
// corrupt true the row is delivered with a value no destination accepts, so
// Scan fails. n <= 0 disables row injection.
func (f *Faults) ArmRow(n int, corrupt bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reset()
	f.rowFailAt = n
	f.rowCorrupt = corrupt
}

// ArmResult resets the counters and makes the nth RowsAffected read on an
// exec result fail, which real drivers may do but pgx never does. n <= 0
// disables result injection.
func (f *Faults) ArmResult(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reset()
	f.resultFailAt = n
}

// ArmHook resets the counters and runs hook just before the nth round trip,
// so a test can make a concurrent change land between two statements of the
// operation under test. The hook must use a different handle.
func (f *Faults) ArmHook(n int, hook func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reset()
	f.hookAt, f.hook = n, hook
}

// resultStep counts one RowsAffected read and reports whether to fail it.
func (f *Faults) resultStep() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results++
	return f.resultFailAt > 0 && f.results == f.resultFailAt
}

// rowStep counts one row and reports whether to fail or corrupt it.
func (f *Faults) rowStep(first bool) (fail, corrupt bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows++
	if first {
		f.firstRows = append(f.firstRows, f.rows)
	}
	if f.rowFailAt > 0 && f.rows == f.rowFailAt {
		return !f.rowCorrupt, f.rowCorrupt
	}
	return false, false
}

// Count reports the round trips seen since the last Arm.
func (f *Faults) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

func (f *Faults) step() error {
	f.mu.Lock()
	f.count++
	failed := f.failAt > 0 && f.count == f.failAt
	var hook func()
	if f.hook != nil && f.count == f.hookAt {
		hook = f.hook
	}
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if failed {
		return ErrInjectedFault
	}
	return nil
}

var faultDriverSeq atomic.Int64

// FaultInjectingDB opens a second handle on the same isolated schema as db,
// routed through a counting driver. Statements made through the returned
// handle are observed by the returned Faults.
func FaultInjectingDB(t testing.TB, envVar string, db *sql.DB) (*sql.DB, *Faults) {
	t.Helper()
	faults := &Faults{}
	name := fmt.Sprintf("pgx-fault-%d", faultDriverSeq.Add(1))
	sql.Register(name, faultDriver{inner: stdlib.GetDefaultDriver(), faults: faults})
	// sql.Open only fails for unknown drivers, and this one was just registered.
	faulty, _ := sql.Open(name, SchemaDSN(t, envVar, db))
	// No connection cap: production paths hold one connection (an advisory
	// lock, an open transaction) while issuing statements on another, and a
	// one-connection pool would deadlock them. Ordering stays deterministic for
	// the sequential operations the sweep drives.
	t.Cleanup(func() { faulty.Close() })
	return faulty, faults
}

// SchemaDSN returns the DSN in envVar pointed at db's isolated schema, for code
// under test that opens its own connection from configuration.
func SchemaDSN(t testing.TB, envVar string, db *sql.DB) string {
	t.Helper()
	var schema string
	if err := db.QueryRowContext(context.Background(), `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("resolve test schema: %v", err)
	}
	parsed, err := url.Parse(os.Getenv(envVar))
	if err != nil {
		t.Fatalf("%s must be a URL-form DSN: %v", envVar, err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type faultDriver struct {
	inner  driver.Driver
	faults *Faults
}

func (d faultDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &faultConn{inner: conn, faults: d.faults}, nil
}

type faultConn struct {
	inner  driver.Conn
	faults *Faults
}

var (
	_ driver.ConnBeginTx        = (*faultConn)(nil)
	_ driver.ExecerContext      = (*faultConn)(nil)
	_ driver.QueryerContext     = (*faultConn)(nil)
	_ driver.ConnPrepareContext = (*faultConn)(nil)
	_ driver.NamedValueChecker  = (*faultConn)(nil)
	_ driver.SessionResetter    = (*faultConn)(nil)
	_ driver.Validator          = (*faultConn)(nil)
	_ driver.Pinger             = (*faultConn)(nil)
)

func (c *faultConn) Prepare(query string) (driver.Stmt, error) { return c.inner.Prepare(query) }
func (c *faultConn) Close() error                              { return c.inner.Close() }

// Begin is required by driver.Conn; database/sql always prefers BeginTx.
func (c *faultConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *faultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.faults.step(); err != nil {
		return nil, err
	}
	tx, err := c.inner.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return faultTx{inner: tx, faults: c.faults}, nil
}

func (c *faultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.faults.step(); err != nil {
		return nil, err
	}
	result, err := c.inner.(driver.ExecerContext).ExecContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return faultResult{Result: result, faults: c.faults}, nil
}

// faultResult applies result faults to an exec result.
type faultResult struct {
	driver.Result
	faults *Faults
}

func (r faultResult) RowsAffected() (int64, error) {
	if r.faults.resultStep() {
		return 0, ErrInjectedFault
	}
	return r.Result.RowsAffected()
}

func (c *faultConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.faults.step(); err != nil {
		return nil, err
	}
	rows, err := c.inner.(driver.QueryerContext).QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return &faultRows{Rows: rows, faults: c.faults}, nil
}

// faultRows applies row faults to a result set.
type faultRows struct {
	driver.Rows
	faults *Faults
	seen   bool
}

// unscannable is a driver value no Scan destination can convert.
type unscannable struct{}

func (r *faultRows) Next(dest []driver.Value) error {
	if err := r.Rows.Next(dest); err != nil {
		return err
	}
	fail, corrupt := r.faults.rowStep(!r.seen)
	r.seen = true
	if fail {
		return ErrInjectedFault
	}
	if corrupt {
		for i := range dest {
			dest[i] = unscannable{}
		}
	}
	return nil
}

func (c *faultConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.inner.(driver.ConnPrepareContext).PrepareContext(ctx, query)
}

func (c *faultConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.inner.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *faultConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.inner.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *faultConn) IsValid() bool {
	if validator, ok := c.inner.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *faultConn) Ping(ctx context.Context) error {
	if pinger, ok := c.inner.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

type faultTx struct {
	inner  driver.Tx
	faults *Faults
}

func (t faultTx) Commit() error {
	if err := t.faults.step(); err != nil {
		// A failed commit must not leave the transaction open on the pooled conn.
		_ = t.inner.Rollback()
		return err
	}
	return t.inner.Commit()
}

func (t faultTx) Rollback() error { return t.inner.Rollback() }

// SweepFaults runs op once to count its round trips, then once more per round
// trip with that one failing. op must tolerate being re-run (vary unique
// inputs by the run index). It fails the test if op panics, and it checks the
// store recovers: after each injected fault op must succeed when re-run clean.
func SweepFaults(t testing.TB, faults *Faults, name string, op func(run int) error) {
	t.Helper()
	cleanRun(t, faults, name, op)
	total := faults.Count()
	if total == 0 {
		t.Fatalf("%s: made no database round trips; is it using the fault-injecting handle?", name)
	}
	sweep(t, faults, name, op, positions(total), faults.Arm, func(at int) string {
		return fmt.Sprintf("fault at round trip %d/%d", at, total)
	})
}

// SweepResultFaults is SweepFaults for exec results: it fails each
// RowsAffected read op makes, checking op does not panic and recovers when
// re-run. An op that reads no results is swept trivially.
func SweepResultFaults(t testing.TB, faults *Faults, name string, op func(run int) error) {
	t.Helper()
	cleanRun(t, faults, name, op)
	faults.mu.Lock()
	total := faults.results
	faults.mu.Unlock()
	sweep(t, faults, name, op, positions(total), faults.ArmResult, func(at int) string {
		return fmt.Sprintf("result fault %d/%d", at, total)
	})
}

// cleanRun runs op with injection disarmed; Fatalf ends the test (and its
// goroutine) if it errors.
func cleanRun(t testing.TB, faults *Faults, name string, op func(run int) error) {
	t.Helper()
	faults.Arm(0)
	if err := op(0); err != nil {
		t.Fatalf("%s: clean run failed: %v", name, err)
	}
}

func positions(total int) []int {
	all := make([]int, total)
	for i := range all {
		all[i] = i + 1
	}
	return all
}

// sweep arms each position in turn, runs op under that fault expecting no
// panic, then re-runs it clean expecting success. Runs after the clean one
// are numbered from 1 so op can vary unique inputs.
func sweep(t testing.TB, faults *Faults, name string, op func(run int) error, at []int, arm func(int), describe func(int) string) {
	t.Helper()
	run := 0
	for _, position := range at {
		run++
		arm(position)
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("%s: panicked with %s: %v", name, describe(position), recovered)
				}
			}()
			_ = op(run)
		}()
		run++
		faults.Arm(0)
		if err := op(run); err != nil {
			t.Errorf("%s: did not recover after %s: %v", name, describe(position), err)
		}
	}
	faults.Arm(0)
}

// SweepRowFaults is SweepFaults for row reads: it fails, then corrupts, the
// first row of each result set op reads, checking op does not panic and
// recovers when re-run. Row errors are handled once per query, so the first
// row of each result set reaches every row-handling branch.
func SweepRowFaults(t testing.TB, faults *Faults, name string, op func(run int) error) {
	t.Helper()
	cleanRun(t, faults, name, op)
	faults.mu.Lock()
	firstRows := append([]int(nil), faults.firstRows...)
	faults.mu.Unlock()
	for _, corrupt := range []bool{false, true} {
		sweep(t, faults, name, op, firstRows, func(at int) { faults.ArmRow(at, corrupt) }, func(at int) string {
			return fmt.Sprintf("row fault at row %d (corrupt=%v)", at, corrupt)
		})
	}
}
