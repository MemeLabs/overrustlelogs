# OverRustle Logs

A chat log suite for [Destiny.gg](https://www.destiny.gg/bigscreen) and [Twitch.tv](http://twitch.tv).

## Setting Up OverRustle Logs

These instructions assume you are installing on Ubuntu 14.04 or higher.

### Step 1

Install git.

```bash
sudo apt-get install git --assume-yes
```

### Step 2

Clone the overrustlelogs repo.

```bash
git clone https://github.com/MemeLabs/overrustlelogs.git

cd overrustlelogs
```

### Step 3

Copy and edit the .env file. Edit the overrustlelogs.toml file.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
cp ./.env.example ./.env

# changing paths in this requires to change paths in install.sh
vim .env

# few things you need to edit here too
vim ./package/var/overrustlelogs/overrustlelogs.toml

# change server_name's in the nginx config if you need
vim ./package/etc/nginx/sites-enabled/overrustlelogs.net.conf
```

### Step 4

Run the install script from the repo root directory.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
# use sudo if you're not root
# only use all.sh if you're on ubuntu and don't have nginx, varnish, docker and
# docker-compose installed, otherwise install everything manually and run install.sh afterwards
./scripts/all.sh
```

## Updating

Run the update script from the repo root directory.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
# use sudo if you're not root
./scripts/update.sh
```
