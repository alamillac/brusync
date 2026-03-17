#!/bin/bash

# Set token with 
# secret-tool store --label="Github bruno Token" token GITHUB_BRUNO_TOKEN

INTERVAL=10s
GITHUB_TOKEN=$(secret-tool lookup token GITHUB_BRUNO_TOKEN) ./bin/brusync -repo "https://github.com/andrestpp/http_workspace" -branch "master" -dir "/home/alamilla/Documents/work/http_workspace" -interval $INTERVAL
