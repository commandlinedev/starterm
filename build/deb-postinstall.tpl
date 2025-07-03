#!/bin/bash

if type update-alternatives 2>/dev/null >&1; then
    # Remove previous link if it doesn't use update-alternatives
    if [ -L '/usr/bin/starterm' -a -e '/usr/bin/starterm' -a "`readlink '/usr/bin/starterm'`" != '/etc/alternatives/starterm' ]; then
        rm -f '/usr/bin/starterm'
    fi
    update-alternatives --install '/usr/bin/starterm' 'starterm' '/opt/Star/starterm' 100 || ln -sf '/opt/Star/starterm' '/usr/bin/starterm'
else
    ln -sf '/opt/Star/starterm' '/usr/bin/starterm'
fi

chmod 4755 '/opt/Star/chrome-sandbox' || true

if hash update-mime-database 2>/dev/null; then
    update-mime-database /usr/share/mime || true
fi

if hash update-desktop-database 2>/dev/null; then
    update-desktop-database /usr/share/applications || true
fi
