// SPDX-License-Identifier: MIT
// Package fetch downloads a selected client JAR using Mojang's version manifest.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
)

const ManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type Manifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"versions"`
}

type Options struct {
	Version  string
	CacheDir string
}
type Result struct {
	Version asset.VersionInfo
	JarPath string
}

func get(ctx context.Context, client *http.Client, url string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	return response.Body, nil
}

func Run(ctx context.Context, opt Options) (Result, error) {
	client := &http.Client{Timeout: 3 * time.Minute}
	body, err := get(ctx, client, ManifestURL)
	if err != nil {
		return Result{}, err
	}
	var manifest Manifest
	err = json.NewDecoder(body).Decode(&manifest)
	body.Close()
	if err != nil {
		return Result{}, err
	}
	version := opt.Version
	if version == "" || version == "latest" {
		version = manifest.Latest.Release
	}
	metadataURL := ""
	for _, entry := range manifest.Versions {
		if entry.ID == version {
			metadataURL = entry.URL
			break
		}
	}
	if metadataURL == "" {
		return Result{}, fmt.Errorf("unknown Minecraft version %q", version)
	}
	body, err = get(ctx, client, metadataURL)
	if err != nil {
		return Result{}, err
	}
	var info asset.VersionInfo
	err = json.NewDecoder(body).Decode(&info)
	body.Close()
	if err != nil {
		return Result{}, err
	}
	directory := filepath.Join(opt.CacheDir, version)
	if err := jsonfile.Write(filepath.Join(directory, "version.json"), info); err != nil {
		return Result{}, err
	}
	jarPath := filepath.Join(directory, "client.jar")
	if stat, err := os.Stat(jarPath); err == nil && stat.Size() == info.Downloads.Client.Size {
		return Result{info, jarPath}, nil
	}
	body, err = get(ctx, client, info.Downloads.Client.URL)
	if err != nil {
		return Result{}, err
	}
	defer body.Close()
	temporary, err := os.CreateTemp(directory, "client-*.part")
	if err != nil {
		return Result{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	_, copyErr := io.Copy(temporary, body)
	closeErr := temporary.Close()
	if copyErr != nil {
		return Result{}, copyErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	if err := os.Rename(temporaryPath, jarPath); err != nil {
		return Result{}, err
	}
	return Result{info, jarPath}, nil
}
