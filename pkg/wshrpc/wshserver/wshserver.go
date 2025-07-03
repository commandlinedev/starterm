// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wshserver

// this file contains the implementation of the wsh server methods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/commandlinedev/starterm/pkg/blockcontroller"
	"github.com/commandlinedev/starterm/pkg/blocklogger"
	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/genconn"
	"github.com/commandlinedev/starterm/pkg/panichandler"
	"github.com/commandlinedev/starterm/pkg/remote"
	"github.com/commandlinedev/starterm/pkg/remote/awsconn"
	"github.com/commandlinedev/starterm/pkg/remote/conncontroller"
	"github.com/commandlinedev/starterm/pkg/remote/fileshare"
	"github.com/commandlinedev/starterm/pkg/starai"
	"github.com/commandlinedev/starterm/pkg/starbase"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/suggestion"
	"github.com/commandlinedev/starterm/pkg/telemetry"
	"github.com/commandlinedev/starterm/pkg/telemetry/telemetrydata"
	"github.com/commandlinedev/starterm/pkg/util/envutil"
	"github.com/commandlinedev/starterm/pkg/util/iochan/iochantypes"
	"github.com/commandlinedev/starterm/pkg/util/iterfn"
	"github.com/commandlinedev/starterm/pkg/util/shellutil"
	"github.com/commandlinedev/starterm/pkg/util/starfileutil"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/commandlinedev/starterm/pkg/wcloud"
	"github.com/commandlinedev/starterm/pkg/wconfig"
	"github.com/commandlinedev/starterm/pkg/wcore"
	"github.com/commandlinedev/starterm/pkg/wps"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshutil"
	"github.com/commandlinedev/starterm/pkg/wsl"
	"github.com/commandlinedev/starterm/pkg/wslconn"
	"github.com/commandlinedev/starterm/pkg/wstore"
	"github.com/skratchdot/open-golang/open"
)

var InvalidWslDistroNames = []string{"docker-desktop", "docker-desktop-data"}

type WshServer struct{}

func (*WshServer) WshServerImpl() {}

var WshServerImpl = WshServer{}

// TODO remove this after implementing in multiproxy, just for wsl
func (ws *WshServer) AuthenticateTokenCommand(ctx context.Context, data wshrpc.CommandAuthenticateTokenData) (wshrpc.CommandAuthenticateRtnData, error) {
	entry := shellutil.GetAndRemoveTokenSwapEntry(data.Token)
	if entry == nil {
		return wshrpc.CommandAuthenticateRtnData{}, fmt.Errorf("invalid token")
	}
	rtn := wshrpc.CommandAuthenticateRtnData{
		Env:            entry.Env,
		InitScriptText: entry.ScriptText,
	}
	return rtn, nil
}

func (ws *WshServer) TestCommand(ctx context.Context, data string) error {
	defer func() {
		panichandler.PanicHandler("TestCommand", recover())
	}()
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	log.Printf("TEST src:%s | %s\n", rpcSource, data)
	return nil
}

// for testing
func (ws *WshServer) MessageCommand(ctx context.Context, data wshrpc.CommandMessageData) error {
	log.Printf("MESSAGE: %s | %q\n", data.ORef, data.Message)
	return nil
}

// for testing
func (ws *WshServer) StreamTestCommand(ctx context.Context) chan wshrpc.RespOrErrorUnion[int] {
	rtn := make(chan wshrpc.RespOrErrorUnion[int])
	go func() {
		defer func() {
			panichandler.PanicHandler("StreamTestCommand", recover())
		}()
		for i := 1; i <= 5; i++ {
			rtn <- wshrpc.RespOrErrorUnion[int]{Response: i}
			time.Sleep(1 * time.Second)
		}
		close(rtn)
	}()
	return rtn
}

func (ws *WshServer) StreamStarAiCommand(ctx context.Context, request wshrpc.StarAIStreamRequest) chan wshrpc.RespOrErrorUnion[wshrpc.StarAIPacketType] {
	return starai.RunAICommand(ctx, request)
}

