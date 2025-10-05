#!/bin/bash
#
#go install github.com/fyne-io/fyne-cross@latest
#
~/go/bin/fyne-cross windows -arch=amd64 -tags=no_animations,gles
~/go/bin/fyne-cross windows -arch=arm64 -tags=no_animations,gles
~/go/bin/fyne-cross linux -arch=amd64 -tags=no_animations,gles
~/go/bin/fyne-cross linux -arch=arm64 -tags=no_animations,gles
mv fyne-cross/dist/linux-arm64/fyne-authenticator.tar.xz fyne-cross/dist/fyne-authenticator-arm64.tar.xz
mv fyne-cross/dist/linux-amd64/fyne-authenticator.tar.xz fyne-cross/dist/fyne-authenticator-amd64.tar.xz
mv fyne-cross/dist/windows-arm64/fyne-authenticator.exe.zip fyne-cross/dist/fyne-authenticator-arm64.exe.zip
mv fyne-cross/dist/windows-amd64/fyne-authenticator.exe.zip fyne-cross/dist/fyne-authenticator-amd64.exe.zip
