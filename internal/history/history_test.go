package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Run with: go test ./internal/history/ -v

func TestHash(t *testing.T) {
	e := Entry{User: "ali", Summary: "x", Services: map[string]string{"web": "sha256:a", "db": "sha256:b"}}

	// Same entry, same seal, every time (map order must not matter).
	if Hash(e) != Hash(e) {
		t.Fatal("hash is not deterministic")
	}

	// Stamping the seal onto the entry must not change what the seal is.
	e.Hash = Hash(e)
	if Hash(e) != e.Hash {
		t.Fatal("hash changes once stored in the entry; Hash must blank the Hash field first")
	}

	// Any edit changes the seal.
	edited := e
	edited.Summary = "y"
	if Hash(edited) == e.Hash {
		t.Fatal("editing the entry did not change the hash")
	}
}

func TestAppendAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".compose-plan", "history.jsonl") // directory does not exist yet

	// Missing file is an empty history, not an error.
	if got, err := Read(path); err != nil || len(got) != 0 {
		t.Fatalf("Read on missing file: got %d entries, err=%v", len(got), err)
	}

	first, err := Append(Entry{User: "ali", Summary: "add redis", Services: map[string]string{"web": "sha256:a"}}, path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Append(Entry{User: "sara", Summary: "web image", Services: map[string]string{"web": "sha256:b"}}, path)
	if err != nil {
		t.Fatal(err)
	}

	if first.Seq != 1 || second.Seq != 2 {
		t.Errorf("seq: got %d then %d, want 1 then 2", first.Seq, second.Seq)
	}
	if first.PrevHash != "" || second.PrevHash != first.Hash {
		t.Errorf("chain not linked: first.Prev=%q second.Prev=%q first.Hash=%q", first.PrevHash, second.PrevHash, first.Hash)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Summary != "add redis" || got[1].Summary != "web image" {
		t.Fatalf("Read: got %+v", got)
	}
}

func TestReadDetectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if _, err := Append(Entry{User: "ali", Summary: "add redis"}, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(Entry{User: "ali", Summary: "web image"}, path); err != nil {
		t.Fatal(err)
	}

	// Edit entry 1 by hand.
	raw, _ := os.ReadFile(path)
	tampered := strings.Replace(string(raw), "add redis", "add nothing", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Read(path); err == nil {
		t.Fatal("Read accepted a tampered file")
	}
}

func TestFind(t *testing.T) {
	entries := []Entry{{Seq: 1}, {Seq: 2, Summary: "second"}}

	if e, err := Find(entries, 2); err != nil || e.Summary != "second" {
		t.Errorf("Find(2) = %+v, %v", e, err)
	}
	if e, err := Find(entries, 9); err == nil || e != nil {
		t.Errorf("Find(9) = %+v, %v; want nil and an error", e, err)
	}
}
