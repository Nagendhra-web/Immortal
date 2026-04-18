package pqaudit

// WORMWriter implements an append-only, fsync-per-write, file-locked on-disk
// audit ledger (Write-Once-Read-Many pattern).
//
// Durability: every Append call performs an os.File.Sync() (fdatasync on Linux)
// so the OS page cache is flushed before Append returns. A power-loss event
// after a successful Append will not lose the entry.
//
// Exclusion: a sidecar ".lock" file is created with O_CREATE|O_EXCL (atomic on
// POSIX and Windows NTFS). This is ADVISORY — a process that ignores the lock
// file can still corrupt the ledger. For stronger guarantees on Linux/macOS use
// syscall.Flock; on Windows use LockFileEx via golang.org/x/sys/windows.
// Avoiding that dependency keeps the module self-contained.
//
// File format: one JSON-encoded Entry per line (newline-delimited JSON / NDJSON).
// Lines are never modified or deleted; new entries are always appended.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// WORMWriter appends JSON-encoded Entry records to a file, fsyncing after each
// write. It holds an exclusive advisory lock for the lifetime of the writer.
type WORMWriter struct {
	f        *os.File
	lockFile *os.File
	enc      *json.Encoder
}

// NewWORMWriter opens (or creates) the ledger file at path for appending.
// It also creates a sidecar lock file (<path>.lock); if that file already
// exists another process holds the writer — an error is returned.
func NewWORMWriter(path string) (*WORMWriter, error) {
	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: acquire WORM lock %q: %w (another writer may be running)", lockPath, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		lf.Close()
		os.Remove(lockPath)
		return nil, fmt.Errorf("pqaudit: open WORM ledger %q: %w", path, err)
	}

	w := &WORMWriter{
		f:        f,
		lockFile: lf,
		enc:      json.NewEncoder(f),
	}
	return w, nil
}

// Append JSON-encodes e, writes it followed by a newline, then fsyncs the file.
// Returns an error if encoding, writing, or syncing fails.
func (w *WORMWriter) Append(e *Entry) error {
	if err := w.enc.Encode(e); err != nil { // Encode appends '\n'
		return fmt.Errorf("pqaudit: WORM encode: %w", err)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("pqaudit: WORM fsync: %w", err)
	}
	return nil
}

// Close releases the file handle and removes the advisory lock file.
func (w *WORMWriter) Close() error {
	var errs []error
	if err := w.f.Close(); err != nil {
		errs = append(errs, err)
	}
	lockPath := w.lockFile.Name()
	if err := w.lockFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("pqaudit: WORM close: %v", errs)
	}
	return nil
}

// ---------------------------------------------------------------------------

// WORMReader streams Entry records from a WORM ledger file.
type WORMReader struct {
	f       *os.File
	scanner *bufio.Scanner
}

// NewWORMReader opens path for reading.
func NewWORMReader(path string) (*WORMReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: open WORM ledger for reading %q: %w", path, err)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20) // 1 MiB per line max
	return &WORMReader{f: f, scanner: sc}, nil
}

// Next returns the next Entry from the file, or io.EOF when exhausted.
// Returns an error on malformed JSON.
func (r *WORMReader) Next() (*Entry, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, fmt.Errorf("pqaudit: WORM scan: %w", err)
		}
		return nil, io.EOF
	}
	line := r.scanner.Bytes()
	if len(line) == 0 {
		return r.Next() // skip blank lines
	}
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return nil, fmt.Errorf("pqaudit: WORM decode: %w", err)
	}
	return &e, nil
}

// Close releases the file handle.
func (r *WORMReader) Close() error {
	return r.f.Close()
}

// ---------------------------------------------------------------------------

// LoadWORM replays all entries from a WORM file into a new Ledger.
// The chain hash and signatures are verified; an error is returned if any
// entry fails verification or if the file is corrupt.
func LoadWORM(path string, cfg Config) (*Ledger, error) {
	r, err := NewWORMReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	l, err := New(cfg)
	if err != nil {
		return nil, err
	}

	for {
		e, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		l.mu.Lock()
		l.entries = append(l.entries, *e)
		l.seq = e.Seq
		l.mu.Unlock()
	}

	ok, issues := l.Verify()
	if !ok {
		return nil, fmt.Errorf("pqaudit: WORM replay verification failed: %v", issues)
	}
	return l, nil
}
