nginx
```
wget http://nginx.org/download/nginx-1.8.0.tar.gz
tar xzf nginx-1.8.0.tar.gz
cd nginx-1.8.0.tar.gz
./configure --with-http_gzip_static_module --with-http_gunzip_module
make
sudo make install
```