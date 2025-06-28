# syntax=docker/dockerfile:1.7.0

FROM registry.suse.com/bci/bci-base:15.6

# Proxy configuration
ARG http_proxy
ARG https_proxy
ARG no_proxy

ENV http_proxy $http_proxy
ENV https_proxy $https_proxy
ENV no_proxy $no_proxy

# Configure zypper proxy if https_proxy is set
RUN if [ -n "$https_proxy" ]; then \
    PROXY_URL_NO_SCHEME=$(echo "$https_proxy" | sed -e 's#http://##g' -e 's#https://##g'); \
    PROXY_HOST=$(echo "$PROXY_URL_NO_SCHEME" | cut -d':' -f1); \
    PROXY_PORT=$(echo "$PROXY_URL_NO_SCHEME" | cut -d':' -f2 | cut -d'/' -f1); \
    echo "proxy.enabled = true" >> /etc/zypp/zypp.conf; \
    echo "proxy.host = $PROXY_HOST" >> /etc/zypp/zypp.conf; \
    echo "proxy.port = $PROXY_PORT" >> /etc/zypp/zypp.conf; \
    echo "proxy.protocol = http" >> /etc/zypp/zypp.conf; \
    echo "Zypper proxy configured to $PROXY_HOST:$PROXY_PORT"; \
    else \
    echo "No https_proxy set, skipping zypper proxy configuration."; \
    fi

# Copy SUSE credentials
COPY SCCcredentials /etc/zypp/credentials.d/SCCcredentials

RUN zypper ref -s && zypper -n update && \
    zypper -n rm container-suseconnect && \
    zypper -n in curl dhcp-tools jq

ARG TARGETPLATFORM

RUN if [ "$TARGETPLATFORM" != "linux/amd64" ] && [ "$TARGETPLATFORM" != "linux/arm64" ]; then \
    echo "Error: Unsupported TARGETPLATFORM: $TARGETPLATFORM" && \
    exit 1; \
    fi

ENV ARCH=${TARGETPLATFORM#linux/}

COPY bin/vm-dhcp-controller-${ARCH} /usr/bin/vm-dhcp-controller

ENTRYPOINT [ "vm-dhcp-controller" ]

