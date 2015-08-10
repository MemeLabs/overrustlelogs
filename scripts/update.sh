#!/bin/bash

export source="github.com/slugalisk/overrustlelogs"

git pull

go install $source/logger
go install $source/server

stop orlogger
stop orserver

cp $GOPATH/bin/logger /usr/bin/orlogger
cp $GOPATH/bin/server /usr/bin/orserver

start orlogger
start orserver