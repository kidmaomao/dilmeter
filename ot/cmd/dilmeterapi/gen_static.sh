#!/bin/sh -e

rm -rf ./static_v134/*
PREV_PATH=$(pwd)
pwd
cd ../../front
npm install
npm run build
cp -r dist/* ../cmd/dilmeterapi/static_v134/
cd "$PREV_PATH"
