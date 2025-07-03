// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshclient"
	"github.com/spf13/cobra"
)

var editConfigCmd = &cobra.Command{
	Use:     "editconfig [configfile]",
	Short:   "edit Star configuration files",
	Long:    "Edit Star configuration files. Defaults to settings.json if no file specified. Common files: settings.json, presets.json, widgets.json",
	Args:    cobra.MaximumNArgs(1),
	RunE:    editConfigRun,
	PreRunE: preRunSetupRpcClient,
}

func init() {
	rootCmd.AddCommand(editConfigCmd)
}

func editConfigRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("editconfig", rtnErr == nil)
	}()

	// Get config directory from Star info
	resp, err := wshclient.StarInfoCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return fmt.Errorf("getting Star info: %w", err)
	}

	configFile := "settings.json" // default
	if len(args) > 0 {
		configFile = args[0]
	}

	settingsFile := filepath.Join(resp.ConfigDir, configFile)

	wshCmd := &wshrpc.CommandCreateBlockData{
		BlockDef: &starobj.BlockDef{
			Meta: map[string]interface{}{
				starobj.MetaKey_View: "preview",
				starobj.MetaKey_File: settingsFile,
				starobj.MetaKey_Edit: true,
			},
		},
	}

	_, err = RpcClient.SendRpcRequest(wshrpc.Command_CreateBlock, wshCmd, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}
	return nil
}
