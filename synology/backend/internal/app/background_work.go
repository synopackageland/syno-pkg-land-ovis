package app

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/synopackageland/syno-pkg-land-sdk/background"
)

// hashCheckpointEvery spaces HashWork's checkpoints; a resumed job repeats at
// most this much hashing. A Work over larger input saves less often.
const hashCheckpointEvery = 64 << 10

// hashState is HashWork's checkpoint: the bytes already hashed and the hash
// state after them.
type hashState struct {
	Offset int64  `json:"offset"`
	State  []byte `json:"state"`
}

// HashWork is the App's sha256-v1 Work: it reports progress as it reads and
// checkpoints the hash state, so a job paused by its owner or by a package
// stop resumes where it left off instead of from the start.
func HashWork(ctx context.Context, t *background.Task) (background.Result, error) {
	h, done, err := resumeHash(t.Job.Checkpoint, t.Input)
	if err != nil {
		return background.Result{}, err
	}
	total, saved := int64(t.Job.InputBytes), done
	buf := make([]byte, 32<<10)
	for {
		if err := ctx.Err(); err != nil {
			return background.Result{}, err
		}
		n, err := t.Input.Read(buf)
		h.Write(buf[:n])
		done += int64(n)
		t.Report(background.Progress{Bytes: done, TotalBytes: total})
		if errors.Is(err, io.EOF) {
			return background.Result{SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: int(done)}, nil
		}
		if err != nil {
			return background.Result{}, err
		}
		if done-saved >= hashCheckpointEvery {
			state, err := saveHash(h, done)
			if err == nil {
				err = t.Checkpoint(state)
			}
			if err != nil {
				return background.Result{}, err
			}
			saved = done
		}
	}
}

func saveHash(h hash.Hash, offset int64) (string, error) {
	state, err := h.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(hashState{Offset: offset, State: state})
	return string(out), err
}

// resumeHash continues the hash a checkpoint saved, past the input it covered.
func resumeHash(checkpoint string, in io.Reader) (hash.Hash, int64, error) {
	h := sha256.New()
	if checkpoint == "" {
		return h, 0, nil
	}
	var saved hashState
	if err := json.Unmarshal([]byte(checkpoint), &saved); err != nil {
		return nil, 0, fmt.Errorf("checkpoint: %w", err)
	}
	if saved.Offset < 0 {
		return nil, 0, errors.New("checkpoint: negative offset")
	}
	if err := h.(encoding.BinaryUnmarshaler).UnmarshalBinary(saved.State); err != nil {
		return nil, 0, fmt.Errorf("checkpoint: %w", err)
	}
	if _, err := io.CopyN(io.Discard, in, saved.Offset); err != nil {
		return nil, 0, fmt.Errorf("checkpoint: %w", err)
	}
	return h, saved.Offset, nil
}
