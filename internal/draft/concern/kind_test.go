package concern_test

import (
	"reflect"
	"testing"

	"github.com/rfizzle/hearth/internal/draft/concern"
)

// A model concern on a field the kind's sheet has no row for is dropped.
func TestCap_OnlyTheKindsFields(t *testing.T) {
	all := []concern.Concern{
		c("goal"), c("role"), c("working_style"), c("job_description"), c("name"), c("done_when"),
	}
	for _, tc := range []struct {
		name string
		kind concern.Kind
		want []string
	}{
		{"a project", concern.ForProject, []string{"goal", "name", "done_when"}},
		{"a hire", concern.ForBot, []string{"role", "job_description", "name"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fields(concern.Cap(tc.kind, nil, all)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Cap fields = %v, want %v", got, tc.want)
			}
		})
	}
}
