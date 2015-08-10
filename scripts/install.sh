#!/bin/bash

export src="github.com/slugalisk/overrustlelogs"
source /etc/profile

mkdir -p $GOPATH/src/github.com/slugalisk
ln -s $(readlink -e $(dirname $0)/..) $GOPATH/src/$src

go install $src/logger
go install $src/server

cp $GOPATH/bin/logger /usr/bin/orl-logger
cp $GOPATH/bin/server /usr/bin/orl-server

mkdir -p /var/overrustlelogs
cp -r $GOPATH/src/$src/server/views /var/overrustlelogs/views
cp -r $GOPATH/src/$src/package/* /
chown -R overrustlelogs:overrustlelogs /var/overrustlelogs

echo "next steps:"
echo "1.) add twitch creds to /etc/overrustlelogs/overrustlelogs.conf"
echo "2.) run $ start logger && start server"