// Package history is the append-only deploy diary at
// .compose-plan/history.jsonl: one JSON line per entry, each carrying the
// hash of the previous entry so hand edits show. Takes a lock file while
// appending so two deploys cannot interleave.
package history

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gofrs/flock"
)

type Entry struct {
	Seq      int
	Time     time.Time
	User     string
	Commit   string
	Summary  string
	Services map[string]string
	Configs  map[string]map[string]any `json:",omitempty"` // service -> rendered config, env values hashed
	Hash     string
	PrevHash string
}

func Hash(entry Entry) string {
	entry.Hash = ""
	docEntry, _ := json.Marshal(entry)
	bytes := sha256.Sum256(docEntry)
	return hex.EncodeToString(bytes[:])
}

func Append(entry Entry, storageFilePath string) (Entry, error) {
	if err := os.MkdirAll(filepath.Dir(storageFilePath), 0o755); err != nil {
		return Entry{}, err
	}
	fileLock := flock.New(storageFilePath)
	defer fileLock.Unlock()

	locked, err := fileLock.TryLock()
	if err != nil {
		return Entry{}, err
	}

	if !locked {
		return Entry{}, fmt.Errorf("another deploy is in progress")
	}
	file, err := os.OpenFile(storageFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return Entry{}, fmt.Errorf("error opening history file: %v", err)
	}
	defer file.Close()
	entries, err := Read(storageFilePath)
	if err != nil {
		return Entry{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
	var lastEntry Entry
	if len(entries) > 0 {
		lastEntry = entries[len(entries)-1]
	}
	entry.Seq = lastEntry.Seq + 1
	entry.PrevHash = lastEntry.Hash
	entry.Hash = Hash(entry)
	line, _ := json.Marshal(entry)
	file.Write(append(line, '\n'))
	return entry, nil
}

func Read(storageFilePath string) ([]Entry, error) {
	file, err := os.OpenFile(storageFilePath, os.O_RDONLY, 0o644)

	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	previousEntry := Entry{}
	for scanner.Scan() {
		var inputEntry Entry
		line := scanner.Text()
		err := json.Unmarshal([]byte(line), &inputEntry)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling: %v", err)
		}
		hash := Hash(inputEntry)
		if hash != inputEntry.Hash || previousEntry.Hash != inputEntry.PrevHash {
			return nil, fmt.Errorf("hash mismatch between %d and %d", previousEntry.Seq, inputEntry.Seq)
		}
		entries = append(entries, inputEntry)
		previousEntry = inputEntry

	}
	return entries, nil

}

func Find(entries []Entry, seq int) (*Entry, error) {
	for i := range entries {
		if entries[i].Seq == seq {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("no entry with seq %d", seq)
}
