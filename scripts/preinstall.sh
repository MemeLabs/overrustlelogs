#!/bin/bash

if [ -z `which nginx` ]; then
  apt-get update
  apt-get install nginx -y
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

  echo "export GOPATH=\$HOME/go" >> /etc/profile
  echo "export GOROOT=/usr/local/go" >> /etc/profile
  echo "export PATH=\$PATH:\$GOPATH/bin:\$GOROOT/bin" >> /etc/profile
  source /etc/profile

  wget https://storage.googleapis.com/golang/go1.9.src.tar.gz
  tar xzf go1.9.src.tar.gz
  cd go/src
  GOROOT_BOOTSTRAP=$GOROOT bash ./make.bash
  cd /tmp
  rm -rf /usr/local/go
  mv go /usr/local/

  mkdir -p $GOPATH

  popd > /dev/null
fi

go get -u "github.com/cloudflare/golz4"

# checkout to 1.x branch
go get -u "github.com/datadog/zstd"
pushd $GOPATH/github.com/datadog/zstd
git checkout 1.x
popd

go get -u "github.com/gorilla/websocket"
go get -u "github.com/gorilla/mux"
go get -u "github.com/gorilla/handlers"
go get -u "github.com/hashicorp/golang-lru"
go get -u "github.com/CloudyKit/jet"

useradd overrustlelogs