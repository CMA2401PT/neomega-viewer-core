// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"neomega-viewer-core/viewer-assets/resources"
	"neomega-viewer-core/viewer-assets/s01_extract"
	"neomega-viewer-core/viewer-assets/s02_patch"
	"neomega-viewer-core/viewer-assets/s03_models"
	"neomega-viewer-core/viewer-assets/s04_special"
	"neomega-viewer-core/viewer-assets/s05_atlas"
	"neomega-viewer-core/viewer-assets/utils/fetch"
	"neomega-viewer-core/viewer-assets/utils/inventory"
)

type paths []string

func (value *paths) String() string        { return fmt.Sprint([]string(*value)) }
func (value *paths) Set(path string) error { *value = append(*value, path); return nil }

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: neomega-assets <fetch|extract|patch|models|special|atlas|audit|build> -help")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var overlays paths
	switch args[0] {
	case "fetch":
		version := flags.String("version", resources.JavaTargetVersion, "Mojang release ID, or latest for the latest stable release")
		cache := flags.String("cache", ".cache", "download directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		result, err := fetch.Run(ctx, fetch.Options{Version: *version, CacheDir: *cache})
		if err != nil {
			return err
		}
		fmt.Printf("Minecraft %s: %s\n", result.Version.ID, result.JarPath)
		return nil
	case "extract":
		jar := flags.String("jar", "", "local client JAR")
		output := flags.String("out", "dist/works/01-extracted", "extracted assets and snapshot.json")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *jar == "" {
			return fmt.Errorf("extract requires -jar")
		}
		snapshot, err := s01_extract.Run(ctx, s01_extract.Options{JarPath: *jar, OutputDir: *output})
		if err != nil {
			return err
		}
		fmt.Printf("s01: %s, %d blockstates, %d models, %d PNGs -> %s\n", snapshot.Version, len(snapshot.BlockStates), len(snapshot.Models), len(snapshot.Textures), *output)
		return nil
	case "patch":
		input := flags.String("in", "dist/works/01-extracted", "extracted snapshot directory")
		output := flags.String("out", "dist/works/02-patched", "patched snapshot directory")
		flags.Var(&overlays, "overlay", "resource pack directory; may be repeated, last wins")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		snapshot, err := s02_patch.Run(ctx, s02_patch.Options{InputDir: *input, OutputDir: *output, ResourcePacks: overlays})
		if err != nil {
			return err
		}
		fmt.Printf("s02: %d blockstates, %d models -> %s\n", len(snapshot.BlockStates), len(snapshot.Models), *output)
		return nil
	case "models":
		input := flags.String("in", "dist/works/02-patched", "patched snapshot directory")
		output := flags.String("out", "dist/works/03-models", "model table and report directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		report, err := s03_models.Run(s03_models.Options{InputDir: *input, OutputDir: *output})
		if err != nil {
			return err
		}
		fmt.Printf("s03: %d blockstates, %d models, %d referenced textures, %d geometryless blockstates -> %s\n", report.BlockStateCount, report.ModelCount, len(report.RequiredTextures), len(report.GeometrylessBlockStates), *output)
		return nil
	case "special":
		input := flags.String("in", "dist/works/02-patched", "patched snapshot directory")
		models := flags.String("models", "dist/works/03-models", "model table/report directory")
		output := flags.String("out", "dist/works/04-renderers", "completed models and renderer plans")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		report, err := s04_special.Run(ctx, s04_special.Options{InputDir: *input, ModelDir: *models, OutputDir: *output})
		if err != nil {
			return err
		}
		fmt.Printf("s04: renderer counts %v -> %s\n", report.Counts, *output)
		return nil
	case "atlas":
		input := flags.String("in", "dist/works/04-renderers", "s04 completed snapshot directory")
		models := flags.String("models", "dist/works/04-renderers", "model table/report directory")
		output := flags.String("out", "dist/final", "final output directory")
		tile := flags.Int("tile-size", 16, "block texture tile size, power of two")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		atlas, err := s05_atlas.Run(ctx, s05_atlas.Options{InputDir: *input, ModelDir: *models, OutputDir: *output, TileSize: *tile})
		if err != nil {
			return err
		}
		fmt.Printf("s05: %d textures, %dx%d atlas -> %s\n", len(atlas.Textures), atlas.Width, atlas.Height, *output)
		return nil
	case "audit":
		baseline := flags.String("baseline", "", "baseline mc-assets directory with tables")
		current := flags.String("current", "dist/final", "current final output")
		snapshot := flags.String("snapshot", "dist/works/01-extracted", "original JAR snapshot")
		raw := flags.String("raw-models", "dist/works/03-models", "models before special previews")
		completed := flags.String("completed", "dist/works/04-renderers", "completed models/index")
		output := flags.String("out", "resources/audits/atlas-inventory.json", "inventory report path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *baseline == "" {
			return fmt.Errorf("audit requires -baseline")
		}
		report, err := inventory.Run(inventory.Options{BaselineDir: *baseline, CurrentDir: *current, SnapshotDir: *snapshot, RawModelDir: *raw, CompletedDir: *completed, OutputPath: *output})
		if err != nil {
			return err
		}
		fmt.Printf("audit: %d -> %d textures, %d missing JAR block textures, %d missing references, geometry regressions %d -> %d\n", report.BaselineTextures, report.CurrentTextures, len(report.MissingCurrentBlockPNGs), len(report.MissingRequiredTextures), len(report.BeforeMissingGeometry), len(report.AfterMissingGeometry))
		return nil
	case "build":
		jar := flags.String("jar", "", "local JAR; when omitted fetch -version")
		version := flags.String("version", resources.JavaTargetVersion, "release ID when downloading")
		cache := flags.String("cache", ".cache", "download directory")
		work := flags.String("work", "dist/works", "intermediate directories")
		output := flags.String("out", "dist/final", "final output directory")
		flags.Var(&overlays, "overlay", "resource pack directory; may be repeated, last wins")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		jarPath := *jar
		if jarPath == "" {
			result, err := fetch.Run(ctx, fetch.Options{Version: *version, CacheDir: *cache})
			if err != nil {
				return err
			}
			jarPath = result.JarPath
			fmt.Printf("fetch: Minecraft %s -> %s\n", result.Version.ID, jarPath)
		}
		extracted := filepath.Join(*work, "01-extracted")
		patched := filepath.Join(*work, "02-patched")
		models := filepath.Join(*work, "03-models")
		snapshot, err := s01_extract.Run(ctx, s01_extract.Options{JarPath: jarPath, OutputDir: extracted})
		if err != nil {
			return err
		}
		fmt.Printf("s01: extracted Minecraft %s\n", snapshot.Version)
		if _, err := s02_patch.Run(ctx, s02_patch.Options{InputDir: extracted, OutputDir: patched, ResourcePacks: overlays}); err != nil {
			return err
		}
		fmt.Println("s02: patched assets")
		report, err := s03_models.Run(s03_models.Options{InputDir: patched, OutputDir: models})
		if err != nil {
			return err
		}
		fmt.Printf("s03: %d models, %d geometryless blockstates\n", report.ModelCount, len(report.GeometrylessBlockStates))
		completed := filepath.Join(*work, "04-renderers")
		rendering, err := s04_special.Run(ctx, s04_special.Options{InputDir: patched, ModelDir: models, OutputDir: completed})
		if err != nil {
			return err
		}
		fmt.Printf("s04: renderer counts %v\n", rendering.Counts)
		atlas, err := s05_atlas.Run(ctx, s05_atlas.Options{InputDir: completed, ModelDir: completed, OutputDir: *output, TileSize: 16})
		if err != nil {
			return err
		}
		fmt.Printf("s05: %d textures, %dx%d -> %s\n", len(atlas.Textures), atlas.Width, atlas.Height, *output)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
