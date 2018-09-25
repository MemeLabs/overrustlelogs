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

if [ -z `which go` ]; then
  apt-get update
  apt-get install build-essential git wget curl -y

  pushd . > /dev/null
  cd /tmp

  git clone https://github.com/golang/go
  cd go
  git checkout release-branch.go1.4
  cd src
  bash ./make.bash
  cd /tmp
  mv go /usr/local/

  echo "export GOROOT=/usr/local/go" >> /etc/profile
  echo "export PATH=\$PATH:\$HOME/go/bin:\$GOROOT/bin" >> /etc/profile
  source /etc/profile

  wget https://dl.google.com/go/go1.11.src.tar.gz
  tar xzf go1.11.src.tar.gz
  cd go/src
  GOROOT_BOOTSTRAP=$GOROOT bash ./make.bash
  cd /tmp
  rm -rf /usr/local/go
  mv go /usr/local/

  mkdir -p $GOPATH

  popd > /dev/null
fi

useradd overrustlelogs