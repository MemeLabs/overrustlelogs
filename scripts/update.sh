#!/bin/bash

export src="github.com/MemeLabs/overrustlelogs"

## local mode to deploy ignoring git
if [[ $1 == "local" ]]; then
	TODO=$2
	MODE=local
else
	TODO=$1
	MODE=default
	git pull
fi

source /etc/profile

updateBot(){
	docker-compose up -d --build bot
	echo "updated the orl-bot"
}

updateServer(){
	docker-compose up -d --build server
	service varnish restart
	echo "updated the orl-server"
}

updateLogger(){
	docker-compose up -d --build logger
	echo "updated the orl-logger"
}

updateServerPack(){
	rm -rf /var/overrustlelogs/public/assets

	cp -r $HOME/overrustlelogs/server/assets /var/overrustlelogs/public/
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/public/assets

	service varnish restart
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
else
	echo "updating everything..."
	updateBot
	updateLogger
	updateServer
	updateServerPack
	echo "updating complete"
fi