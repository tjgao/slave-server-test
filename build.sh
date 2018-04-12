#!/bin/sh
go get github.com/gorilla/websocket
go get github.com/sirupsen/logrus

mkdir -p build

GOOS=linux GOARCH=amd64 go build -o build/slave.linux

GOOS=windows GOARCH=amd64 go build -o build/slave.exe

GOOS=darwin GOARCH=amd64 go build -o build/slave.mac

