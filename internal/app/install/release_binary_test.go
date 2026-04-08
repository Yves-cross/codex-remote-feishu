package install

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureReleaseBinaryDownloadsAndExtractsPackage(t *testing.T) {
	version := "v1.2.3"
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	assetName := releaseAssetName(version, goos, goarch)
	packageDir := releasePackageDir(version, goos, goarch)
	archivePath := filepath.Join(t.TempDir(), assetName)
	writeReleaseArchive(t, archivePath, packageDir, executableName(goos), "release-binary")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(r.URL.Path) != assetName {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, archivePath)
	}))
	defer server.Close()

	versionsRoot := filepath.Join(t.TempDir(), "releases")
	binaryPath, err := EnsureReleaseBinary(context.Background(), ReleaseBinaryOptions{
		BaseURL:      server.URL,
		Version:      version,
		VersionsRoot: versionsRoot,
	})
	if err != nil {
		t.Fatalf("EnsureReleaseBinary: %v", err)
	}
	if got, want := binaryPath, filepath.Join(versionsRoot, version, executableName(goos)); got != want {
		t.Fatalf("binary path = %q, want %q", got, want)
	}
	raw, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile binary: %v", err)
	}
	if string(raw) != "release-binary" {
		t.Fatalf("binary contents = %q", string(raw))
	}
}

func writeReleaseArchive(t *testing.T, archivePath, packageDir, binaryName, content string) {
	t.Helper()

	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("OpenFile archive: %v", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     packageDir + "/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}); err != nil {
		t.Fatalf("WriteHeader dir: %v", err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     filepath.Join(packageDir, binaryName),
		Typeflag: tar.TypeReg,
		Mode:     0o755,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatalf("WriteHeader file: %v", err)
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		t.Fatalf("Write file contents: %v", err)
	}
}
