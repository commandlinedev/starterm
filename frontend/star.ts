// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { App } from "@/app/app";
import {
    globalRefocus,
    registerControlShiftStateUpdateHandler,
    registerElectronReinjectKeyHandler,
    registerGlobalKeys,
} from "@/app/store/keymodel";
import { modalsModel } from "@/app/store/modalmodel";
import { RpcApi } from "@/app/store/wshclientapi";
import { initWshrpc, TabRpcClient } from "@/app/store/wshrpcutil";
import { loadMonaco } from "@/app/view/codeeditor/codeeditor";
import { getLayoutModelForStaticTab } from "@/layout/index";
import {
    atoms,
    countersClear,
    countersPrint,
    getApi,
    globalStore,
    initGlobal,
    initGlobalStarEventSubs,
    loadConnStatus,
    pushFlashError,
    pushNotification,
    removeNotificationById,
    subscribeToConnEvents,
} from "@/store/global";
import * as WOS from "@/store/wos";
import { loadFonts } from "@/util/fontutil";
import { setKeyUtilPlatform } from "@/util/keyutil";
import { createElement } from "react";
import { createRoot } from "react-dom/client";

const platform = getApi().getPlatform();
document.title = `Star Terminal`;
let savedInitOpts: StarInitOpts = null;

(window as any).WOS = WOS;
(window as any).globalStore = globalStore;
(window as any).globalAtoms = atoms;
(window as any).RpcApi = RpcApi;
(window as any).isFullScreen = false;
(window as any).countersPrint = countersPrint;
(window as any).countersClear = countersClear;
(window as any).getLayoutModelForStaticTab = getLayoutModelForStaticTab;
(window as any).pushFlashError = pushFlashError;
(window as any).pushNotification = pushNotification;
(window as any).removeNotificationById = removeNotificationById;
(window as any).modalsModel = modalsModel;

async function initBare() {
    getApi().sendLog("Init Bare");
    document.body.style.visibility = "hidden";
    document.body.style.opacity = "0";
    document.body.classList.add("is-transparent");
    getApi().onStarInit(initStarWrap);
    setKeyUtilPlatform(platform);
    loadFonts();
    document.fonts.ready.then(() => {
        console.log("Init Bare Done");
        getApi().setWindowInitStatus("ready");
    });
}

document.addEventListener("DOMContentLoaded", initBare);

async function initStarWrap(initOpts: StarInitOpts) {
    try {
        if (savedInitOpts) {
            await reinitStar();
            return;
        }
        savedInitOpts = initOpts;
        await initStar(initOpts);
    } catch (e) {
        getApi().sendLog("Error in initStar " + e.message);
        console.error("Error in initStar", e);
    } finally {
        document.body.style.visibility = null;
        document.body.style.opacity = null;
        document.body.classList.remove("is-transparent");
    }
}

async function reinitStar() {
    console.log("Reinit Star");
    getApi().sendLog("Reinit Star");

    // We use this hack to prevent a flicker of the previously-hovered tab when this view was last active.
    document.body.classList.add("nohover");
    requestAnimationFrame(() =>
        setTimeout(() => {
            document.body.classList.remove("nohover");
        }, 100)
    );

    await WOS.reloadStarObject<Client>(WOS.makeORef("client", savedInitOpts.clientId));
    const starWindow = await WOS.reloadStarObject<StarWindow>(WOS.makeORef("window", savedInitOpts.windowId));
    const ws = await WOS.reloadStarObject<Workspace>(WOS.makeORef("workspace", starWindow.workspaceid));
    const initialTab = await WOS.reloadStarObject<Tab>(WOS.makeORef("tab", savedInitOpts.tabId));
    await WOS.reloadStarObject<LayoutState>(WOS.makeORef("layout", initialTab.layoutstate));
    reloadAllWorkspaceTabs(ws);
    document.title = `Star Terminal - ${initialTab.name}`; // TODO update with tab name change
    getApi().setWindowInitStatus("star-ready");
    globalStore.set(atoms.reinitVersion, globalStore.get(atoms.reinitVersion) + 1);
    globalStore.set(atoms.updaterStatusAtom, getApi().getUpdaterStatus());
    setTimeout(() => {
        globalRefocus();
    }, 50);
}

