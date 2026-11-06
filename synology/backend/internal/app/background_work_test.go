package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/background"
)

func TestHashWorkResumesFromItsCheckpoint(t *testing.T) {
	head, tail := bytes.Repeat([]byte("h"), 1000), bytes.Repeat([]byte("t"), 3000)
	h := sha256.New()
	h.Write(head)
	saved, err := saveHash(h, int64(len(head)))
	if err != nil {
		t.Fatal(err)
	}
	// The input's first bytes differ from what the checkpoint hashed: only a
	// Work that skips them and continues the saved hash reaches this digest.
	input := append(bytes.Repeat([]byte("x"), len(head)), tail...)
	task := &background.Task{Job: background.Job{Checkpoint: saved}, Input: bytes.NewReader(input)}
	got, err := HashWork(context.Background(), task)
	want := sha256.Sum256(append(head, tail...))
	if err != nil || got.SHA256 != hex.EncodeToString(want[:]) || got.Bytes != len(input) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestHashWorkRefusesAForeignCheckpoint(t *testing.T) {
	for _, saved := range []string{"{", `{"offset":-1,"state":""}`, `{"offset":3,"state":"AAAA"}`} {
		task := &background.Task{Job: background.Job{Checkpoint: saved}, Input: bytes.NewReader([]byte("abc"))}
		if _, err := HashWork(context.Background(), task); err == nil {
			t.Errorf("%s: accepted", saved)
		}
	}
}

// stopAfter ends the worker's context once n bytes have been read, like a
// package stop in the middle of a job.
type stopAfter struct {
	io.Reader
	n    int
	stop context.CancelFunc
}

func (s *stopAfter) Read(p []byte) (int, error) {
	n, err := s.Reader.Read(p)
	if s.n -= n; s.n <= 0 {
		s.stop()
	}
	return n, err
}

func TestHashWorkReportsProgressAndCheckpointsBeforeAStop(t *testing.T) {
	store := background.NewStore(filepath.Join(t.TempDir(), "background"))
	ctx := context.Background()
	data := bytes.Repeat([]byte("0123456789abcdef"), 16<<10)
	job, _, err := store.Submit(ctx, background.SubmitRequest{Key: "k", Name: "a.bin", Data: data, Submitter: background.Submitter{User: "alice", UID: "1024"}})
	if err != nil {
		t.Fatal(err)
	}
	worker, stop := context.WithCancel(ctx)
	runner := &background.Runner{Store: store, Timeout: time.Minute, CancelPoll: 10 * time.Millisecond, Work: func(ctx context.Context, t *background.Task) (background.Result, error) {
		t.Input = &stopAfter{Reader: t.Input, n: 3 * int(hashCheckpointEvery) / 2, stop: stop}
		return HashWork(ctx, t)
	}}
	claimed, err := store.Claim(ctx)
	if err != nil || claimed == nil {
		t.Fatalf("%+v %v", claimed, err)
	}
	runner.Process(worker, claimed)
	paused, err := store.Get(ctx, job.ID)
	if err != nil || paused.State != background.StatePaused || paused.Checkpoint == "" || paused.Progress == nil ||
		paused.Progress.Bytes < hashCheckpointEvery || paused.Progress.TotalBytes != int64(len(data)) {
		t.Fatalf("%+v %+v %v", paused, paused.Progress, err)
	}
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	claimed, err = store.Claim(ctx)
	if err != nil || claimed == nil || claimed.Checkpoint != paused.Checkpoint {
		t.Fatalf("%+v %v", claimed, err)
	}
	runner.Work = HashWork
	runner.Process(ctx, claimed)
	if done, err := store.Get(ctx, job.ID); err != nil || done.State != background.StateSucceeded || done.Progress.Bytes != int64(len(data)) {
		t.Fatalf("%+v %v", done, err)
	}
}
