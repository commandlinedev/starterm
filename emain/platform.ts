// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { fireAndForget } from "@/util/util";
import { app, dialog, ipcMain, shell } from "electron";
import envPaths from "env-paths";
import { existsSync, mkdirSync } from "fs";
import os from "os";
import path from "path";
import { StarDevVarName, StarDevViteVarName } from "../frontend/util/isdev";
import * as keyutil from "../frontend/util/keyutil";

// This is a little trick to ensure that Electron puts all its runtime data into a subdirectory to avoid conflicts with our own data.
// On macOS, it will store to ~/Library/Application \Support/starterm/electron
// On Linux, it will store to ~/.config/starterm/electron
// On Windows, it will store to %LOCALAPPDATA%/starterm/electron
app.setName("starterm/electron");

const isDev = !app.isPackaged;
const isDevVite = isDev && process.env.ELECTRON_RENDERER_URL;
console.log(`Running in ${isDev ? "development" : "production"} mode`);
if (isDev) {
    process.env[StarDevVarName] = "1";
}
if (isDevVite) {
    process.env[StarDevViteVarName] = "1";
}

const starDirNamePrefix = "starterm";
const starDirNameSuffix = isDev ? "dev" : "";
const starDirName = `${starDirNamePrefix}${starDirNameSuffix ? `-${starDirNameSuffix}` : ""}`;

const paths = envPaths("starterm", { suffix: starDirNameSuffix });

app.setName(isDev ? "Star (Dev)" : "Star");
const unamePlatform = process.platform;
const unameArch: string = process.arch;
keyutil.setKeyUtilPlatform(unamePlatform);

const StarConfigHomeVarName = "STARTERM_CONFIG_HOME";
const StarDataHomeVarName = "STARTERM_DATA_HOME";
const StarHomeVarName = "STARTERM_HOME";

export function checkIfRunningUnderARM64Translation(fullConfig: FullConfigType) {
    if (!fullConfig.settings["app:dismissarchitecturewarning"] && app.runningUnderARM64Translation) {
        console.log("Running under ARM64 translation, alerting user");
        const dialogOpts: Electron.MessageBoxOptions = {
            type: "warning",
            buttons: ["Dismiss", "Learn More"],
            title: "Star has detected a performance issue",
            message: `Star is running in ARM64 translation mode which may impact performance.\n\nRecommendation: Download the native ARM64 version from our website for optimal performance.`,
        };

        const choice = dialog.showMessageBoxSync(null, dialogOpts);
        if (choice === 1) {
            // Open the documentation URL
            console.log("User chose to learn more");
            fireAndForget(() =>
                shell.openExternal(
                    "https://docs.starterm.dev/faq#why-does-star-warn-me-about-arm64-translation-when-it-launches"
                )
            );
            throw new Error("User redirected to docsite to learn more about ARM64 translation, exiting");
        } else {
            console.log("User dismissed the dialog");
        }
    }
}

/**
 * Gets the path to the old Star home directory (defaults to `~/.starterm`).
 * @returns The path to the directory if it exists and contains valid data for the current app, otherwise null.
 */
function getStarHomeDir(): string {
    let home = process.env[StarHomeVarName];
    if (!home) {
        const homeDir = app.getPath("home");
        if (homeDir) {
            home = path.join(homeDir, `.${starDirName}`);
        }
    }
    // If home exists and it has `star.lock` in it, we know it has valid data from Star >=v0.8. Otherwise, it could be for StarLegacy (<v0.8)
    if (home && existsSync(home) && existsSync(path.join(home, "star.lock"))) {
        return home;
    }
    return null;
}

/**
 * Ensure the given path exists, creating it recursively if it doesn't.
 * @param path The path to ensure.
 * @returns The same path, for chaining.
 */
function ensurePathExists(path: string): string {
    if (!existsSync(path)) {
        mkdirSync(path, { recursive: true });
    }
    return path;
}

/**
 * Gets the path to the directory where Star configurations are stored. Creates the directory if it does not exist.
 * Handles backwards compatibility with the old Star Home directory model, where configurations and data were stored together.
 * @returns The path where configurations should be stored.
 */
function getStarConfigDir(): string {
    // If star home dir exists, use it for backwards compatibility
    const starHomeDir = getStarHomeDir();
    if (starHomeDir) {
        return path.join(starHomeDir, "config");
    }

    const override = process.env[StarConfigHomeVarName];
    const xdgConfigHome = process.env.XDG_CONFIG_HOME;
    let retVal: string;
    if (override) {
        retVal = override;
    } else if (xdgConfigHome) {
        retVal = path.join(xdgConfigHome, starDirName);
    } else {
        retVal = path.join(app.getPath("home"), ".config", starDirName);
    }
    return ensurePathExists(retVal);
}

/**
 * Gets the path to the directory where Star data is stored. Creates the directory if it does not exist.
 * Handles backwards compatibility with the old Star Home directory model, where configurations and data were stored together.
 * @returns The path where data should be stored.
 */
