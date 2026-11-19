package apigateway

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

// sourceStore serves one challenge with configurable gating on top of a
// memory store that still owns sessions.
type sourceStore struct {
	Store
	challenge adminChallenge
	listErr   error
	startErr  error
	started   bool
	pauseErr  error
	paused    bool
	blockErr  error
	blocked   bool
}

func (s *sourceStore) ListAdminChallenges(context.Context) ([]adminChallenge, error) {
	return []adminChallenge{s.challenge}, s.listErr
}
func (s *sourceStore) IsMatchStarted(context.Context) (bool, error) { return s.started, s.startErr }
func (s *sourceStore) IsMatchPaused(context.Context) (bool, error)  { return s.paused, s.pauseErr }
func (s *sourceStore) ChallengePlayBlocked(context.Context, int) (bool, error) {
	return s.blocked, s.blockErr
}

func writeSourceTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("bundle/main.py", "print('hi')\n", 0o644)
	write("bundle/sub/x.txt", "x\n", 0o600)
	write("single.zip", "PK-not-really", 0o644)
	write("README", "read me\n", 0o644)
	write("file.txt", "plain\n", 0o644)
	write("unreadable.txt", "secret\n", 0o000)
	if err := os.Symlink("main.py", filepath.Join(root, "bundle", "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "bundle", "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "locked"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "locked"), 0o755) })
	return root
}

func TestChallengeSourceDownloadBranches(t *testing.T) {
	root := writeSourceTree(t)
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", root)
	store := &sourceStore{Store: NewMemoryStore(101)}
	env, token := residualMux(t, store, func(s *Server) { s.WithRateLimiter(testRateLimiter{}) })

	reset := func() {
		*store = sourceStore{Store: store.Store, started: true, challenge: adminChallenge{ID: 1, Name: "Web 2.0!!", Published: true, SourceBundlePath: "single.zip"}}
	}
	cases := []struct {
		name   string
		path   string
		mutate func(*sourceStore)
		want   int
	}{
		{"bad id", "/api/v2/challenges/abc/source", nil, http.StatusBadRequest},
		{"list fails", "", func(s *sourceStore) { s.listErr = errStoreDown }, http.StatusInternalServerError},
		{"start check fails", "", func(s *sourceStore) { s.startErr = errStoreDown }, http.StatusInternalServerError},
		{"not started", "", func(s *sourceStore) { s.started = false }, http.StatusServiceUnavailable},
		{"pause check fails", "", func(s *sourceStore) { s.pauseErr = errStoreDown }, http.StatusInternalServerError},
		{"paused", "", func(s *sourceStore) { s.paused = true }, http.StatusServiceUnavailable},
		{"block check fails", "", func(s *sourceStore) { s.blockErr = errStoreDown }, http.StatusInternalServerError},
		{"maintenance", "", func(s *sourceStore) { s.blocked, s.challenge.Maintenance = true, true }, http.StatusServiceUnavailable},
		{"deferred", "", func(s *sourceStore) { s.blocked = true }, http.StatusServiceUnavailable},
		{"no source", "", func(s *sourceStore) { s.challenge.SourceBundlePath = "" }, http.StatusNotFound},
		{"missing on disk", "", func(s *sourceStore) { s.challenge.SourceBundlePath = "gone.zip" }, http.StatusNotFound},
		{"not a directory", "", func(s *sourceStore) { s.challenge.SourceBundlePath = "file.txt/inner" }, http.StatusBadRequest},
		{"unreadable file", "", func(s *sourceStore) { s.challenge.SourceBundlePath = "unreadable.txt" }, http.StatusNotFound},
		{"unreadable directory", "", func(s *sourceStore) { s.challenge.SourceBundlePath = "locked" }, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		reset()
		if tc.mutate != nil {
			tc.mutate(store)
		}
		path := tc.path
		if path == "" {
			path = "/api/v2/challenges/1/source"
		}
		if response := env.do(http.MethodGet, path, token, nil); response.Code != tc.want {
			t.Errorf("%s: want %d, got %d %s", tc.name, tc.want, response.Code, response.Body.String())
		}
	}

	for sourcePath, wantType := range map[string]string{"single.zip": "application/zip", "README": "application/octet-stream"} {
		reset()
		store.challenge.SourceBundlePath = sourcePath
		response := env.do(http.MethodGet, "/api/v2/challenges/1/source", token, nil)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != wantType {
			t.Errorf("%s: %d %q", sourcePath, response.Code, response.Header().Get("Content-Type"))
		}
		if !strings.Contains(response.Header().Get("Content-Disposition"), sourcePath) {
			t.Errorf("%s: disposition %q", sourcePath, response.Header().Get("Content-Disposition"))
		}
	}

	reset()
	store.challenge.SourceBundlePath = "bundle"
	response := env.do(http.MethodGet, "/api/v2/challenges/1/source", token, nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Disposition"), "web-2-0-source.tar.gz") {
		t.Fatalf("directory download: %d %q", response.Code, response.Header().Get("Content-Disposition"))
	}
	entries := readSourceArchive(t, response.Body.Bytes())
	want := []string{"bundle/main.py", "bundle/sub/", "bundle/sub/x.txt"}
	if strings.Join(entries, ",") != strings.Join(want, ",") {
		t.Fatalf("the archive holds only regular files and directories, got %v", entries)
	}
}

// readSourceArchive lists entries and checks none carries server identity.
func readSourceArchive(t *testing.T, body []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gz)
	names := []string{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Errorf("%s leaks owner %d/%d %q/%q", header.Name, header.Uid, header.Gid, header.Uname, header.Gname)
		}
		names = append(names, header.Name)
	}
	sort.Strings(names)
	return names
}

