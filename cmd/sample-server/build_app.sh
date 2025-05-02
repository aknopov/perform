#! /bin/bash

go mod init sample-server
go mod tidy
go build -o server-app