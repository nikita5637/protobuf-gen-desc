package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/flowchartsman/swaggerui"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	examplepb "github.com/nikita5637/protobuf-gen-desc/examples/example"
	"github.com/posener/ctxutil"
	"github.com/rs/cors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	gatewayAddr = ":8080"
	grpcAddr    = ":8082"
	swaggerAddr = ":8084"
)

type ServiceDescription interface {
	RegisterGateway(ctx context.Context, mux *runtime.ServeMux) error
	RegisterGRPC(server *grpc.Server)
}

func main() {
	ctx := ctxutil.Interrupt()

	service1implementation := examplepb.UnimplementedExampleService1Server{}
	service2implementation := examplepb.UnimplementedExampleService2Server{}

	service1desc := examplepb.NewExampleService1Description(service1implementation)
	service2desc := examplepb.NewExampleService2Description(service2implementation)

	swaggerDef := examplepb.SwaggerDef()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodConnect,
			http.MethodOptions,
			http.MethodTrace,
		},
	})

	gatewayServer := runtime.NewServeMux()
	swaggerServer := swaggerui.Handler(swaggerDef)
	grpcServer := grpc.NewServer()

	reflection.Register(grpcServer)

	// registration of gateway and GRPC
	for _, serviceDecription := range []ServiceDescription{service1desc, service2desc} {
		err := serviceDecription.RegisterGateway(ctx, gatewayServer)
		if err != nil {
			panic(err)
		}

		serviceDecription.RegisterGRPC(grpcServer)
	}

	log.Println("starting servers")

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return runGateway(ctx, c.Handler(gatewayServer))
	})
	eg.Go(func() error {
		return runGRPC(ctx, grpcServer)
	})
	eg.Go(func() error {
		return runSwagger(ctx, swaggerServer)
	})
	if err := eg.Wait(); err != nil {
		panic(err)
	}
}

func runGateway(ctx context.Context, mux http.Handler) error {
	errCh := make(chan error)
	go func(ctx context.Context) {
		if err := http.ListenAndServe(gatewayAddr, mux); err != nil {
			errCh <- err
		}
	}(ctx)

	select {
	case <-ctx.Done():
		break
	case err := <-errCh:
		return err
	}

	log.Println("gracefull stop gateway")
	return nil
}

func runGRPC(ctx context.Context, server *grpc.Server) error {
	errCh := make(chan error)

	lis, err := net.Listen("tcp4", grpcAddr)
	if err != nil {
		return err
	}

	go func(ctx context.Context) {
		if err := server.Serve(lis); err != nil {
			errCh <- err
		}
	}(ctx)

	select {
	case <-ctx.Done():
		break
	case err := <-errCh:
		return err
	}

	log.Println("gracefull stop GRPC server")
	server.GracefulStop()
	return nil
}

func runSwagger(ctx context.Context, server http.Handler) error {
	errCh := make(chan error)

	go func(ctx context.Context) {
		if err := http.ListenAndServe(swaggerAddr, server); err != nil {
			errCh <- err
		}
	}(ctx)

	select {
	case <-ctx.Done():
		break
	case err := <-errCh:
		return err
	}

	log.Println("gracefull stop swagger")
	return nil
}
