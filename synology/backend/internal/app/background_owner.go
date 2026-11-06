package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
	"net/http"
	"path/filepath"
)

type backgroundAccessKey struct{}

func accessFrom(ctx context.Context) background.Access {
	return ctx.Value(backgroundAccessKey{}).(background.Access)
}
func (b backgroundAPI) owner(h backgroundHandler) api.RequestHandler {
	return func(ctx context.Context, c api.Caller, r *http.Request, params json.RawMessage) (any, error) {
		access := background.Access{Admin: c.Admin()}
		access.Owner = background.Submitter{User: c.User(), UID: lookupSubmitterUID(c.User())}
		if !access.Admin && !access.Owner.Valid() {
			return nil, &api.Error{Code: "identity_unavailable", Message: "DSM user UID could not be resolved"}
		}
		return h(context.WithValue(ctx, backgroundAccessKey{}, access), c, r, params)
	}
}
func (b backgroundAPI) purge(ctx context.Context, c api.Caller, _ *http.Request, params json.RawMessage) (any, error) {
	var in struct {
		Owner *background.Submitter `json:"owner"`
	}
	if err := decodeParams(params, &in); err != nil || in.Owner == nil {
		return nil, api.BadRequest("exact owner is required")
	}
	audit, err := b.store.PurgeOwner(ctx, *in.Owner, c.User())
	if err != nil {
		return nil, backgroundError(err)
	}
	return audit, nil
}

// BackgroundInbox is the private input directory a trusted App producer fills
// for an authenticated owner. Both account name and UID are part of its key.
func BackgroundInbox(dataDir string, owner background.Submitter) (string, error) {
	if !owner.Valid() {
		return "", api.BadRequest("exact owner is required")
	}
	raw, _ := json.Marshal([2]string{owner.User, owner.UID})
	key := sha256.Sum256(raw)
	return filepath.Join(dataDir, "inbox", hex.EncodeToString(key[:])), nil
}
