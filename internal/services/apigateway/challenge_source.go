package apigateway

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type challengeSourceDescriptor struct {
	Name             string
	SourceBundlePath string
}

var errChallengeSourceUnavailable = errors.New("challenge source unavailable")

const (
	maxChallengeSourceArchiveBytes   int64 = 128 << 20
	maxChallengeSourceArchiveEntries       = 4096
	maxConcurrentSourceArchives            = 2
)

var (
	errChallengeSourceTooLarge  = errors.New("challenge source archive exceeds size or entry limit")
	challengeSourceArchiveSlots = make(chan struct{}, maxConcurrentSourceArchives)
)

func validateChallengeSourceReference(sourceBundlePath string) error {
	trimmed := sanitizeSourceBundlePath(sourceBundlePath)
	if trimmed == "" {
		return nil
	}
	_, _, err := resolveChallengeSourcePath(trimmed)
	return err
}

func (s *Server) handleChallengeSourceDownload(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before source access.")
	if !ok {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamChallengeKey("source-download", teamID, challengeID), challengeSourceRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	descriptor, err := s.lookupChallengeSourceDescriptor(r.Context(), challengeID)
	if err != nil {
		if errors.Is(err, ErrChallengeMaintenance) ||
			errors.Is(err, ErrChallengeDeferred) ||
			errors.Is(err, ErrMatchNotStarted) ||
			errors.Is(err, ErrMatchPaused) {
			writeDomainFailure(w, err)
			return
		}
		if errors.Is(err, errChallengeSourceUnavailable) || errors.Is(err, ErrChallengeNotFound) {
			writeProblem(w, http.StatusNotFound, "Source unavailable", "challenge source is unavailable.")
			return
		}
		writeStoreFailure(w, err)
		return
	}

	resolvedPath, info, err := resolveChallengeSourcePath(descriptor.SourceBundlePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errChallengeSourceUnavailable) {
			writeProblem(w, http.StatusNotFound, "Source unavailable", "challenge source is unavailable.")
			return
		}
		writeProblem(w, http.StatusBadRequest, "Invalid source path", "challenge source path is invalid.")
		return
	}

	if info.IsDir() {
		serveChallengeSourceDirectory(w, r, resolvedPath, descriptor.Name)
		return
	}
	serveChallengeSourceFile(w, r, resolvedPath, info)
}

func (s *Server) lookupChallengeSourceDescriptor(ctx context.Context, challengeID int) (challengeSourceDescriptor, error) {
	challenges, err := s.store.ListAdminChallenges(ctx)
	if err != nil {
		return challengeSourceDescriptor{}, err
	}
	for _, challenge := range challenges {
		if challenge.ID != challengeID || !challenge.Published {
			continue
		}
		started, startErr := s.store.IsMatchStarted(ctx)
		if startErr != nil {
			return challengeSourceDescriptor{}, startErr
		}
		if !started {
			return challengeSourceDescriptor{}, ErrMatchNotStarted
		}
		paused, pauseErr := s.store.IsMatchPaused(ctx)
		if pauseErr != nil {
			return challengeSourceDescriptor{}, pauseErr
		}
		if paused {
			return challengeSourceDescriptor{}, ErrMatchPaused
		}
		blocked, blockErr := s.store.ChallengePlayBlocked(ctx, challengeID)
		if blockErr != nil {
			return challengeSourceDescriptor{}, blockErr
		}
		if blocked {
			// Maintenance and deferred resume both block whitebox until open.
			if challenge.Maintenance {
				return challengeSourceDescriptor{}, ErrChallengeMaintenance
			}
			return challengeSourceDescriptor{}, ErrChallengeDeferred
		}
		path := sanitizeSourceBundlePath(challenge.SourceBundlePath)
		if path == "" {
			return challengeSourceDescriptor{}, errChallengeSourceUnavailable
		}
		return challengeSourceDescriptor{
			Name:             challenge.Name,
			SourceBundlePath: path,
		}, nil
	}
	return challengeSourceDescriptor{}, ErrChallengeNotFound
}

func resolveChallengeSourcePath(sourceBundlePath string) (string, fs.FileInfo, error) {
	trimmed := sanitizeSourceBundlePath(sourceBundlePath)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return "", nil, errChallengeSourceUnavailable
	}
	cleanRelative := filepath.Clean(trimmed)
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", nil, errChallengeSourceUnavailable
	}

	root := strings.TrimSpace(os.Getenv("AD_CHALLENGE_SOURCE_ROOT"))
	if root == "" {
		root = "."
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", nil, err
	}
	// Both paths are absolute and fully resolved, so containment is a prefix
	// test. The root itself is never a challenge's source.
	resolvedCandidate, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, cleanRelative))
	if err != nil {
		return "", nil, err
	}
	rootPrefix := strings.TrimSuffix(resolvedRoot, string(filepath.Separator)) + string(filepath.Separator)
	info, statErr := os.Stat(resolvedCandidate)
	if statErr != nil || !strings.HasPrefix(resolvedCandidate, rootPrefix) {
		// Outside the root, or gone since it was resolved: fail closed.
		return "", nil, errChallengeSourceUnavailable
	}
	return resolvedCandidate, info, nil
}

