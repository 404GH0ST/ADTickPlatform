package dbtest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
)

// recordingTB stands in for a test so helper failure paths can be observed.
// Fatal and Skip end the calling goroutine, as the real methods do.
type recordingTB struct {
	testing.TB
	mu    sync.Mutex
	fatal []string
	errs  []string
	skips []string
	logs  []string
}

func (r *recordingTB) Helper() {}
func (r *recordingTB) Fatalf(format string, args ...any) {
	r.mu.Lock()
	r.fatal = append(r.fatal, fmt.Sprintf(format, args...))
	r.mu.Unlock()
	runtime.Goexit()
}
func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}
func (r *recordingTB) Skip(args ...any) {
	r.mu.Lock()
	r.skips = append(r.skips, fmt.Sprint(args...))
	r.mu.Unlock()
	runtime.Goexit()
}
func (r *recordingTB) Logf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}

// inGoroutine runs fn where Goexit cannot end the real test.
func inGoroutine(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

func requireDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	return dsn
}

func TestIsolatedSchemaFailurePaths(t *testing.T) {
	dsn := requireDSN(t)
	cases := []struct {
		name, env, prefix, want string
		skip                    bool
	}{
		{name: "unset env", skip: true},
		{name: "unreachable", env: "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1", prefix: "x", want: "open postgres"},
		{name: "bad schema name", env: dsn, prefix: "bad-name", want: "create schema"},
		{name: "keyword dsn", env: keywordDSN(dsn) + " application_name=%zz", prefix: "kw", want: "URL-form DSN"},
		{name: "unopenable schema dsn", env: keywordDSN(dsn), prefix: "kw2", want: "open schema"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DBTEST_CASE_DSN", tc.env)
			rec := &recordingTB{TB: t}
			inGoroutine(func() { IsolatedSchema(rec, "DBTEST_CASE_DSN", tc.prefix) })
			if tc.skip {
				if len(rec.skips) != 1 {
					t.Fatalf("want a skip, got %+v", rec)
				}
				return
			}
			if len(rec.fatal) != 1 || !strings.Contains(rec.fatal[0], tc.want) {
				t.Fatalf("want fatal containing %q, got %+v", tc.want, rec.fatal)
			}
		})
	}
}

// keywordDSN converts a URL DSN to libpq keyword form, which pgx accepts.
func keywordDSN(dsn string) string {
	return "host=127.0.0.1 port=" + portOf(dsn) + " user=postgres password=audit dbname=audit sslmode=disable"
}

func portOf(dsn string) string {
	host := strings.SplitN(strings.SplitN(dsn, "@", 2)[1], "/", 2)[0]
	return strings.SplitN(host, ":", 2)[1]
}

func TestIsolatedSchemaLogsFailedCleanup(t *testing.T) {
	requireDSN(t)
	rec := &recordingTB{TB: t}
	var db *sql.DB
	t.Run("inner", func(t *testing.T) {
		rec.TB = t
		db = IsolatedSchema(rec, "TEST_POSTGRES_DSN", "cleanup_probe")
		var schema string
		if err := db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Fatal(err)
		}
	})
	if len(rec.logs) != 1 || !strings.Contains(rec.logs[0], "drop test schema") {
		t.Fatalf("a failed cleanup must be logged, got %+v", rec.logs)
	}
}

func TestSchemaDSNFailurePaths(t *testing.T) {
	requireDSN(t)
	closed := IsolatedSchema(t, "TEST_POSTGRES_DSN", "schema_dsn")
	live := IsolatedSchema(t, "TEST_POSTGRES_DSN", "schema_dsn_live")
	closedHandle, _ := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	closedHandle.Close()
	_ = closed

	rec := &recordingTB{TB: t}
	inGoroutine(func() { SchemaDSN(rec, "TEST_POSTGRES_DSN", closedHandle) })
	if len(rec.fatal) != 1 || !strings.Contains(rec.fatal[0], "resolve test schema") {
		t.Fatalf("closed db: %+v", rec.fatal)
	}
	t.Setenv("DBTEST_BAD_URL", "postgres://%zz")
	rec = &recordingTB{TB: t}
	inGoroutine(func() { SchemaDSN(rec, "DBTEST_BAD_URL", live) })
	if len(rec.fatal) != 1 || !strings.Contains(rec.fatal[0], "URL-form DSN") {
		t.Fatalf("bad env DSN: %+v", rec.fatal)
	}
}

// bareConn implements only driver.Conn, to exercise the optional-interface
// fallbacks of faultConn.
type bareConn struct{}

func (bareConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("bare prepare") }
func (bareConn) Close() error                        { return nil }
func (bareConn) Begin() (driver.Tx, error)           { return nil, errors.New("bare begin") }

