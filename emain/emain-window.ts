// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { ClientService, ObjectService, WindowService, WorkspaceService } from "@/app/store/services";
import { RpcApi } from "@/app/store/wshclientapi";
import { fireAndForget } from "@/util/util";
import { BaseWindow, BaseWindowConstructorOptions, dialog, globalShortcut, ipcMain, screen } from "electron";
import { globalEvents } from "emain/emain-events";
import path from "path";
import { debounce } from "throttle-debounce";
import {
    getGlobalIsQuitting,
    getGlobalIsRelaunching,
    setGlobalIsRelaunching,
    setWasActive,
    setWasInFg,
} from "./emain-activity";
import { getOrCreateWebViewForTab, getStarTabViewByWebContentsId, StarTabView } from "./emain-tabview";
import { delay, ensureBoundsAreVisible, starKeyToElectronKey } from "./emain-util";
import { ElectronWshClient } from "./emain-wsh";
import { log } from "./log";
import { getElectronAppBasePath, unamePlatform } from "./platform";
import { updater } from "./updater";

export type WindowOpts = {
    unamePlatform: string;
};

export const starWindowMap = new Map<string, StarBrowserWindow>(); // starWindowId -> StarBrowserWindow

// on blur we do not set this to null (but on destroy we do), so this tracks the *last* focused window
// e.g. it persists when the app itself is not focused
export let focusedStarWindow: StarBrowserWindow = null;

let cachedClientId: string = null;

async function getClientId() {
    if (cachedClientId != null) {
        return cachedClientId;
    }
    const clientData = await ClientService.GetClientData();
    cachedClientId = clientData?.oid;
    return cachedClientId;
}

type WindowActionQueueEntry =
    | {
          op: "switchtab";
          tabId: string;
          setInBackend: boolean;
      }
    | {
          op: "createtab";
          pinned: boolean;
      }
    | {
          op: "closetab";
          tabId: string;
      }
    | {
          op: "switchworkspace";
          workspaceId: string;
      };

function isNonEmptyUnsavedWorkspace(workspace: Workspace): boolean {
    return !workspace.name && !workspace.icon && (workspace.tabids?.length > 1 || workspace.pinnedtabids?.length > 1);
}

export class StarBrowserWindow extends BaseWindow {
    starWindowId: string;
    workspaceId: string;
    allLoadedTabViews: Map<string, StarTabView>;
    activeTabView: StarTabView;
    private canClose: boolean;
    private deleteAllowed: boolean;
    private actionQueue: WindowActionQueueEntry[];