func MakePlotData(ctx context.Context, blockId string) error {
	block, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	viewName := block.Meta.GetString(starobj.MetaKey_View, "")
	if viewName != "cpuplot" && viewName != "sysinfo" {
		return fmt.Errorf("invalid view type: %s", viewName)
	}
	return filestore.WFS.MakeFile(ctx, blockId, "cpuplotdata", nil, wshrpc.FileOpts{})
}

func SavePlotData(ctx context.Context, blockId string, history string) error {
	block, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	viewName := block.Meta.GetString(starobj.MetaKey_View, "")
	if viewName != "cpuplot" && viewName != "sysinfo" {
		return fmt.Errorf("invalid view type: %s", viewName)
	}
	// todo: interpret the data being passed
	// for now, this is just to throw an error if the block was closed
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("unable to serialize plot data: %v", err)
	}
	// ignore MakeFile error (already exists is ok)
	return filestore.WFS.WriteFile(ctx, blockId, "cpuplotdata", historyBytes)
}

func (ws *WshServer) GetMetaCommand(ctx context.Context, data wshrpc.CommandGetMetaData) (starobj.MetaMapType, error) {
	obj, err := wstore.DBGetORef(ctx, data.ORef)
	if err != nil {
		return nil, fmt.Errorf("error getting object: %w", err)
	}
	if obj == nil {
		return nil, fmt.Errorf("object not found: %s", data.ORef)
	}
	return starobj.GetMeta(obj), nil
}

func (ws *WshServer) SetMetaCommand(ctx context.Context, data wshrpc.CommandSetMetaData) error {
	log.Printf("SetMetaCommand: %s | %v\n", data.ORef, data.Meta)
	oref := data.ORef
	err := wstore.UpdateObjectMeta(ctx, oref, data.Meta, false)
	if err != nil {
		return fmt.Errorf("error updating object meta: %w", err)
	}
	sendStarObjUpdate(oref)
	return nil
}

func sendStarObjUpdate(oref starobj.ORef) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFn()
	// send a starobj:update event
	starObj, err := wstore.DBGetORef(ctx, oref)
	if err != nil {
		log.Printf("error getting object for update event: %v", err)
		return
	}
	wps.Broker.Publish(wps.StarEvent{
		Event:  wps.Event_StarObjUpdate,
		Scopes: []string{oref.String()},
		Data: starobj.StarObjUpdate{
			UpdateType: starobj.UpdateType_Update,
			OType:      starObj.GetOType(),
			OID:        starobj.GetOID(starObj),
			Obj:        starObj,
		},
	})
}

func (ws *WshServer) ResolveIdsCommand(ctx context.Context, data wshrpc.CommandResolveIdsData) (wshrpc.CommandResolveIdsRtnData, error) {
	rtn := wshrpc.CommandResolveIdsRtnData{}
	rtn.ResolvedIds = make(map[string]starobj.ORef)
	var firstErr error
	for _, simpleId := range data.Ids {
		oref, err := resolveSimpleId(ctx, data, simpleId)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if oref == nil {
			continue
		}
		rtn.ResolvedIds[simpleId] = *oref
	}
	if firstErr != nil && len(data.Ids) == 1 {
		return rtn, firstErr
	}
	return rtn, nil
}

func (ws *WshServer) CreateBlockCommand(ctx context.Context, data wshrpc.CommandCreateBlockData) (*starobj.ORef, error) {
	ctx = starobj.ContextWithUpdates(ctx)
	tabId := data.TabId
	blockData, err := wcore.CreateBlock(ctx, tabId, data.BlockDef, data.RtOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}
	var layoutAction *starobj.LayoutActionData
	if data.TargetBlockId != "" {
		switch data.TargetAction {
		case "replace":
			layoutAction = &starobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_Replace,
				TargetBlockId: data.TargetBlockId,
				BlockId:       blockData.OID,
				Focused:       true,
			}
			err = wcore.DeleteBlock(ctx, data.TargetBlockId, false)
			if err != nil {
				return nil, fmt.Errorf("error deleting block (trying to do block replace): %w", err)
			}
		case "splitright":
			layoutAction = &starobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitHorizontal,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "after",
			}
		case "splitleft":
			layoutAction = &starobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitHorizontal,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "before",
			}
		case "splitup":
			layoutAction = &starobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitVertical,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "before",
			}
		case "splitdown":
			layoutAction = &starobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitVertical,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "after",
			}
		default:
			return nil, fmt.Errorf("invalid target action: %s", data.TargetAction)
		}
	} else {
		layoutAction = &starobj.LayoutActionData{
			ActionType: wcore.LayoutActionDataType_Insert,
			BlockId:    blockData.OID,
			Magnified:  data.Magnified,
			Ephemeral:  data.Ephemeral,
			Focused:    true,
		}
	}
	err = wcore.QueueLayoutActionForTab(ctx, tabId, *layoutAction)
	if err != nil {
		return nil, fmt.Errorf("error queuing layout action: %w", err)
	}
	updates := starobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return &starobj.ORef{OType: starobj.OType_Block, OID: blockData.OID}, nil
}

