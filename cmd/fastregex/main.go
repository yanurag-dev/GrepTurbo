package main

import (
	"flag"
	"fmt"
	"os"

	"fastregex/internal/index"
	"fastregex/internal/query"
)

const defaultIndexDir = ".fastregex"

func main() {
	// Subcommands
	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildRoot := buildCmd.String("root", ".", "root directory to index")
	buildOut := buildCmd.String("out", defaultIndexDir, "directory to write the index")

	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	searchIdx := searchCmd.String("index", defaultIndexDir, "index directory to query")

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		runBuild(*buildRoot, *buildOut)

	case "search":
		searchCmd.Parse(os.Args[2:])
		if searchCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fastregex search [flags] <pattern>")
			os.Exit(1)
		}
		pattern := searchCmd.Arg(0)
		runSearch(*searchIdx, pattern)

	default:
		usage()
		os.Exit(1)
	}
}

func runBuild(root, outDir string) {
	fmt.Fprintf(os.Stderr, "Building index for %s → %s\n", root, outDir)

	b := index.NewBuilder()
	if err := b.Build(root); err != nil {
		fmt.Fprintf(os.Stderr, "error walking directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Indexed %d files, %d unique trigrams\n",
		len(b.Files), len(b.Posts))

	if err := index.Write(b, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error writing index: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Done.\n")
}

func runSearch(idxDir, pattern string) {
	r, err := index.NewReader(idxDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening index: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	matches, err := query.Search(r, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error searching: %v\n", err)
		os.Exit(1)
	}

	if len(matches) == 0 {
		os.Exit(1) // grep convention: exit 1 when no matches
	}

	for _, m := range matches {
		fmt.Printf("%s:%d:%s\n", m.File, m.Line, m.Text)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `fastregex — index-accelerated regex search

Commands:
  build   Walk a directory and build the search index
  search  Query the index with a regex pattern

Usage:
  fastregex build  [-root <dir>] [-out <index-dir>]
  fastregex search [-index <index-dir>] <pattern>

Examples:
  fastregex build -root ./myproject -out .fastregex
  fastregex search -index .fastregex 'func.*Error'
  fastregex search 'TODO'`)
}
