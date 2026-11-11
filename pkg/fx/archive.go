package fx

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractDownloadedArchive selects a decoder and owns the destination root.
// Unrecognized extensions are left intact as ordinary downloaded files.
func extractDownloadedArchive(archivePath, destination string) error {
	var extract func(string, *archiveEntryWriter) error
	switch name := strings.ToLower(archivePath); {
	case strings.HasSuffix(name, ".zip"):
		extract = extractZIP
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		extract = extractTARGZ
	case strings.HasSuffix(name, ".tar"):
		extract = extractTAR
	default:
		return nil
	}

	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()

	return extract(
		archivePath,
		&archiveEntryWriter{
			root:        root,
			archiveName: filepath.Base(archivePath),
		},
	)
}

// archiveEntryWriter owns path validation and filesystem writes, not decoding.
// The caller owns root's lifetime. Entries are written incrementally; failures
// do not roll back previously extracted files.
type archiveEntryWriter struct {
	root        *os.Root
	archiveName string
}

func (w *archiveEntryWriter) write(name string, mode os.FileMode, reader io.Reader) error {
	if strings.ContainsAny(name, "\\:\x00") || !filepath.IsLocal(name) {
		return fmt.Errorf("unsafe archive entry %q", name)
	}

	name = filepath.FromSlash(name)
	if strings.EqualFold(filepath.Clean(name), w.archiveName) {
		return fmt.Errorf("archive entry would overwrite archive: %q", name)
	}

	if mode.IsDir() {
		return w.root.MkdirAll(name, 0755)
	}
	if !mode.IsRegular() {
		return fmt.Errorf("unsupported archive entry type: %q", name)
	}

	if err := w.root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}

	// Replace directory entries rather than truncating existing linked files.
	if err := w.root.Remove(name); err != nil && !os.IsNotExist(err) {
		return err
	}

	f, err := w.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, reader)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// extractZIP owns the ZIP reader and each entry reader.
func extractZIP(archivePath string, writer *archiveEntryWriter) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open ZIP: %w", err)
	}
	defer archive.Close()

	for _, entry := range archive.File {
		r, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open ZIP entry %q: %w", entry.Name, err)
		}

		writeErr := writer.write(entry.Name, entry.Mode(), r)
		closeErr := r.Close()
		if writeErr != nil {
			return fmt.Errorf("extract ZIP entry %q: %w", entry.Name, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close ZIP entry %q: %w", entry.Name, closeErr)
		}
	}
	return nil
}

// extractTAR owns the file; extractTARStream handles the TAR entries.
func extractTAR(archivePath string, writer *archiveEntryWriter) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open TAR: %w", err)
	}
	defer f.Close()

	return extractTARStream(f, writer)
}

// extractTARGZ owns the file and gzip decoder, including checksum validation.
// TAR.GZ and TGZ share the same format and therefore the same implementation.
func extractTARGZ(archivePath string, writer *archiveEntryWriter) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open TAR.GZ: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gz.Close()

	if err = extractTARStream(gz, writer); err != nil {
		return err
	}
	// TAR can end before the gzip trailer; drain to validate the checksum.
	if _, err = io.Copy(io.Discard, gz); err != nil {
		return fmt.Errorf("validate gzip stream: %w", err)
	}
	return nil
}

// extractTARStream decodes TAR entries without closing the caller's reader.
func extractTARStream(reader io.Reader, writer *archiveEntryWriter) error {
	archive := tar.NewReader(reader)
	for {
		entry, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read TAR: %w", err)
		}
		if entry.Typeflag != tar.TypeReg && entry.Typeflag != tar.TypeDir {
			return fmt.Errorf("unsupported TAR entry type: %q", entry.Name)
		}
		if err := writer.write(entry.Name, entry.FileInfo().Mode(), archive); err != nil {
			return fmt.Errorf("extract TAR entry %q: %w", entry.Name, err)
		}
	}
}
