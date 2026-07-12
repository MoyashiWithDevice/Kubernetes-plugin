package aggregator

import (
	"context"
	"fmt"
	"net"
	"os"

	detectivev1 "Kubernetes-plugin/api/detective/v1"

	"google.golang.org/grpc"
)

type Server struct {
	detectivev1.UnimplementedAgentServiceServer
	detectivev1.UnimplementedDetectiveServiceServer
	store *Store
}

func NewServer(store *Store) *Server {
	return &Server{store: store}
}

func (s *Server) SendSnapshot(ctx context.Context, req *detectivev1.SnapshotRequest) (*detectivev1.SnapshotResponse, error) {
	snap := req.GetSnapshot()
	if snap == nil {
		return &detectivev1.SnapshotResponse{Ok: false}, nil
	}
	s.store.Update(snap.NodeName, snap)
	return &detectivev1.SnapshotResponse{Ok: true}, nil
}

func (s *Server) GetFlows(ctx context.Context, req *detectivev1.Empty) (*detectivev1.FlowList, error) {
	return s.store.GetFlows(), nil
}

func (s *Server) GetTop(ctx context.Context, req *detectivev1.Empty) (*detectivev1.TopTalkerList, error) {
	return s.store.GetTop(), nil
}

func (s *Server) GetRetrans(ctx context.Context, req *detectivev1.Empty) (*detectivev1.RetransList, error) {
	return s.store.GetRetrans(), nil
}

func (s *Server) GetLatency(ctx context.Context, req *detectivev1.Empty) (*detectivev1.LatencyList, error) {
	return s.store.GetLatency(), nil
}

func (s *Server) GetDNS(ctx context.Context, req *detectivev1.Empty) (*detectivev1.DNSList, error) {
	return s.store.GetDNS(), nil
}

func (s *Server) Serve(ctx context.Context, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	gs := grpc.NewServer()
	detectivev1.RegisterAgentServiceServer(gs, s)
	detectivev1.RegisterDetectiveServiceServer(gs, s)

	fmt.Fprintf(os.Stderr, "aggregator: listening on %s\n", addr)

	go func() {
		<-ctx.Done()
		gs.GracefulStop()
	}()

	return gs.Serve(lis)
}
