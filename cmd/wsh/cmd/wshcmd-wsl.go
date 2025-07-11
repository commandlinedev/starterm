// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshclient"
	"github.com/spf13/cobra"
)

var distroName string

var wslCmd = &cobra.Command{
	Use:     "wsl [-d <distribution-name>]",
	Short:   "connect this terminal to a local wsl connection",
	Args:    cobra.NoArgs,
	RunE:    wslRun,
	PreRunE: preRunSetupRpcClient,
}

func init() {
	wslCmd.Flags().StringVarP(&distroName, "distribution", "d", "", "Run the specified distribution")
	rootCmd.AddCommand(wslCmd)
}

func wslRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("wsl", rtnErr == nil)
	}()

	var err error
	if distroName == "" {
		// get default distro from the host
		distroName, err = wshclient.WslDefaultDistroCommand(RpcClient, nil)
		if err != nil {
			return err
		}
	}
	if !strings.HasPrefix(distroName, "wsl://") {
		distroName = "wsl://" + distroName
	}
	blockId := RpcContext.BlockId
	if blockId == "" {
		return fmt.Errorf("cannot determine blockid (not in JWT)")
	}
	data := wshrpc.CommandSetMetaData{
		ORef: starobj.MakeORef(starobj.OType_Block, blockId),
		Meta: map[string]any{
			starobj.MetaKey_Connection: distroName,
		},
	}
	err = wshclient.SetMetaCommand(RpcClient, data, nil)
	if err != nil {
		return fmt.Errorf("setting connection in block: %w", err)
	}
	WriteStderr("switched connection to %q\n", distroName)
	return nil
}
