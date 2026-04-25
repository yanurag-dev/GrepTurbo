// benchcmd is an instrumented version of the build phase that emits per-phase
// timing as JSON. Used by scripts/benchmark_build.sh.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"grepturbo/internal/index"
	"grepturbo/internal/trigram"
	"time"
)

const maxFileSize = 1 << 20

type result struct {
	Files         int     `json:"files"`
	Trigrams      int     `json:"trigrams"`
	WalkExtractMs float64 `json:"walk_extract_ms"`
	FinalizeMs    float64 `json:"finalize_ms"`
	WriteMs       float64 `json:"write_ms"`
	TotalMs       float64 `json:"total_ms"`
}

type extractResult struct {
	path     string
	trigrams []trigram.T
}

func main() {
	root := flag.String("root", ".", "root directory to index")
	out := flag.String("out", "/tmp/grepturbo_bench_idx", "output index dir")
	skipFlag := flag.String("skip", "", "comma-separated dir names to skip")
	flag.Parse()

	skipSet := map[string]bool{
		"node_modules": true, ".git": true, ".hg": true,
		"vendor": true, "dist": true, "build": true, ".grepturbo": true,
	}
	if *skipFlag != "" {
		for _, s := range strings.Split(*skipFlag, ",") {
			if s != "" {
				skipSet[s] = true
			}
		}
	}

	b := index.NewBuilder()
	totalStart := time.Now()

	// ── Phase: walk + extract ─────────────────────────────────────────────────
	walkStart := time.Now()

	paths := make(chan string, 100)
	results := make(chan extractResult, 100)

	var wg sync.WaitGroup
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				data, err := os.ReadFile(path)
				if err != nil || !utf8.Valid(data) || len(data) > maxFileSize {
					continue
				}
				results <- extractResult{
					path:     path,
					trigrams: trigram.Extract(string(data)),
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	done := make(chan struct{})
	go func() {
		for res := range results {
			fileID := uint32(len(b.Files))
			b.Files = append(b.Files, res.path)
			for _, t := range res.trigrams {
				b.Posts.AddBatch(t, []uint32{fileID})
			}
		}
		close(done)
	}()

	filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipSet[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		paths <- path
		return nil
	})
	close(paths)
	<-done

	walkExtractMs := ms(time.Since(walkStart))

	// ── Phase: finalize ───────────────────────────────────────────────────────
	finalizeStart := time.Now()
	b.Posts.Finalize()
	finalizeMs := ms(time.Since(finalizeStart))

	// ── Phase: write ──────────────────────────────────────────────────────────
	if err := os.MkdirAll(*out, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir error:", err)
		os.Exit(1)
	}
	writeStart := time.Now()
	if err := index.Write(b, *out); err != nil {
		fmt.Fprintln(os.Stderr, "write error:", err)
		os.Exit(1)
	}
	writeMs := ms(time.Since(writeStart))

	totalMs := ms(time.Since(totalStart))

	json.NewEncoder(os.Stdout).Encode(result{
		Files:         len(b.Files),
		Trigrams:      len(b.Posts),
		WalkExtractMs: walkExtractMs,
		FinalizeMs:    finalizeMs,
		WriteMs:       writeMs,
		TotalMs:       totalMs,
	})
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}
