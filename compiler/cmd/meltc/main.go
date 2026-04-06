package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"meltlang/compiler/internal/build"
	"meltlang/compiler/internal/diag"
	"meltlang/compiler/internal/interp"
	"meltlang/compiler/internal/lexer"
	"meltlang/compiler/internal/lower"
	"meltlang/compiler/internal/parser"
	"meltlang/compiler/internal/sema"
)

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}

	command := os.Args[1]
	path := os.Args[2]
	outPath := ""
	if command == "build" {
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "-o" && i+1 < len(os.Args) {
				outPath = os.Args[i+1]
				i++
			}
		}
		if outPath == "" {
			outPath = filepath.Join("build", "app")
		}
	}

	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	toks, lexDiags := lexer.Lex(path, string(src))
	if len(lexDiags) > 0 {
		printDiags(lexDiags)
		os.Exit(1)
	}

	mod, parseDiags := parser.Parse(toks)
	if len(parseDiags) > 0 {
		printDiags(parseDiags)
		os.Exit(1)
	}

	info, semaDiags := sema.Analyze(mod)
	if len(semaDiags) > 0 {
		printDiags(semaDiags)
		os.Exit(1)
	}

	mirMod, err := lower.Lower(mod, info)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch command {
	case "check":
		fmt.Println("ok")
	case "ast":
		out, err := json.MarshalIndent(mod, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "mir":
		out, err := json.MarshalIndent(mirMod, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "run":
		if err := interp.New(mirMod).RunMain(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "build":
		root, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := build.Build(mirMod, outPath, root, path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(outPath)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: meltc <check|ast|mir|run|build> <file.melt> [-o output]")
}

func printDiags(diags []diag.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(os.Stderr, d.Error())
	}
}
