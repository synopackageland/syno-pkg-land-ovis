package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
)

// workerFresh is how recent the worker heartbeat must be to count as running.
const workerFresh = 15 * time.Second

// lookupSubmitterUID resolves the authenticated DSM username on every request.
// Unknown UIDs fail closed for owners and submission.
var lookupSubmitterUID = osLookupUID

func osLookupUID(name string) string {
	u, err := user.Lookup(name)
	if err != nil {
		return ""
	}
	return u.Uid
}

type backgroundAPI struct {
	store   *background.Store
	dataDir string
}

// BackgroundRoutes serves job controls for App-owned imported input.
func BackgroundRoutes(dataDir string) []api.Route {
	return newBackgroundRoutes(background.NewRequestStore(filepath.Join(dataDir, "background")), dataDir)
}

// Every request authenticates against DSM afresh. Owner routes are available
// to signed-in users; schedule reapproval and owner cleanup are admin-only.
func newBackgroundRoutes(store *background.Store, dataDir string) []api.Route {
	b := backgroundAPI{store: store, dataDir: dataDir}
	return []api.Route{
		api.HandleRequest("bg.status", b.owner(b.status)),
		api.HandleRequest("bg.schedules.list", b.owner(b.schedules)),
		api.HandleRequest("bg.schedules.create", b.owner(b.createSchedule)),
		api.HandleRequest("bg.schedules.disable", b.owner(b.disableSchedule)),
		api.HandleRequest("bg.schedules.delete", b.owner(b.deleteSchedule)),
		api.HandleRequest("bg.schedules.enable", b.admin(b.enableSchedule)),
		api.HandleRequest("bg.jobs.purge", b.admin(b.purge)),
		api.HandleRequest("bg.jobs.submit", b.owner(b.submit)),
		api.HandleRequest("bg.jobs.list", b.owner(b.list)),
		api.HandleRequest("bg.jobs.get", b.owner(b.get)),
		api.HandleRequest("bg.jobs.cancel", b.owner(b.cancel)),
		api.HandleRequest("bg.jobs.pause", b.owner(b.pause)),
		api.HandleRequest("bg.jobs.resume", b.owner(b.resume)),
		api.HandleRequest("bg.jobs.result", b.owner(b.result)),
	}
}

type backgroundHandler func(ctx context.Context, c api.Caller, r *http.Request, params json.RawMessage) (any, error)

func (backgroundAPI) admin(h backgroundHandler) api.RequestHandler {
	return func(ctx context.Context, c api.Caller, r *http.Request, params json.RawMessage) (any, error) {
		if !c.Admin() {
			return nil, &api.Error{Code: "unsupported", Message: "background jobs need a DSM administrator"}
		}
		return h(ctx, c, r, params)
	}
}

type idParams struct {
	ID string `json:"id"`
}

type jobView struct {
	Job background.Job `json:"job"`
}

func (b backgroundAPI) status(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	if err := decodeParams(params, &struct{}{}); err != nil {
		return nil, err
	}
	st, err := b.store.StatusFor(ctx, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	var worker map[string]any
	if st.Worker != nil {
		worker = map[string]any{"pid": st.Worker.PID, "uid": st.Worker.UID, "username": st.Worker.Username,
			"revision": st.Worker.Revision, "at": st.Worker.At, "alive": workerAlive(*st.Worker, time.Now())}
	}
	return map[string]any{
		"owner": accessFrom(ctx).Owner, "supported": true, "admin": accessFrom(ctx).Admin, "algorithm": st.Algorithm, "revision": st.Revision,
		"queued": st.Queued, "running": st.Running, "worker": worker,
		"limits": map[string]any{"maxInputBytes": background.MaxInputBytes, "maxQueued": background.MaxQueued,
			"maxSchedules": background.MaxSchedules, "intervalSeconds": []int{3600, 86400, 604800}, "maxTotalBytes": background.MaxTotalBytes, "maxOwnerPending": background.MaxOwnerPending, "maxOwnerBytes": background.MaxOwnerBytes, "jobTimeoutSeconds": int(background.JobTimeout / time.Second)},
	}, nil
}

func workerAlive(w background.WorkerInfo, now time.Time) bool {
	return now.Sub(w.At) <= workerFresh
}

func (b backgroundAPI) submit(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	var in struct {
		Key   string `json:"key"`
		Name  string `json:"name"`
		Input string `json:"input"`
	}
	if err := decodeParams(params, &in); err != nil {
		return nil, err
	}
	if in.Key == "" || in.Name == "" || in.Input == "" {
		return nil, api.BadRequest("key, name and input are required")
	}
	access := accessFrom(ctx)
	if !access.Owner.Valid() {
		return nil, &api.Error{Code: "identity_unavailable", Message: "DSM user UID could not be resolved"}
	}
	data, err := b.readInput(in.Input, access.Owner)
	if err != nil {
		return nil, err
	}
	job, duplicate, err := b.store.Submit(ctx, background.SubmitRequest{
		Key: in.Key, Name: in.Name, Data: data, Submitter: access.Owner,
	})
	if err != nil {
		return nil, backgroundError(err)
	}
	return map[string]any{"job": job, "duplicate": duplicate}, nil
}

// readInput opens only the authenticated owner's producer inbox. os.Root
// confines symlinks as well as lexical paths to that inbox, including for admins.
func (b backgroundAPI) readInput(input string, owner background.Submitter) ([]byte, error) {
	bad := api.BadRequest("input must be a regular file in your App inbox")
	inbox, err := BackgroundInbox(b.dataDir, owner)
	if err != nil {
		return nil, bad
	}
	rel, err := filepath.Rel(inbox, input)
	if err != nil || !filepath.IsAbs(input) || !filepath.IsLocal(rel) {
		return nil, bad
	}
	root, err := os.OpenRoot(inbox)
	if err != nil {
		return nil, bad
	}
	defer root.Close()
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, bad
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, bad
	}
	if info.Size() > background.MaxInputBytes {
		return nil, backgroundError(background.ErrTooLarge)
	}
	data, err := io.ReadAll(io.LimitReader(f, background.MaxInputBytes+1))
	if err != nil {
		return nil, bad
	}
	if len(data) > background.MaxInputBytes {
		return nil, backgroundError(background.ErrTooLarge)
	}
	return data, nil
}

