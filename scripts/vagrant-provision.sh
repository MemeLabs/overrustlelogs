#!/bin/bash

export base=`dirname $0`
bash "$base/all.sh"

rm /etc/nginx/sites-enabled/default
/etc/init.d/nginx restart

echo
echo "PROVISIONER -- enabling all services for you on default..."
systemctl enable orl-logger
systemctl enable orl-server
systemctl enable orl-bot