func TestFaultConnForwardsOptionalInterfaces(t *testing.T) {
	dsn := requireDSN(t)
	faults := &Faults{}
	d := faultDriver{inner: stdlib.GetDefaultDriver(), faults: faults}
	if _, err := d.Open("postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("inner open failure must surface")
	}
	raw, err := d.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	conn := raw.(*faultConn)
	ctx := context.Background()
	if stmt, err := conn.Prepare("SELECT 1"); err != nil {
		t.Fatal(err)
	} else {
		stmt.Close()
	}
	if stmt, err := conn.PrepareContext(ctx, "SELECT 1"); err != nil {
		t.Fatal(err)
	} else {
		stmt.Close()
	}
	tx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !conn.IsValid() || conn.Ping(ctx) != nil || conn.ResetSession(ctx) != nil {
		t.Fatal("a healthy conn must validate, ping, and reset")
	}
	if err := conn.CheckNamedValue(&driver.NamedValue{Value: 1}); err != nil {
		t.Fatalf("pgx accepts ints: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.BeginTx(ctx, driver.TxOptions{}); err == nil {
		t.Fatal("begin on a closed inner conn must fail")
	}

	bare := &faultConn{inner: bareConn{}, faults: faults}
	if err := bare.CheckNamedValue(&driver.NamedValue{}); !errors.Is(err, driver.ErrSkip) {
		t.Fatalf("missing checker must skip, got %v", err)
	}
	if bare.ResetSession(ctx) != nil || !bare.IsValid() || bare.Ping(ctx) != nil {
		t.Fatal("missing optional interfaces must be treated as healthy")
	}
}

func TestSweepFaultsReportsMisbehavingOperations(t *testing.T) {
	faults := &Faults{}
	cases := map[string]struct {
		op   func(int) error
		want string
	}{
		"clean failure":  {func(int) error { return errors.New("broken") }, "clean run failed"},
		"no round trips": {func(int) error { return nil }, "made no database round trips"},
		"panics": {func(run int) error {
			_ = faults.step()
			if run%2 == 1 {
				panic("boom")
			}
			return nil
		}, "panicked"},
		"never recovers": {func(run int) error {
			_ = faults.step()
			if run > 0 {
				return errors.New("stuck")
			}
			return nil
		}, "did not recover"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &recordingTB{TB: t}
			inGoroutine(func() { SweepFaults(rec, faults, name, tc.op) })
			all := strings.Join(append(rec.fatal, rec.errs...), "\n")
			if !strings.Contains(all, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, all)
			}
		})
	}
}

type validatingConn struct {
	bareConn
	valid bool
}

func (v validatingConn) IsValid() bool { return v.valid }

func TestFaultConnForwardsValidator(t *testing.T) {
	conn := &faultConn{inner: validatingConn{valid: false}, faults: &Faults{}}
	if conn.IsValid() {
		t.Fatal("an inner validator's verdict must be forwarded")
	}
}

func TestFaultInjectingDBPassesRealQueryErrors(t *testing.T) {
	requireDSN(t)
	db := IsolatedSchema(t, "TEST_POSTGRES_DSN", "real_query_error")
	faulty, _ := FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	if _, err := faulty.Query(`SELECT * FROM no_such_table`); err == nil {
		t.Fatal("a real query error must pass through the wrapper")
	}
}

func TestSweepRowFaultsReportsMisbehavingOperations(t *testing.T) {
	requireDSN(t)
	db := IsolatedSchema(t, "TEST_POSTGRES_DSN", "row_sweep_reports")
	faulty, faults := FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	readOne := func() error {
		var one int
		return faulty.QueryRow(`SELECT 1`).Scan(&one)
	}
	cases := map[string]struct {
		op   func(int) error
		want string
	}{
		"clean failure": {func(int) error { return errors.New("broken") }, "clean run failed"},
		"panics": {func(run int) error {
			if err := readOne(); err != nil {
				panic("row fault")
			}
			return nil
		}, "panicked with row fault"},
		"never recovers": {func(run int) error {
			if run > 0 {
				return errors.New("stuck")
			}
			return readOne()
		}, "did not recover after row fault"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &recordingTB{TB: t}
			inGoroutine(func() { SweepRowFaults(rec, faults, name, tc.op) })
			all := strings.Join(append(rec.fatal, rec.errs...), "\n")
			if !strings.Contains(all, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, all)
			}
		})
	}
	// Both injection modes reach the caller: an error, then an unscannable row.
	for _, corrupt := range []bool{false, true} {
		faults.ArmRow(1, corrupt)
		if err := readOne(); err == nil {
			t.Fatalf("row fault (corrupt=%v) must surface", corrupt)
		}
	}
	faults.Arm(0)
}

