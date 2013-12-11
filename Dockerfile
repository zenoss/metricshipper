FROM ubuntu
MAINTAINER Zenoss, Inc <dev@zenoss.com>

RUN wget -O- http://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | tar -C / -xz
ENV GOROOT /go
env GOPATH /go
ENV PATH $PATH:/go/bin
