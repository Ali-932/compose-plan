// Package history is the append-only deploy diary at
// .compose-plan/history.jsonl: one JSON line per entry, each carrying the
// hash of the previous entry so hand edits show. Takes a lock file while
// appending so two deploys cannot interleave.
package history
