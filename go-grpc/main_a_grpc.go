package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

// --------------------
// gRPC JSON codec (so you don't need protoc)
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
// "Proto" message types (plain structs)
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
// Manual service definition (similar to generated pb.go)
// --------------------

const echoServiceName = "echo.EchoService"

type EchoServiceServer interface {
	Echo(context.Context, *EchoRequest) (*EchoResponse, error)
	Health(context.Context, *HealthRequest) (*HealthResponse, error)
}

func RegisterEchoServiceServer(s *grpc.Server, srv EchoServiceServer) {
	s.RegisterService(&EchoService_ServiceDesc, srv)
}

func _EchoService_Echo_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(EchoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	baseHandler := func(ctx context.Context, req any) (any, error) {
		return srv.(EchoServiceServer).Echo(ctx, req.(*EchoRequest))
	}
	if interceptor == nil {
		return baseHandler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/" + echoServiceName + "/Echo",
	}
	return interceptor(ctx, in, info, baseHandler)
}

func _EchoService_Health_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(HealthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	baseHandler := func(ctx context.Context, req any) (any, error) {
		return srv.(EchoServiceServer).Health(ctx, req.(*HealthRequest))
	}
	if interceptor == nil {
		return baseHandler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/" + echoServiceName + "/Health",
	}
	return interceptor(ctx, in, info, baseHandler)
}

var EchoService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: echoServiceName,
	HandlerType: (*EchoServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Echo", Handler: _EchoService_Echo_Handler},
		{MethodName: "Health", Handler: _EchoService_Health_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "echo.proto",
}

// --------------------
// Service A implementation
// --------------------

type serviceA struct{}

func (serviceA) Health(ctx context.Context, _ *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok"}, nil
}

func (serviceA) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	// Keep original behavior: echo back msg
	return &EchoResponse{Echo: req.Msg}, nil
}

// Basic logging per request: service name, endpoint, status, latency
func loggingUnaryInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)
		log.Printf("service=%s endpoint=%s status=%s latency_ms=%d", serviceName, info.FullMethod, code.String(), time.Since(start).Milliseconds())
		return resp, err
	}
}

func main() {
	var listen string
	flag.StringVar(&listen, "listen", ":50051", "gRPC listen address for service A")
	flag.Parse()

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalf("service=A failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingUnaryInterceptor("A")),
	)

	RegisterEchoServiceServer(s, serviceA{})

	log.Printf("service=A gRPC listening on %s", listen)
	log.Fatal(s.Serve(lis))
}
