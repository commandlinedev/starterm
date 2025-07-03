// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"runtime"
	"sync"
	"time"

	"github.com/commandlinedev/starterm/pkg/authkey"
	"github.com/commandlinedev/starterm/pkg/blockcontroller"
	"github.com/commandlinedev/starterm/pkg/blocklogger"
	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/panichandler"
	"github.com/commandlinedev/starterm/pkg/remote/conncontroller"
	"github.com/commandlinedev/starterm/pkg/remote/fileshare/wshfs"
	"github.com/commandlinedev/starterm/pkg/service"
	"github.com/commandlinedev/starterm/pkg/starbase"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/telemetry"
	"github.com/commandlinedev/starterm/pkg/telemetry/telemetrydata"
	"github.com/commandlinedev/starterm/pkg/util/shellutil"
	"github.com/commandlinedev/starterm/pkg/util/sigutil"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/commandlinedev/starterm/pkg/wcloud"
	"github.com/commandlinedev/starterm/pkg/wconfig"
	"github.com/commandlinedev/starterm/pkg/wcore"
	"github.com/commandlinedev/starterm/pkg/web"
	"github.com/commandlinedev/starterm/pkg/wps"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshremote"
	"github.com/commandlinedev/starterm/pkg/wshrpc/wshserver"
	"github.com/commandlinedev/starterm/pkg/wshutil"
	"github.com/commandlinedev/starterm/pkg/wslconn"
	"github.com/commandlinedev/starterm/pkg/wstore"
)

// these are set at build time
var StarVersion = "0.0.0"
var BuildTime = "0"

const InitialTelemetryWait = 10 * time.Second
const TelemetryTick = 2 * time.Minute
const TelemetryInterval = 4 * time.Hour
const TelemetryInitialCountsWait = 5 * time.Second
const TelemetryCountsInterval = 1 * time.Hour

var shutdownOnce sync.Once

func doShutdown(reason string) {
	shutdownOnce.Do(func() {
		log.Printf("shutting down: %s\n", reason)
		ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFn()
		go blockcontroller.StopAllBlockControllers()
		shutdownActivityUpdate()
		sendTelemetryWrapper()
		// TODO deal with flush in progress
		clearTempFiles()
		filestore.WFS.FlushCache(ctx)
		watcher := wconfig.GetWatcher()
		if watcher != nil {
			watcher.Close()
		}
		time.Sleep(500 * time.Millisecond)
		log.Printf("shutdown complete\n")
		os.Exit(0)
	})
}

// watch stdin, kill server if stdin is closed
func stdinReadWatch() {
	buf := make([]byte, 1024)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			doShutdown(fmt.Sprintf("stdin closed/error (%v)", err))
			break
		}
	}
}

func startConfigWatcher() {
	watcher := wconfig.GetWatcher()
	if watcher != nil {
		watcher.Start()
	}
}

func telemetryLoop() {
	var nextSend int64
	time.Sleep(InitialTelemetryWait)
	for {
		if time.Now().Unix() > nextSend {
			nextSend = time.Now().Add(TelemetryInterval).Unix()
			sendTelemetryWrapper()
		}
		time.Sleep(TelemetryTick)
	}
}

func panicTelemetryHandler(panicName string) {
	activity := wshrpc.ActivityUpdate{NumPanics: 1}
	err := telemetry.UpdateActivity(context.Background(), activity)
	if err != nil {
		log.Printf("error updating activity (panicTelemetryHandler): %v\n", err)
	}
	telemetry.RecordTEvent(context.Background(), telemetrydata.MakeTEvent("debug:panic", telemetrydata.TEventProps{
		PanicType: panicName,
	}))
}

func sendTelemetryWrapper() {
	defer func() {
		panichandler.PanicHandler("sendTelemetryWrapper", recover())
	}()
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()
	beforeSendActivityUpdate(ctx)
	client, err := wstore.DBGetSingleton[*starobj.Client](ctx)
	if err != nil {
		log.Printf("[error] getting client data for telemetry: %v\n", err)
		return
	}
	err = wcloud.SendAllTelemetry(ctx, client.OID)
	if err != nil {
		log.Printf("[error] sending telemetry: %v\n", err)
	}
}

