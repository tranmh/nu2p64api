#!/bin/bash

go test
go build main.go

systemctl restart nu2p64api.service
systemctl status nu2p64api.service

# precondition:
#  curl https://github.com/ovh/venom/releases/download/v1.1.0/venom.linux-amd64 -L -o /usr/local/bin/venom && chmod +x /usr/local/bin/venom
#cd test
#venom run
# cd ..

# precondition:
# sudo apt install nodejs npm -y; npm install -g newman
cd test
newman run nu2p64api.postman_collection.json
