package service

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/logger"
)

const (
	githubAPIURL = "https://api.github.com/repos/alireza0/s-ui/releases/latest"
)

// GitHubRelease represents the GitHub release API response
type GitHubRelease struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// ReleaseAsset represents a release asset from GitHub
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	HasUpdate      bool   `json:"hasUpdate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	PublishedAt    string `json:"publishedAt"`
	DownloadURL    string `json:"downloadURL"`
	AssetSize      int64  `json:"assetSize"`
}

type UpgradeService struct {
}

// getArchName returns the architecture name used in release asset filenames
func getArchName() string {
	arch := runtime.GOARCH
	switch arch {
	case "arm":
		// Determine ARM version from GOARM environment or default to v7
		goarm := os.Getenv("GOARM")
		if goarm == "" {
			goarm = "7"
		}
		return "armv" + goarm
	default:
		return arch
	}
}

// CheckUpdate checks GitHub for the latest release and compares with current version
func (s *UpgradeService) CheckUpdate() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "s-ui/"+config.GetVersion())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GitHub API failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	currentVersion := config.GetVersion()
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	info := &UpdateInfo{
		HasUpdate:      latestVersion != currentVersion,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt,
	}

	// Find the matching asset for current platform
	goos := runtime.GOOS
	arch := getArchName()
	assetName := fmt.Sprintf("s-ui-%s-%s.tar.gz", goos, arch)

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			info.DownloadURL = asset.BrowserDownloadURL
			info.AssetSize = asset.Size
			break
		}
	}

	return info, nil
}

// downloadArchive downloads the release archive and returns the response body
func downloadArchive(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	return resp, nil
}

// extractBinaryFromArchive extracts the binary from a tar.gz archive
func extractBinaryFromArchive(body io.Reader, execName string) ([]byte, error) {
	gzReader, err := gzip.NewReader(body)
	if err != nil {
		return nil, fmt.Errorf("create gzip reader failed: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive failed: %w", err)
		}

		// The binary is named "sui" inside the "s-ui/" directory in the archive
		name := filepath.Base(header.Name)
		if name == "sui" || name == execName {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("read binary from archive failed: %w", err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("binary not found in the downloaded archive")
}

// replaceBinary backs up the current binary and replaces it with the new one
func replaceBinary(execPath string, newBinary []byte) error {
	execDir := filepath.Dir(execPath)
	execName := filepath.Base(execPath)

	// Backup the current binary
	backupPath := execPath + ".bak"
	if err := copyFile(execPath, backupPath); err != nil {
		logger.Warningf("Failed to create backup: %v", err)
	} else {
		logger.Infof("Backed up current binary to %s", backupPath)
	}

	// Write the new binary to a temporary file first
	tmpPath := filepath.Join(execDir, execName+".new")
	if err := os.WriteFile(tmpPath, newBinary, 0755); err != nil {
		return fmt.Errorf("write new binary failed: %w", err)
	}

	// Replace the current binary with the new one
	if err := os.Rename(tmpPath, execPath); err != nil {
		// On some systems, rename may fail if the binary is running
		os.Remove(tmpPath)
		if err := os.WriteFile(execPath, newBinary, 0755); err != nil {
			return fmt.Errorf("replace binary failed: %w", err)
		}
	}
	return nil
}

// Upgrade downloads the latest release and replaces the current binary
func (s *UpgradeService) Upgrade() error {
	info, err := s.CheckUpdate()
	if err != nil {
		return fmt.Errorf("check update failed: %w", err)
	}
	if !info.HasUpdate {
		return fmt.Errorf("already running the latest version (%s)", info.CurrentVersion)
	}
	if info.DownloadURL == "" {
		return fmt.Errorf("no download available for %s/%s", runtime.GOOS, getArchName())
	}

	logger.Infof("Starting upgrade from %s to %s", info.CurrentVersion, info.LatestVersion)

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path failed: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable path failed: %w", err)
	}

	// Download and extract the new binary
	logger.Infof("Downloading %s", info.DownloadURL)
	resp, err := downloadArchive(info.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	newBinary, err := extractBinaryFromArchive(resp.Body, filepath.Base(execPath))
	if err != nil {
		return err
	}
	logger.Infof("Downloaded new binary (%d bytes)", len(newBinary))

	// Replace the binary
	if err := replaceBinary(execPath, newBinary); err != nil {
		return err
	}

	logger.Infof("Successfully upgraded to version %s", info.LatestVersion)

	// Also update s-ui.sh if it exists
	s.updateShellScript(info.DownloadURL, filepath.Dir(execPath))

	return nil
}

// extractFileFromArchive extracts a specific file from a tar.gz archive
func extractFileFromArchive(body io.Reader, targetName string) ([]byte, error) {
	gzReader, err := gzip.NewReader(body)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == targetName {
			return io.ReadAll(tarReader)
		}
	}
}

// updateShellScript re-downloads the archive and extracts s-ui.sh
func (s *UpgradeService) updateShellScript(downloadURL string, installDir string) {
	shPath := filepath.Join(installDir, "s-ui.sh")
	if _, err := os.Stat(shPath); os.IsNotExist(err) {
		return
	}

	resp, err := downloadArchive(downloadURL)
	if err != nil {
		logger.Warningf("Failed to download archive for s-ui.sh update: %v", err)
		return
	}
	defer resp.Body.Close()

	data, err := extractFileFromArchive(resp.Body, "s-ui.sh")
	if err != nil {
		logger.Warningf("Failed to extract s-ui.sh: %v", err)
		return
	}

	if err := os.WriteFile(shPath, data, 0755); err != nil {
		logger.Warningf("Failed to update s-ui.sh: %v", err)
		return
	}
	logger.Info("Updated s-ui.sh")

	// Also copy to /usr/bin/s-ui if it exists
	usrBinPath := "/usr/bin/s-ui"
	if _, err := os.Stat(usrBinPath); err == nil {
		if err := os.WriteFile(usrBinPath, data, 0755); err != nil {
			logger.Warningf("Failed to update %s: %v", usrBinPath, err)
		}
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Preserve file permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
