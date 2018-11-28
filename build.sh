#!/bin/bash

project="godbart"

release=./release
rm -rf $release/
mkdir -p $release

cd  $(dirname $0)

gofmt -w ./

for goos in "linux" "darwin" "freebsd" "windows"; do
    if [ "$goos" == "windows" ]; then
      obj_name=$project.exe
    else
      obj_name=$project
    fi

    GOOS=$goos GOARCH=amd64 go build
    zip $release/$project-$goos-amd64.zip $obj_name
#    GOOS=$goos GOARCH=386 go build
#    zip $release/$project-$goos-386.zip $obj_name
    rm -f $obj_name
done

cd $release
for file in `ls`; do
    md5sum $file >> md5sum.txt
done
