// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package agent contains Talos metal agent mode functionality.
package agent

import (
	"context"
	"fmt"
	"github.com/jhump/grpctunnel"
	"github.com/jhump/grpctunnel/tunnelpb"
	agentpb "github.com/siderolabs/talos-metal-agent/api/agent"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Agent is the Talos agent.
type Agent struct {
	logger          *zap.Logger
	providerAddress string
}

// New creates a new agent.
func New(providerAddress string, logger *zap.Logger) (*Agent, error) {
	return &Agent{
		providerAddress: providerAddress,
		logger:          logger,
	}, nil
}

// Run starts the agent.
func (a *Agent) Run(ctx context.Context) error {
	a.logger.Info("running metal agent", zap.String("provider_address", a.providerAddress))

	ctx = buildAdminAuthzContext(ctx)

	talosClient, err := buildTalosClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to build Talos client: %w", err)
	}

	versionResponse, err := talosClient.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to read Talos version: %w", err)
	}

	a.logger.Info("connected to Talos", zap.String("version", versionResponse.Messages[0].String()))

	conn, err := grpc.NewClient(a.providerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create grpc client: %w", err)
	}

	tunnelStub := tunnelpb.NewTunnelServiceClient(conn)
	channelServer := grpctunnel.NewReverseTunnelServer(tunnelStub)

	agentpb.RegisterAgentServiceServer(channelServer, &serviceServer{
		logger: a.logger,
	})

	// Open the reverse tunnel and serve requests.
	if _, err = channelServer.Serve(ctx); err != nil {
		return fmt.Errorf("failed to serve over grpc tunnel: %w", err)
	}

	return nil
}

func buildAdminAuthzContext(ctx context.Context) context.Context {
	md := metadata.New(nil)

	authz.SetMetadata(md, talosrole.MakeSet(talosrole.Admin))

	return metadata.NewOutgoingContext(ctx, md)
}

func buildTalosClient(ctx context.Context) (*talosclient.Client, error) {
	opts := []talosclient.OptionFunc{
		talosclient.WithUnixSocket(talosconstants.MachineSocketPath),
		talosclient.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	client, err := talosclient.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to construct client: %w", err)
	}

	return client, nil
}
