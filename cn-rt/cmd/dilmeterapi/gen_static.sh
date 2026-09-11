#!/bin/sh -e

rm -rf ./static_v130_boss_alert_layout_test/*
PREV_PATH=$(pwd)
pwd
cd ../../front
npm install
npm run build
cp -r dist/* ../cmd/dilmeterapi/static_v130_boss_alert_layout_test/
cd "$PREV_PATH"
