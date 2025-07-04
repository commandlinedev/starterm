#!/bin/bash

# Wrapper script to handle yarn list command for electron-builder compatibility

if [[ "$1" == "list" ]]; then
    # If it's a yarn list command, use our compatibility script
    exec node yarn-list-compat.js
else
    # Otherwise, pass through to the real yarn
    exec /usr/local/opt/node@18/bin/yarn "$@"
fi
