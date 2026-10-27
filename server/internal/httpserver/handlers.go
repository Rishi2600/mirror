package httpserver

import (
	"bytes"
	"context"
	"time"

	"mirror/server/api"
)

// handlers implements the strict server interface generated from the contract.
type handlers struct {
	version string
	now     func() time.Time
}

var _ api.StrictServerInterface = (*handlers)(nil)

func (h *handlers) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{
		Status:  api.Ok,
		Version: h.version,
		Time:    h.now().UTC().Truncate(time.Second),
	}, nil
}

func (h *handlers) GetOpenAPISpec(ctx context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	return api.GetOpenAPISpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(api.Spec),
		ContentLength: int64(len(api.Spec)),
	}, nil
}
