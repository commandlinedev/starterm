// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/commandlinedev/starterm/pkg/sconfig"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshclient"
	"github.com/spf13/cobra"
)

var (
	identityFiles []string
	newBlock      bool
)

var sshCmd = &cobra.Command{
	Use:     "ssh",
	Short:   "connect this terminal to a remote host",
	Args:    cobra.ExactArgs(1),
	RunE:    sshRun,
	PreRunE: preRunSetupRpcClient,
}

func init() {
	sshCmd.Flags().StringArrayVarP(&identityFiles, "identityfile", "i", []string{}, "add an identity file for publickey authentication")
	sshCmd.Flags().BoolVarP(&newBlock, "new", "n", false, "create a new terminal block with this connection")
	rootCmd.AddCommand(sshCmd)
}

func sshRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("ssh", rtnErr == nil)
	}()

	sshArg := args[0]
	blockId := RpcContext.BlockId
	if blockId == "" && !newBlock {
		return fmt.Errorf("cannot determine blockid (not in JWT)")
	}

	// Create connection request
	connOpts := wshrpc.ConnRequest{
		Host:       sshArg,
		LogBlockId: blockId,
		Keywords: sconfig.ConnKeywords{
			SshIdentityFile: identityFiles,
		},
	}
	wshclient.ConnConnectCommand(RpcClient, connOpts, &wshrpc.RpcOpts{Timeout: 60000})

	if newBlock {
		// Create a new block with the SSH connection
		createMeta := map[string]any{
			starobj.MetaKey_View:       "term",
			starobj.MetaKey_Controller: "shell",
			starobj.MetaKey_Connection: sshArg,
		}
		if RpcContext.Conn != "" {
			createMeta[starobj.MetaKey_Connection] = RpcContext.Conn
		}
		createBlockData := wshrpc.CommandCreateBlockData{
			BlockDef: &starobj.BlockDef{
				Meta: createMeta,
			},
		}
		oref, err := wshclient.CreateBlockCommand(RpcClient, createBlockData, nil)
		if err != nil {
			return fmt.Errorf("creating new terminal block: %w", err)
		}
		WriteStdout("new terminal block created with connection to %q: %s\n", sshArg, oref)
		return nil
	}

	// Update existing block with the new connection
	data := wshrpc.CommandSetMetaData{
		ORef: starobj.MakeORef(starobj.OType_Block, blockId),
		Meta: map[string]any{
			starobj.MetaKey_Connection: sshArg,
		},
	}
	err := wshclient.SetMetaCommand(RpcClient, data, nil)
	if err != nil {
		return fmt.Errorf("setting connection in block: %w", err)
	}
	WriteStderr("switched connection to %q\n", sshArg)
	return nil
}
