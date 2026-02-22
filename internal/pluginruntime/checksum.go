package pluginruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyFileChecksum(path, expected string) error {
	if !strings.HasPrefix(expected, "sha256:") {
		return fmt.Errorf("unsupported checksum format %q", expected)
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", path, got, expected)
	}
	return nil
}
