// SPDX-License-Identifier: MIT
package resourcefiles

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"neomega-assets/asset"
	jmodel "neomega-blocks/geometry/java_block_model"
)

func CopyFile(source, destination string) error {
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
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func CopySnapshot(ctx context.Context, snapshot asset.Snapshot, input, output string) error {
	for _, files := range []map[jmodel.ResourceID]asset.File{snapshot.BlockStates, snapshot.Models, snapshot.Textures, snapshot.TextureMetadata} {
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := CopyFile(filepath.Join(input, filepath.FromSlash(file.Path)), filepath.Join(output, filepath.FromSlash(file.Path))); err != nil {
				return err
			}
		}
	}
	return nil
}
