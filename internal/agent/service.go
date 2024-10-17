package agent

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentpb "github.com/siderolabs/talos-metal-agent/api/agent"
	"github.com/siderolabs/talos-metal-agent/internal/bmc"
)

const ipmiUsername = "talos-agent"

type serviceServer struct {
	agentpb.UnimplementedAgentServiceServer

	logger *zap.Logger
}

func (s *serviceServer) SetIPMICredentials(context.Context, *agentpb.SetIPMICredentialsRequest) (*agentpb.SetIPMICredentialsResponse, error) {
	s.logger.Debug("set ipmi credentials", zap.String("username", ipmiUsername))

	password, err := bmc.AttemptBMCUserSetup(ipmiUsername, s.logger)
	if err != nil {
		return nil, fmt.Errorf("error setting ipmi credentials: %w", err)
	}

	_ = password

	return &agentpb.SetIPMICredentialsResponse{Password: password}, nil
}

func (s *serviceServer) GetIPMIInfo(context.Context, *agentpb.GetIPMIInfoRequest) (*agentpb.GetIPMIInfoResponse, error) {
	ip, port, err := bmc.GetBMCIPPort()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting bmc ip port: %v", err)
	}

	return &agentpb.GetIPMIInfoResponse{Ip: ip, Port: uint32(port)}, nil
}