func TestFaultDriverExecTxAndResultFaults(t *testing.T) {
	requireDSN(t)
	db := IsolatedSchema(t, "TEST_POSTGRES_DSN", "exec_tx_results")
	faulty, faults := FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	ctx := context.Background()
	if _, err := faulty.ExecContext(ctx, `CREATE TABLE items (id INT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := faulty.ExecContext(ctx, `INSERT INTO nowhere VALUES (1)`); err == nil {
		t.Fatal("a real exec error must pass through")
	}
	if _, err := faulty.QueryContext(ctx, `SELECT * FROM nowhere`); err == nil {
		t.Fatal("a real query error must pass through")
	}
	faults.Arm(1)
	if _, err := faulty.ExecContext(ctx, `INSERT INTO items VALUES (1)`); !errors.Is(err, ErrInjectedFault) {
		t.Fatalf("an armed exec must fail: %v", err)
	}

	faults.ArmResult(1)
	result, err := faulty.ExecContext(ctx, `INSERT INTO items VALUES (1), (2)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := result.RowsAffected(); !errors.Is(err, ErrInjectedFault) {
		t.Fatalf("the armed result read must fail: %v", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 2 {
		t.Fatalf("later reads pass through: %d %v", affected, err)
	}

	// The first round trip is the begin, the second the commit.
	faults.Arm(2)
	tx, err := faulty.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); !errors.Is(err, ErrInjectedFault) {
		t.Fatalf("an armed commit must fail: %v", err)
	}
	faults.Arm(0)
	tx, err = faulty.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("a clean commit: %v", err)
	}
	if _, err := faulty.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelLinearizable}); err == nil {
		t.Fatal("a begin the database refuses must fail")
	}
	faults.Arm(1)
	if _, err := faulty.BeginTx(ctx, nil); !errors.Is(err, ErrInjectedFault) {
		t.Fatalf("an armed begin must fail: %v", err)
	}
	faults.Arm(1)
	if _, err := faulty.QueryContext(ctx, `SELECT 1`); !errors.Is(err, ErrInjectedFault) {
		t.Fatalf("an armed query must fail: %v", err)
	}
	faults.Arm(0)
	rows, err := faulty.QueryContext(ctx, `SELECT id FROM items`)
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	for rows.Next() {
		read++
	}
	if err := rows.Err(); err != nil || read != 2 {
		t.Fatalf("reading to the end passes the driver's end of rows through: %d %v", read, err)
	}
}

func TestSweepResultFaultsReportsMisbehavingOperations(t *testing.T) {
	requireDSN(t)
	db := IsolatedSchema(t, "TEST_POSTGRES_DSN", "result_sweep_reports")
	faulty, faults := FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	deleteNothing := func() error {
		result, err := faulty.Exec(`DELETE FROM pg_temp_none WHERE false`)
		if err != nil {
			result, err = faulty.Exec(`SELECT 1`)
			if err != nil {
				return err
			}
		}
		_, err = result.RowsAffected()
		return err
	}
	cases := map[string]struct {
		op   func(int) error
		want string
	}{
		"clean failure": {func(int) error { return errors.New("broken") }, "clean run failed"},
		"panics": {func(int) error {
			if err := deleteNothing(); err != nil {
				panic("result fault")
			}
			return nil
		}, "panicked with result fault 1/1"},
		"never recovers": {func(run int) error {
			if run > 0 {
				return errors.New("stuck")
			}
			return deleteNothing()
		}, "did not recover after result fault 1/1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &recordingTB{TB: t}
			inGoroutine(func() { SweepResultFaults(rec, faults, name, tc.op) })
			all := strings.Join(append(rec.fatal, rec.errs...), "\n")
			if !strings.Contains(all, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, all)
			}
		})
	}
	// An operation that reads no results sweeps trivially.
	SweepResultFaults(t, faults, "no results", func(int) error { return nil })
}

func TestArmHookInterleavesAChange(t *testing.T) {
	requireDSN(t)
	db := IsolatedSchema(t, "TEST_POSTGRES_DSN", "hook_interleave")
	faulty, faults := FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	if _, err := db.Exec(`CREATE TABLE flags (on_ BOOLEAN); INSERT INTO flags VALUES (FALSE)`); err != nil {
		t.Fatal(err)
	}
	faults.ArmHook(2, func() {
		if _, err := db.Exec(`UPDATE flags SET on_ = TRUE`); err != nil {
			t.Error(err)
		}
	})
	var before, after bool
	if err := faulty.QueryRow(`SELECT on_ FROM flags`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := faulty.QueryRow(`SELECT on_ FROM flags`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before || !after {
		t.Fatalf("the hook runs between the first and second round trip: %v %v", before, after)
	}
	faults.Arm(0)
}
