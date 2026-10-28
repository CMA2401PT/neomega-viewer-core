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

	"neomega-assets/fetch"
	"neomega-assets/s01_extract"
	"neomega-assets/s02_patch"
	"neomega-assets/s03_models"
	"neomega-assets/s04_atlas"
)

type paths []string

func (value *paths) String() string        { return fmt.Sprint([]string(*value)) }
func (value *paths) Set(path string) error { *value = append(*value, path); return nil }

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: neomega-assets <fetch|extract|patch|models|atlas|build> -help")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var overlays paths
	switch args[0] {
	case "fetch":
		version := flags.String("version", "latest", "Mojang release ID, or latest for the latest stable release")
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
		output := flags.String("out", "work/01-extracted", "extracted assets and snapshot.json")
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
		input := flags.String("in", "work/01-extracted", "extracted snapshot directory")
		output := flags.String("out", "work/02-patched", "patched snapshot directory")
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
		input := flags.String("in", "work/02-patched", "patched snapshot directory")
		output := flags.String("out", "work/03-models", "model table and report directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		report, err := s03_models.Run(s03_models.Options{InputDir: *input, OutputDir: *output})
		if err != nil {
			return err
		}
		fmt.Printf("s03: %d blockstates, %d models, %d referenced textures, %d builtin models -> %s\n", report.BlockStateCount, report.ModelCount, len(report.RequiredTextures), len(report.BuiltinModels), *output)
		return nil
	case "atlas":
		input := flags.String("in", "work/02-patched", "patched snapshot directory")
		models := flags.String("models", "work/03-models", "model table/report directory")
		output := flags.String("out", "dist", "final output directory")
		tile := flags.Int("tile-size", 16, "block texture tile size, power of two")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		atlas, err := s04_atlas.Run(ctx, s04_atlas.Options{InputDir: *input, ModelDir: *models, OutputDir: *output, TileSize: *tile})
		if err != nil {
			return err
		}
		fmt.Printf("s04: %d textures, %dx%d atlas -> %s\n", len(atlas.Textures), atlas.Width, atlas.Height, *output)
		return nil
	case "build":
		jar := flags.String("jar", "", "local JAR; when omitted fetch -version")
		version := flags.String("version", "latest", "release ID when downloading")
		cache := flags.String("cache", ".cache", "download directory")
		work := flags.String("work", "work", "intermediate directories")
		output := flags.String("out", "dist", "final output directory")
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
		fmt.Printf("s03: %d models, %d builtin models\n", report.ModelCount, len(report.BuiltinModels))
		atlas, err := s04_atlas.Run(ctx, s04_atlas.Options{InputDir: patched, ModelDir: models, OutputDir: *output, TileSize: 16})
		if err != nil {
			return err
		}
		fmt.Printf("s04: %d textures, %dx%d -> %s\n", len(atlas.Textures), atlas.Width, atlas.Height, *output)
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