function getStarDataDir(): string {
    // If star home dir exists, use it for backwards compatibility
    const starHomeDir = getStarHomeDir();
    if (starHomeDir) {
        return starHomeDir;
    }

    const override = process.env[StarDataHomeVarName];
    const xdgDataHome = process.env.XDG_DATA_HOME;
    let retVal: string;
    if (override) {
        retVal = override;
    } else if (xdgDataHome) {
        retVal = path.join(xdgDataHome, starDirName);
    } else {
        retVal = paths.data;
    }
    return ensurePathExists(retVal);
}

function getElectronAppBasePath(): string {
    return path.dirname(import.meta.dirname);
}

function getElectronAppUnpackedBasePath(): string {
    return getElectronAppBasePath().replace("app.asar", "app.asar.unpacked");
}

const starsrvBinName = `starsrv.${unameArch}`;

function getStarSrvPath(): string {
    if (process.platform === "win32") {
        const winBinName = `${starsrvBinName}.exe`;
        const appPath = path.join(getElectronAppUnpackedBasePath(), "bin", winBinName);
        return `${appPath}`;
    }
    return path.join(getElectronAppUnpackedBasePath(), "bin", starsrvBinName);
}

function getStarSrvCwd(): string {
    return getStarDataDir();
}

ipcMain.on("get-is-dev", (event) => {
    event.returnValue = isDev;
});
ipcMain.on("get-platform", (event, url) => {
    event.returnValue = unamePlatform;
});
ipcMain.on("get-user-name", (event) => {
    const userInfo = os.userInfo();
    event.returnValue = userInfo.username;
});
ipcMain.on("get-host-name", (event) => {
    event.returnValue = os.hostname();
});
ipcMain.on("get-webview-preload", (event) => {
    event.returnValue = path.join(getElectronAppBasePath(), "preload", "preload-webview.cjs");
});
ipcMain.on("get-data-dir", (event) => {
    event.returnValue = getStarDataDir();
});
ipcMain.on("get-config-dir", (event) => {
    event.returnValue = getStarConfigDir();
});

/**
 * Gets the value of the XDG_CURRENT_DESKTOP environment variable. If ORIGINAL_XDG_CURRENT_DESKTOP is set, it will be returned instead.
 * This corrects for a strange behavior in Electron, where it sets its own value for XDG_CURRENT_DESKTOP to improve Chromium compatibility.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @returns The value of the XDG_CURRENT_DESKTOP environment variable, or ORIGINAL_XDG_CURRENT_DESKTOP if set, or undefined if neither are set.
 */
function getXdgCurrentDesktop(): string {
    if (process.env.ORIGINAL_XDG_CURRENT_DESKTOP) {
        return process.env.ORIGINAL_XDG_CURRENT_DESKTOP;
    } else if (process.env.XDG_CURRENT_DESKTOP) {
        return process.env.XDG_CURRENT_DESKTOP;
    } else {
        return undefined;
    }
}

/**
 * Calls the given callback with the value of the XDG_CURRENT_DESKTOP environment variable set to ORIGINAL_XDG_CURRENT_DESKTOP if it is set.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @param callback The callback to call.
 */
function callWithOriginalXdgCurrentDesktop(callback: () => void) {
    const currXdgCurrentDesktopDefined = "XDG_CURRENT_DESKTOP" in process.env;
    const currXdgCurrentDesktop = process.env.XDG_CURRENT_DESKTOP;
    const originalXdgCurrentDesktop = getXdgCurrentDesktop();
    if (originalXdgCurrentDesktop) {
        process.env.XDG_CURRENT_DESKTOP = originalXdgCurrentDesktop;
    }
    callback();
    if (originalXdgCurrentDesktop) {
        if (currXdgCurrentDesktopDefined) {
            process.env.XDG_CURRENT_DESKTOP = currXdgCurrentDesktop;
        } else {
            delete process.env.XDG_CURRENT_DESKTOP;
        }
    }
}

/**
 * Calls the given async callback with the value of the XDG_CURRENT_DESKTOP environment variable set to ORIGINAL_XDG_CURRENT_DESKTOP if it is set.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @param callback The async callback to call.
 */
async function callWithOriginalXdgCurrentDesktopAsync(callback: () => Promise<void>) {
    const currXdgCurrentDesktopDefined = "XDG_CURRENT_DESKTOP" in process.env;
    const currXdgCurrentDesktop = process.env.XDG_CURRENT_DESKTOP;
    const originalXdgCurrentDesktop = getXdgCurrentDesktop();
    if (originalXdgCurrentDesktop) {
        process.env.XDG_CURRENT_DESKTOP = originalXdgCurrentDesktop;
    }
    await callback();
    if (originalXdgCurrentDesktop) {
        if (currXdgCurrentDesktopDefined) {
            process.env.XDG_CURRENT_DESKTOP = currXdgCurrentDesktop;
        } else {
            delete process.env.XDG_CURRENT_DESKTOP;
        }
    }
}

export {
    callWithOriginalXdgCurrentDesktop,
    callWithOriginalXdgCurrentDesktopAsync,
    getElectronAppBasePath,
    getElectronAppUnpackedBasePath,
    getStarConfigDir,
    getStarDataDir,
    getStarSrvCwd,
    getStarSrvPath,
    getXdgCurrentDesktop,
    isDev,
    isDevVite,
    unameArch,
    unamePlatform,
    StarConfigHomeVarName,
    StarDataHomeVarName,
};
