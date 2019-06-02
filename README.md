# OverRustle Logs

A chat log suite for [Destiny.gg](https://www.destiny.gg/bigscreen) and [Twitch.tv](http://twitch.tv).

## Setting Up OverRustle Logs

These instructions assume you are installing on Ubuntu 14.04.

### Step 1

Install git.

```bash
sudo apt-get install git --assume-yes
```

### Step 2

Install docker. https://docs.docker.com/install/

### Step 3

Install docker-compose. https://docs.docker.com/compose/install/

### Step 4

Clone the overrustlelogs repo.

```bash
git clone https://github.com/MemeLabs/overrustlelogs.git

cd overrustlelogs
# change to docker branch
git checkout docker
```

### Step 5

Copy and edit the .env file. Edit the overrustlelogs.toml file.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
cp ./.env.example ./.env

# changing paths in this requires to change paths in install.sh
vim .env

# few things you need to edit here too
vim ./package/var/overrustlelogs/overrustlelogs.toml
```

### Step 6

Run the install script from the repo root directory.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
# use sudo if you're not root
./scripts/install.sh
```

## Updating

Run the update script from the repo root directory.

```bash
# cd into overrustlelogs if not already in there
cd overrustlelogs
# use sudo if you're not root
./scripts/update.sh
```
