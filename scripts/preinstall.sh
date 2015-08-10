#!/bin/bash

apt-get install git,build-essential --assume-yes

pushd . > /dev/null
cd /tmp

wget https://storage.googleapis.com/golang/go1.4.2.src.tar.gz
tar xzf go1.4.2.src.tar.gz
cd go/src
bash ./all.bash
cd /tmp
mv go /usr/local/

echo "export GOPATH=\$HOME/go" >> /etc/profile
echo "export GOROOT=/usr/local/go" >> /etc/profile
echo "export PATH=\$PATH:\$GOPATH/bin:\$GOROOT/bin" >> /etc/profile

export GOPATH=$HOME/go
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

mkdir $GOPATH

go get "github.com/cloudflare/golz4"
go get "github.com/hashicorp/golang-lru"
go get "github.com/gorilla/websocket"
go get "github.com/gorilla/mux"
go get "github.com/xlab/handysort"
go get "github.com/yosssi/ace"

useradd overrustle

popd > /dev/null