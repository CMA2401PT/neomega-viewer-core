// SPDX-License-Identifier: MIT
// Package s02_patch applies explicit resource overlays without changing inputs.
// The explicit overlay stage follows the separation used by my-mc-assets.
package s02_patch

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"neomega-assets/asset"
	"neomega-assets/internal/jsonfile"
	"neomega-assets/s01_extract"
)

type Options struct {
	InputDir  string
	OutputDir string
	// ResourcePacks are directories with the standard assets/<namespace>/ layout.
	ResourcePacks []string
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func Run(ctx context.Context, opt Options) (*asset.Snapshot, error) {
	input, err := jsonfile.Read[asset.Snapshot](filepath.Join(opt.InputDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	inputPath, err := filepath.Abs(opt.InputDir)
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(opt.OutputDir)
	if err != nil {
		return nil, err
	}
	if inputPath == outputPath {
		return nil, fmt.Errorf("patch output must differ from input")
	}
	maps := []map[asset.ID]asset.File{input.BlockStates, input.Models, input.Textures, input.TextureMetadata}
	for _, files := range maps {
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := copyFile(filepath.Join(opt.InputDir, filepath.FromSlash(file.Path)), filepath.Join(opt.OutputDir, filepath.FromSlash(file.Path))); err != nil {
				return nil, err
			}
		}
	}

	for _, pack := range opt.ResourcePacks {
		if err := applyPack(&input, opt.OutputDir, pack); err != nil {
			return nil, err
		}
		input.Sources = append(input.Sources, asset.Source{Kind: "resource-pack", Location: pack, License: "Terms supplied by resource-pack author"})
	}
	if err := addPlaceholders(&input, opt.OutputDir); err != nil {
		return nil, err
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "snapshot.json"), input); err != nil {
		return nil, err
	}
	return &input, nil
}

func applyPack(snapshot *asset.Snapshot, output, pack string) error {
	return filepath.WalkDir(filepath.Join(pack, "assets"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(pack, path)
		if err != nil {
			return err
		}
		category, id, ok := s01_extract.Classify(filepath.ToSlash(relative))
		if !ok {
			return nil
		}
		if err := copyFile(path, filepath.Join(output, relative)); err != nil {
			return err
		}
		file := asset.File{Path: filepath.ToSlash(relative), Origin: path}
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
		return nil
	})
}
