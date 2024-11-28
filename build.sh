#!/bin/bash
set -e
rm -r bin
mkdir -p bin

echo "building lsptrace..."
pushd lsptrace
go build -o ../bin .
popd

echo "successful build"

