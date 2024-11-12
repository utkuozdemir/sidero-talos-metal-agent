// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package service provides the agent GRPC service server.
package service

import (
	"context"
	"fmt"
	"io"

	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentpb "github.com/siderolabs/talos-metal-agent/api/agent"
)

// IPMIClientFactory is the factory to create IPMI clients.
type IPMIClientFactory func() (IPMIClient, error)

// IPMIClient represents an IPMI client.
type IPMIClient interface {
	io.Closer

	// UserExists checks if the user exists.
	UserExists(username string) (bool, error)

	// AttemptUserSetup attempts to set up the BMC user.
	AttemptUserSetup(username, password string, logger *zap.Logger) error

	// GetIPPort returns the BMC IP and port.
	GetIPPort() (string, uint16, error)
}

// TalosClient represents a Talos API client.
type TalosClient interface {
	Reboot(ctx context.Context, opts ...talosclient.RebootMode) error
}

// Server is the agent service server.
type Server struct {
	agentpb.UnimplementedAgentServiceServer

	talosClient       TalosClient
	ipmiClientFactory IPMIClientFactory

	logger *zap.Logger

	testMode bool
}

// NewServer creates a new service server.
func NewServer(talosClient TalosClient, ipmiClientFactory IPMIClientFactory, testMode bool, logger *zap.Logger) *Server {
	return &Server{
		talosClient:       talosClient,
		ipmiClientFactory: ipmiClientFactory,
		logger:            logger,
		testMode:          testMode,
	}
}

// Hello is an endpoint to check if the service is available.
func (s *Server) Hello(_ context.Context, _ *agentpb.HelloRequest) (*agentpb.HelloResponse, error) {
	s.logger.Debug("hello", zap.Bool("test_mode", s.testMode))

	return &agentpb.HelloResponse{}, nil
}

// GetPowerManagement returns the power management info.
func (s *Server) GetPowerManagement(_ context.Context, req *agentpb.GetPowerManagementRequest) (*agentpb.GetPowerManagementResponse, error) {
	s.logger.Debug("get power management", zap.Bool("test_mode", s.testMode))

	if s.testMode {
		return &agentpb.GetPowerManagementResponse{
			Api: &agentpb.GetPowerManagementResponse_API{},
		}, nil
	}

	ipmiClient, err := s.ipmiClientFactory()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error creating ipmi client: %v", err)
	}

	defer ipmiClient.Close() //nolint:errcheck

	ip, port, err := ipmiClient.GetIPPort()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting bmc ip port: %v", err)
	}

	checkUsername := req.GetIpmi().GetCheckUsername()

	exists, err := ipmiClient.UserExists(checkUsername)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error checking if user %q exists: %v", checkUsername, err)
	}

	return &agentpb.GetPowerManagementResponse{
		Ipmi: &agentpb.GetPowerManagementResponse_IPMI{
			Address:    ip,
			Port:       uint32(port),
			UserExists: exists,
		},
	}, nil
}

// SetPowerManagement sets the power management info.
func (s *Server) SetPowerManagement(_ context.Context, req *agentpb.SetPowerManagementRequest) (*agentpb.SetPowerManagementResponse, error) {
	s.logger.Debug("set power management", zap.Bool("test_mode", s.testMode), zap.String("ipmi_username", req.GetIpmi().GetUsername()))

	if s.testMode {
		return &agentpb.SetPowerManagementResponse{}, nil
	}

	ipmiClient, err := s.ipmiClientFactory()
	if err != nil {
		return nil, fmt.Errorf("error creating ipmi client: %w", err)
	}

	defer ipmiClient.Close() //nolint:errcheck

	if err = ipmiClient.AttemptUserSetup(req.GetIpmi().GetUsername(), req.GetIpmi().GetPassword(), s.logger); err != nil {
		return nil, fmt.Errorf("failed to set up IPMI user: %w", err)
	}

	return &agentpb.SetPowerManagementResponse{}, nil
}

// Reboot reboots the machine.
func (s *Server) Reboot(ctx context.Context, _ *agentpb.RebootRequest) (*agentpb.RebootResponse, error) {
	s.logger.Info("reboot requested")

	if err := s.talosClient.Reboot(ctx, talosclient.WithPowerCycle); err != nil {
		return nil, err
	}

	return &agentpb.RebootResponse{}, nil
}

// WipeDisks wipes the disks.
//
// todo: eventually implement (probably in machined)
func (s *Server) WipeDisks(context.Context, *agentpb.WipeDisksRequest) (*agentpb.WipeDisksResponse, error) {
	s.logger.Info("wipe disks requested")

	return nil, status.Errorf(codes.Unimplemented, "method WipeDisks not implemented")
}
