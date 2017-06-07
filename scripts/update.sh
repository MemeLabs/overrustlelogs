#!/bin/bash

export src="github.com/slugalisk/overrustlelogs"

## local mode to deploy ignoring git
if [[ $1 == "local" ]]; then
	TODO=$2
	MODE=local
else
	TODO=$1
	MODE=default
	git pull
fi

## systemd support
if [ -z `which start` ]; then
	SSS=systemctl
else
	SSS=
fi

source /etc/profile

go get -u "github.com/cloudflare/golz4"
go get -u "github.com/gorilla/websocket"
go get -u "github.com/gorilla/mux"
go get -u "github.com/hashicorp/golang-lru"
go get -u "github.com/xlab/handysort"
go get -u "github.com/yosssi/ace"

updateBot(){
	go install $src/bot

	$SSS stop orl-bot
	
	cp $GOPATH/bin/bot /usr/bin/orl-bot
	
	$SSS start orl-bot
	echo "updated the orl-bot"
}

updateServer(){
	go install $src/server
	$SSS stop orl-server
	cp $GOPATH/bin/server /usr/bin/orl-server

	$SSS start orl-server
	echo "updated the orl-server"
}

updateLogger(){
	go install $src/logger
	go install $src/tool

	$SSS stop orl-logger

	cp $GOPATH/bin/logger /usr/bin/orl-logger
	cp $GOPATH/bin/tool /usr/bin/orl-tool

	$SSS start orl-logger
	echo "updated the orl-logger"
}

updatePack(){
	cp -p /etc/overrustlelogs/overrustlelogs.conf /etc/overrustlelogs/overrustlelogs.local.conf
	cp -r $GOPATH/src/$src/package/* /
	cp -p /etc/overrustlelogs/overrustlelogs.local.conf /etc/overrustlelogs/overrustlelogs.conf

	echo "pulling channels.json from server"
	rm /var/overrustlelogs/channels.json
	curl https://overrustlelogs.net/api/v1/channels.json >> /var/overrustlelogs/channels.json

	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs
	systemctl daemon-reload
	echo "updated package etc & var"
}

updateServerPack(){
	$SSS stop orl-server

	rm -rf /var/overrustlelogs/views
	rm -rf /var/overrustlelogs/public/assets

	cp -r $GOPATH/src/$src/server/views /var/overrustlelogs/
	cp -r $GOPATH/src/$src/server/assets /var/overrustlelogs/public/
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/views
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/public/assets

	$SSS start orl-server
	echo "updated the server assets"
}

if [[ $TODO == "bot" ]]; then
	echo "updating the orl-bot..."
	updateBot
elif [[ $TODO == "server" ]]; then
	echo "updating the orl-server..."
	updateServer
elif [[ $TODO == "serverpack" ]]; then
	echo "updating the orl-server assets..."
	updateServerPack
elif [[ $TODO == "logger" ]]; then
	echo "updating the orl-logger"
	updateLogger
elif [[ $TODO == "pack" ]]; then
	echo "updating package etc & var"
	updatePack
else
	echo "updating everything..."
	updateBot
	updateLogger
	updateServer
	updateServerPack
	updatePack
	echo "updating complete"
fi