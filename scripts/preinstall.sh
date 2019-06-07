#!/bin/bash

if [ -z `which nginx` ]; then
  apt-get update
  apt-get install nginx -y
  rm /etc/nginx/sites-enabled/default
  systemctl restart nginx
fi

if [ -z `which varnishd` ]; then
  apt-get update
  apt-get install varnish -y
fi

if [ -z `which docker` ]; then
	apt-get update
	apt-get install -y apt-transport-https ca-certificates curl software-properties-common
	curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
	add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
	apt-get update
	apt-get install -y docker-ce
fi

if [ -z `which docker-compose` ]; then
	curl -L https://github.com/docker/compose/releases/download/1.21.2/docker-compose-$(uname -s)-$(uname -m) -o /usr/local/bin/docker-compose
	chmod +x /usr/local/bin/docker-compose
fi

useradd overrustlelogs