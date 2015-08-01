#/bin/bash

base="/var/www/public/_public"
month=`date -u '+%B %Y' -d 'last month'`

IFS="$(printf '\n\t')"

for chan in `ls $base`; do
  if [ -e "$base/$chan/$month" ]; then
    for file in `ls $base/$chan/$month/*.txt`; do
      echo "compressing $file"
      gzip -9 "$file"
    done
    for file in `ls $base/$chan/$month/userlogs/*.txt`; do
      echo "compressing $file"
      gzip -9 "$file"
    done
    for file in `ls $base/$chan/$month/premium/*.txt`; do
      echo "compressing $file"
      gzip -9 "$file"
    done
    for file in `ls $base/$chan/$month/subscriber/*.txt`; do
      echo "compressing $file"
      gzip -9 "$file"
    done
    for file in `ls $base/$chan/$month/broadcaster/*.txt`; do
      echo "compressing $file"
      gzip -9 "$file"
    done
  fi
done