func serveChallengeSourceFile(w http.ResponseWriter, r *http.Request, path string, info fs.FileInfo) {
	file, err := openResolvedChallengeSource(path, info)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "Source unavailable", "challenge source is unavailable.")
		return
	}
	defer file.Close()

	// path is strictly inside the source root, so it has a real base name.
	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func serveChallengeSourceDirectory(w http.ResponseWriter, r *http.Request, root string, challengeName string) {
	select {
	case challengeSourceArchiveSlots <- struct{}{}:
		defer func() { <-challengeSourceArchiveSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		writeProblem(w, http.StatusTooManyRequests, "Source busy", "too many source archives are being prepared; retry shortly.")
		return
	}

	if err := inspectChallengeSourceDirectory(r.Context(), root); err != nil {
		writeChallengeSourceArchiveFailure(w, err)
		return
	}

	// Prepare the archive before sending a success response. A file changing or
	// disappearing during the walk must not leave a successful partial download.
	archiveFile, err := os.CreateTemp("", "adplatform-source-*.tar.gz")
	if err != nil {
		writeChallengeSourceArchiveFailure(w, err)
		return
	}
	defer func() {
		archiveFile.Close()
		os.Remove(archiveFile.Name())
	}()
	if err := writeChallengeSourceArchive(r.Context(), archiveFile, root); err != nil {
		writeChallengeSourceArchiveFailure(w, err)
		return
	}
	if _, err := archiveFile.Seek(0, io.SeekStart); err != nil {
		writeChallengeSourceArchiveFailure(w, err)
		return
	}

	filename := fmt.Sprintf("%s-source.tar.gz", sanitizeArchiveName(challengeName))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-store")
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	http.ServeContent(w, r, filename, time.Time{}, archiveFile)
}

func inspectChallengeSourceDirectory(ctx context.Context, root string) error {
	var bytes int64
	entries := 0
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		entries++
		if entries > maxChallengeSourceArchiveEntries || info.Size() < 0 || info.Size() > maxChallengeSourceArchiveBytes-bytes {
			return errChallengeSourceTooLarge
		}
		if info.Mode().IsRegular() {
			bytes += info.Size()
		}
		return nil
	})
}

func writeChallengeSourceArchive(ctx context.Context, target io.Writer, root string) error {
	gzipWriter := gzip.NewWriter(target)
	tarWriter := tar.NewWriter(gzipWriter)
	archive := sourceArchive{ctx: ctx, root: root, base: filepath.Base(root), tar: tarWriter, remainingBytes: maxChallengeSourceArchiveBytes}
	walkErr := filepath.WalkDir(root, archive.add)
	tarErr := tarWriter.Close()
	gzipErr := gzipWriter.Close()
	return errors.Join(walkErr, tarErr, gzipErr)
}

func writeChallengeSourceArchiveFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if errors.Is(err, errChallengeSourceTooLarge) {
		writeProblem(w, http.StatusRequestEntityTooLarge, "Source too large", "source archive exceeds the download limit.")
		return
	}
	log.Printf("challenge source archive failed: %v", err)
	writeProblem(w, http.StatusInternalServerError, "Source unavailable", "challenge source archive could not be prepared.")
}

// sourceArchive writes a challenge's source tree into a tar stream. Only
// regular files and directories are archived: symlinks and special files are
// skipped so the archive can never reach outside the resolved source root.
type sourceArchive struct {
	ctx            context.Context
	root           string
	base           string
	tar            *tar.Writer
	entries        int
	remainingBytes int64
}

func (a *sourceArchive) add(path string, entry fs.DirEntry, err error) error {
	if ctxErr := a.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	if path == a.root || entry.Type()&os.ModeSymlink != 0 {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil
	}
	a.entries++
	if a.entries > maxChallengeSourceArchiveEntries || info.Size() < 0 || info.Size() > a.remainingBytes {
		return errChallengeSourceTooLarge
	}
	if info.Mode().IsRegular() {
		a.remainingBytes -= info.Size()
	}
	// The header is built by hand rather than from the FileInfo so the
	// archive carries no server uid, gid or owner names.
	header := &tar.Header{
		Name:     filepath.ToSlash(filepath.Join(a.base, strings.TrimPrefix(path, a.root+string(filepath.Separator)))),
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime().UTC().Truncate(time.Second),
		Typeflag: tar.TypeReg,
		Size:     info.Size(),
	}
	if info.IsDir() {
		header.Name += "/"
		header.Typeflag = tar.TypeDir
		header.Size = 0
	}
	if err := a.tar.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	file, err := openResolvedChallengeSource(path, info)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.CopyN(a.tar, &challengeSourceContextReader{ctx: a.ctx, reader: file}, info.Size())
	return err
}

type challengeSourceContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *challengeSourceContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func openResolvedChallengeSource(path string, expected fs.FileInfo) (*os.File, error) {
	file, err := os.Open(path) // #nosec G304 -- path is resolved under AD_CHALLENGE_SOURCE_ROOT and checked after open.
	if err != nil {
		return nil, err
	}
	// The opened file must be the regular file that was resolved: anything
	// swapped in since (or a handle that cannot be inspected) is refused.
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		file.Close()
		return nil, errChallengeSourceUnavailable
	}
	return file, nil
}

func sanitizeArchiveName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return "challenge"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	value := strings.Trim(builder.String(), "-")
	if value == "" {
		return "challenge"
	}
	return value
}
