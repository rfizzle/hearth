package orchestrate

import (
	"context"

	"github.com/rfizzle/hearth/internal/project"
)

// Status is the status of the project id, derived on every read. It is the
// one place the counts are read, for the events and for the service that
// lists projects.
func (s *Service) Status(ctx context.Context, id string) (project.Status, error) {
	return s.status(ctx, id)
}
