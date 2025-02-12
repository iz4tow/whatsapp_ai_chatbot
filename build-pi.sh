#!/bin/bash
mkdir -p build
export GOOS=linux; \
export GOARCH=arm; \
export GOARM=7; \
export CC=arm-linux-gnueabi-gcc; \
CGO_ENABLED=1 go build -ldflags "-linkmode external -extldflags -static" --trimpath  whatsapp_bot.go
mv whatsapp_bot build/
cp install_chatbot.sh ./build
cd build
tar -czvf ../RPI-chatbot.tar.gz *
