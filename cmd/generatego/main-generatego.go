// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/commandlinedev/starterm/pkg/gogen"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/commandlinedev/starterm/pkg/wconfig"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
)

const WshClientFileName = "pkg/wshrpc/wshclient/wshclient.go"
const StarObjMetaConstsFileName = "pkg/starobj/metaconsts.go"
const SettingsMetaConstsFileName = "pkg/wconfig/metaconsts.go"

func GenerateWshClient() error {
	fmt.Fprintf(os.Stderr, "generating wshclient file to %s\n", WshClientFileName)
	var buf strings.Builder
	gogen.GenerateBoilerplate(&buf, "wshclient", []string{
		"github.com/commandlinedev/starterm/pkg/telemetry/telemetrydata",
		"github.com/commandlinedev/starterm/pkg/wshutil",
		"github.com/commandlinedev/starterm/pkg/wshrpc",
		"github.com/commandlinedev/starterm/pkg/wconfig",
		"github.com/commandlinedev/starterm/pkg/starobj",
		"github.com/commandlinedev/starterm/pkg/wps",
		"github.com/commandlinedev/starterm/pkg/vdom",
		"github.com/commandlinedev/starterm/pkg/util/iochan/iochantypes",
	})
	wshDeclMap := wshrpc.GenerateWshCommandDeclMap()
	for _, key := range utilfn.GetOrderedMapKeys(wshDeclMap) {
		methodDecl := wshDeclMap[key]
		if methodDecl.CommandType == wshrpc.RpcType_ResponseStream {
			gogen.GenMethod_ResponseStream(&buf, methodDecl)
		} else if methodDecl.CommandType == wshrpc.RpcType_Call {
			gogen.GenMethod_Call(&buf, methodDecl)
		} else {
			panic("unsupported command type " + methodDecl.CommandType)
		}
	}
	buf.WriteString("\n")
	written, err := utilfn.WriteFileIfDifferent(WshClientFileName, []byte(buf.String()))
	if !written {
		fmt.Fprintf(os.Stderr, "no changes to %s\n", WshClientFileName)
	}
	return err
}

func GenerateStarObjMetaConsts() error {
	fmt.Fprintf(os.Stderr, "generating starobj meta consts file to %s\n", StarObjMetaConstsFileName)
	var buf strings.Builder
	gogen.GenerateBoilerplate(&buf, "starobj", []string{})
	gogen.GenerateMetaMapConsts(&buf, "MetaKey_", reflect.TypeOf(starobj.MetaTSType{}), false)
	buf.WriteString("\n")
	written, err := utilfn.WriteFileIfDifferent(StarObjMetaConstsFileName, []byte(buf.String()))
	if !written {
		fmt.Fprintf(os.Stderr, "no changes to %s\n", StarObjMetaConstsFileName)
	}
	return err
}

func GenerateSettingsMetaConsts() error {
	fmt.Fprintf(os.Stderr, "generating settings meta consts file to %s\n", SettingsMetaConstsFileName)
	var buf strings.Builder
	gogen.GenerateBoilerplate(&buf, "wconfig", []string{})
	gogen.GenerateMetaMapConsts(&buf, "ConfigKey_", reflect.TypeOf(wconfig.SettingsType{}), false)
	buf.WriteString("\n")
	written, err := utilfn.WriteFileIfDifferent(SettingsMetaConstsFileName, []byte(buf.String()))
	if !written {
		fmt.Fprintf(os.Stderr, "no changes to %s\n", SettingsMetaConstsFileName)
	}
	return err
}

func main() {
	err := GenerateWshClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating wshclient: %v\n", err)
		return
	}
	err = GenerateStarObjMetaConsts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating starobj meta consts: %v\n", err)
		return
	}
	err = GenerateSettingsMetaConsts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating settings meta consts: %v\n", err)
		return
	}
}
