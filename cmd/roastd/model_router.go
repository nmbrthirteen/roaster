package main

import (
	"context"
	"fmt"

	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// writable is a pack's audit that a stand can point at a model of its choice.
type writable interface {
	roast.Provider
	WithWriter(verdict.Writer) roast.Provider
}

// modelRouter selects an allowlisted writer after terminal authentication.
// Model keys remain on the service, and a stand can only select known models.
type modelRouter struct {
	audit    writable
	writers  map[string]verdict.Writer
	fallback string
}

func (r modelRouter) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	name := req.ModelProvider
	if name == "" {
		name = r.fallback
	}
	writer, ok := r.writers[name]
	if !ok && name != "" {
		return roast.Roast{}, fmt.Errorf("roast model %q is not configured on this service", name)
	}
	return r.audit.WithWriter(writer).Roast(ctx, req, emit)
}

// Search needs no model, so it goes straight to the audit.
func (r modelRouter) Search(ctx context.Context, s roast.Search) ([]roast.Candidate, error) {
	searcher, ok := r.audit.(roast.Searcher)
	if !ok {
		return nil, fmt.Errorf("%s accounts are found by handle, not by name", roast.PackFor(s.Pack).Name)
	}
	return searcher.Search(ctx, s)
}
