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

updateBot(){
	go install $src/bot || exit 2

	$SSS stop orl-bot
	
	cp $GOPATH/bin/bot /usr/bin/orl-bot
	
	$SSS start orl-bot
	echo "updated the orl-bot"
}

updateServer(){
	go install $src/server || exit 2
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
	go install $src/logger || exit 2
	go install $src/tool || exit 2

	$SSS stop orl-logger

	cp $GOPATH/bin/logger /usr/bin/orl-logger
	cp $GOPATH/bin/tool /usr/bin/orl-tool

	$SSS start orl-logger
	echo "updated the orl-logger"
}

updatePack(){
	cp -r $GOPATH/src/$src/package/* /
	if [ -f "$GOPATH/src/$src/package/etc/overrustlelogs/overrustlelogs.local.conf" ]; then
		echo "NOTICE: found overrustlelogs.local.conf, overwriting default file..."
		cp -p "$GOPATH/src/$src/package/etc/overrustlelogs/overrustlelogs.local.conf" /etc/overrustlelogs/overrustlelogs.conf
	fi
	chown -R overrustlelogs:overrustlelogs /var/overrustlelogs
	systemctl daemon-reload
	echo "updated package etc & var"
}

if [[ $TODO == "bot" ]]; then
	echo "updating the orl-bot..."
	updateBot
elif [[ $TODO == "server" ]]; then
	echo "updating the orl-server..."
	updateServer
elif [[ $TODO == "logger" ]]; then
	echo "updating the orl-logger"
	updateLogger
elif [[ $TODO == "pack" ]]; then
	echo "updating package etc & var"
	updatePack
else
	echo "updating everything..."
	if [[ $MODE == "local" ]]; then
		echo
		echo "NOTE, local mode will replace etc with the pack!!!"
		sleep 3
		echo "..."
		sleep 2
		updatePack
	fi
	updateBot
	updateLogger
	updateServer
	echo "updating complete"
fi