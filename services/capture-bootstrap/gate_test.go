package main

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeProc builds a /proc stand-in: PID 1 has the given children, each child's
// exe links to target, and self/exe links to the gate binary.
func fakeProc(t *testing.T, children string, leaderExe string) string {
	t.Helper()
	root := t.TempDir()
	self := filepath.Join(root, "gate-binary")
	other := filepath.Join(root, "other-binary")
	for _, path := range []string{self, other} {
		if err := os.WriteFile(path, []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(t, filepath.Join(root, "1", "task", "1"))
	if children != "" {
		if err := os.WriteFile(filepath.Join(root, "1", "task", "1", "children"), []byte(children), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(t, filepath.Join(root, "self"))
	mustSymlink(t, self, filepath.Join(root, "self", "exe"))
	for _, pid := range strings.Fields(children) {
		mustMkdir(t, filepath.Join(root, pid))
		target := self
		switch leaderExe {
		case "other":
			target = other
		case "missing":
			target = filepath.Join(root, "does-not-exist")
		}
		mustSymlink(t, target, filepath.Join(root, pid, "exe"))
	}
	return root
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func useProc(t *testing.T, root string) {
	t.Helper()
	original := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = original })
}

func TestVerifyGateLeader(t *testing.T) {
	useProc(t, fakeProc(t, "7", "self"))
	if err := verifyGateLeader(); err != nil {
		t.Fatalf("the gate itself as sole init child must verify: %v", err)
	}

	for name, root := range map[string]string{
		"no children file": fakeProc(t, "", "self"),
		"two children":     fakeProc(t, "7 8", "self"),
		"missing leader":   fakeProc(t, "7", "missing"),
		"other leader":     fakeProc(t, "7", "other"),
	} {
		useProc(t, root)
		if err := verifyGateLeader(); err == nil {
			t.Fatalf("%s: verification must fail", name)
		}
	}

	root := fakeProc(t, "7", "self")
	useProc(t, root)
	if err := os.Remove(filepath.Join(root, "self", "exe")); err != nil {
		t.Fatal(err)
	}
	if err := verifyGateLeader(); err == nil {
		t.Fatal("an unreadable self exe must fail")
	}

	// An upgraded gate still waiting on its replaced inode is trusted only via
	// the immutable volume path.
	useProc(t, fakeProc(t, "7", "other"))
	originalReadLink := readLink
	t.Cleanup(func() { readLink = originalReadLink })
	readLink = func(string) (string, error) { return "/opt/adplatform-capture/start (deleted)", nil }
	if err := verifyGateLeader(); err != nil {
		t.Fatalf("a replaced gate binary on the volume path must verify: %v", err)
	}
	readLink = func(string) (string, error) { return "/tmp/evil (deleted)", nil }
	if err := verifyGateLeader(); err == nil {
		t.Fatal("a deleted binary outside the volume path must not verify")
	}
}

func TestLookPath(t *testing.T) {
	if got, err := lookPath("/bin/sh"); err != nil || got != "/bin/sh" {
		t.Fatalf("absolute: %q %v", got, err)
	}
	if got, err := lookPath("./local"); err != nil || got != "./local" {
		t.Fatalf("relative path: %q %v", got, err)
	}
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "tool")) // a directory named like the tool is skipped
	plainDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(plainDir, "tool"), []byte("x"), 0o644); err != nil { // not executable
		t.Fatal(err)
	}
	execDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(execDir, "tool"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", strings.Join([]string{dir, plainDir, execDir}, string(os.PathListSeparator)))
	if got, err := lookPath("tool"); err != nil || got != filepath.Join(execDir, "tool") {
		t.Fatalf("PATH search: %q %v", got, err)
	}
	if _, err := lookPath("no-such-tool-anywhere"); err == nil {
		t.Fatal("a missing tool must fail")
	}
}

// runGate starts the gate with exec stubbed out and returns what exec saw.
func runGate(t *testing.T, args []string) (<-chan error, *[]string) {
	t.Helper()
	originalExec := execProgram
	t.Cleanup(func() { execProgram = originalExec })
	var execed []string
	execProgram = func(path string, argv []string, _ []string) error {
		execed = append([]string{path}, argv...)
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- gate(args) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", address); err == nil {
			_, _ = c.Write([]byte("not json\n"))
			c.Close()
			return done, &execed
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("gate did not start listening")
	return nil, nil
}

func TestGateReleasesOnlyWithItsNonce(t *testing.T) {
	done, execed := runGate(t, []string{"/bin/true", "--flag"})
	if _, err := exchange("release", "wrong-nonce"); err == nil {
		t.Fatal("a wrong nonce must be rejected")
	}
	reply, err := exchange("probe", "")
	if err != nil || len(reply.Nonce) != 64 {
		t.Fatalf("probe: %+v %v", reply, err)
	}
	if _, err := exchange("release", reply.Nonce); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("gate: %v", err)
	}
	if strings.Join(*execed, " ") != "/bin/true /bin/true --flag" {
		t.Fatalf("gate must exec the original command, got %v", *execed)
	}
}

func TestGateErrors(t *testing.T) {
	if err := gate(nil); err == nil {
		t.Fatal("a missing command must fail")
	}
	busy, err := net.Listen("unix", address)
	if err != nil {
		t.Fatal(err)
	}
	if err := gate([]string{"/bin/true"}); err == nil {
		t.Fatal("an occupied socket must fail")
	}
	busy.Close()

	originalListen := listen
	t.Cleanup(func() { listen = originalListen })
	listen = func(string, string) (net.Listener, error) { return brokenListener{}, nil }
	if err := gate([]string{"/bin/true"}); err == nil {
		t.Fatal("an accept failure must end the gate")
	}
	listen = originalListen

	done, _ := runGate(t, []string{"no-such-command-xyz"})
	reply, err := exchange("probe", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exchange("release", reply.Nonce); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("an unknown original command must fail after release")
	}
}

type brokenListener struct{}

func (brokenListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }
func (brokenListener) Close() error              { return nil }
func (brokenListener) Addr() net.Addr            { return &net.UnixAddr{Name: address, Net: "unix"} }

func TestExchangeWithoutGateFails(t *testing.T) {
	if _, err := exchange("probe", ""); err == nil {
		t.Fatal("no gate listening must fail")
	}
}

func TestCLICommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		return cli(append([]string{"start"}, args...), &stdout, &stderr)
	}
	if run() != 1 || !strings.Contains(stderr.String(), "expected install") {
		t.Fatalf("usage error: %q", stderr.String())
	}

	root := fakeProc(t, "7", "self")
	useProc(t, root)
	if run("status") != 0 {
		t.Fatalf("status: %q", stderr.String())
	}
	installDir := filepath.Join(t.TempDir(), "volume")
	if run("install", installDir) != 0 {
		t.Fatalf("install: %q", stderr.String())
	}
	if info, err := os.Stat(filepath.Join(installDir, "start")); err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("installed binary must be read-only executable: %v %v", info, err)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if run("install", filepath.Join(blocker, "sub")) != 1 {
		t.Fatal("install under a file must fail")
	}
	readOnly := filepath.Join(t.TempDir(), "ro")
	mustMkdir(t, readOnly)
	if err := os.Chmod(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
	if os.Getuid() != 0 && run("install", readOnly) != 1 {
		t.Fatal("install into a read-only directory must fail")
	}

	done, _ := runGate(t, []string{"/bin/true"})
	if run("probe") != 0 || len(strings.TrimSpace(stdout.String())) != 64 {
		t.Fatalf("probe must print the nonce: %q %q", stdout.String(), stderr.String())
	}
	nonce := strings.TrimSpace(stdout.String())
	if run("release", nonce) != 0 {
		t.Fatalf("release: %q", stderr.String())
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	useProc(t, fakeProc(t, "7 8", "self"))
	for _, args := range [][]string{{"status"}, {"probe"}, {"release", "x"}} {
		if run(args...) != 1 {
			t.Fatalf("%v must fail when this is not the gate leader", args)
		}
	}
	useProc(t, t.TempDir())
	if run("install", t.TempDir()) != 1 {
		t.Fatal("install without a readable self exe must fail")
	}
	useProc(t, root)

	originalExec := execProgram
	t.Cleanup(func() { execProgram = originalExec })
	execProgram = func(string, []string, []string) error { return errors.New("exec failed") }
	go func() {
		for i := 0; i < 200; i++ {
			if reply, err := exchange("probe", ""); err == nil {
				_, _ = exchange("release", reply.Nonce)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	if run("run", "/bin/true") != 1 {
		t.Fatal("a failed exec must be reported")
	}
}

func TestMainExitsWithCLIStatus(t *testing.T) {
	originalExit, originalArgs := exit, os.Args
	t.Cleanup(func() { exit, os.Args = originalExit, originalArgs })
	var code = -1
	exit = func(c int) { code = c }
	os.Args = []string{"start"}
	main()
	if code != 1 {
		t.Fatalf("usage error must exit 1, got %d", code)
	}
}

func TestExchangeReportsRequestWriteFailure(t *testing.T) {
	originalDial := dial
	t.Cleanup(func() { dial = originalDial })
	dial = func(string, string, time.Duration) (net.Conn, error) {
		client, server := net.Pipe()
		server.Close()
		return client, nil
	}
	if _, err := exchange("probe", ""); err == nil {
		t.Fatal("a failed request write must be reported")
	}
}
