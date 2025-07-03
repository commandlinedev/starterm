// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as electron from "electron";
import * as child_process from "node:child_process";
import * as readline from "readline";
import { WebServerEndpointVarName, WSServerEndpointVarName } from "../frontend/util/endpoints";
import { AuthKey, StarAuthKeyEnv } from "./authkey";
import { setForceQuit } from "./emain-activity";
import { StarAppPathVarName } from "./emain-util";
import {
    getElectronAppUnpackedBasePath,
    getStarConfigDir,
    getStarDataDir,
    getStarSrvCwd,
    getStarSrvPath,
    getXdgCurrentDesktop,
    StarConfigHomeVarName,
    StarDataHomeVarName,
} from "./platform";
import { updater } from "./updater";

let isStarSrvDead = false;
let starSrvProc: child_process.ChildProcessWithoutNullStreams | null = null;
let StarVersion = "unknown"; // set by STARSRV-ESTART
let StarBuildTime = 0; // set by STARSRV-ESTART

export function getStarVersion(): { version: string; buildTime: number } {
    return { version: StarVersion, buildTime: StarBuildTime };
}

let starSrvReadyResolve = (value: boolean) => {};
const starSrvReady: Promise<boolean> = new Promise((resolve, _) => {
    starSrvReadyResolve = resolve;
});

export function getStarSrvReady(): Promise<boolean> {
    return starSrvReady;
}

export function getStarSrvProc(): child_process.ChildProcessWithoutNullStreams | null {
    return starSrvProc;
}

export function getIsStarSrvDead(): boolean {
    return isStarSrvDead;
}

export function runStarSrv(handleWSEvent: (evtMsg: WSEventType) => void): Promise<boolean> {
    let pResolve: (value: boolean) => void;
    let pReject: (reason?: any) => void;
    const rtnPromise = new Promise<boolean>((argResolve, argReject) => {
        pResolve = argResolve;
        pReject = argReject;
    });
    const envCopy = { ...process.env };
    const xdgCurrentDesktop = getXdgCurrentDesktop();
    if (xdgCurrentDesktop != null) {
        envCopy["XDG_CURRENT_DESKTOP"] = xdgCurrentDesktop;
    }
    envCopy[StarAppPathVarName] = getElectronAppUnpackedBasePath();
    envCopy[StarAuthKeyEnv] = AuthKey;
    envCopy[StarDataHomeVarName] = getStarDataDir();
    envCopy[StarConfigHomeVarName] = getStarConfigDir();
    const starSrvCmd = getStarSrvPath();
    console.log("trying to run local server", starSrvCmd);
    const proc = child_process.spawn(getStarSrvPath(), {
        cwd: getStarSrvCwd(),
        env: envCopy,
    });
    proc.on("exit", (e) => {
        if (updater?.status == "installing") {
            return;
        }
        console.log("starsrv exited, shutting down");
        setForceQuit(true);
        isStarSrvDead = true;
        electron.app.quit();
    });
    proc.on("spawn", (e) => {
        console.log("spawned starsrv");
        starSrvProc = proc;
        pResolve(true);
    });
    proc.on("error", (e) => {
        console.log("error running starsrv", e);
        pReject(e);
    });
    const rlStdout = readline.createInterface({
        input: proc.stdout,
        terminal: false,
    });
    rlStdout.on("line", (line) => {
        console.log(line);
    });
    const rlStderr = readline.createInterface({
        input: proc.stderr,
        terminal: false,
    });
    rlStderr.on("line", (line) => {
        if (line.includes("STARSRV-ESTART")) {
            const startParams = /ws:([a-z0-9.:]+) web:([a-z0-9.:]+) version:([a-z0-9.\-]+) buildtime:(\d+)/gm.exec(
                line
            );
            if (startParams == null) {
                console.log("error parsing STARSRV-ESTART line", line);
                electron.app.quit();
                return;
            }
            process.env[WSServerEndpointVarName] = startParams[1];
            process.env[WebServerEndpointVarName] = startParams[2];
            StarVersion = startParams[3];
            StarBuildTime = parseInt(startParams[4]);
            starSrvReadyResolve(true);
            return;
        }
        if (line.startsWith("STARSRV-EVENT:")) {
            const evtJson = line.slice("STARSRV-EVENT:".length);
            try {
                const evtMsg: WSEventType = JSON.parse(evtJson);
                handleWSEvent(evtMsg);
            } catch (e) {
                console.log("error handling STARSRV-EVENT", e);
            }
            return;
        }
        console.log(line);
    });
    return rtnPromise;
}