func (ws *WshServer) CreateSubBlockCommand(ctx context.Context, data wshrpc.CommandCreateSubBlockData) (*starobj.ORef, error) {
	parentBlockId := data.ParentBlockId
	blockData, err := wcore.CreateSubBlock(ctx, parentBlockId, data.BlockDef)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}
	blockRef := &starobj.ORef{OType: starobj.OType_Block, OID: blockData.OID}
	return blockRef, nil
}

func (ws *WshServer) SetViewCommand(ctx context.Context, data wshrpc.CommandBlockSetViewData) error {
	log.Printf("SETVIEW: %s | %q\n", data.BlockId, data.View)
	ctx = starobj.ContextWithUpdates(ctx)
	block, err := wstore.DBGet[*starobj.Block](ctx, data.BlockId)
	if err != nil {
		return fmt.Errorf("error getting block: %w", err)
	}
	block.Meta[starobj.MetaKey_View] = data.View
	err = wstore.DBUpdate(ctx, block)
	if err != nil {
		return fmt.Errorf("error updating block: %w", err)
	}
	updates := starobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return nil
}

func (ws *WshServer) ControllerStopCommand(ctx context.Context, blockId string) error {
	bc := blockcontroller.GetBlockController(blockId)
	if bc == nil {
		return nil
	}
	bc.StopShellProc(true)
	return nil
}

func (ws *WshServer) ControllerResyncCommand(ctx context.Context, data wshrpc.CommandControllerResyncData) error {
	ctx = genconn.ContextWithConnData(ctx, data.BlockId)
	ctx = termCtxWithLogBlockId(ctx, data.BlockId)
	return blockcontroller.ResyncController(ctx, data.TabId, data.BlockId, data.RtOpts, data.ForceRestart)
}

func (ws *WshServer) ControllerInputCommand(ctx context.Context, data wshrpc.CommandBlockInputData) error {
	bc := blockcontroller.GetBlockController(data.BlockId)
	if bc == nil {
		return fmt.Errorf("block controller not found for block %q", data.BlockId)
	}
	inputUnion := &blockcontroller.BlockInputUnion{
		SigName:  data.SigName,
		TermSize: data.TermSize,
	}
	if len(data.InputData64) > 0 {
		inputBuf := make([]byte, base64.StdEncoding.DecodedLen(len(data.InputData64)))
		nw, err := base64.StdEncoding.Decode(inputBuf, []byte(data.InputData64))
		if err != nil {
			return fmt.Errorf("error decoding input data: %w", err)
		}
		inputUnion.InputData = inputBuf[:nw]
	}
	return bc.SendInput(inputUnion)
}

func (ws *WshServer) ControllerAppendOutputCommand(ctx context.Context, data wshrpc.CommandControllerAppendOutputData) error {
	outputBuf := make([]byte, base64.StdEncoding.DecodedLen(len(data.Data64)))
	nw, err := base64.StdEncoding.Decode(outputBuf, []byte(data.Data64))
	if err != nil {
		return fmt.Errorf("error decoding output data: %w", err)
	}
	err = blockcontroller.HandleAppendBlockFile(data.BlockId, starbase.BlockFile_Term, outputBuf[:nw])
	if err != nil {
		return fmt.Errorf("error appending to block file: %w", err)
	}
	return nil
}

