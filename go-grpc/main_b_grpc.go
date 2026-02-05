package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

// --------------------
// gRPC JSON codec (must match service A)
// --------------------

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

// --------------------
// Message types (same as service A)
// --------------------

type EchoRequest struct {
	Msg string `json:"msg"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

type HealthRequest struct{}

type HealthResponse struct {
	Status string `json:"status"`
}

// --------------------
// Manual client stub (similar to generated pb.go)
// --------------------

const echoServiceName = "echo.EchoService"

type EchoServiceClient interface {
	Echo(ctx context.Context, in *EchoRequest, opts ...grpc.CallOption) (*EchoResponse, error)
	Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error)
}

type echoServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewEchoServiceClient(cc grpc.ClientConnInterface) EchoServiceClient {
	return &echoServiceClient{cc: cc}
}

func (c *echoServiceClient) Echo(ctx context.Context, in *EchoRequest, opts ...grpc.CallOption) (*EchoResponse, error) {
	out := new(EchoResponse)
	err := c.cc.Invoke(ctx, "/"+echoServiceName+"/Echo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *echoServiceClient) Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, "/"+echoServiceName+"/Health", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --------------------
// HTTP logging (service B)
// --------------------

type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func httpLoggingMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		overall := "ok"
		if sw.status >= 400 {
			overall = "error"
		}

		log.Printf("service=%s endpoint=%s status=%s http_status=%d latency_ms=%d",
			serviceName, r.URL.Path, overall, sw.status, time.Since(start).Milliseconds())
	})
}

func main() {
	var (
		httpListen      string
		serviceAAddr    string
		upstreamTimeout time.Duration
	)

	flag.StringVar(&httpListen, "listen", ":8081", "HTTP listen address for service B")
	flag.StringVar(&serviceAAddr, "service-a", "127.0.0.1:50051", "service A gRPC address")
	flag.DurationVar(&upstreamTimeout, "timeout", 1*time.Second, "timeout for calls from B -> A")
	flag.Parse()

	// Dial service A (non-blocking: B starts even if A is down).
	conn, err := grpc.Dial(
		serviceAAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)
	if err != nil {
		log.Fatalf("service=B failed to dial service A: %v", err)
	}
	defer conn.Close()

	echoClient := NewEchoServiceClient(conn)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/call-echo", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		msg := r.URL.Query().Get("msg")

		// Timeout handling in service B
		ctxUp, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
		defer cancel()

		resp, err := echoClient.Echo(ctxUp, &EchoRequest{Msg: msg})
		if err != nil {
			// Independent failure: if A is stopped, return 503 and log error
			log.Printf("service=B endpoint=/call-echo status=error error=%q latency_ms=%d",
				err.Error(), time.Since(start).Milliseconds())

			respBody := map[string]any{
				"service_b": "ok",
				"service_a": "unavailable",
				"error":     err.Error(),
				"message":   "failed to reach service A",
				"status":    http.StatusServiceUnavailable,
			}
			b, _ := json.MarshalIndent(respBody, "", "  ")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(b)
			return
		}

		respBody := map[string]any{
			"service_b": "ok",
			"service_a": map[string]any{"echo": resp.Echo},
		}
		b, _ := json.MarshalIndent(respBody, "", "  ")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})

	srv := &http.Server{
		Addr:              httpListen,
		Handler:           httpLoggingMiddleware("B", mux),
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Printf("service=B listening on %s (HTTP). Calling service A over gRPC at %s", httpListen, serviceAAddr)
	log.Fatal(srv.ListenAndServe())
}