    constructor(starWindow: StarWindow, fullConfig: FullConfigType, opts: WindowOpts) {
        const settings = fullConfig?.settings;

        console.log("create win", starWindow.oid);
        let winWidth = starWindow?.winsize?.width;
        let winHeight = starWindow?.winsize?.height;
        let winPosX = starWindow.pos.x;
        let winPosY = starWindow.pos.y;

        if (
            (winWidth == null || winWidth === 0 || winHeight == null || winHeight === 0) &&
            settings?.["window:dimensions"]
        ) {
            const dimensions = settings["window:dimensions"];
            const match = dimensions.match(/^(\d+)[xX](\d+)$/);

            if (match) {
                const [, dimensionWidth, dimensionHeight] = match;
                const parsedWidth = parseInt(dimensionWidth, 10);
                const parsedHeight = parseInt(dimensionHeight, 10);

                if ((!winWidth || winWidth === 0) && Number.isFinite(parsedWidth) && parsedWidth > 0) {
                    winWidth = parsedWidth;
                }
                if ((!winHeight || winHeight === 0) && Number.isFinite(parsedHeight) && parsedHeight > 0) {
                    winHeight = parsedHeight;
                }
            } else {
                console.warn('Invalid window:dimensions format. Expected "widthxheight".');
            }
        }

        if (winWidth == null || winWidth == 0) {
            const primaryDisplay = screen.getPrimaryDisplay();
            const { width } = primaryDisplay.workAreaSize;
            winWidth = width - winPosX - 100;
            if (winWidth > 2000) {
                winWidth = 2000;
            }
        }
        if (winHeight == null || winHeight == 0) {
            const primaryDisplay = screen.getPrimaryDisplay();
            const { height } = primaryDisplay.workAreaSize;
            winHeight = height - winPosY - 100;
            if (winHeight > 1200) {
                winHeight = 1200;
            }
        }
        let winBounds = {
            x: winPosX,
            y: winPosY,
            width: winWidth,
            height: winHeight,
        };
        winBounds = ensureBoundsAreVisible(winBounds);
        const winOpts: BaseWindowConstructorOptions = {
            titleBarStyle:
                opts.unamePlatform === "darwin"
                    ? "hiddenInset"
                    : settings["window:nativetitlebar"]
                      ? "default"
                      : "hidden",
            titleBarOverlay:
                opts.unamePlatform !== "darwin"
                    ? {
                          symbolColor: "white",
                          color: "#00000000",
                      }
                    : false,
            x: winBounds.x,
            y: winBounds.y,
            width: winBounds.width,
            height: winBounds.height,
            minWidth: 400,
            minHeight: 300,
            icon:
                opts.unamePlatform == "linux"
                    ? path.join(getElectronAppBasePath(), "public/logos/star-logo-dark.png")
                    : undefined,
            show: false,
            autoHideMenuBar: !settings?.["window:showmenubar"],
        };
        const isTransparent = settings?.["window:transparent"] ?? false;
        const isBlur = !isTransparent && (settings?.["window:blur"] ?? false);
        if (isTransparent) {
            winOpts.transparent = true;
        } else if (isBlur) {
            switch (opts.unamePlatform) {
                case "win32": {
                    winOpts.backgroundMaterial = "acrylic";
                    break;
                }
                case "darwin": {
                    winOpts.vibrancy = "fullscreen-ui";
                    break;
                }
            }
        } else {
            winOpts.backgroundColor = "#222222";
        }

        super(winOpts);
        this.actionQueue = [];
        this.starWindowId = starWindow.oid;
        this.workspaceId = starWindow.workspaceid;
        this.allLoadedTabViews = new Map<string, StarTabView>();
        const winBoundsPoller = setInterval(() => {
            if (this.isDestroyed()) {
                clearInterval(winBoundsPoller);
                return;
            }
            if (this.actionQueue.length > 0) {
                return;
            }
            this.finalizePositioning();
        }, 1000);
        this.on(
            // @ts-expect-error
            "resize",
            debounce(400, (e) => this.mainResizeHandler(e))
        );
        this.on("resize", () => {
            if (this.isDestroyed()) {
                return;
            }
            this.activeTabView?.positionTabOnScreen(this.getContentBounds());
        });
        this.on(
            // @ts-expect-error
            "move",
            debounce(400, (e) => this.mainResizeHandler(e))
        );
        this.on("enter-full-screen", async () => {
            if (this.isDestroyed()) {
                return;
            }
            console.log("enter-full-screen event", this.getContentBounds());
            const tabView = this.activeTabView;
            if (tabView) {
                tabView.webContents.send("fullscreen-change", true);
            }
            this.activeTabView?.positionTabOnScreen(this.getContentBounds());
        });
        this.on("leave-full-screen", async () => {
            if (this.isDestroyed()) {
                return;
            }
            const tabView = this.activeTabView;
            if (tabView) {
                tabView.webContents.send("fullscreen-change", false);
            }
            this.activeTabView?.positionTabOnScreen(this.getContentBounds());
        });
        this.on("focus", () => {
            if (this.isDestroyed()) {
                return;
            }
            if (getGlobalIsRelaunching()) {
                return;
            }
            focusedStarWindow = this;
            console.log("focus win", this.starWindowId);
            fireAndForget(() => ClientService.FocusWindow(this.starWindowId));
            setWasInFg(true);
            setWasActive(true);
        });
        this.on("blur", () => {
            // nothing for now
        });
        this.on("close", (e) => {
            if (this.canClose) {
                return;
            }
            if (this.isDestroyed()) {
                return;
            }
            console.log("win 'close' handler fired", this.starWindowId);
            if (getGlobalIsQuitting() || updater?.status == "installing" || getGlobalIsRelaunching()) {
                return;
            }
            e.preventDefault();
            fireAndForget(async () => {
                const numWindows = starWindowMap.size;
                const fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
                if (numWindows > 1 || !fullConfig.settings["window:savelastwindow"]) {
                    if (fullConfig.settings["window:confirmclose"]) {
                        const workspace = await WorkspaceService.GetWorkspace(this.workspaceId);
                        if (isNonEmptyUnsavedWorkspace(workspace)) {
                            const choice = dialog.showMessageBoxSync(this, {
                                type: "question",
                                buttons: ["Cancel", "Close Window"],
                                title: "Confirm",
                                message:
                                    "Window has unsaved tabs, closing window will delete existing tabs.\n\nContinue?",
                            });
                            if (choice === 0) {
                                return;
                            }
                        }
                    }
                    this.deleteAllowed = true;
                }
                this.canClose = true;
                this.close();
            });
        });
        this.on("closed", () => {
            console.log("win 'closed' handler fired", this.starWindowId);
            if (getGlobalIsQuitting() || updater?.status == "installing") {
                console.log("win quitting or updating", this.starWindowId);
                return;
            }
            setTimeout(() => globalEvents.emit("windows-updated"), 50);
            starWindowMap.delete(this.starWindowId);
            if (focusedStarWindow == this) {
                focusedStarWindow = null;
            }
            this.removeAllChildViews();
            if (getGlobalIsRelaunching()) {
                console.log("win relaunching", this.starWindowId);
                this.destroy();
                return;
            }
            if (this.deleteAllowed) {
                console.log("win removing window from backend DB", this.starWindowId);
                fireAndForget(() => WindowService.CloseWindow(this.starWindowId, true));
            }
        });
        starWindowMap.set(starWindow.oid, this);
        setTimeout(() => globalEvents.emit("windows-updated"), 50);
    }