func updateTelemetryCounts(lastCounts telemetrydata.TEventProps) telemetrydata.TEventProps {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()
	var props telemetrydata.TEventProps
	props.CountBlocks, _ = wstore.DBGetCount[*starobj.Block](ctx)
	props.CountTabs, _ = wstore.DBGetCount[*starobj.Tab](ctx)
	props.CountWindows, _ = wstore.DBGetCount[*starobj.Window](ctx)
	props.CountWorkspaces, _, _ = wstore.DBGetWSCounts(ctx)
	props.CountSSHConn = conncontroller.GetNumSSHHasConnected()
	props.CountWSLConn = wslconn.GetNumWSLHasConnected()
	props.CountViews, _ = wstore.DBGetBlockViewCounts(ctx)
	if utilfn.CompareAsMarshaledJson(props, lastCounts) {
		return lastCounts
	}
	tevent := telemetrydata.MakeTEvent("app:counts", props)
	err := telemetry.RecordTEvent(ctx, tevent)
	if err != nil {
		log.Printf("error recording counts tevent: %v\n", err)
	}
	return props
}

func updateTelemetryCountsLoop() {
	defer func() {
		panichandler.PanicHandler("updateTelemetryCountsLoop", recover())
	}()
	var nextSend int64
	var lastCounts telemetrydata.TEventProps
	time.Sleep(TelemetryInitialCountsWait)
	for {
		if time.Now().Unix() > nextSend {
			nextSend = time.Now().Add(TelemetryCountsInterval).Unix()
			lastCounts = updateTelemetryCounts(lastCounts)
		}
		time.Sleep(TelemetryTick)
	}
}

func beforeSendActivityUpdate(ctx context.Context) {
	activity := wshrpc.ActivityUpdate{}
	activity.NumTabs, _ = wstore.DBGetCount[*starobj.Tab](ctx)
	activity.NumBlocks, _ = wstore.DBGetCount[*starobj.Block](ctx)
	activity.Blocks, _ = wstore.DBGetBlockViewCounts(ctx)
	activity.NumWindows, _ = wstore.DBGetCount[*starobj.Window](ctx)
	activity.NumSSHConn = conncontroller.GetNumSSHHasConnected()
	activity.NumWSLConn = wslconn.GetNumWSLHasConnected()
	activity.NumWSNamed, activity.NumWS, _ = wstore.DBGetWSCounts(ctx)
	err := telemetry.UpdateActivity(ctx, activity)
	if err != nil {
		log.Printf("error updating before activity: %v\n", err)
	}
}

func startupActivityUpdate() {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()
	activity := wshrpc.ActivityUpdate{Startup: 1}
	err := telemetry.UpdateActivity(ctx, activity) // set at least one record into activity (don't use go routine wrap here)
	if err != nil {
		log.Printf("error updating startup activity: %v\n", err)
	}
	autoUpdateChannel := telemetry.AutoUpdateChannel()
	autoUpdateEnabled := telemetry.IsAutoUpdateEnabled()
	tevent := telemetrydata.MakeTEvent("app:startup", telemetrydata.TEventProps{
		UserSet: &telemetrydata.TEventUserProps{
			ClientVersion:     "v" + StarVersion,
			ClientBuildTime:   BuildTime,
			ClientArch:        starbase.ClientArch(),
			ClientOSRelease:   starbase.UnameKernelRelease(),
			ClientIsDev:       starbase.IsDevMode(),
			AutoUpdateChannel: autoUpdateChannel,
			AutoUpdateEnabled: autoUpdateEnabled,
		},
		UserSetOnce: &telemetrydata.TEventUserProps{
			ClientInitialVersion: "v" + StarVersion,
		},
	})
	err = telemetry.RecordTEvent(ctx, tevent)
	if err != nil {
		log.Printf("error recording startup event: %v\n", err)
	}
}

func shutdownActivityUpdate() {
	ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelFn()
	activity := wshrpc.ActivityUpdate{Shutdown: 1}
	err := telemetry.UpdateActivity(ctx, activity) // do NOT use the go routine wrap here (this needs to be synchronous)
	if err != nil {
		log.Printf("error updating shutdown activity: %v\n", err)
	}
	err = telemetry.TruncateActivityTEventForShutdown(ctx)
	if err != nil {
		log.Printf("error truncating activity t-event for shutdown: %v\n", err)
	}
	tevent := telemetrydata.MakeTEvent("app:shutdown", telemetrydata.TEventProps{})
	err = telemetry.RecordTEvent(ctx, tevent)
	if err != nil {
		log.Printf("error recording shutdown event: %v\n", err)
	}
}

func createMainWshClient() {
	rpc := wshserver.GetMainRpcClient()
	wshfs.RpcClient = rpc
	wshutil.DefaultRouter.RegisterRoute(wshutil.DefaultRoute, rpc, true)
	wps.Broker.SetClient(wshutil.DefaultRouter)
	localConnWsh := wshutil.MakeWshRpc(nil, nil, wshrpc.RpcContext{Conn: wshrpc.LocalConnName}, &wshremote.ServerImpl{}, "conn:local")
	go wshremote.RunSysInfoLoop(localConnWsh, wshrpc.LocalConnName)
	wshutil.DefaultRouter.RegisterRoute(wshutil.MakeConnectionRouteId(wshrpc.LocalConnName), localConnWsh, true)
}

