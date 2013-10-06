FROM ubuntu:12.10
MAINTAINER Niels Sandholt Busch "niels.busch@gmail.com"
RUN apt-get -qq update
RUN apt-get install -y golang git
ADD imgresize /usr/local/bin/imgresize
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/imgresize"]