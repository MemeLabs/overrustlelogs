#!/bin/bash

export src="github.com/MemeLabs/overrustlelogs"
source /etc/profile

mkdir -p /var/overrustlelogs/public
cp -r $HOME/overrustlelogs/server/assets /var/overrustlelogs/public/assets
chown -R overrustlelogs:overrustlelogs /var/overrustlelogs/public/assets
cp -r $HOME/overrustlelogs/package/* /

service varnish restart

echo "pulling channels.json from server"
rm /var/overrustlelogs/channels.json
curl https://overrustlelogs.net/api/v1/channels.json > /var/overrustlelogs/channels.json

chown -R overrustlelogs:overrustlelogs /var/overrustlelogs

mkdir -p /var/nginx/cache
chown -R www-data:www-data /var/nginx

echo "next steps:"
echo "1.) add creds to /etc/overrustlelogs/overrustlelogs.toml"
echo "2.) run $ docker-compose up -d logger && docker-compose up -d server"