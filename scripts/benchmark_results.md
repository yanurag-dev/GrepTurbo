# Build Phase Benchmark Results

Run `./scripts/benchmark_build.sh <repo> <n>` before and after concurrency
changes to compare. Each run appends a new section.

---

## Run: 2026-04-08 22:04:27

| Field              | Value                      |
|--------------------|----------------------------|
| Commit             | `a1f0b8c` (main) |
| Target repo        | fastregex               |
| Files indexed      | 21                     |
| Unique trigrams    | 16610                  |
| Runs (excl warmup) | 3                      |

| Phase          | Avg (ms)          | What happens here                  |
|----------------|-------------------|------------------------------------|
| Walk + Extract | 33.2         | ReadFile + trigram.Extract per file |
| Finalize       | 1.7     | sort + dedup all posting lists     |
| Write          | 171.2        | write 3 index files to disk        |
| **Total**      | **206.2**    |                                    |

---

## Run: 2026-04-08 22:14:01

| Field              | Value                      |
|--------------------|----------------------------|
| Commit             | `a1f0b8c` (main) |
| Target repo        | openclaw               |
| Files indexed      | 12950                     |
| Unique trigrams    | 223416                  |
| Runs (excl warmup) | 5                      |

| Phase          | Avg (ms)          | What happens here                  |
|----------------|-------------------|------------------------------------|
| Walk + Extract | 5032.2         | ReadFile + trigram.Extract per file |
| Finalize       | 107.0     | sort + dedup all posting lists     |
| Write          | 44014.3        | write 3 index files to disk        |
| **Total**      | **49153.5**    |                                    |

---

## Run: 2026-04-09 20:53:36

| Field              | Value                      |
|--------------------|----------------------------|
| Commit             | `a1f0b8c` (main) |
| Target repo        | openclaw               |
| Files indexed      | 12950                     |
| Unique trigrams    | 223416                  |
| Runs (excl warmup) | 5                      |

| Phase          | Avg (ms)          | What happens here                  |
|----------------|-------------------|------------------------------------|
| Walk + Extract | 5616.5         | ReadFile + trigram.Extract per file |
| Finalize       | 112.6     | sort + dedup all posting lists     |
| Write          | 390.7        | write 3 index files to disk        |
| **Total**      | **6119.8**    |                                    |

---

