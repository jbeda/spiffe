package main

import (
	"flag"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	gw "github.com/spiffe/spiffe/workload/workloadpb"
	"google.golang.org/grpc/metadata"
)

var (
	echoGRPCEndpoint = flag.String("echo_grpc_endpoint", "localhost:9090", "endpoint of YourService")
	echoHTTPEndpoint = flag.String("echo_http_endpoint", "localhost:8080", "endpoint of YourService")
	debugMode        = flag.Bool("debug", false, "endpoint of YourService")
)

// server is used to implement gw.EchoServer
type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) Echo(ctx context.Context, message *gw.StringMessage) (*gw.StringMessage, error) {
	md, ok := metadata.FromContext(ctx)
	if !ok {
		return nil, trail.Send(ctx, trace.AccessDenied("missing authorization"))
	}
	_, ok = md["authorization"]
	if !ok {
		return nil, trail.Send(ctx, trace.AccessDenied("missing authorization"))
	}
	return message, nil
}

func run() error {
	if *debugMode {
		trace.SetDebug(true)
	}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := gw.RegisterYourServiceHandlerFromEndpoint(ctx, mux, *echoGRPCEndpoint, opts)
	if err != nil {
		return err
	}

	doneC := make(chan error, 2)
	lis, err := net.Listen("tcp", *echoGRPCEndpoint)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	gw.RegisterYourServiceServer(s, &server{})
	go func() {
		doneC <- trace.Wrap(s.Serve(lis))
	}()

	go func() {
		doneC <- trace.Wrap(http.ListenAndServe(*echoHTTPEndpoint, mux))
	}()

	var errors []error
	for i := 0; i < 2; i++ {
		err := <-doneC
		errors = append(errors, err)
	}

	return trace.NewAggregate(errors...)
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}
