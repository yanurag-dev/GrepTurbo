# 0001-git-sync-strategy

To ensure search results reflect real-time code changes without the overhead of a background daemon or full index rebuilds, we are implementing a Git-based on-demand synchronization strategy.

### Context
Users expect the search index to be "always fresh," but rebuilding a trigram index on every search is too slow. Approaches like background file watchers introduce complexity (daemons, locking), and content-hashing requires reading every file (defeating the index's performance).

### Decision
We will use `git status` to identify modified, deleted, and untracked files during every search.
- **Baseline**: The on-disk index is tied to a specific Git commit hash stored in `metadata.json`.
- **Overlay**: Dirty files (modified/untracked) are re-indexed in-memory on the fly and their results are merged with the Baseline.
- **Tombstones**: Deleted files are filtered out of the Baseline results.
- **Commit Drift**: If the current Git HEAD differs from the Baseline hash, an automatic full rebuild is triggered.

### Consequences
- **Trade-off**: Search depends on Git being present in the repository.
- **Accuracy**: Search results are 100% accurate relative to the current working tree.
- **Performance**: High-volume changes (thousands of uncommitted files) will increase search latency as they must be indexed in-memory, but we choose to prioritize accuracy over a "too many changes" threshold for now.