    private removeAllChildViews() {
        for (const tabView of this.allLoadedTabViews.values()) {
            if (!this.isDestroyed()) {
                this.contentView.removeChildView(tabView);
            }
            tabView?.destroy();
        }
    }

    async switchWorkspace(workspaceId: string) {
        console.log("switchWorkspace", workspaceId, this.starWindowId);
        if (workspaceId == this.workspaceId) {
            console.log("switchWorkspace already on this workspace", this.starWindowId);
            return;
        }

        // If the workspace is already owned by a window, then we can just call SwitchWorkspace without first prompting the user, since it'll just focus to the other window.
        const workspaceList = await WorkspaceService.ListWorkspaces();
        if (!workspaceList?.find((wse) => wse.workspaceid === workspaceId)?.windowid) {
            const curWorkspace = await WorkspaceService.GetWorkspace(this.workspaceId);

            if (curWorkspace && isNonEmptyUnsavedWorkspace(curWorkspace)) {
                console.log(
                    `existing unsaved workspace ${this.workspaceId} has content, opening workspace ${workspaceId} in new window`
                );
                await createWindowForWorkspace(workspaceId);
                return;
            }
        }
        await this._queueActionInternal({ op: "switchworkspace", workspaceId });
    }

    async setActiveTab(tabId: string, setInBackend: boolean) {
        console.log("setActiveTab", tabId, this.starWindowId, this.workspaceId, setInBackend);
        await this._queueActionInternal({ op: "switchtab", tabId, setInBackend });
    }