func (ws *WshServer) FileCreateCommand(ctx context.Context, data wshrpc.FileData) error {
	data.Data64 = ""
	err := fileshare.PutFile(ctx, data)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	return nil
}

func (ws *WshServer) FileMkdirCommand(ctx context.Context, data wshrpc.FileData) error {
	return fileshare.Mkdir(ctx, data.Info.Path)
}

func (ws *WshServer) FileDeleteCommand(ctx context.Context, data wshrpc.CommandDeleteFileData) error {
	return fileshare.Delete(ctx, data)
}

func (ws *WshServer) FileInfoCommand(ctx context.Context, data wshrpc.FileData) (*wshrpc.FileInfo, error) {
	return fileshare.Stat(ctx, data.Info.Path)
}

func (ws *WshServer) FileListCommand(ctx context.Context, data wshrpc.FileListData) ([]*wshrpc.FileInfo, error) {
	return fileshare.ListEntries(ctx, data.Path, data.Opts)
}

func (ws *WshServer) FileListStreamCommand(ctx context.Context, data wshrpc.FileListData) <-chan wshrpc.RespOrErrorUnion[wshrpc.CommandRemoteListEntriesRtnData] {
	return fileshare.ListEntriesStream(ctx, data.Path, data.Opts)
}

func (ws *WshServer) FileWriteCommand(ctx context.Context, data wshrpc.FileData) error {
	return fileshare.PutFile(ctx, data)
}

func (ws *WshServer) FileReadCommand(ctx context.Context, data wshrpc.FileData) (*wshrpc.FileData, error) {
	return fileshare.Read(ctx, data)
}

func (ws *WshServer) FileReadStreamCommand(ctx context.Context, data wshrpc.FileData) <-chan wshrpc.RespOrErrorUnion[wshrpc.FileData] {
	return fileshare.ReadStream(ctx, data)
}

func (ws *WshServer) FileCopyCommand(ctx context.Context, data wshrpc.CommandFileCopyData) error {
	return fileshare.Copy(ctx, data)
}

func (ws *WshServer) FileMoveCommand(ctx context.Context, data wshrpc.CommandFileCopyData) error {
	return fileshare.Move(ctx, data)
}

func (ws *WshServer) FileStreamTarCommand(ctx context.Context, data wshrpc.CommandRemoteStreamTarData) <-chan wshrpc.RespOrErrorUnion[iochantypes.Packet] {
	return fileshare.ReadTarStream(ctx, data)
}

func (ws *WshServer) FileAppendCommand(ctx context.Context, data wshrpc.FileData) error {
	return fileshare.Append(ctx, data)
}

func (ws *WshServer) FileAppendIJsonCommand(ctx context.Context, data wshrpc.CommandAppendIJsonData) error {
	tryCreate := true
	if data.FileName == starbase.BlockFile_VDom && tryCreate {
		err := filestore.WFS.MakeFile(ctx, data.ZoneId, data.FileName, nil, wshrpc.FileOpts{MaxSize: blockcontroller.DefaultHtmlMaxFileSize, IJson: true})
		if err != nil && err != fs.ErrExist {
			return fmt.Errorf("error creating blockfile[vdom]: %w", err)
		}
	}
	err := filestore.WFS.AppendIJson(ctx, data.ZoneId, data.FileName, data.Data)
	if err != nil {
		return fmt.Errorf("error appending to blockfile(ijson): %w", err)
	}
	wps.Broker.Publish(wps.StarEvent{
		Event:  wps.Event_BlockFile,
		Scopes: []string{starobj.MakeORef(starobj.OType_Block, data.ZoneId).String()},
		Data: &wps.WSFileEventData{
			ZoneId:   data.ZoneId,
			FileName: data.FileName,
			FileOp:   wps.FileOp_Append,
			Data64:   base64.StdEncoding.EncodeToString([]byte("{}")),
		},
	})
	return nil
}

func (ws *WshServer) FileJoinCommand(ctx context.Context, paths []string) (*wshrpc.FileInfo, error) {
	if len(paths) < 2 {
		if len(paths) == 0 {
			return nil, fmt.Errorf("no paths provided")
		}
		return fileshare.Stat(ctx, paths[0])
	}
	return fileshare.Join(ctx, paths[0], paths[1:]...)
}

