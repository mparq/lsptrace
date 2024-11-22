#!/bin/bash
set -e
rm -r bin
mkdir -p bin

echo "building lsptrace..."
go build -o bin ./lsptrace/lsptrace.go
echo "building dummyclient..."
go build -o bin ./dummyclient/dummy-client.go
echo "building dummyserver..."
go build -o bin ./dummyserver/dummy-server.go

echo "successful build"

