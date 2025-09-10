#!/bin/bash
export VOSK_PATH=`pwd`/vosk-linux-x86_64-0.3.45
export LD_LIBRARY_PATH=$VOSK_PATH
export CGO_CPPFLAGS="-I$VOSK_PATH"
export CGO_LDFLAGS="-L $VOSK_PATH"
go build
./sp2tbot
exit 0