    private async initializeTab(tabView: StarTabView) {
        const clientId = await getClientId();
        await tabView.initPromise;
        this.contentView.addChildView(tabView);
        const initOpts = {
            tabId: tabView.starTabId,
            clientId: clientId,
            windowId: this.starWindowId,
            activate: true,
        };
        tabView.savedInitOpts = { ...initOpts };
        tabView.savedInitOpts.activate = false;
        let startTime = Date.now();
        console.log("before star ready, init tab, sending star-init", tabView.starTabId);
        tabView.webContents.send("star-init", initOpts);
        await tabView.starReadyPromise;
        console.log("star-ready init time", Date.now() - startTime + "ms");
    }

    private async setTabViewIntoWindow(tabView: StarTabView, tabInitialized: boolean) {
        if (this.activeTabView == tabView) {
            return;
        }
        const oldActiveView = this.activeTabView;
        tabView.isActiveTab = true;
        if (oldActiveView != null) {
            oldActiveView.isActiveTab = false;
        }
        this.activeTabView = tabView;
        this.allLoadedTabViews.set(tabView.starTabId, tabView);
        if (!tabInitialized) {
            console.log("initializing a new tab");
            const p1 = this.initializeTab(tabView);
            const p2 = this.repositionTabsSlowly(100);
            await Promise.all([p1, p2]);
        } else {
            console.log("reusing an existing tab, calling star-init", tabView.starTabId);
            const p1 = this.repositionTabsSlowly(35);
            const p2 = tabView.webContents.send("star-init", tabView.savedInitOpts); // reinit
            await Promise.all([p1, p2]);
        }

        // something is causing the new tab to lose focus so it requires manual refocusing
        tabView.webContents.focus();
        setTimeout(() => {
            if (tabView.webContents && this.activeTabView == tabView && !tabView.webContents.isFocused()) {
                tabView.webContents.focus();
            }
        }, 10);
        setTimeout(() => {
            if (tabView.webContents && this.activeTabView == tabView && !tabView.webContents.isFocused()) {
                tabView.webContents.focus();
            }
        }, 30);
    }

    private async repositionTabsSlowly(delayMs: number) {
        const activeTabView = this.activeTabView;
        const winBounds = this.getContentBounds();
        if (activeTabView == null) {
            return;
        }
        if (activeTabView.isOnScreen()) {
            activeTabView.setBounds({
                x: 0,
                y: 0,
                width: winBounds.width,
                height: winBounds.height,
            });
        } else {
            activeTabView.setBounds({
                x: winBounds.width - 10,
                y: winBounds.height - 10,
                width: winBounds.width,
                height: winBounds.height,
            });
        }
        await delay(delayMs);
        if (this.activeTabView != activeTabView) {
            // another tab view has been set, do not finalize this layout
            return;
        }
        this.finalizePositioning();
    }

    private finalizePositioning() {
        if (this.isDestroyed()) {
            return;
        }
        const curBounds = this.getContentBounds();
        this.activeTabView?.positionTabOnScreen(curBounds);
        for (const tabView of this.allLoadedTabViews.values()) {
            if (tabView == this.activeTabView) {
                continue;
            }
            tabView?.positionTabOffScreen(curBounds);
        }
    }

    async queueCreateTab(pinned = false) {
        await this._queueActionInternal({ op: "createtab", pinned });
    }

    async queueCloseTab(tabId: string) {
        await this._queueActionInternal({ op: "closetab", tabId });
    }

    private async _queueActionInternal(entry: WindowActionQueueEntry) {
        if (this.actionQueue.length >= 2) {
            this.actionQueue[1] = entry;
            return;
        }
        const wasEmpty = this.actionQueue.length === 0;
        this.actionQueue.push(entry);
        if (wasEmpty) {
            await this.processActionQueue();
        }
    }

    private removeTabViewLater(tabId: string, delayMs: number) {
        setTimeout(() => {
            this.removeTabView(tabId, false);
        }, 1000);
    }

