// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config contains the configuration for the agent.
package config

import (
	"strconv"

	"github.com/siderolabs/go-procfs/procfs"
)

const (
	// MetalProviderAddressKernelArg is the kernel argument that contains the provider address.
	MetalProviderAddressKernelArg = "metal.provider.address"

	// MetalProviderLogToKMSGKernelArg is the kernel argument that instructs the agent to log to KMSG as well as its stderr.
	MetalProviderLogToKMSGKernelArg = "metal.provider.log_to_kmsg"
)

// Config contains the configuration for the agent.
type Config struct {
	ProviderAddress string
	LogToKmsg       bool
}

// LoadFromKernelCmdline loads the Config from the kernel arguments.
func LoadFromKernelCmdline() Config {
	var (
		providerAddress string
		logToKMSG       bool
	)

	cmdline := procfs.ProcCmdline()

	providerAddressParam := cmdline.Get(MetalProviderAddressKernelArg)
	if providerAddressParam != nil {
		providerAddressVal := providerAddressParam.First()
		if providerAddressVal != nil {
			providerAddress = *providerAddressVal
		}
	}

	logToKMSGParam := cmdline.Get(MetalProviderLogToKMSGKernelArg)
	if logToKMSGParam != nil {
		logToKMSGVal := logToKMSGParam.First()
		if logToKMSGVal != nil {
			logToKMSG, _ = strconv.ParseBool(*logToKMSGVal) //nolint:errcheck
		}
	}

	return Config{
		ProviderAddress: providerAddress,
		LogToKmsg:       logToKMSG,
	}
}
