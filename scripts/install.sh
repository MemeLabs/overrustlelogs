#!/bin/bash

export source="github.com/slugalisk/overrustlelogs"

go get -u $source

go install $source/logger
go install $source/server

cp $GOPATH/bin/logger /usr/bin/orlogger
cp $GOPATH/bin/server /usr/bin/orserver

mkdir -p /var/overrustle
cp -r $GOPATH/src/$source/package /
chown -R overrustle:overrustle /var/overrustle

echo "be sure to add your twitch creds to /etc/overrustle/overrustle.conf"