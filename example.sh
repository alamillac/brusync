#!/bin/bash

# Set token with 
# secret-tool store --label="Github bruno Token" token GITHUB_BRUNO_TOKEN

INTERVAL=2m
DEBOUNCE=2s
#DIR=/home/alamilla/Documents/work/http_workspace
DIR=~/Documents/work/http_workspace
LOG_LEVEL=error
GITHUB_TOKEN=$(secret-tool lookup token GITHUB_BRUNO_TOKEN) ./bin/brusync -repo "https://github.com/andrestpp/http_workspace" -branch "master" -dir "$DIR" -interval $INTERVAL -debounce $DEBOUNCE -log-level $LOG_LEVEL
