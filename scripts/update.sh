#!/bin/bash

export src="github.com/slugalisk/overrustlelogs"

git pull

source /etc/profile

if [[ $1 == "bot" ]]; then
	echo "updating the orl-bot...\n"
	updateBot
elif [[ $1 == "server" ]]; then
	echo "updating the orl-server...\n"
	updateServer
elif [[ $1 == "logger" ]]; then
	echo "updating the orl-logger"
	updateLogger
else
	echo "updating everything...\n"
	updateBot
	updateLogger
	updateServer
	echo "updating complete"
fi


updateBot(){
	go install $src/bot

	stop orl-bot
	
	cp $GOPATH/bin/bot /usr/bin/orl-bot
	
	start orl-bot
	echo "updated the orl-bot"
}

updateServer(){
	go install $src/server
	stop orl-server

	cp -r $GOPATH/src/$src/server/views /var/overrustlelogs/
	cp -r $GOPATH/src/$src/server/assets /var/overrustlelogs/public/
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/views
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/public/assets

	cp $GOPATH/bin/server /usr/bin/orl-server

	start orl-server
	echo "updated the orl-server"
}

updateLogger(){
	go install $src/logger
	go install $src/tool

	stop orl-logger

	cp $GOPATH/bin/logger /usr/bin/orl-logger
	cp $GOPATH/bin/tool /usr/bin/orl-tool

	start orl-logger
	echo "updated the orl-logger"
}
