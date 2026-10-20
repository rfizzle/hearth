package deliverables

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/measure"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// maxIDs is how many ids one ReadMeasures may ask for; the count is request
// input and each id reads the books.
const maxIDs = 100

// Deps is what the services here are handed.
type Deps struct {
	// Measures is where the figures are read from.
	Measures measure.Deps
	// Root is the Hearth folder now, or "" when the org has none.
	Root func() string
}

// The services.
type (
	Measures  struct{ Deps } // MeasureService: the office's figures by id
	Templates struct{ Deps } // DeliverableService: the templates of shaped deliverables
)

// Register serves both services on mux.
func Register(mux *http.ServeMux, d Deps) {
	mux.Handle(hearthv1connect.NewMeasureServiceHandler(Measures{d}))
	mux.Handle(hearthv1connect.NewDeliverableServiceHandler(Templates{d}))
}

// refused is a refusal in words as invalid_argument, and any other error as
// it is.
func refused(err error) error {
	if errors.Is(err, measure.ErrInvalid) || errors.Is(err, deliverable.ErrInvalid) {
		return wire.Invalid("%s", err.Error())
	}
	return wire.Fail(err)
}

// ListMeasures is every id that resolves now, with its label and no value.
// See docs/model.md#work
func (s Measures) ListMeasures(ctx context.Context, _ *connect.Request[hearthv1.ListMeasuresRequest]) (*connect.Response[hearthv1.ListMeasuresResponse], error) {
	list, err := s.Measures.List(ctx)
	if err != nil {
		return nil, refused(err)
	}
	return connect.NewResponse(&hearthv1.ListMeasuresResponse{Measures: wire.Each(list, valueToProto)}), nil
}

// ReadMeasures is one value per resolved id, in the order asked.
func (s Measures) ReadMeasures(ctx context.Context, req *connect.Request[hearthv1.ReadMeasuresRequest]) (*connect.Response[hearthv1.ReadMeasuresResponse], error) {
	in := req.Msg.GetIds()
	if len(in) > maxIDs {
		return nil, wire.Invalid("at most %d ids at a time; %d were asked", maxIDs, len(in))
	}
	ids := make([]measure.ID, 0, len(in))
	for _, raw := range in {
		id, err := measure.ParseID(raw)
		if err != nil {
			return nil, refused(err)
		}
		ids = append(ids, id)
	}
	values, err := s.Measures.Read(ctx, ids)
	if err != nil {
		return nil, refused(err)
	}
	return connect.NewResponse(&hearthv1.ReadMeasuresResponse{Values: wire.Each(values, valueToProto)}), nil
}

var kinds = map[measure.Kind]hearthv1.MeasureKind{
	measure.Number:   hearthv1.MeasureKind_MEASURE_KIND_NUMBER,
	measure.Progress: hearthv1.MeasureKind_MEASURE_KIND_PROGRESS,
	measure.Light:    hearthv1.MeasureKind_MEASURE_KIND_LIGHT,
}

var tones = map[measure.Tone]hearthv1.Tone{
	measure.ToneNeutral:   hearthv1.Tone_TONE_NEUTRAL,
	measure.TonePositive:  hearthv1.Tone_TONE_POSITIVE,
	measure.ToneAttention: hearthv1.Tone_TONE_ATTENTION,
	measure.ToneNegative:  hearthv1.Tone_TONE_NEGATIVE,
}

func valueToProto(v measure.Value) *hearthv1.MeasureValue {
	return &hearthv1.MeasureValue{
		Id: v.ID, Kind: kinds[v.Kind], Label: v.Label, Number: v.Number, Unit: v.Unit,
		Done: v.Done, Total: v.Total, Tone: tones[v.Tone], Word: v.Word,
		AsOf: wire.Timestamp(v.AsOf), Missing: v.Missing,
	}
}
