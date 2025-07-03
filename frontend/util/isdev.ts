// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { getEnv } from "./getenv";
import { lazy } from "./util";

export const StarDevVarName = "STARTERM_DEV";
export const StarDevViteVarName = "STARTERM_DEV_VITE";

/**
 * Determines whether the current app instance is a development build.
 * @returns True if the current app instance is a development build.
 */
export const isDev = lazy(() => !!getEnv(StarDevVarName));

/**
 * Determines whether the current app instance is running via the Vite dev server.
 * @returns True if the app is running via the Vite dev server.
 */
export const isDevVite = lazy(() => !!getEnv(StarDevViteVarName));
