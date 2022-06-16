FROM golang:1.17-alpine
MAINTAINER jungin.kim <jungin.kim@catenoid.net>

ENV GO111MODULE=on  \
    CGO_ENABLED=0   \
    GOOS=linux      \
    GOARCH=amd64

# Run apk update
RUN apk update && \
    apk add libc6-compat

ENV APP_USER kollus
ENV APP_UID 2001
ENV APP_GROUP kollus
ENV APP_HOME /home/$APP_USER

RUN adduser $APP_USER -h /home/$APP_USER -u $APP_UID -D

WORKDIR /home/kollus

RUN set -ex && \
    mkdir -p /opt/go_work/bin/staticfiles && \
    mkdir -p /opt/kollus/conf && \
    chmod -R 777 /opt && \
    mkdir -p /var/log/kollus && \
    chmod -R 777 /var

WORKDIR /home/kollus

VOLUME ["/opt/go_work/bin/staticfiles", "/opt/kollus/conf", "/var/log/kollus"]

COPY ./main /home/kollus/main

COPY ./crossdomain.xml /opt/go_work/bin/staticfiles
COPY ./kollus_webhook.json /opt/kollus/conf

RUN cd /tmp && rm -rf * && mkdir -p /tmp/contents && chmod -R 777 /tmp
RUN mkdir -p /tmp_passthrough && chmod -R 777 /tmp_passthrough
VOLUME ["/tmp/contents"]

CMD ./main

EXPOSE 4242