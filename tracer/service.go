package tracer

import (
	"context"
	"encoding/hex"
	"log"

	"github.com/ServiGraph/servilens/api/tracer"
	"go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcePb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracePb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// TraceCollectorService implements both the OpenTelemetry TraceServiceServer
// and a custom TracerServiceServer. It receives, stores, and processes trace data
// for later analysis and visualization.
type TraceCollectorService struct {
	v1.UnimplementedTraceServiceServer
	tracer.UnimplementedTracerServiceServer
	// TODO: Replace with a more efficient storage
	traceDb []*tracePb.ResourceSpans // Raw trace storage
	// TODO: Replace with a more efficient storage
	db map[int64]*Trace // Processed trace storage
}

// NewTraceCollectorService creates and initializes a new TraceCollectorService.
func NewTraceCollectorService() *TraceCollectorService {
	return &TraceCollectorService{
		db: make(map[int64]*Trace),
	}
}

// Trace represents a processed trace consisting of nodes and spans.
// Nodes represent services; Spans represent interactions.
type Trace struct {
	Nodes []*tracer.Node
	Spans []*tracer.Span
}

// getServiceName extracts the "service.name" attribute from a given OTLP Resource.
// Returns the service name as a string, or an empty string if not present.
func getServiceName(r *resourcePb.Resource) string {
	for _, kv := range r.GetAttributes() {
		if kv.GetKey() == "service.name" {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

// Export implements the OTLP TraceServiceServer interface.
// It receives trace data from clients and appends it to the internal traceDb.
// Always returns an empty ExportTraceServiceResponse.
func (t *TraceCollectorService) Export(_ context.Context, req *v1.ExportTraceServiceRequest) (*v1.ExportTraceServiceResponse, error) {
	t.traceDb = append(t.traceDb, req.GetResourceSpans()...)
	log.Println("Received trace data")
	return &v1.ExportTraceServiceResponse{}, nil
}

// FetchTraceData processes stored traces and constructs a service dependency graph.
// It returns a list of service nodes and edges (spans) representing cross-service calls.
// Only spans starting after the provided timestamp are considered.
func (t *TraceCollectorService) FetchTraceData(_ context.Context, req *tracer.FetchTraceDataRequest) (*tracer.FetchTraceDataResponse, error) {
	var nodes []*tracer.Node
	var traces []*tracer.Span
	type edge struct{ id, src, dst string }
	traceEdges := map[string]map[edge]struct{}{} // inner map ensures dedupe
	traceSpans := map[string][]*tracePb.Span{}   // TraceId -> []*Span
	spanSvc := map[string]string{}               // SpanId -> service.name
	// Build service nodes and group spans by trace
	for _, resource := range t.traceDb {
		svc := getServiceName(resource.Resource)
		nodes = append(nodes, &tracer.Node{
			Id: svc,
		})
		for _, ss := range resource.ScopeSpans {
			for _, sp := range ss.Spans {
				if sp.GetStartTimeUnixNano()/1000 < req.GetFromUnixTimestamp() {
					continue
				}
				id := hex.EncodeToString(sp.SpanId)
				tid := hex.EncodeToString(sp.TraceId)
				traceSpans[tid] = append(traceSpans[tid], sp)
				spanSvc[id] = svc
			}
		}
	}
	// Identify cross-service edges based on client spans and parent-child relationships
	for tid, spans := range traceSpans {
		if traceEdges[tid] == nil {
			traceEdges[tid] = map[edge]struct{}{}
		}
		for _, sp := range spans {
			if sp.Kind != tracePb.Span_SPAN_KIND_CLIENT {
				continue // only client spans initiate cross-service calls
			}
			sid := hex.EncodeToString(sp.SpanId)
			for _, child := range spans {
				if hex.EncodeToString(child.ParentSpanId) == sid {
					src, dst := spanSvc[sid], spanSvc[hex.EncodeToString(child.SpanId)]
					if src != "" && dst != "" && src != dst {
						traceEdges[tid][edge{hex.EncodeToString(child.GetSpanId()), src, dst}] = struct{}{}
					}
				}
			}
		}
	}
	log.Println(traceEdges)
	// Aggregate edge counts across all traces
	var edgeCount = make(map[edge]int64)
	for _, m := range traceEdges {
		for e := range m {
			uniqEdge := edge{"uid", e.src, e.dst}
			_, ok := edgeCount[uniqEdge]
			if !ok {
				edgeCount[uniqEdge] = 0
			}
			edgeCount[uniqEdge]++
		}
	}
	// Build response spans (edges) with aggregated counts
	for e, count := range edgeCount {
		traces = append(traces, &tracer.Span{
			Source: e.src,
			Target: e.dst,
			Data: &tracer.SpanData{
				TotalCount: count,
			},
		})
	}
	return &tracer.FetchTraceDataResponse{
		Nodes: nodes,
		Links: traces,
	}, nil
}
