#!/bin/bash

go test
go build main.go

systemctl restart nu2p64api.service
systemctl status nu2p64api.service

# precondition:
#  curl https://github.com/ovh/venom/releases/download/v1.1.0/venom.linux-amd64 -L -o /usr/local/bin/venom && chmod +x /usr/local/bin/venom
cd test
venom run

