package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	updateOwner       = "hiyamamo"
	updateRepo        = "ws-dev"
	updateBinaryName  = "ws-dev"
	updateChecksumTxt = "checksums.txt"
)

func newUpdateCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Update the ws-dev binary to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdate(cmd.OutOrStdout(), force)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "Reinstall even if already on the latest version")
	return c
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

func runUpdate(out io.Writer, force bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	current, _, _ := resolveBuildInfo()
	_, _ = fmt.Fprintf(out, "Current version: %s\n", current)

	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}
	if !force && current != "dev" && sameVersion(current, rel.TagName) {
		_, _ = fmt.Fprintf(out, "Already up to date (%s)\n", rel.TagName)
		return nil
	}

	want := assetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
	asset, ok := findAsset(rel.Assets, want)
	if !ok {
		return fmt.Errorf("no release asset %q for %s/%s in %s", want, runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	_, _ = fmt.Fprintf(out, "Downloading %s ...\n", asset.Name)
	tarGz, err := download(asset.URL)
	if err != nil {
		return err
	}

	if err := verifyChecksum(rel.Assets, want, tarGz); err != nil {
		return err
	}

	bin, err := extractBinary(tarGz, updateBinaryName)
	if err != nil {
		return err
	}

	if err := replaceExecutable(exePath, bin); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "Updated %s -> %s (%s)\n", current, rel.TagName, exePath)
	return nil
}

// fetchLatestRelease queries the GitHub API for the latest (non-prerelease)
// release of the repo.
func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateOwner, updateRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ws-dev-updater")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("github api returned no tag_name")
	}
	return &rel, nil
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ws-dev-updater")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum downloads checksums.txt and confirms the sha256 of tarGz.
func verifyChecksum(assets []ghAsset, assetFile string, tarGz []byte) error {
	sums, ok := findAsset(assets, updateChecksumTxt)
	if !ok {
		return fmt.Errorf("release has no %s to verify against", updateChecksumTxt)
	}
	data, err := download(sums.URL)
	if err != nil {
		return err
	}
	want, ok := parseChecksum(data, assetFile)
	if !ok {
		return fmt.Errorf("%s has no entry for %s", updateChecksumTxt, assetFile)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(tarGz))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", assetFile, got, want)
	}
	return nil
}

// replaceExecutable atomically swaps the binary at exePath with bin. The temp
// file is created in the same directory so os.Rename stays on one filesystem
// and works even while the current binary is running.
func replaceExecutable(exePath string, bin []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".ws-dev-update-*")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("cannot write to %s: %w\n(the binary lives in a protected dir; re-run with sudo or move it onto a user-writable PATH)", dir, err)
		}
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(bin); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		cleanup()
		if os.IsPermission(err) {
			return fmt.Errorf("cannot replace %s: %w\n(re-run with sudo)", exePath, err)
		}
		return err
	}
	return nil
}

// --- pure helpers (unit-tested) ---

func assetName(tag, goos, goarch string) string {
	v := strings.TrimPrefix(tag, "v")
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", updateBinaryName, v, goos, goarch)
}

func findAsset(assets []ghAsset, name string) (ghAsset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghAsset{}, false
}

func sameVersion(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	if a == "" || a == "dev" {
		return false
	}
	return a == b
}

func parseChecksum(data []byte, filename string) (string, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return fields[0], true
		}
	}
	return "", false
}

func extractBinary(tarGz []byte, binName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarGz))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == binName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", binName)
}