type cutoffWriter struct{ after int }

func (w *cutoffWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("client went away")
	}
	if len(p) > w.after {
		n := w.after
		w.after = 0
		return n, errors.New("client went away")
	}
	w.after -= len(p)
	return len(p), nil
}

type brokenEntry struct{ fs.DirEntry }

func (brokenEntry) Type() fs.FileMode          { return 0 }
func (brokenEntry) Info() (fs.FileInfo, error) { return nil, errors.New("entry vanished") }

func TestSourceArchiveFailures(t *testing.T) {
	root := writeSourceTree(t)
	bundle := filepath.Join(root, "bundle")
	mainPath := filepath.Join(bundle, "main.py")
	entryFor := func(path string) fs.DirEntry {
		t.Helper()
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		return fs.FileInfoToDirEntry(info)
	}
	archive := func(w io.Writer) sourceArchive {
		return sourceArchive{root: bundle, base: "bundle", tar: tar.NewWriter(w)}
	}

	if err := archive(io.Discard).add(mainPath, brokenEntry{}, nil); err == nil {
		t.Error("an entry whose info cannot be read must fail the archive")
	}
	if err := archive(&cutoffWriter{}).add(mainPath, entryFor(mainPath), nil); err == nil {
		t.Error("a failed header write must fail the archive")
	}
	if err := archive(&cutoffWriter{after: 512}).add(mainPath, entryFor(mainPath), nil); err == nil {
		t.Error("a failed content write must fail the archive")
	}
	// The resolved entry is not the file that opens: refused.
	if err := archive(io.Discard).add(mainPath, entryFor(filepath.Join(root, "README")), nil); !errors.Is(err, errChallengeSourceUnavailable) {
		t.Errorf("a swapped file must be refused, got %v", err)
	}
	if _, err := openResolvedChallengeSource(filepath.Join(root, "missing"), nil); err == nil {
		t.Error("a missing file cannot be opened")
	}

	for name, want := range map[string]string{"": "challenge", "  ": "challenge", "!!!": "challenge", "Web 2.0!!": "web-2-0", "a--b": "a-b"} {
		if got := sanitizeArchiveName(name); got != want {
			t.Errorf("sanitizeArchiveName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestResolveChallengeSourcePathRoots(t *testing.T) {
	root := writeSourceTree(t)
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", filepath.Join(root, "absent"))
	if _, _, err := resolveChallengeSourcePath("bundle"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing root: %v", err)
	}
	// A link back to the root would hand out every challenge's source.
	if err := os.Symlink(".", filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", root)
	if _, _, err := resolveChallengeSourcePath("self"); !errors.Is(err, errChallengeSourceUnavailable) {
		t.Errorf("a link to the root itself must be unavailable, got %v", err)
	}
	for _, bad := range []string{"/etc/passwd", "..", "../x", "."} {
		if _, _, err := resolveChallengeSourcePath(bad); !errors.Is(err, errChallengeSourceUnavailable) {
			t.Errorf("%q must be unavailable, got %v", bad, err)
		}
	}

	// With no root configured the working directory is the root; a deleted
	// working directory cannot be resolved.
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", "")
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveChallengeSourcePath("bundle"); err == nil {
		t.Error("an unresolvable working directory must fail")
	}
}