func (ws *WshServer) FileShareCapabilityCommand(ctx context.Context, path string) (wshrpc.FileShareCapability, error) {
	return fileshare.GetCapability(ctx, path)
}

func (ws *WshServer) DeleteSubBlockCommand(ctx context.Context, data wshrpc.CommandDeleteBlockData) error {
	err := wcore.DeleteBlock(ctx, data.BlockId, false)
	if err != nil {
		return fmt.Errorf("error deleting block: %w", err)
	}
	return nil
}

func (ws *WshServer) DeleteBlockCommand(ctx context.Context, data wshrpc.CommandDeleteBlockData) error {
	ctx = starobj.ContextWithUpdates(ctx)
	tabId, err := wstore.DBFindTabForBlockId(ctx, data.BlockId)
	if err != nil {
		return fmt.Errorf("error finding tab for block: %w", err)
	}
	if tabId == "" {
		return fmt.Errorf("no tab found for block")
	}
	err = wcore.DeleteBlock(ctx, data.BlockId, true)
	if err != nil {
		return fmt.Errorf("error deleting block: %w", err)
	}
	wcore.QueueLayoutActionForTab(ctx, tabId, starobj.LayoutActionData{
		ActionType: wcore.LayoutActionDataType_Remove,
		BlockId:    data.BlockId,
	})
	updates := starobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return nil
}

func (ws *WshServer) WaitForRouteCommand(ctx context.Context, data wshrpc.CommandWaitForRouteData) (bool, error) {
	waitCtx, cancelFn := context.WithTimeout(ctx, time.Duration(data.WaitMs)*time.Millisecond)
	defer cancelFn()
	err := wshutil.DefaultRouter.WaitForRegister(waitCtx, data.RouteId)
	return err == nil, nil
}

func (ws *WshServer) EventRecvCommand(ctx context.Context, data wps.StarEvent) error {
	return nil
}

func (ws *WshServer) EventPublishCommand(ctx context.Context, data wps.StarEvent) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	if data.Sender == "" {
		data.Sender = rpcSource
	}
	wps.Broker.Publish(data)
	return nil
}

func (ws *WshServer) EventSubCommand(ctx context.Context, data wps.SubscriptionRequest) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.Subscribe(rpcSource, data)
	return nil
}

func (ws *WshServer) EventUnsubCommand(ctx context.Context, data string) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.Unsubscribe(rpcSource, data)
	return nil
}

func (ws *WshServer) EventUnsubAllCommand(ctx context.Context) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.UnsubscribeAll(rpcSource)
	return nil
}

func (ws *WshServer) EventReadHistoryCommand(ctx context.Context, data wshrpc.CommandEventReadHistoryData) ([]*wps.StarEvent, error) {
	events := wps.Broker.ReadEventHistory(data.Event, data.Scope, data.MaxItems)
	return events, nil
}

func (ws *WshServer) SetConfigCommand(ctx context.Context, data wshrpc.MetaSettingsType) error {
	log.Printf("SETCONFIG: %v\n", data)
	return wconfig.SetBaseConfigValue(data.MetaMapType)
}

func (ws *WshServer) SetConnectionsConfigCommand(ctx context.Context, data wshrpc.ConnConfigRequest) error {
	log.Printf("SET CONNECTIONS CONFIG: %v\n", data)
	return wconfig.SetConnectionsConfigValue(data.Host, data.MetaMapType)
}

func (ws *WshServer) GetFullConfigCommand(ctx context.Context) (wconfig.FullConfigType, error) {
	watcher := wconfig.GetWatcher()
	return watcher.GetFullConfig(), nil
}

func (ws *WshServer) ConnStatusCommand(ctx context.Context) ([]wshrpc.ConnStatus, error) {
	rtn := conncontroller.GetAllConnStatus()
	return rtn, nil
}

func (ws *WshServer) WslStatusCommand(ctx context.Context) ([]wshrpc.ConnStatus, error) {
	rtn := wslconn.GetAllConnStatus()
	return rtn, nil
}

