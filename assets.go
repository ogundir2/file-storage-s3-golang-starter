package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return "", err
	}

	result := struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}{}

	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		return "", err
	}

	if result.Streams[0].Height == 0 {
		return "", fmt.Errorf("image height is 0")
	}

	ratio := float64(result.Streams[0].Width) / float64(result.Streams[0].Height)
	if equalsWithTolerance(ratio, 16.0/9.0, 0.1) {
		return "16:9", nil
	} else if equalsWithTolerance(ratio, 9.0/16.0, 0.1) {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func (cfg apiConfig) getObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func equalsWithTolerance[T int | float64](v1, v2, tolerance T) bool {
	abs := v1 - v2
	if abs < 0 {
		abs = v2 - v1
	}
	return abs <= tolerance
}

func getVideoOrientationFromAspectRatio(aspectRatio string) string {
	if aspectRatio == "16:9" {
		return "landscape"
	} else if aspectRatio == "9:16" {
		return "portrait"
	} else {
		return "other"
	}
}
