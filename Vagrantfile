# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below.
Vagrant.configure(2) do |config|
  config.vm.box = "driebit/debian-8-x86_64"
  config.vm.hostname = "overrustlelogs"

  config.vm.network "forwarded_port", guest: 80, host: 2780
  config.vm.network "forwarded_port", guest: 8080, host: 2788
  config.vm.network "forwarded_port", guest: 22, host: 2755


  config.vm.provider "virtualbox" do |vb|
    vb.memory = 4069
    vb.cpus = 3
  end

  config.vm.provision "provision", type: "shell", inline: "cd /vagrant; pwd; echo 'running ./scripts/vagrant-provision.sh'; bash ./scripts/vagrant-provision.sh"

  config.vm.provision "build-helper", type: "shell", inline: "echo '#!/bin/bash' > /home/vagrant/env.sh; echo 'sudo sh -c '\\''cd /vagrant; exec \"${SHELL:-sh}\"'\\''' >> /home/vagrant/env.sh; chmod +x /home/vagrant/env.sh"

  config.vm.provision "update", type: "shell", run: "always", inline: "cd /vagrant; pwd; echo 'running ./scripts/update.sh local'; bash ./scripts/update.sh local"

end