func termCtxWithLogBlockId(ctx context.Context, logBlockId string) context.Context {
	if logBlockId == "" {
		return ctx
	}
	block, err := wstore.DBMustGet[*starobj.Block](ctx, logBlockId)
	if err != nil {
		return ctx
	}
	connDebug := block.Meta.GetString(starobj.MetaKey_TermConnDebug, "")
	if connDebug == "" {
		return ctx
	}
	return blocklogger.ContextWithLogBlockId(ctx, logBlockId, connDebug == "debug")
}

func (ws *WshServer) ConnEnsureCommand(ctx context.Context, data wshrpc.ConnExtData) error {
	// TODO: if we add proper wsh connections via aws, we'll need to handle that here
	if strings.HasPrefix(data.ConnName, "aws:") {
		profiles := awsconn.ParseProfiles()
		for profile := range profiles {
			if strings.HasPrefix(data.ConnName, profile) {
				return nil
			}
		}
	}
	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)
	if strings.HasPrefix(data.ConnName, "wsl://") {
		distroName := strings.TrimPrefix(data.ConnName, "wsl://")
		return wslconn.EnsureConnection(ctx, distroName)
	}
	return conncontroller.EnsureConnection(ctx, data.ConnName)
}

func (ws *WshServer) ConnDisconnectCommand(ctx context.Context, connName string) error {
	// TODO: if we add proper wsh connections via aws, we'll need to handle that here
	if strings.HasPrefix(connName, "aws:") {
		return nil
	}
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("distro not found: %s", connName)
		}
		return conn.Close()
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.Close()
}

func (ws *WshServer) ConnConnectCommand(ctx context.Context, connRequest wshrpc.ConnRequest) error {
	// TODO: if we add proper wsh connections via aws, we'll need to handle that here
	if strings.HasPrefix(connRequest.Host, "aws:") {
		return nil
	}
	ctx = genconn.ContextWithConnData(ctx, connRequest.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, connRequest.LogBlockId)
	connName := connRequest.Host
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		return conn.Connect(ctx)
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.Connect(ctx, &connRequest.Keywords)
}

func (ws *WshServer) ConnReinstallWshCommand(ctx context.Context, data wshrpc.ConnExtData) error {
	// TODO: if we add proper wsh connections via aws, we'll need to handle that here
	if strings.HasPrefix(data.ConnName, "aws:") {
		return nil
	}
	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)
	connName := data.ConnName
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		return conn.InstallWsh(ctx, "")
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.InstallWsh(ctx, "")
}

func (ws *WshServer) ConnUpdateWshCommand(ctx context.Context, remoteInfo wshrpc.RemoteInfo) (bool, error) {
	handler := wshutil.GetRpcResponseHandlerFromContext(ctx)
	if handler == nil {
		return false, fmt.Errorf("could not determine handler from context")
	}
	connName := handler.GetRpcContext().Conn
	if connName == "" {
		return false, fmt.Errorf("invalid remote info: missing connection name")
	}

	log.Printf("checking wsh version for connection %s (current: %s)", connName, remoteInfo.ClientVersion)
	upToDate, _, _, err := conncontroller.IsWshVersionUpToDate(ctx, remoteInfo.ClientVersion)
	if err != nil {
		return false, fmt.Errorf("unable to compare wsh version: %w", err)
	}
	if upToDate {
		// no need to update
		log.Printf("wsh is already up to date for connection %s", connName)
		return false, nil
	}

	// todo: need to add user input code here for validation

	if strings.HasPrefix(connName, "wsl://") {
		return false, fmt.Errorf("connupdatewshcommand is not supported for wsl connections")
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return false, fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return false, fmt.Errorf("connection not found: %s", connName)
	}
	err = conn.UpdateWsh(ctx, connName, &remoteInfo)
	if err != nil {
		return false, fmt.Errorf("wsh update failed for connection %s: %w", connName, err)
	}

	// todo: need to add code for modifying configs?
	return true, nil
}

func (ws *WshServer) ConnListCommand(ctx context.Context) ([]string, error) {
	return conncontroller.GetConnectionsList()
}

func (ws *WshServer) ConnListAWSCommand(ctx context.Context) ([]string, error) {
	profilesMap := awsconn.ParseProfiles()
	return iterfn.MapKeysToSorted(profilesMap), nil
}