func (b backgroundAPI) list(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	if err := decodeParams(params, &struct{}{}); err != nil {
		return nil, err
	}
	jobs, err := b.store.ListFor(ctx, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	newestFirst := make([]background.Job, len(jobs))
	for i, j := range jobs {
		newestFirst[len(jobs)-1-i] = j
	}
	return map[string]any{"jobs": newestFirst}, nil
}

func (b backgroundAPI) get(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	return b.byID(ctx, params, b.store.GetFor)
}

func (b backgroundAPI) cancel(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	return b.byID(ctx, params, b.store.CancelFor)
}

// pause holds a queued job at once; a running job pauses at its next checkpoint.
func (b backgroundAPI) pause(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	return b.byID(ctx, params, b.store.PauseFor)
}

func (b backgroundAPI) resume(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	return b.byID(ctx, params, b.store.ResumeFor)
}

func (b backgroundAPI) byID(ctx context.Context, params json.RawMessage, act func(context.Context, string, background.Access) (background.Job, error)) (any, error) {
	var in idParams
	if err := decodeParams(params, &in); err != nil {
		return nil, err
	}
	job, err := act(ctx, in.ID, accessFrom(ctx))
	return jobOrError(job, err)
}

func jobOrError(job background.Job, err error) (any, error) {
	if err != nil {
		return nil, backgroundError(err)
	}
	return jobView{Job: job}, nil
}

func (b backgroundAPI) result(ctx context.Context, _ api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	var in idParams
	if err := decodeParams(params, &in); err != nil {
		return nil, err
	}
	job, err := b.store.GetFor(ctx, in.ID, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	if job.State != background.StateSucceeded || job.Result == nil {
		return nil, &api.Error{Code: "not_ready", Message: "the job has no result"}
	}
	return map[string]any{"job": job, "result": map[string]any{
		"jobId": job.ID, "worker": job.Worker, "workerRevision": job.WorkerRevision, "algorithm": "sha256",
		"sha256": job.Result.SHA256, "bytes": job.Result.Bytes, "inputDigest": job.InputDigest, "name": job.Name,
	}}, nil
}

func backgroundError(err error) error {
	switch {
	case errors.Is(err, background.ErrBusy):
		return &api.Error{Code: "jobs_busy", Message: "background jobs are busy; retry later"}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, background.ErrScheduleFull):
		return &api.Error{Code: "schedule-full", Message: "the schedule limit has been reached"}
	case errors.Is(err, background.ErrInputUnavailable):
		return &api.Error{Code: "input-unavailable", Message: "import the private copy again"}
	case errors.Is(err, background.ErrOwnerQueueFull):
		return &api.Error{Code: "owner-queue-full", Message: "your pending job limit has been reached"}
	case errors.Is(err, background.ErrOwnerQuota):
		return &api.Error{Code: "owner-quota-exceeded", Message: "your background storage limit has been reached"}
	case errors.Is(err, background.ErrInvalid):
		return api.BadRequest("invalid background request")
	case errors.Is(err, background.ErrTooLarge):
		return &api.Error{Code: "limit-exceeded", Message: "the file is larger than the background limit"}
	case errors.Is(err, background.ErrQueueFull):
		return &api.Error{Code: "queue-full", Message: "the background queue is full"}
	case errors.Is(err, background.ErrQuota):
		return &api.Error{Code: "quota-exceeded", Message: "the background spool is full"}
	case errors.Is(err, background.ErrKeyConflict), errors.Is(err, background.ErrNotCancelable),
		errors.Is(err, background.ErrNotPausable):
		return &api.Error{Code: "conflict", Message: "the request conflicts with the job's current state"}
	case errors.Is(err, background.ErrNotFound):
		return &api.Error{Code: "not_found", Message: "job not found"}
	}
	log.Printf("background spool error: %v", err)
	return &api.Error{Code: "unavailable", Message: "the background spool is unavailable"}
}
