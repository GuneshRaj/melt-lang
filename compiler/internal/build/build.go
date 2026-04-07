package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"meltlang/compiler/internal/metalgen"
	"meltlang/compiler/internal/mir"
	"meltlang/compiler/internal/swiftgen"
)

func Build(module *mir.Module, outPath string, root string, sourcePath string) error {
	buildDir := filepath.Join(root, "build")
	swiftDir := filepath.Join(buildDir, "swift")
	metalDir := filepath.Join(buildDir, "metal")
	moduleCacheDir := filepath.Join(buildDir, "clang-module-cache")
	if err := os.MkdirAll(swiftDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(metalDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(moduleCacheDir, 0o755); err != nil {
		return err
	}

	swiftSrc, err := swiftgen.Generate(module, swiftgen.Options{
		BenchmarkMode: strings.Contains(sourcePath, "/benchmarks/") || strings.Contains(sourcePath, "benchmarks/"),
	})
	if err != nil {
		return err
	}
	swiftPath := filepath.Join(swiftDir, "main.swift")
	if err := os.WriteFile(swiftPath, []byte(swiftSrc), 0o644); err != nil {
		return err
	}

	metalPath := filepath.Join(metalDir, "Kernels.metal")
	if err := os.WriteFile(metalPath, []byte(metalgen.GenerateMapScaleKernel()), 0o644); err != nil {
		return err
	}

	airPath := filepath.Join(metalDir, "Kernels.air")
	libPath := filepath.Join(metalDir, "default.metallib")
	if err := run(root, moduleCacheDir, "xcrun", "-sdk", "macosx", "metal", "-c", metalPath, "-o", airPath); err != nil {
		return err
	}
	if err := run(root, moduleCacheDir, "xcrun", "-sdk", "macosx", "metallib", airPath, "-o", libPath); err != nil {
		return err
	}
	if err := run(root, moduleCacheDir, "swiftc", swiftPath,
		"-parse-as-library",
		filepath.Join(root, "support", "Sources", "MeltSupport", "CSV.swift"),
		filepath.Join(root, "support", "Sources", "MeltSupport", "FileIO.swift"),
		filepath.Join(root, "support", "Sources", "MeltSupport", "JSON.swift"),
		filepath.Join(root, "support", "Sources", "MeltSupport", "MetalSupport.swift"),
		filepath.Join(root, "support", "Sources", "MeltSupport", "Parquet.swift"),
		filepath.Join(root, "support", "Sources", "MeltSupport", "RuntimeTypes.swift"),
		"-o", outPath,
	); err != nil {
		return err
	}
	return copyFile(libPath, filepath.Join(filepath.Dir(outPath), "default.metallib"))
}

func run(dir string, moduleCacheDir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"CLANG_MODULE_CACHE_PATH="+moduleCacheDir,
		"MODULE_CACHE_DIR="+moduleCacheDir,
	)
	return cmd.Run()
}

func copyFile(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
