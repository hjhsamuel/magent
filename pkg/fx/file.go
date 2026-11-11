package fx

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DownloadFileByUrl(remoteUrl string, tempPath, path string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(remoteUrl)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download file: unexpected HTTP status %s", resp.Status)
	}

	// ParseMediaType also decodes RFC 2231 filename* parameters.
	var filename string
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		filename = params["filename"]
	}
	if !validDownloadFilename(filename) {
		escapedPath := resp.Request.URL.EscapedPath()
		filename, err = url.PathUnescape(escapedPath[strings.LastIndex(escapedPath, "/")+1:])
		if err != nil || !validDownloadFilename(filename) {
			return fmt.Errorf("download response and URL have no valid filename")
		}
	}

	for _, name := range []string{tempPath, path} {
		if err := os.MkdirAll(name, 0755); err != nil {
			return fmt.Errorf("create download directory: %w", err)
		}
	}

	f, err := os.CreateTemp(tempPath, "download-*")
	if err != nil {
		return fmt.Errorf("create temporary download: %w", err)
	}
	moved := false
	defer func() {
		if !moved {
			os.Remove(f.Name())
		}
	}()

	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("write download: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close download: %w", closeErr)
	}

	if err := os.Rename(f.Name(), filepath.Join(path, filename)); err != nil {
		return fmt.Errorf("move download to destination: %w", err)
	}
	moved = true

	return extractDownloadedArchive(filepath.Join(path, filename), path)
}

func validDownloadFilename(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\:\x00") && filepath.IsLocal(name)
}
