// SPDX-License-Identifier: MIT
// Package s01_extract reads resource entries from a Java client JAR.
package s01_extract

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
	jmodel "neomega-blocks/geometry/java_block_model"
)

type Options struct {
	JarPath   string
	OutputDir string
}

// Classify selects blockstates, models, PNGs and their metadata, preserving
// nested directories and namespaces. Class files and unrelated JAR data stay out.
func Classify(name string) (category string, id jmodel.ResourceID, ok bool) {
	if path.Clean(name) != name || strings.Contains(name, "\\") {
		return "", "", false
	}
	parts := strings.SplitN(name, "/", 4)
	if len(parts) != 4 || parts[0] != "assets" {
		return "", "", false
	}
	category = parts[2]
	resource := parts[3]
	switch category {
	case "blockstates", "models":
		if !strings.HasSuffix(resource, ".json") {
			return "", "", false
		}
		resource = strings.TrimSuffix(resource, ".json")
	case "textures":
		if strings.HasSuffix(resource, ".png.mcmeta") {
			category = "textureMetadata"
			resource = strings.TrimSuffix(resource, ".png.mcmeta")
		} else if strings.HasSuffix(resource, ".png") {
			resource = strings.TrimSuffix(resource, ".png")
		} else {
			return "", "", false
		}
	default:
		return "", "", false
	}
	id, err := jmodel.ParseResourceID(parts[1] + ":" + resource)
	return category, id, err == nil
}

func Run(ctx context.Context, opt Options) (*asset.Snapshot, error) {
	jar, err := zip.OpenReader(opt.JarPath)
	if err != nil {
		return nil, err
	}
	defer jar.Close()
	var version struct {
		ID string `json:"id"`
	}
	versionFile, err := jar.Open("version.json")
	if err != nil {
		return nil, fmt.Errorf("client JAR version.json: %w", err)
	}
	err = json.NewDecoder(versionFile).Decode(&version)
	versionFile.Close()
	if err != nil {
		return nil, err
	}
	if version.ID == "" {
		return nil, fmt.Errorf("client JAR has no version ID")
	}
	snapshot := asset.NewSnapshot(version.ID)
	snapshot.Sources = []asset.Source{{Kind: "minecraft-client-jar", Location: opt.JarPath, License: "Minecraft EULA and Usage Guidelines"}}
	for _, entry := range jar.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		category, id, ok := Classify(entry.Name)
		if !ok {
			continue
		}
		destination := filepath.Join(opt.OutputDir, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return nil, err
		}
		input, err := entry.Open()
		if err != nil {
			return nil, err
		}
		output, err := os.Create(destination)
		if err != nil {
			input.Close()
			return nil, err
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		file := asset.File{Path: entry.Name, Origin: "jar:" + entry.Name}
		switch category {
		case "blockstates":
			snapshot.BlockStates[id] = file
		case "models":
			snapshot.Models[id] = file
		case "textures":
			snapshot.Textures[id] = file
		case "textureMetadata":
			snapshot.TextureMetadata[id] = file
		}
	}
	if len(snapshot.BlockStates) == 0 {
		return nil, fmt.Errorf("JAR has no blockstate resources")
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "snapshot.json"), snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}
