package apigateway

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
		serveChallengeSourceDirectory(w, resolvedPath, descriptor.Name)
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

func serveChallengeSourceDirectory(w http.ResponseWriter, root string, challengeName string) {
	filename := fmt.Sprintf("%s-source.tar.gz", sanitizeArchiveName(challengeName))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	gzipWriter := gzip.NewWriter(w)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	archive := sourceArchive{root: root, base: filepath.Base(root), tar: tarWriter}
	if err := filepath.WalkDir(root, archive.add); err != nil {
		http.Error(w, "challenge source archive failed", http.StatusInternalServerError)
		return
	}
}

// sourceArchive writes a challenge's source tree into a tar stream. Only
// regular files and directories are archived: symlinks and special files are
// skipped so the archive can never reach outside the resolved source root.
type sourceArchive struct {
	root string
	base string
	tar  *tar.Writer
}

func (a sourceArchive) add(path string, entry fs.DirEntry, err error) error {
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
	_, err = io.Copy(a.tar, file)
	return err
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
