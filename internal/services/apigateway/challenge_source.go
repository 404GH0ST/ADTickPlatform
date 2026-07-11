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
		if errors.Is(err, ErrChallengeMaintenance) {
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
	serveChallengeSourceFile(w, r, resolvedPath, info, descriptor.Name)
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
		if challenge.Maintenance {
			return challengeSourceDescriptor{}, ErrChallengeMaintenance
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
	candidate := filepath.Join(absRoot, cleanRelative)
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", nil, err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return "", nil, err
	}
	relToRoot, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return "", nil, err
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", nil, errChallengeSourceUnavailable
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return "", nil, err
	}
	return resolvedCandidate, info, nil
}

func serveChallengeSourceFile(w http.ResponseWriter, r *http.Request, path string, info fs.FileInfo, challengeName string) {
	file, err := openResolvedChallengeSource(path, info)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "Source unavailable", "challenge source is unavailable.")
		return
	}
	defer file.Close()

	filename := filepath.Base(path)
	if filename == "" || filename == "." {
		filename = fmt.Sprintf("%s-source", sanitizeArchiveName(challengeName))
	}
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

	base := filepath.Base(root)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if !(info.Mode().IsRegular() || info.IsDir()) {
			return nil
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		tarPath := filepath.ToSlash(filepath.Join(base, relPath))
		header, headerErr := tar.FileInfoHeader(info, "")
		if headerErr != nil {
			return headerErr
		}
		header.Name = tarPath
		header.ModTime = info.ModTime().UTC().Truncate(time.Second)
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, openErr := openResolvedChallengeSource(path, info)
		if openErr != nil {
			return openErr
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}); err != nil {
		http.Error(w, "challenge source archive failed", http.StatusInternalServerError)
		return
	}
}

func openResolvedChallengeSource(path string, expected fs.FileInfo) (*os.File, error) {
	file, err := os.Open(path) // #nosec G304 -- path is resolved under AD_CHALLENGE_SOURCE_ROOT and checked after open.
	if err != nil {
		return nil, err
	}
	actual, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
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
