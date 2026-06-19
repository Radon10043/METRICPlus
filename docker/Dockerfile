FROM ubuntu:24.04

SHELL [ "/bin/bash", "-c" ]

ENV GOROOT=/usr/local/go
ENV GOPATH=/root/go
ENV PATH=$GOROOT/bin:$GOPATH/bin:$LLVM_HOME/bin:$PATH
ENV TZ=Asia/Shanghai

# Install basic packages
RUN apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y tzdata
RUN apt-get install -y sudo wget curl git vim build-essential make gcc g++ flex bison bc debootstrap
RUN apt-get install -y cmake gdb libdw-dev libssl-dev sqlite3 libsqlite3-dev unzip cloc jq lsd ripgrep
RUN apt-get install -y python3 python3-pip python3-dev pkg-config

# Install golang-1.26.3
RUN wget https://go.dev/dl/go1.26.3.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.26.3.linux-amd64.tar.gz && \
    rm go1.26.3.linux-amd64.tar.gz

# Install fzf
RUN git clone --depth 1 https://github.com/junegunn/fzf.git /root/.fzf && \
    yes | /root/.fzf/install

# Install lazygit
RUN go install github.com/jesseduffield/lazygit@latest

RUN echo "alias ls=lsd" >> /root/.bashrc
