#!/bin/bash
set -e
rm -r bin
mkdir -p bin

echo "building lsptrace..."
pushd lsptrace
go build -o ../bin .
popd
echo "building dummyclient..."
go build -o bin ./dummyclient/dummy-client.go
echo "building dummyserver..."
go build -o bin ./dummyserver/dummy-server.go

echo "successful build"

