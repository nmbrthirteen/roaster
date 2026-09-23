package main

import (
	"context"
	"fmt"

	"github.com/upgaming/roaster/internal/audit"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// modelRouter selects an allowlisted writer after terminal authentication.
// Model keys remain on the service, and a stand can only select known models.
type modelRouter struct {
	audit    audit.Audit
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

	provider := r.audit
	provider.Writer = writer
	return provider.Roast(ctx, req, emit)
}