    // the queue and this function are used to serialize operations that update the window contents view
    // processActionQueue will replace [1] if it is already set
    // we don't mess with [0] because it is "in process"
    // we replace [1] because there is no point to run an action that is going to be overwritten
    private async processActionQueue() {
        while (this.actionQueue.length > 0) {
            try {
                if (this.isDestroyed()) {
                    break;
                }
                const entry = this.actionQueue[0];
                let tabId: string = null;
                // have to use "===" here to get the typechecker to work :/
                switch (entry.op) {
                    case "createtab":
                        tabId = await WorkspaceService.CreateTab(this.workspaceId, null, true, entry.pinned);
                        break;
                    case "switchtab":
                        tabId = entry.tabId;
                        if (this.activeTabView?.starTabId == tabId) {
                            continue;
                        }
                        if (entry.setInBackend) {
                            await WorkspaceService.SetActiveTab(this.workspaceId, tabId);
                        }
                        break;
                    case "closetab":
                        tabId = entry.tabId;
                        const rtn = await WorkspaceService.CloseTab(this.workspaceId, tabId, true);
                        if (rtn == null) {
                            console.log(
                                "[error] closeTab: no return value",
                                tabId,
                                this.workspaceId,
                                this.starWindowId
                            );
                            return;
                        }
                        this.removeTabViewLater(tabId, 1000);
                        if (rtn.closewindow) {
                            this.close();
                            return;
                        }
                        if (!rtn.newactivetabid) {
                            return;
                        }
                        tabId = rtn.newactivetabid;
                        break;
                    case "switchworkspace":
                        const newWs = await WindowService.SwitchWorkspace(this.starWindowId, entry.workspaceId);
                        if (!newWs) {
                            return;
                        }
                        console.log("processActionQueue switchworkspace newWs", newWs);
                        this.removeAllChildViews();
                        console.log("destroyed all tabs", this.starWindowId);
                        this.workspaceId = entry.workspaceId;
                        this.allLoadedTabViews = new Map();
                        tabId = newWs.activetabid;
                        break;
                }
                if (tabId == null) {
                    return;
                }
                const [tabView, tabInitialized] = await getOrCreateWebViewForTab(this.starWindowId, tabId);
                await this.setTabViewIntoWindow(tabView, tabInitialized);
            } catch (e) {
                console.log("error caught in processActionQueue", e);
            } finally {
                this.actionQueue.shift();
            }
        }
    }

    private async mainResizeHandler(_: any) {
        if (this == null || this.isDestroyed() || this.fullScreen) {
            return;
        }
        const bounds = this.getBounds();
        try {
            await WindowService.SetWindowPosAndSize(
                this.starWindowId,
                { x: bounds.x, y: bounds.y },
                { width: bounds.width, height: bounds.height }
            );
        } catch (e) {
            console.log("error sending new window bounds to backend", e);
        }
    }

    removeTabView(tabId: string, force: boolean) {
        if (!force && this.activeTabView?.starTabId == tabId) {
            console.log("cannot remove active tab", tabId, this.starWindowId);
            return;
        }
        const tabView = this.allLoadedTabViews.get(tabId);
        if (tabView == null) {
            console.log("removeTabView -- tabView not found", tabId, this.starWindowId);
            // the tab was never loaded, so just return
            return;
        }
        this.contentView.removeChildView(tabView);
        this.allLoadedTabViews.delete(tabId);
        tabView.destroy();
    }

    destroy() {
        console.log("destroy win", this.starWindowId);
        this.deleteAllowed = true;
        super.destroy();
    }
}

export function getStarWindowByTabId(tabId: string): StarBrowserWindow {
    for (const ww of starWindowMap.values()) {
        if (ww.allLoadedTabViews.has(tabId)) {
            return ww;
        }
    }
}

export function getStarWindowByWebContentsId(webContentsId: number): StarBrowserWindow {
    const tabView = getStarTabViewByWebContentsId(webContentsId);
    if (tabView == null) {
        return null;
    }
    return getStarWindowByTabId(tabView.starTabId);
}

