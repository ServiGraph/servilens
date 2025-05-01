package tracer

import (
	"context"
	"encoding/hex"
	"log"
	"strings"

	"github.com/ServiGraph/servilens/api/tracer"
	"go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcePb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracePb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type TraceCollectorService struct {
	v1.UnimplementedTraceServiceServer
	tracer.UnimplementedTracerServiceServer
	traceDb []*tracePb.ResourceSpans
	db      map[int64]*Trace
}

func NewTraceCollectorService() *TraceCollectorService {
	return &TraceCollectorService{
		db: make(map[int64]*Trace),
	}
}

type Trace struct {
	Nodes []*tracer.Node
	Spans []*tracer.Span
}

func getServiceName(r *resourcePb.Resource) string {
	for _, kv := range r.GetAttributes() {
		if kv.GetKey() == "service.name" {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func collectServiceNames(req *v1.ExportTraceServiceRequest) map[string]struct{} {
	seen := make(map[string]struct{})
	for _, rs := range req.ResourceSpans {
		if n := getServiceName(rs.Resource); n != "" {
			seen[n] = struct{}{}
		}
	}
	return seen
}

func remoteService(span *tracePb.Span, known map[string]struct{}) string {
	var peerSvc, rpcSvc string

	for _, kv := range span.Attributes {
		if kv == nil || kv.Value == nil {
			continue
		}
		switch kv.Key {
		case "peer.service":
			peerSvc = kv.Value.GetStringValue()
		case "rpc.service":
			rpcSvc = kv.Value.GetStringValue()
		}
	}

	// 1. explicit peer.service wins if we’ve seen it in a resource block
	if _, ok := known[peerSvc]; ok {
		return peerSvc
	}

	// 2. try to match the package-prefix of rpc.service
	//    "order.OrderService" -> "order", "foo.bar.Baz" -> "foo"
	if rpcSvc != "" {
		prefix := strings.Split(rpcSvc, ".")[0]
		if _, ok := known[prefix]; ok {
			return prefix
		}
	}

	return ""
}

func (t *TraceCollectorService) Export(ctx context.Context, req *v1.ExportTraceServiceRequest) (*v1.ExportTraceServiceResponse, error) {
	t.traceDb = append(t.traceDb, req.GetResourceSpans()...)
	log.Println("Received trace data")
	return &v1.ExportTraceServiceResponse{}, nil
}

func (t *TraceCollectorService) FetchTraceData(ctx context.Context, req *tracer.FetchTraceDataRequest) (*tracer.FetchTraceDataResponse, error) {
	var nodes []*tracer.Node
	var traces []*tracer.Span
	type edge struct{ id, src, dst string }
	traceEdges := map[string]map[edge]struct{}{} // inner map ensures dedup
	traceSpans := map[string][]*tracePb.Span{}   // TraceId -> []*Span
	spanSvc := map[string]string{}               // SpanId -> service.name
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
