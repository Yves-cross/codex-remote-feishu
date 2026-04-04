package editor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ManagedShimRealBinaryPath(entrypointPath string) string {
	ext := filepath.Ext(entrypointPath)
	if ext == "" {
		return entrypointPath + ".real"
	}
	return strings.TrimSuffix(entrypointPath, ext) + ".real" + ext
}

func PatchBundleEntrypoint(entrypointPath, wrapperBinary string) error {
	if strings.TrimSpace(entrypointPath) == "" {
		return fmt.Errorf("bundle entrypoint path is required")
	}
	if strings.TrimSpace(wrapperBinary) == "" {
		return fmt.Errorf("wrapper binary path is required")
	}
	if err := os.MkdirAll(filepath.Dir(entrypointPath), 0o755); err != nil {
		return err
	}

	realBinaryPath := ManagedShimRealBinaryPath(entrypointPath)
	if _, err := os.Stat(realBinaryPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if _, statErr := os.Stat(entrypointPath); statErr != nil {
			return statErr
		}
		if err := os.Rename(entrypointPath, realBinaryPath); err != nil {
			return err
		}
	}
	return copyExecutable(wrapperBinary, entrypointPath)
}

func copyExecutable(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return targetFile.Chmod(info.Mode().Perm())
}
