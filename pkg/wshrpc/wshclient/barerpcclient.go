// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wshclient

import (
	"sync"

	"github.com/commandlinedev/starterm/pkg/wps"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshutil"
)

type WshServer struct{}

func (*WshServer) WshServerImpl() {}

var WshServerImpl = WshServer{}

const (
	DefaultOutputChSize = 32
	DefaultInputChSize  = 32
)

var starSrvClient_Singleton *wshutil.WshRpc
var starSrvClient_Once = &sync.Once{}

const BareClientRoute = "bare"

func GetBareRpcClient() *wshutil.WshRpc {
	starSrvClient_Once.Do(func() {
		inputCh := make(chan []byte, DefaultInputChSize)
		outputCh := make(chan []byte, DefaultOutputChSize)
		starSrvClient_Singleton = wshutil.MakeWshRpc(inputCh, outputCh, wshrpc.RpcContext{}, &WshServerImpl, "bare-client")
		wshutil.DefaultRouter.RegisterRoute(BareClientRoute, starSrvClient_Singleton, true)
		wps.Broker.SetClient(wshutil.DefaultRouter)
	})
	return starSrvClient_Singleton
}