func (ws *WshServer) WslListCommand(ctx context.Context) ([]string, error) {
	distros, err := wsl.RegisteredDistros(ctx)
	if err != nil {
		return nil, err
	}
	var distroNames []string
	for _, distro := range distros {
		distroName := distro.Name()
		if utilfn.ContainsStr(InvalidWslDistroNames, distroName) {
			continue
		}
		distroNames = append(distroNames, distroName)
	}
	return distroNames, nil
}

func (ws *WshServer) WslDefaultDistroCommand(ctx context.Context) (string, error) {
	distro, ok, err := wsl.DefaultDistro(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to determine default distro: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("unable to determine default distro")
	}
	return distro.Name(), nil
}

/**
 * Dismisses the WshFail Command in runtime memory on the backend
 */
func (ws *WshServer) DismissWshFailCommand(ctx context.Context, connName string) error {
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		conn.ClearWshError()
		conn.FireConnChangeEvent()
		return nil
	}
	opts, err := remote.ParseOpts(connName)
	if err != nil {
		return err
	}
	conn := conncontroller.GetConn(opts)
	if conn == nil {
		return fmt.Errorf("connection %s not found", connName)
	}
	conn.ClearWshError()
	conn.FireConnChangeEvent()
	return nil
}

func (ws *WshServer) BlockInfoCommand(ctx context.Context, blockId string) (*wshrpc.BlockInfoData, error) {
	blockData, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error getting block: %w", err)
	}
	tabId, err := wstore.DBFindTabForBlockId(ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error finding tab for block: %w", err)
	}
	workspaceId, err := wstore.DBFindWorkspaceForTabId(ctx, tabId)
	if err != nil {
		return nil, fmt.Errorf("error finding window for tab: %w", err)
	}
	fileList, err := filestore.WFS.ListFiles(ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error listing blockfiles: %w", err)
	}
	fileInfoList := starfileutil.StarFileListToFileInfoList(fileList)
	return &wshrpc.BlockInfoData{
		BlockId:     blockId,
		TabId:       tabId,
		WorkspaceId: workspaceId,
		Block:       blockData,
		Files:       fileInfoList,
	}, nil
}

func (ws *WshServer) StarInfoCommand(ctx context.Context) (*wshrpc.StarInfoData, error) {
	client, err := wstore.DBGetSingleton[*starobj.Client](ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}
	return &wshrpc.StarInfoData{
		Version:   starbase.StarVersion,
		ClientId:  client.OID,
		BuildTime: starbase.BuildTime,
		ConfigDir: starbase.GetStarConfigDir(),
		DataDir:   starbase.GetStarDataDir(),
	}, nil
}

func (ws *WshServer) WorkspaceListCommand(ctx context.Context) ([]wshrpc.WorkspaceInfoData, error) {
	workspaceList, err := wcore.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing workspaces: %w", err)
	}
	var rtn []wshrpc.WorkspaceInfoData
	for _, workspaceEntry := range workspaceList {
		workspaceData, err := wcore.GetWorkspace(ctx, workspaceEntry.WorkspaceId)
		if err != nil {
			return nil, fmt.Errorf("error getting workspace: %w", err)
		}
		rtn = append(rtn, wshrpc.WorkspaceInfoData{
			WindowId:      workspaceEntry.WindowId,
			WorkspaceData: workspaceData,
		})
	}
	return rtn, nil
}

func (ws *WshServer) RecordTEventCommand(ctx context.Context, data telemetrydata.TEvent) error {
	err := telemetry.RecordTEvent(ctx, &data)
	if err != nil {
		log.Printf("error recording telemetry event: %v", err)
	}
	return err
}

func (ws WshServer) SendTelemetryCommand(ctx context.Context) error {
	client, err := wstore.DBGetSingleton[*starobj.Client](ctx)
	if err != nil {
		return fmt.Errorf("getting client data for telemetry: %v", err)
	}
	return wcloud.SendAllTelemetry(ctx, client.OID)
}

var wshActivityRe = regexp.MustCompile(`^[a-z:#]+$`)

