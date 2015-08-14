#!/bin/bash

export src="github.com/slugalisk/overrustlelogs"

git pull

source /etc/profile
go install $src/logger
go install $src/server
go install $src/tool

stop orl-logger
stop orl-server

cp -r $GOPATH/src/$src/server/views /var/overrustlelogs/views
chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/views
cp $GOPATH/bin/logger /usr/bin/orl-logger
cp $GOPATH/bin/server /usr/bin/orl-server
cp $GOPATH/bin/tool /usr/bin/orl-tool

start orl-logger
start orl-server