#!/bin/bash

scriptdir=$(readlink -f $(dirname $BASH_SOURCE))

echo
export GOPATH="$scriptdir/"
echo "Set \$GOPATH to $GOPATH..."

#export GOBIN=`readlink -f "$scriptdir/bin/"`
#echo "Set \$GOBIN to $GOBIN..."

echo
if ! echo "$PATH" | grep "$scriptdir/bin" >/dev/null; then
    export PATH="$scriptdir/bin:$PATH"
    echo "Set \$PATH to $PATH..."
else
    echo "\$PATH already set to $PATH"
fi

alias grep='grep --color=auto --exclude-dir=github.com --exclude-dir=golang.org --exclude-dir=gopkg.in --exclude-dir=google.golang.org --exclude-dir=stablelib.com --exclude-dir=.git'

echo

echo "Setting Go 1.5 environment..."
export GOROOT="$HOME/go"
export PATH="$GOROOT/bin:$PATH" # $GOROOT/bin should be first
echo "Set \$GOROOT to $GOROOT"
echo "Added \$GOROOT/bin to \$PATH"
