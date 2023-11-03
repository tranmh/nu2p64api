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

if [ "$HOSTNAME" = "mivis" ]; then 
    newman run nu2p64api.postman_collection.json --env-var "base_url=https://portal.svw.info:3030/api"
else
    newman run nu2p64api.postman_collection.json --env-var "base_url=https://test.svw.info:3030/api"
fi