func (ws *WshServer) WshActivityCommand(ctx context.Context, data map[string]int) error {
	if len(data) == 0 {
		return nil
	}
	props := telemetrydata.TEventProps{}
	for key, value := range data {
		if len(key) > 20 {
			delete(data, key)
		}
		if !wshActivityRe.MatchString(key) {
			delete(data, key)
		}
		if value != 1 {
			delete(data, key)
		}
		if strings.HasSuffix(key, "#error") {
			props.WshHadError = true
		} else {
			props.WshCmd = key
		}
	}
	activityUpdate := wshrpc.ActivityUpdate{
		WshCmds: data,
	}
	telemetry.GoUpdateActivityWrap(activityUpdate, "wsh-activity")
	telemetry.GoRecordTEventWrap(&telemetrydata.TEvent{
		Event: "wsh:run",
		Props: props,
	})
	return nil
}

func (ws *WshServer) ActivityCommand(ctx context.Context, activity wshrpc.ActivityUpdate) error {
	telemetry.GoUpdateActivityWrap(activity, "wshrpc-activity")
	return nil
}

func (ws *WshServer) GetVarCommand(ctx context.Context, data wshrpc.CommandVarData) (*wshrpc.CommandVarResponseData, error) {
	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.FileName)
	if err == fs.ErrNotExist {
		return &wshrpc.CommandVarResponseData{Key: data.Key, Exists: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading blockfile: %w", err)
	}
	envMap := envutil.EnvToMap(string(fileData))
	value, ok := envMap[data.Key]
	return &wshrpc.CommandVarResponseData{Key: data.Key, Exists: ok, Val: value}, nil
}

func (ws *WshServer) SetVarCommand(ctx context.Context, data wshrpc.CommandVarData) error {
	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.FileName)
	if err == fs.ErrNotExist {
		fileData = []byte{}
		err = filestore.WFS.MakeFile(ctx, data.ZoneId, data.FileName, nil, wshrpc.FileOpts{})
		if err != nil {
			return fmt.Errorf("error creating blockfile: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("error reading blockfile: %w", err)
	}
	envMap := envutil.EnvToMap(string(fileData))
	if data.Remove {
		delete(envMap, data.Key)
	} else {
		envMap[data.Key] = data.Val
	}
	envStr := envutil.MapToEnv(envMap)
	return filestore.WFS.WriteFile(ctx, data.ZoneId, data.FileName, []byte(envStr))
}

func (ws *WshServer) PathCommand(ctx context.Context, data wshrpc.PathCommandData) (string, error) {
	pathType := data.PathType
	openInternal := data.Open
	openExternal := data.OpenExternal
	var path string
	switch pathType {
	case "config":
		path = starbase.GetStarConfigDir()
	case "data":
		path = starbase.GetStarDataDir()
	case "log":
		path = filepath.Join(starbase.GetStarDataDir(), "starapp.log")
	}

	if openInternal && openExternal {
		return "", fmt.Errorf("open and openExternal cannot both be true")
	}

	if openInternal {
		_, err := ws.CreateBlockCommand(ctx, wshrpc.CommandCreateBlockData{BlockDef: &starobj.BlockDef{Meta: map[string]any{
			starobj.MetaKey_View: "preview",
			starobj.MetaKey_File: path,
		}}, Ephemeral: true, TabId: data.TabId})

		if err != nil {
			return path, fmt.Errorf("error opening path: %w", err)
		}
	} else if openExternal {
		err := open.Run(path)
		if err != nil {
			return path, fmt.Errorf("error opening path: %w", err)
		}
	}
	return path, nil
}

func (ws *WshServer) FetchSuggestionsCommand(ctx context.Context, data wshrpc.FetchSuggestionsData) (*wshrpc.FetchSuggestionsResponse, error) {
	return suggestion.FetchSuggestions(ctx, data)
}

func (ws *WshServer) DisposeSuggestionsCommand(ctx context.Context, widgetId string) error {
	suggestion.DisposeSuggestions(ctx, widgetId)
	return nil
}

func (ws *WshServer) GetTabCommand(ctx context.Context, tabId string) (*starobj.Tab, error) {
	tab, err := wstore.DBGet[*starobj.Tab](ctx, tabId)
	if err != nil {
		return nil, fmt.Errorf("error getting tab: %w", err)
	}
	return tab, nil
}
