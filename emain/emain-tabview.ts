// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { RpcApi } from "@/app/store/wshclientapi";
import { adaptFromElectronKeyEvent } from "@/util/keyutil";
import { CHORD_TIMEOUT } from "@/util/sharedconst";
import { Rectangle, shell, WebContentsView } from "electron";
import { getStarWindowById } from "emain/emain-window";
import path from "path";
import { configureAuthKeyRequestInjection } from "./authkey";
import { setWasActive } from "./emain-activity";
import { handleCtrlShiftFocus, handleCtrlShiftState, shFrameNavHandler, shNavHandler } from "./emain-util";
import { ElectronWshClient } from "./emain-wsh";
import { getElectronAppBasePath, isDevVite } from "./platform";

function computeBgColor(fullConfig: FullConfigType): string {
    const settings = fullConfig?.settings;
    const isTransparent = settings?.["window:transparent"] ?? false;
    const isBlur = !isTransparent && (settings?.["window:blur"] ?? false);
    if (isTransparent) {
        return "#00000000";
    } else if (isBlur) {
        return "#00000000";
    } else {
        return "#222222";
    }
}

const wcIdToStarTabMap = new Map<number, StarTabView>();

export function getStarTabViewByWebContentsId(webContentsId: number): StarTabView {
    return wcIdToStarTabMap.get(webContentsId);
}

export class StarTabView extends WebContentsView {
    starWindowId: string; // this will be set for any tabviews that are initialized. (unset for the hot spare)
    isActiveTab: boolean;
    private _starTabId: string; // always set, StarTabViews are unique per tab
    lastUsedTs: number; // ts milliseconds
    createdTs: number; // ts milliseconds
    initPromise: Promise<void>;
    initResolve: () => void;
    savedInitOpts: StarInitOpts;
    starReadyPromise: Promise<void>;
    starReadyResolve: () => void;
    isInitialized: boolean = false;
    isStarReady: boolean = false;
    isDestroyed: boolean = false;
    keyboardChordMode: boolean = false;
    resetChordModeTimeout: NodeJS.Timeout = null;

    constructor(fullConfig: FullConfigType) {
        console.log("createBareTabView");
        super({
            webPreferences: {
                preload: path.join(getElectronAppBasePath(), "preload", "index.cjs"),
                webviewTag: true,
            },
        });
        this.createdTs = Date.now();
        this.savedInitOpts = null;
        this.initPromise = new Promise((resolve, _) => {
            this.initResolve = resolve;
        });
        this.initPromise.then(() => {
            this.isInitialized = true;
            console.log("tabview init", Date.now() - this.createdTs + "ms");
        });
        this.starReadyPromise = new Promise((resolve, _) => {
            this.starReadyResolve = resolve;
        });
        this.starReadyPromise.then(() => {
            this.isStarReady = true;
        });
        wcIdToStarTabMap.set(this.webContents.id, this);
        if (isDevVite) {
            this.webContents.loadURL(`${process.env.ELECTRON_RENDERER_URL}/index.html}`);
        } else {
            this.webContents.loadFile(path.join(getElectronAppBasePath(), "frontend", "index.html"));
        }
        this.webContents.on("destroyed", () => {
            wcIdToStarTabMap.delete(this.webContents.id);
            removeStarTabView(this.starTabId);
            this.isDestroyed = true;
        });
        this.setBackgroundColor(computeBgColor(fullConfig));
    }

    get starTabId(): string {
        return this._starTabId;
    }

    set starTabId(starTabId: string) {
        this._starTabId = starTabId;
    }

    setKeyboardChordMode(mode: boolean) {
        this.keyboardChordMode = mode;
        if (mode) {
            if (this.resetChordModeTimeout) {
                clearTimeout(this.resetChordModeTimeout);
            }
            this.resetChordModeTimeout = setTimeout(() => {
                this.keyboardChordMode = false;
            }, CHORD_TIMEOUT);
        } else {
            if (this.resetChordModeTimeout) {
                clearTimeout(this.resetChordModeTimeout);
                this.resetChordModeTimeout = null;
            }
        }
    }

    positionTabOnScreen(winBounds: Rectangle) {
        const curBounds = this.getBounds();
        if (
            curBounds.width == winBounds.width &&
            curBounds.height == winBounds.height &&
            curBounds.x == 0 &&
            curBounds.y == 0
        ) {
            return;
        }
        this.setBounds({ x: 0, y: 0, width: winBounds.width, height: winBounds.height });
    }

    positionTabOffScreen(winBounds: Rectangle) {
        this.setBounds({
            x: -15000,
            y: -15000,
            width: winBounds.width,
            height: winBounds.height,
        });
    }

    isOnScreen() {
        const bounds = this.getBounds();
        return bounds.x == 0 && bounds.y == 0;
    }

    destroy() {
        console.log("destroy tab", this.starTabId);
        removeStarTabView(this.starTabId);
        if (!this.isDestroyed) {
            this.webContents?.close();
        }
        this.isDestroyed = true;
    }
}

let MaxCacheSize = 10;
const wcvCache = new Map<string, StarTabView>();

