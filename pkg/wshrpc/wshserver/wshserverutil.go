// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wshserver

import (
	"sync"

	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshutil"
)

const (
	DefaultOutputChSize = 32
	DefaultInputChSize  = 32
)

var starSrvClient_Singleton *wshutil.WshRpc
var starSrvClient_Once = &sync.Once{}

// returns the starsrv main rpc client singleton
func GetMainRpcClient() *wshutil.WshRpc {
	starSrvClient_Once.Do(func() {
		inputCh := make(chan []byte, DefaultInputChSize)
		outputCh := make(chan []byte, DefaultOutputChSize)
		starSrvClient_Singleton = wshutil.MakeWshRpc(inputCh, outputCh, wshrpc.RpcContext{}, &WshServerImpl, "main-client")
	})
	return starSrvClient_Singleton
}
