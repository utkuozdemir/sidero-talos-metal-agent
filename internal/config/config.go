// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config contains the configuration for the agent.
package config

import (
	"strconv"

	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos-metal-agent/pkg/config"
)

// Config contains the configuration for the agent.
type Config struct {
	ProviderAddress string
	TestMode        bool
}

// LoadFromKernelCmdline loads the Config from the kernel arguments.
func LoadFromKernelCmdline(logger *zap.Logger) Config {
	var providerAddress string

	cmdline := procfs.ProcCmdline()

	providerAddressParam := cmdline.Get(config.MetalProviderAddressKernelArg)
	if providerAddressParam != nil {
		providerAddressVal := providerAddressParam.First()
		if providerAddressVal != nil {
			providerAddress = *providerAddressVal
		}
	}

	var testMode bool

	testModeParam := cmdline.Get(config.TestModeKernelArg)
	if testModeParam != nil {
		testModeVal := testModeParam.First()
		if testModeVal != nil {
			var err error

			testMode, err = strconv.ParseBool(*testModeVal)
			if err != nil {
				logger.Error("failed to parse test mode", zap.String("key", config.TestModeKernelArg), zap.String("value", *testModeVal), zap.Error(err))
			}
		}
	}

	return Config{
		ProviderAddress: providerAddress,
		TestMode:        testMode,
	}
}
