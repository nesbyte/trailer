#!/usr/bin/env bash


# Convenience function to build various binaries
buildTargetBinary(){
    env GOOS=$1 GOARCH=$2 go build -ldflags="main.Version=$GITHUB_REF_NAME" -o ./builds/trailer
    tar -zcvf ./releases/trailer-$1-$2.tar.gz -C ./builds trailer
} 

rm -rf ./builds
rm -rf ./releases
mkdir ./releases

buildTargetBinary linux amd64
buildTargetBinary linux arm64
