
FROM multiarch/ubuntu-debootstrap:arm64-bionic
LABEL maintainer="Mingxia Cai"

RUN apt-get update \
    && apt-get install -y build-essential git

#RUN apt-get update \
#    && apt-get install flex -y

RUN wget http://www.musl-libc.org/releases/musl-1.1.21.tar.gz \
    && tar -zxvf musl-1.1.21.tar.gz \
    && rm musl-1.1.21.tar.gz \
    && cd musl* \
    && ./configure \
    && make \
    && make install \
    && rm -rf musl*

RUN wget https://dl.google.com/go/go1.20.5.linux-arm64.tar.gz \
    && tar -C /usr/local -xzf go1.20.5.linux-arm64.tar.gz \
    && rm -rf go1.20.5.linux-arm64.tar.gz

#RUN apt-get install -y bison

#RUN apt-get install libpcap0.8-dev -y
#
##RUN wget https://www.tcpdump.org/release/libpcap-1.7.4.tar.gz \
##    && tar -zxvf libpcap-1.7.4.tar.gz \
##    && rm libpcap-1.7.4.tar.gz \
##    && cd libpcap* \
##    && ./configure \
##    && make \
##    && make install \
##    && rm -rf libpcap*

RUN ln -s /usr/local/include/* /usr/local/musl/include/
RUN ln -s /usr/local/lib/* /usr/local/musl/lib/
#RUN ln -s /usr/include/pcap/bpf.h /usr/include/net/bpf.h

ENV CC /usr/local/musl/bin/musl-gcc
ENV GOOS linux
ENV PATH /usr/local/go/bin:$PATH

ENTRYPOINT [ "make"]
