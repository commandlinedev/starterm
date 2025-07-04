#!/bin/bash

# Script to run electron-builder with npm instead of yarn to avoid Yarn 4 compatibility issues

# First build the project normally
yarn build:prod

# Then run electron-builder with npm
npx electron-builder -c electron-builder.config.cjs -p never
