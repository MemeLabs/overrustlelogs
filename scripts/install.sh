#!/bin/bash

export source="github.com/slugalisk/overrustlelogs"

mkdir -p $GOPATH/src/github.com/slugalisk
ln -s `dirname $0`/.. $GOPATH/src/$source

go install $source/logger
go install $source/server

cp $GOPATH/bin/logger /usr/bin/orlogger
cp $GOPATH/bin/server /usr/bin/orserver

mkdir -p /var/overrustle
ln -s $GOPATH/src/$source/server/views /var/overrustle/views
cp -r $GOPATH/src/$source/package/* /
chown -R overrustle:overrustle /var/overrustle

ecoh "next steps:"
echo "1.) add twitch creds to /etc/overrustle/overrustle.conf"
echo "2.) run $ start logger && start server"