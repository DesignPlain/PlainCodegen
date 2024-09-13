#!/bin/bash

cd utilities/typescript
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
. ~/.nvm/nvm.sh 2>&1

nvm install node
npm install
. ~/.nvm/nvm.sh 2>&1

cd ../../

mkdir -p CodegenDir/{ts,go}/ds_base

cp basefiles/ts/*.ts CodegenDir/ts/ds_base
cp basefiles/ts/package.json CodegenDir/ts

cp basefiles/go/base.go CodegenDir/go/ds_base
cp basefiles/go/go.mod CodegenDir/go

go build
