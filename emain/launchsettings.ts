// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import fs from "fs";
import path from "path";
import { getStarConfigDir } from "./platform";

/**
 * Get settings directly from the Star Home directory on launch.
 * Only use this when the app is first starting up. Otherwise, prefer the settings.GetFullConfig function.
 * @returns The initial launch settings for the application.
 */
export function getLaunchSettings(): SettingsType {
    const settingsPath = path.join(getStarConfigDir(), "settings.json");
    try {
        const settingsContents = fs.readFileSync(settingsPath, "utf8");
        return JSON.parse(settingsContents);
    } catch (_) {
        // fail silently
    }
}
