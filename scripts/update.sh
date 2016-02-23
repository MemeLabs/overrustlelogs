#!/bin/bash

export src="github.com/slugalisk/overrustlelogs"

git pull

## systemd support
if [ -z `which start` ]; then
	SSS=systemctl
else
	SSS=
fi

source /etc/profile

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

	cp -r $GOPATH/src/$src/server/views /var/overrustlelogs/
	cp -r $GOPATH/src/$src/server/assets /var/overrustlelogs/public/
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/views
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/public/assets

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

if [[ $1 == "bot" ]]; then
	echo "updating the orl-bot..."
	updateBot
elif [[ $1 == "server" ]]; then
	echo "updating the orl-server..."
	updateServer
elif [[ $1 == "logger" ]]; then
	echo "updating the orl-logger"
	updateLogger
else
	echo "updating everything..."
	updateBot
	updateLogger
	updateServer
	echo "updating complete"
fi