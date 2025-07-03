// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/commandlinedev/starterm/pkg/starbase"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/util/envutil"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshclient"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:              "run [flags] -- command [args...]",
	Short:            "run a command in a new block",
	RunE:             runRun,
	PreRunE:          preRunSetupRpcClient,
	TraverseChildren: true,
}

func init() {
	flags := runCmd.Flags()
	flags.BoolP("magnified", "m", false, "open view in magnified mode")
	flags.StringP("command", "c", "", "run command string in shell")
	flags.BoolP("exit", "x", false, "close block if command exits successfully (will stay open if there was an error)")
	flags.BoolP("forceexit", "X", false, "close block when command exits, regardless of exit status")
	flags.IntP("delay", "", 2000, "if -x, delay in milliseconds before closing block")
	flags.BoolP("paused", "p", false, "create block in paused state")
	flags.String("cwd", "", "set working directory for command")
	flags.BoolP("append", "a", false, "append output on restart instead of clearing")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("run", rtnErr == nil)
	}()

	flags := cmd.Flags()
	magnified, _ := flags.GetBool("magnified")
	commandArg, _ := flags.GetString("command")
	exit, _ := flags.GetBool("exit")
	forceExit, _ := flags.GetBool("forceexit")
	paused, _ := flags.GetBool("paused")
	cwd, _ := flags.GetString("cwd")
	delayMs, _ := flags.GetInt("delay")
	appendOutput, _ := flags.GetBool("append")
	var cmdArgs []string
	var useShell bool
	var shellCmd string

	for i, arg := range os.Args {
		if arg == "--" {
			if i+1 >= len(os.Args) {
				OutputHelpMessage(cmd)
				return fmt.Errorf("no command provided after --")
			}
			shellCmd = os.Args[i+1]
			cmdArgs = os.Args[i+2:]
			break
		}
	}
	if shellCmd != "" && commandArg != "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("cannot specify both -c and command arguments")
	}
	if shellCmd == "" && commandArg == "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("command must be specified after -- or with -c")
	}
	if commandArg != "" {
		shellCmd = commandArg
		useShell = true
	}

	// Get current working directory
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
	}
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Get current environment and convert to map
	envMap := make(map[string]string)
	for _, envStr := range os.Environ() {
		env := strings.SplitN(envStr, "=", 2)
		if len(env) == 2 {
			envMap[env[0]] = env[1]
		}
	}

	// Convert to null-terminated format
	envContent := envutil.MapToEnv(envMap)
	createMeta := map[string]any{
		starobj.MetaKey_View:            "term",
		starobj.MetaKey_CmdCwd:          cwd,
		starobj.MetaKey_Controller:      "cmd",
		starobj.MetaKey_CmdClearOnStart: true,
	}
	createMeta[starobj.MetaKey_Cmd] = shellCmd
	createMeta[starobj.MetaKey_CmdArgs] = cmdArgs
	createMeta[starobj.MetaKey_CmdShell] = useShell
	if paused {
		createMeta[starobj.MetaKey_CmdRunOnStart] = false
	} else {
		createMeta[starobj.MetaKey_CmdRunOnce] = true
		createMeta[starobj.MetaKey_CmdRunOnStart] = true
	}
	if forceExit {
		createMeta[starobj.MetaKey_CmdCloseOnExitForce] = true
	} else if exit {
		createMeta[starobj.MetaKey_CmdCloseOnExit] = true
	}
	createMeta[starobj.MetaKey_CmdCloseOnExitDelay] = float64(delayMs)
	if appendOutput {
		createMeta[starobj.MetaKey_CmdClearOnStart] = false
	}

	if RpcContext.Conn != "" {
		createMeta[starobj.MetaKey_Connection] = RpcContext.Conn
	}

	createBlockData := wshrpc.CommandCreateBlockData{
		BlockDef: &starobj.BlockDef{
			Meta: createMeta,
			Files: map[string]*starobj.FileDef{
				starbase.BlockFile_Env: {
					Content: envContent,
				},
			},
		},
		Magnified: magnified,
	}

	oref, err := wshclient.CreateBlockCommand(RpcClient, createBlockData, nil)
	if err != nil {
		return fmt.Errorf("creating new run block: %w", err)
	}

	WriteStdout("run block created: %s\n", oref)
	return nil
}
