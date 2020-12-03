# Webitel CHAT Service(s)

## PROTOC

Ensure *protoc* command installed  
Upgrade *protoc-gen-*\* dependent plugin(s)

```sh
make protoc
```

## PROTO

[RE-]Generate \**.pb.go*, \**.pb.micro.go* files from dependent \**.proto* packages

```sh
make proto
```

## BUILD

Build executable binaries: *chat-srv*, *chat-bot*

```sh
make chat-srv chat-bot
```

## RUN

```sh
make server  # run the webitel.chat.server service
make gateway # run the webitel.chat.bot gateway service
```
