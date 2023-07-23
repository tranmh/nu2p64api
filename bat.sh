#!/bin/bash

go test
go build main.go

systemctl restart nu2p64api.service
systemctl status nu2p64api.service
