// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main contains the entrypoint for the Talos metal agent.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/siderolabs/talos-metal-agent/internal/agent"
	"github.com/siderolabs/talos-metal-agent/internal/config"
	agentlog "github.com/siderolabs/talos-metal-agent/internal/log"
	"github.com/siderolabs/talos-metal-agent/internal/version"
)

const logToKmsgFlag = "log-to-kmsg"

var rootCmdArgs struct {
	providerAddress string
	debug           bool
	logToKMSG       bool
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     version.Name,
	Short:   "Run the Talos metal agent",
	Version: version.Tag,
	Args:    cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true // if the args are parsed fine, no need to show usage
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		conf := config.LoadFromKernelCmdline()

		if rootCmdArgs.providerAddress != "" {
			conf.ProviderAddress = rootCmdArgs.providerAddress
		}

		if cmd.Flags().Changed(logToKmsgFlag) {
			conf.LogToKmsg = rootCmdArgs.logToKMSG
		}

		logger, err := agentlog.InitLogger(rootCmdArgs.debug, conf.LogToKmsg)
		if err != nil {
			return fmt.Errorf("failed to create logger: %w", err)
		}

		defer logger.Sync() //nolint:errcheck

		return run(cmd.Context(), conf.ProviderAddress, logger)
	},
}

func run(ctx context.Context, providerAddress string, logger *zap.Logger) error {
	ag, err := agent.New(providerAddress, logger)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if err = ag.Run(ctx); err != nil {
		return fmt.Errorf("failed to run agent: %w", err)
	}

	return nil
}

func main() {
	if err := runCmd(); err != nil {
		log.Fatalf("failed to run: %v", err)
	}
}

func runCmd() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.Flags().StringVar(&rootCmdArgs.providerAddress, "provider-address", "", fmt.Sprintf(
		"The infra provider address to connect to. If not specified explicitly, the value of the kernel arg %q will be used.", config.MetalProviderAddressKernelArg))
	rootCmd.Flags().BoolVar(&rootCmdArgs.debug, "debug", false, "Enable debug mode & logs.")
	rootCmd.Flags().BoolVar(&rootCmdArgs.logToKMSG, logToKmsgFlag, false,
		fmt.Sprintf("Send logs also to the kernel agentlog buffer.  If not specified explicitly, the value of the kernel arg %q will be used.", config.MetalProviderLogToKMSGKernelArg))
}
