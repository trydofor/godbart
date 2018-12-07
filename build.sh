#!/bin/bash

project="godbart"
release=./release

cd  $(dirname $0)
rm -rf $release/
mkdir -p $release

#gofmt -w ./

# build
for goos in "linux" "darwin" "freebsd" "windows"; do
    if [ "$goos" == "windows" ]; then
      obj_name=$project.exe
    else
      obj_name=$project
    fi

    GOOS=$goos GOARCH=amd64 go build
    zip -m $release/$project-$goos-amd64.zip $obj_name
#    GOOS=$goos GOARCH=386 go build
#    zip -m $release/$project-$goos-386.zip $obj_name
done

# md5sum
cd $release
md5sum * >> md5sum.txt