func grabAndRemoveEnvVars() error {
	err := authkey.SetAuthKeyFromEnv()
	if err != nil {
		return fmt.Errorf("setting auth key: %v", err)
	}
	err = starbase.CacheAndRemoveEnvVars()
	if err != nil {
		return err
	}
	err = wcloud.CacheAndRemoveEnvVars()
	if err != nil {
		return err
	}
	return nil
}

func clearTempFiles() error {
	ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFn()
	client, err := wstore.DBGetSingleton[*starobj.Client](ctx)
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}
	filestore.WFS.DeleteZone(ctx, client.TempOID)
	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("[starsrv] ")
	starbase.StarVersion = StarVersion
	starbase.BuildTime = BuildTime

	err := grabAndRemoveEnvVars()
	if err != nil {
		log.Printf("[error] %v\n", err)
		return
	}
	err = service.ValidateServiceMap()
	if err != nil {
		log.Printf("error validating service map: %v\n", err)
		return
	}
	err = starbase.EnsureStarDataDir()
	if err != nil {
		log.Printf("error ensuring star home dir: %v\n", err)
		return
	}
	err = starbase.EnsureStarDBDir()
	if err != nil {
		log.Printf("error ensuring star db dir: %v\n", err)
		return
	}
	err = starbase.EnsureStarConfigDir()
	if err != nil {
		log.Printf("error ensuring star config dir: %v\n", err)
		return
	}

	// TODO: rather than ensure this dir exists, we should let the editor recursively create parent dirs on save
	err = starbase.EnsureStarPresetsDir()
	if err != nil {
		log.Printf("error ensuring star presets dir: %v\n", err)
		return
	}
	starLock, err := starbase.AcquireStarLock()
	if err != nil {
		log.Printf("error acquiring star lock (another instance of Star is likely running): %v\n", err)
		return
	}
	defer func() {
		err = starLock.Close()
		if err != nil {
			log.Printf("error releasing star lock: %v\n", err)
		}
	}()
	log.Printf("star version: %s (%s)\n", StarVersion, BuildTime)
	log.Printf("star data dir: %s\n", starbase.GetStarDataDir())
	log.Printf("star config dir: %s\n", starbase.GetStarConfigDir())
	err = filestore.InitFilestore()
	if err != nil {
		log.Printf("error initializing filestore: %v\n", err)
		return
	}
	err = wstore.InitWStore()
	if err != nil {
		log.Printf("error initializing wstore: %v\n", err)
		return
	}
	panichandler.PanicTelemetryHandler = panicTelemetryHandler
	go func() {
		defer func() {
			panichandler.PanicHandler("InitCustomShellStartupFiles", recover())
		}()
		err := shellutil.InitCustomShellStartupFiles()
		if err != nil {
			log.Printf("error initializing wsh and shell-integration files: %v\n", err)
		}
	}()
	err = wcore.EnsureInitialData()
	if err != nil {
		log.Printf("error ensuring initial data: %v\n", err)
		return
	}
	err = clearTempFiles()
	if err != nil {
		log.Printf("error clearing temp files: %v\n", err)
		return
	}

	createMainWshClient()
	sigutil.InstallShutdownSignalHandlers(doShutdown)
	sigutil.InstallSIGUSR1Handler()
	startConfigWatcher()
	go stdinReadWatch()
	go telemetryLoop()
	go updateTelemetryCountsLoop()
	startupActivityUpdate() // must be after startConfigWatcher()
	blocklogger.InitBlockLogger()

	webListener, err := web.MakeTCPListener("web")
	if err != nil {
		log.Printf("error creating web listener: %v\n", err)
		return
	}
	wsListener, err := web.MakeTCPListener("websocket")
	if err != nil {
		log.Printf("error creating websocket listener: %v\n", err)
		return
	}
	go web.RunWebSocketServer(wsListener)
	unixListener, err := web.MakeUnixListener()
	if err != nil {
		log.Printf("error creating unix listener: %v\n", err)
		return
	}
	go func() {
		if BuildTime == "" {
			BuildTime = "0"
		}
		// use fmt instead of log here to make sure it goes directly to stderr
		fmt.Fprintf(os.Stderr, "STARSRV-ESTART ws:%s web:%s version:%s buildtime:%s\n", wsListener.Addr(), webListener.Addr(), StarVersion, BuildTime)
	}()
	go wshutil.RunWshRpcOverListener(unixListener)
	web.RunWebServer(webListener) // blocking
	runtime.KeepAlive(starLock)
}
