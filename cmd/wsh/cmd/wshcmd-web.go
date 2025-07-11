// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshclient"
	"github.com/commandlinedev/starterm/pkg/wshutil"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:               "web [open|get|set]",
	Short:             "web commands",
	PersistentPreRunE: preRunSetupRpcClient,
}

var webOpenCmd = &cobra.Command{
	Use:   "open url",
	Short: "open a url a web widget",
	Args:  cobra.ExactArgs(1),
	RunE:  webOpenRun,
}

var webGetCmd = &cobra.Command{
	Use:    "get [--inner] [--all] [--json] css-selector",
	Short:  "get the html for a css selector",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE:   webGetRun,
}

var webGetInner bool
var webGetAll bool
var webGetJson bool
var webOpenMagnified bool
var webOpenReplaceBlock string

func init() {
	webOpenCmd.Flags().BoolVarP(&webOpenMagnified, "magnified", "m", false, "open view in magnified mode")
	webOpenCmd.Flags().StringVarP(&webOpenReplaceBlock, "replace", "r", "", "replace block")
	webCmd.AddCommand(webOpenCmd)
	webGetCmd.Flags().BoolVarP(&webGetInner, "inner", "", false, "get inner html (instead of outer)")
	webGetCmd.Flags().BoolVarP(&webGetAll, "all", "", false, "get all matches (querySelectorAll)")
	webGetCmd.Flags().BoolVarP(&webGetJson, "json", "", false, "output as json")
	webCmd.AddCommand(webGetCmd)
	rootCmd.AddCommand(webCmd)
}

func webGetRun(cmd *cobra.Command, args []string) error {
	fullORef, err := resolveBlockArg()
	if err != nil {
		return fmt.Errorf("resolving blockid: %w", err)
	}
	blockInfo, err := wshclient.BlockInfoCommand(RpcClient, fullORef.OID, nil)
	if err != nil {
		return fmt.Errorf("getting block info: %w", err)
	}
	if blockInfo.Block.Meta.GetString(starobj.MetaKey_View, "") != "web" {
		return fmt.Errorf("block %s is not a web block", fullORef.OID)
	}
	data := wshrpc.CommandWebSelectorData{
		WorkspaceId: blockInfo.WorkspaceId,
		BlockId:     fullORef.OID,
		TabId:       blockInfo.TabId,
		Selector:    args[0],
		Opts: &wshrpc.WebSelectorOpts{
			Inner: webGetInner,
			All:   webGetAll,
		},
	}
	output, err := wshclient.WebSelectorCommand(RpcClient, data, &wshrpc.RpcOpts{
		Route:   wshutil.ElectronRoute,
		Timeout: 5000,
	})
	if err != nil {
		return err
	}
	if webGetJson {
		barr, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("json encoding: %w", err)
		}
		WriteStdout("%s\n", string(barr))
	} else {
		for _, item := range output {
			WriteStdout("%s\n", item)
		}
	}
	return nil
}

func webOpenRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("web", rtnErr == nil)
	}()

	var replaceBlockORef *starobj.ORef
	if webOpenReplaceBlock != "" {
		var err error
		replaceBlockORef, err = resolveSimpleId(webOpenReplaceBlock)
		if err != nil {
			return fmt.Errorf("resolving -r blockid: %w", err)
		}
	}
	if replaceBlockORef != nil && webOpenMagnified {
		return fmt.Errorf("cannot use --replace and --magnified together")
	}
	wshCmd := wshrpc.CommandCreateBlockData{
		BlockDef: &starobj.BlockDef{
			Meta: map[string]any{
				starobj.MetaKey_View: "web",
				starobj.MetaKey_Url:  args[0],
			},
		},
		Magnified: webOpenMagnified,
	}
	if replaceBlockORef != nil {
		wshCmd.TargetBlockId = replaceBlockORef.OID
		wshCmd.TargetAction = wshrpc.CreateBlockAction_Replace
	}
	oref, err := wshclient.CreateBlockCommand(RpcClient, wshCmd, nil)
	if err != nil {
		return fmt.Errorf("creating block: %w", err)
	}
	WriteStdout("created block %s\n", oref)
	return nil
}
