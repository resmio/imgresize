FROM ubuntu:12.10
MAINTAINER Niels Sandholt Busch "niels.busch@gmail.com"
RUN apt-get -qq update
RUN apt-get install -y golang git libmagickwand-dev
RUN go get github.com/gographics/imagick/imagick
ADD . /opt/imgresize
RUN cd /opt/imgresize && go build
EXPOSE 8080
ENTRYPOINT ["/opt/imgresize/imgresize"]