export function setMaxTabCacheSize(size: number) {
    console.log("setMaxTabCacheSize", size);
    MaxCacheSize = size;
}

export function getStarTabView(starTabId: string): StarTabView | undefined {
    const rtn = wcvCache.get(starTabId);
    if (rtn) {
        rtn.lastUsedTs = Date.now();
    }
    return rtn;
}

function tryEvictEntry(starTabId: string): boolean {
    const tabView = wcvCache.get(starTabId);
    if (!tabView) {
        return false;
    }
    if (tabView.isActiveTab) {
        return false;
    }
    const lastUsedDiff = Date.now() - tabView.lastUsedTs;
    if (lastUsedDiff < 1000) {
        return false;
    }
    const ww = getStarWindowById(tabView.starWindowId);
    if (!ww) {
        // this shouldn't happen, but if it does, just destroy the tabview
        console.log("[error] StarWindow not found for StarTabView", tabView.starTabId);
        tabView.destroy();
        return true;
    } else {
        // will trigger a destroy on the tabview
        ww.removeTabView(tabView.starTabId, false);
        return true;
    }
}

function checkAndEvictCache(): void {
    if (wcvCache.size <= MaxCacheSize) {
        return;
    }
    const sorted = Array.from(wcvCache.values()).sort((a, b) => {
        // Prioritize entries which are active
        if (a.isActiveTab && !b.isActiveTab) {
            return -1;
        }
        // Otherwise, sort by lastUsedTs
        return a.lastUsedTs - b.lastUsedTs;
    });
    const now = Date.now();
    for (let i = 0; i < sorted.length - MaxCacheSize; i++) {
        tryEvictEntry(sorted[i].starTabId);
    }
}

export function clearTabCache() {
    const wcVals = Array.from(wcvCache.values());
    for (let i = 0; i < wcVals.length; i++) {
        const tabView = wcVals[i];
        tryEvictEntry(tabView.starTabId);
    }
}

// returns [tabview, initialized]
export async function getOrCreateWebViewForTab(starWindowId: string, tabId: string): Promise<[StarTabView, boolean]> {
    let tabView = getStarTabView(tabId);
    if (tabView) {
        return [tabView, true];
    }
    const fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
    tabView = getSpareTab(fullConfig);
    tabView.starWindowId = starWindowId;
    tabView.lastUsedTs = Date.now();
    setStarTabView(tabId, tabView);
    tabView.starTabId = tabId;
    tabView.webContents.on("will-navigate", shNavHandler);
    tabView.webContents.on("will-frame-navigate", shFrameNavHandler);
    tabView.webContents.on("did-attach-webview", (event, wc) => {
        wc.setWindowOpenHandler((details) => {
            tabView.webContents.send("webview-new-window", wc.id, details);
            return { action: "deny" };
        });
    });
    tabView.webContents.on("before-input-event", (e, input) => {
        const starEvent = adaptFromElectronKeyEvent(input);
        // console.log("WIN bie", tabView.starTabId.substring(0, 8), starEvent.type, starEvent.code);
        handleCtrlShiftState(tabView.webContents, starEvent);
        setWasActive(true);
        if (input.type == "keyDown" && tabView.keyboardChordMode) {
            e.preventDefault();
            tabView.setKeyboardChordMode(false);
            tabView.webContents.send("reinject-key", starEvent);
        }
    });
    tabView.webContents.on("zoom-changed", (e) => {
        tabView.webContents.send("zoom-changed");
    });
    tabView.webContents.setWindowOpenHandler(({ url, frameName }) => {
        if (url.startsWith("http://") || url.startsWith("https://") || url.startsWith("file://")) {
            console.log("openExternal fallback", url);
            shell.openExternal(url);
        }
        console.log("window-open denied", url);
        return { action: "deny" };
    });
    tabView.webContents.on("blur", () => {
        handleCtrlShiftFocus(tabView.webContents, false);
    });
    configureAuthKeyRequestInjection(tabView.webContents.session);
    return [tabView, false];
}

export function setStarTabView(starTabId: string, wcv: StarTabView): void {
    if (starTabId == null) {
        return;
    }
    wcvCache.set(starTabId, wcv);
    checkAndEvictCache();
}

function removeStarTabView(starTabId: string): void {
    if (starTabId == null) {
        return;
    }
    wcvCache.delete(starTabId);
}

let HotSpareTab: StarTabView = null;

export function ensureHotSpareTab(fullConfig: FullConfigType) {
    console.log("ensureHotSpareTab");
    if (HotSpareTab == null) {
        HotSpareTab = new StarTabView(fullConfig);
    }
}

export function getSpareTab(fullConfig: FullConfigType): StarTabView {
    setTimeout(() => ensureHotSpareTab(fullConfig), 500);
    if (HotSpareTab != null) {
        const rtn = HotSpareTab;
        HotSpareTab = null;
        console.log("getSpareTab: returning hotspare");
        return rtn;
    } else {
        console.log("getSpareTab: creating new tab");
        return new StarTabView(fullConfig);
    }
}