export function getStarWindowById(windowId: string): StarBrowserWindow {
    return starWindowMap.get(windowId);
}

export function getStarWindowByWorkspaceId(workspaceId: string): StarBrowserWindow {
    for (const starWindow of starWindowMap.values()) {
        if (starWindow.workspaceId === workspaceId) {
            return starWindow;
        }
    }
}

export function getAllStarWindows(): StarBrowserWindow[] {
    return Array.from(starWindowMap.values());
}

export async function createWindowForWorkspace(workspaceId: string) {
    const newWin = await WindowService.CreateWindow(null, workspaceId);
    if (!newWin) {
        console.log("error creating new window", this.starWindowId);
    }
    const newBwin = await createBrowserWindow(newWin, await RpcApi.GetFullConfigCommand(ElectronWshClient), {
        unamePlatform,
    });
    newBwin.show();
}

// note, this does not *show* the window.
// to show, await win.readyPromise and then win.show()
export async function createBrowserWindow(
    starWindow: StarWindow,
    fullConfig: FullConfigType,
    opts: WindowOpts
): Promise<StarBrowserWindow> {
    if (!starWindow) {
        console.log("createBrowserWindow: no starWindow");
        starWindow = await WindowService.CreateWindow(null, "");
    }
    let workspace = await WorkspaceService.GetWorkspace(starWindow.workspaceid);
    if (!workspace) {
        console.log("createBrowserWindow: no workspace, creating new window");
        await WindowService.CloseWindow(starWindow.oid, true);
        starWindow = await WindowService.CreateWindow(null, "");
        workspace = await WorkspaceService.GetWorkspace(starWindow.workspaceid);
    }
    console.log("createBrowserWindow", starWindow.oid, workspace.oid, workspace);
    const bwin = new StarBrowserWindow(starWindow, fullConfig, opts);
    if (workspace.activetabid) {
        await bwin.setActiveTab(workspace.activetabid, false);
    }
    return bwin;
}

ipcMain.on("set-active-tab", async (event, tabId) => {
    const ww = getStarWindowByWebContentsId(event.sender.id);
    console.log("set-active-tab", tabId, ww?.starWindowId);
    await ww?.setActiveTab(tabId, true);
});

ipcMain.on("create-tab", async (event, opts) => {
    const senderWc = event.sender;
    const ww = getStarWindowByWebContentsId(senderWc.id);
    if (ww != null) {
        await ww.queueCreateTab();
    }
    event.returnValue = true;
    return null;
});

ipcMain.on("close-tab", async (event, workspaceId, tabId) => {
    const ww = getStarWindowByWorkspaceId(workspaceId);
    if (ww == null) {
        console.log(`close-tab: no window found for workspace ws=${workspaceId} tab=${tabId}`);
        return;
    }
    await ww.queueCloseTab(tabId);
    event.returnValue = true;
    return null;
});

ipcMain.on("switch-workspace", (event, workspaceId) => {
    fireAndForget(async () => {
        const ww = getStarWindowByWebContentsId(event.sender.id);
        console.log("switch-workspace", workspaceId, ww?.starWindowId);
        await ww?.switchWorkspace(workspaceId);
    });
});

export async function createWorkspace(window: StarBrowserWindow) {
    const newWsId = await WorkspaceService.CreateWorkspace("", "", "", true);
    if (newWsId) {
        if (window) {
            await window.switchWorkspace(newWsId);
        } else {
            await createWindowForWorkspace(newWsId);
        }
    }
}

ipcMain.on("create-workspace", (event) => {
    fireAndForget(async () => {
        const ww = getStarWindowByWebContentsId(event.sender.id);
        console.log("create-workspace", ww?.starWindowId);
        await createWorkspace(ww);
    });
});

