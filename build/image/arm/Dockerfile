FROM multiarch/ubuntu-debootstrap:arm64-bionic
LABEL maintainer="tiny-x"

# install gcc make git
RUN apt-get update \
    && apt-get install -y build-essential git

# install go
RUN wget https://dl.google.com/go/go1.13.10.linux-arm64.tar.gz \
    && tar -C /usr/local -xzf go1.13.10.linux-arm64.tar.gz

ENV GOOS linux
ENV PATH /usr/local/go/bin:$PATH

ENTRYPOINT [ "make" ]
