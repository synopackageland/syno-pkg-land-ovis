package app

import (
	"context"
	"encoding/json"
	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
	"net/http"
)

func (b backgroundAPI) schedules(ctx context.Context, _ api.Caller, _ *http.Request, p json.RawMessage) (any, error) {
	if err := decodeParams(p, &struct{}{}); err != nil {
		return nil, err
	}
	list, err := b.store.Schedules(ctx, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	return map[string]any{"schedules": list}, nil
}
func (b backgroundAPI) createSchedule(ctx context.Context, _ api.Caller, _ *http.Request, p json.RawMessage) (any, error) {
	var in background.ScheduleRequest
	if err := decodeParams(p, &in); err != nil {
		return nil, err
	}
	if !accessFrom(ctx).Owner.Valid() {
		return nil, &api.Error{Code: "identity_unavailable", Message: "DSM user UID could not be resolved"}
	}
	sc, dup, err := b.store.CreateSchedule(ctx, in, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	return map[string]any{"schedule": sc, "duplicate": dup}, nil
}
func (b backgroundAPI) disableSchedule(ctx context.Context, _ api.Caller, _ *http.Request, p json.RawMessage) (any, error) {
	var in idParams
	if err := decodeParams(p, &in); err != nil {
		return nil, err
	}
	sc, err := b.store.SetSchedule(ctx, in.ID, false, accessFrom(ctx))
	if err != nil {
		return nil, backgroundError(err)
	}
	return map[string]any{"schedule": sc}, nil
}
func (b backgroundAPI) enableSchedule(ctx context.Context, c api.Caller, _ *http.Request, p json.RawMessage) (any, error) {
	var in struct {
		ID      string `json:"id"`
		Consent bool   `json:"consent"`
	}
	if err := decodeParams(p, &in); err != nil || !in.Consent {
		return nil, api.BadRequest("explicit consent is required")
	}
	sc, err := b.store.SetSchedule(ctx, in.ID, true, background.Access{Admin: true, Owner: background.Submitter{User: c.User(), UID: lookupSubmitterUID(c.User())}})
	if err != nil {
		return nil, backgroundError(err)
	}
	return map[string]any{"schedule": sc}, nil
}
func (b backgroundAPI) deleteSchedule(ctx context.Context, _ api.Caller, _ *http.Request, p json.RawMessage) (any, error) {
	var in idParams
	if err := decodeParams(p, &in); err != nil {
		return nil, err
	}
	if err := b.store.DeleteSchedule(ctx, in.ID, accessFrom(ctx)); err != nil {
		return nil, backgroundError(err)
	}
	return map[string]bool{"deleted": true}, nil
}