function reloadAllWorkspaceTabs(ws: Workspace) {
    if (ws == null || (!ws.tabids?.length && !ws.pinnedtabids?.length)) {
        return;
    }
    ws.tabids?.forEach((tabid) => {
        WOS.reloadStarObject<Tab>(WOS.makeORef("tab", tabid));
    });
    ws.pinnedtabids?.forEach((tabid) => {
        WOS.reloadStarObject<Tab>(WOS.makeORef("tab", tabid));
    });
}

function loadAllWorkspaceTabs(ws: Workspace) {
    if (ws == null || (!ws.tabids?.length && !ws.pinnedtabids?.length)) {
        return;
    }
    ws.tabids?.forEach((tabid) => {
        WOS.getObjectValue<Tab>(WOS.makeORef("tab", tabid));
    });
    ws.pinnedtabids?.forEach((tabid) => {
        WOS.getObjectValue<Tab>(WOS.makeORef("tab", tabid));
    });
}

async function initStar(initOpts: StarInitOpts) {
    getApi().sendLog("Init Star " + JSON.stringify(initOpts));
    console.log(
        "Star Init",
        "tabid",
        initOpts.tabId,
        "clientid",
        initOpts.clientId,
        "windowid",
        initOpts.windowId,
        "platform",
        platform
    );
    initGlobal({
        tabId: initOpts.tabId,
        clientId: initOpts.clientId,
        windowId: initOpts.windowId,
        platform,
        environment: "renderer",
    });
    (window as any).globalAtoms = atoms;

    // Init WPS event handlers
    const globalWS = initWshrpc(initOpts.tabId);
    (window as any).globalWS = globalWS;
    (window as any).TabRpcClient = TabRpcClient;
    await loadConnStatus();
    initGlobalStarEventSubs(initOpts);
    subscribeToConnEvents();

    // ensures client/window/workspace are loaded into the cache before rendering
    const [client, starWindow, initialTab] = await Promise.all([
        WOS.loadAndPinStarObject<Client>(WOS.makeORef("client", initOpts.clientId)),
        WOS.loadAndPinStarObject<StarWindow>(WOS.makeORef("window", initOpts.windowId)),
        WOS.loadAndPinStarObject<Tab>(WOS.makeORef("tab", initOpts.tabId)),
    ]);
    const [ws, layoutState] = await Promise.all([
        WOS.loadAndPinStarObject<Workspace>(WOS.makeORef("workspace", starWindow.workspaceid)),
        WOS.reloadStarObject<LayoutState>(WOS.makeORef("layout", initialTab.layoutstate)),
    ]);
    loadAllWorkspaceTabs(ws);
    WOS.wpsSubscribeToObject(WOS.makeORef("workspace", starWindow.workspaceid));

    document.title = `Star Terminal - ${initialTab.name}`; // TODO update with tab name change

    registerGlobalKeys();
    registerElectronReinjectKeyHandler();
    registerControlShiftStateUpdateHandler();
    await loadMonaco();
    const fullConfig = await RpcApi.GetFullConfigCommand(TabRpcClient);
    console.log("fullconfig", fullConfig);
    globalStore.set(atoms.fullConfigAtom, fullConfig);
    console.log("Star First Render");
    let firstRenderResolveFn: () => void = null;
    let firstRenderPromise = new Promise<void>((resolve) => {
        firstRenderResolveFn = resolve;
    });
    const reactElem = createElement(App, { onFirstRender: firstRenderResolveFn }, null);
    const elem = document.getElementById("main");
    const root = createRoot(elem);
    root.render(reactElem);
    await firstRenderPromise;
    console.log("Star First Render Done");
    getApi().setWindowInitStatus("star-ready");
}
