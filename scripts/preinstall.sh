#!/bin/bash

if [ -z `which nginx` ]; then
  apt-get install nginx --assume-yes
fi

if [ -z `which go` ]; then
  apt-get update
  apt-get install build-essential --assume-yes

  pushd . > /dev/null
  cd /tmp

  wget https://storage.googleapis.com/golang/go1.4.3.src.tar.gz
  tar xzf go1.4.3.src.tar.gz
  cd go/src
  bash ./make.bash
  cd /tmp
  mv go /usr/local/

  echo "export GOPATH=\$HOME/go" >> /etc/profile
  echo "export GOROOT=/usr/local/go" >> /etc/profile
  echo "export PATH=\$PATH:\$GOPATH/bin:\$GOROOT/bin" >> /etc/profile
  source /etc/profile

  wget https://storage.googleapis.com/golang/go1.7.4.src.tar.gz
  tar xzf go1.6.3.src.tar.gz
  cd go/src
  GOROOT_BOOTSTRAP=$GOROOT bash ./make.bash
  cd /tmp
  rm -rf /usr/local/go
  mv go /usr/local/

  mkdir -p $GOPATH

  popd > /dev/null
fi

go get "github.com/cloudflare/golz4"
go get "github.com/gorilla/websocket"
go get "github.com/gorilla/mux"
go get "github.com/hashicorp/golang-lru"
go get "github.com/xlab/handysort"
go get "github.com/yosssi/ace"

useradd overrustlelogs
