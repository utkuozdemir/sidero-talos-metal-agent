// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bmc provides BMC management utilities.
package bmc

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"net"
	"strings"

	"go.uber.org/zap"

	"github.com/siderolabs/talos-metal-agent/internal/bmc/ipmi"
)

// GetBMCIPPort returns the BMC IP and port.
func GetBMCIPPort() (ip string, port uint16, err error) {
	ipmiClient, err := ipmi.NewClient()
	if err != nil {
		return "", 0, err
	}

	defer ipmiClient.Close() //nolint:errcheck

	// Fetch BMC IP (param 3 in LAN config)
	ipResp, err := ipmiClient.GetLANConfig(0x03)
	if err != nil {
		return "", 0, err
	}

	// Fetch BMC Port (param 8 in LAN config)
	portResp, err := ipmiClient.GetLANConfig(0x08)
	if err != nil {
		return "", 0, err
	}

	ip = net.IP(ipResp.Data).String()
	port = binary.LittleEndian.Uint16(portResp.Data)

	return ip, port, nil
}

// AttemptBMCUserSetup attempts to setup a BMC user with the given username.
func AttemptBMCUserSetup(username string, logger *zap.Logger) (password string, err error) {
	ipmiClient, err := ipmi.NewClient()
	if err != nil {
		return "", err
	}

	defer ipmiClient.Close() //nolint:errcheck

	// Get user summary to see how many user slots
	summResp, err := ipmiClient.GetUserSummary()
	if err != nil {
		return "", err
	}

	maxUsers := summResp.MaxUsers & 0x1F // Only bits [0:5] provide this number

	// Check if sidero user already exists by combing through all userIDs
	// nb: we start looking at user id 2, because 1 should always be an unamed admin user and
	//     we don't want to confuse that unnamed admin with an open slot we can take over.
	exists := false
	userID := uint8(0)

	for i := uint8(2); i <= maxUsers; i++ {
		userRes, userErr := ipmiClient.GetUserName(i)
		if userErr != nil {
			// nb: A failure here actually seems to mean that the user slot is unused,
			// even though you can also have a slot with empty user as well. *scratches head*
			// We'll take note of this spot if we haven't already found another empty one.
			if userID == 0 {
				userID = i
			}

			continue
		}

		// Found pre-existing sidero user
		if userRes.Username == username {
			exists = true
			userID = i

			logger.Info("user already present in slot, we'll claim it as our own", zap.Uint8("slot", i))

			break
		} else if (userRes.Username == "" || strings.TrimSpace(userRes.Username) == "(Empty User)") && userID == 0 {
			// If this is the first empty user that's not the UID 1 (which we skip),
			// we'll take this spot for our user
			logger.Info("found empty user slot, noting as a possible place for user", zap.Uint8("slot", i))

			userID = i
		}
	}

	// User didn't pre-exist and there's no room
	// Return without sidero user :(
	if userID == 0 {
		return "", errors.New("no slot available for user")
	}

	// Not already present and there's an empty slot so we add sidero user
	if !exists {
		logger.Info("adding user to slot", zap.Uint8("slot", userID))

		if _, err = ipmiClient.SetUserName(userID, username); err != nil { //nolint:errcheck
			return "", err
		}
	}

	// Reset pass for sidero user
	// nb: we _always_ reset the user pass because we can't ever get
	//     it back out when we find an existing sidero user.
	pass, err := genPass16()
	if err != nil {
		return "", err
	}

	if _, err = ipmiClient.SetUserPass(userID, pass); err != nil { //nolint:errcheck
		return "", err
	}

	// Make the user an admin
	// Options: 0x91 == Callin true, Link false, IPMI Msg true, Channel 1
	// Limits: 0x03 == Administrator
	// Session: 0x00 No session limit
	if _, err = ipmiClient.SetUserAccess(0x91, userID, 0x04, 0x00); err != nil { //nolint:errcheck
		return "", err
	}

	// Enable the user
	if _, err = ipmiClient.EnableUser(userID); err != nil { //nolint:errcheck
		return "", err
	}

	return pass, nil
}

// Returns a random pass string of len 16.
func genPass16() (string, error) {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, 16)
	for i := range b {
		rando, err := rand.Int(
			rand.Reader,
			big.NewInt(int64(len(letterRunes))),
		)
		if err != nil {
			return "", err
		}

		b[i] = letterRunes[rando.Int64()]
	}

	return string(b), nil
}
