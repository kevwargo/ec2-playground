#!/bin/sh

# set -e

error() {
    echo "@" >/dev/stderr
    exit 1
}

fetch_url() {
    if command -v curl >/dev/null; then
        if [ -n "$2" ]; then
            curl -sSL -o "$2" "$1"
        else
            curl -sSL "$1"
        fi
    elif command -v wget >/dev/null; then
        if [ -n "$2" ]; then
            wget -q -O "$2" "$1"
        else
            wget -q -O- "$1"
        fi
    else
        error "Cannot find neither wget nor curl"
    fi
}
