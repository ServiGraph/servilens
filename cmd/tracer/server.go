package main

import (
	"context"
	"log"
	"net"
	"net/http"

	tracerPb "github.com/ServiGraph/servilens/api/tracer"
	"github.com/ServiGraph/servilens/tracer"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

func allowCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Adjust this based on frontend origin
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func main() {
	lis, err := net.Listen("tcp", "0.0.0.0:5678") // OTLP default gRPC port
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := tracer.NewTraceCollectorService()
	grpcServer := grpc.NewServer()
	v1.RegisterTraceServiceServer(grpcServer, srv)
	tracerPb.RegisterTracerServiceServer(grpcServer, srv)

	gwmux := runtime.NewServeMux()
	if err = tracerPb.RegisterTracerServiceHandlerServer(context.Background(), gwmux, srv); err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}
	corsWrappedMux := allowCORS(gwmux)
	gwServer := &http.Server{
		Addr:    "0.0.0.0:8090",
		Handler: corsWrappedMux,
	}

	log.Println("OTLP trace consumer running on :5678...")
	go func() {
		log.Fatalln(grpcServer.Serve(lis))
	}()
	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())
}