ipcMain.on("delete-workspace", (event, workspaceId) => {
    fireAndForget(async () => {
        const ww = getStarWindowByWebContentsId(event.sender.id);
        console.log("delete-workspace", workspaceId, ww?.starWindowId);

        const workspaceList = await WorkspaceService.ListWorkspaces();

        const workspaceHasWindow = !!workspaceList.find((wse) => wse.workspaceid === workspaceId)?.windowid;

        const choice = dialog.showMessageBoxSync(this, {
            type: "question",
            buttons: ["Cancel", "Delete Workspace"],
            title: "Confirm",
            message: `Deleting workspace will also delete its contents.\n\nContinue?`,
        });
        if (choice === 0) {
            console.log("user cancelled workspace delete", workspaceId, ww?.starWindowId);
            return;
        }

        const newWorkspaceId = await WorkspaceService.DeleteWorkspace(workspaceId);
        console.log("delete-workspace done", workspaceId, ww?.starWindowId);
        if (ww?.workspaceId == workspaceId) {
            if (newWorkspaceId) {
                await ww.switchWorkspace(newWorkspaceId);
            } else {
                console.log("delete-workspace closing window", workspaceId, ww?.starWindowId);
                ww.destroy();
            }
        }
    });
});

export async function createNewStarWindow() {
    log("createNewStarWindow");
    const clientData = await ClientService.GetClientData();
    const fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
    let recreatedWindow = false;
    const allWindows = getAllStarWindows();
    if (allWindows.length === 0 && clientData?.windowids?.length >= 1) {
        console.log("no windows, but clientData has windowids, recreating first window");
        // reopen the first window
        const existingWindowId = clientData.windowids[0];
        const existingWindowData = (await ObjectService.GetObject("window:" + existingWindowId)) as StarWindow;
        if (existingWindowData != null) {
            const win = await createBrowserWindow(existingWindowData, fullConfig, { unamePlatform });
            win.show();
            recreatedWindow = true;
        }
    }
    if (recreatedWindow) {
        console.log("recreated window, returning");
        return;
    }
    console.log("creating new window");
    const newBrowserWindow = await createBrowserWindow(null, fullConfig, { unamePlatform });
    newBrowserWindow.show();
}

export async function relaunchBrowserWindows() {
    console.log("relaunchBrowserWindows");
    setGlobalIsRelaunching(true);
    const windows = getAllStarWindows();
    if (windows.length > 0) {
        for (const window of windows) {
            console.log("relaunch -- closing window", window.starWindowId);
            window.close();
        }
        await delay(1200);
    }
    setGlobalIsRelaunching(false);

    const clientData = await ClientService.GetClientData();
    const fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
    const wins: StarBrowserWindow[] = [];
    for (const windowId of clientData.windowids.slice().reverse()) {
        const windowData: StarWindow = await WindowService.GetWindow(windowId);
        if (windowData == null) {
            console.log("relaunch -- window data not found, closing window", windowId);
            await WindowService.CloseWindow(windowId, true);
            continue;
        }
        console.log("relaunch -- creating window", windowId, windowData);
        const win = await createBrowserWindow(windowData, fullConfig, { unamePlatform });
        wins.push(win);
    }
    for (const win of wins) {
        console.log("show window", win.starWindowId);
        win.show();
    }
}

export function registerGlobalHotkey(rawGlobalHotKey: string) {
    try {
        const electronHotKey = starKeyToElectronKey(rawGlobalHotKey);
        console.log("registering globalhotkey of ", electronHotKey);
        globalShortcut.register(electronHotKey, () => {
            const selectedWindow = focusedStarWindow;
            const firstStarWindow = getAllStarWindows()[0];
            if (focusedStarWindow) {
                selectedWindow.focus();
            } else if (firstStarWindow) {
                firstStarWindow.focus();
            } else {
                fireAndForget(createNewStarWindow);
            }
        });
    } catch (e) {
        console.log("error registering global hotkey: ", e);
    }
}
