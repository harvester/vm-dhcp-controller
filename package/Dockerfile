FROM registry.suse.com/bci/bci-base:15.5

RUN zypper -n rm container-suseconnect && \
    zypper -n in curl dhcp-tools

COPY bin/vm-dhcp-controller /usr/bin/vm-dhcp-controller

ENTRYPOINT [ "vm-dhcp-controller" ]
CMD [ "manager" ]