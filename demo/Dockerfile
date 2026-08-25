FROM ghcr.io/charmbracelet/vhs:v0.11.0

RUN apt-get update && apt-get install -y \
    git \
    && rm -rf /var/lib/apt/lists/*

COPY dist/goalie_linux_amd64_v1/goalie /usr/local/bin/goalie
RUN chmod +x /usr/local/bin/goalie

RUN git config --global user.email "demo@example.com" \
    && git config --global user.name "Demo User" \
    && git config --global init.defaultBranch main

ENV GOALIE_HOME=/demo-home-alice

COPY demo/setup.sh /demo/setup.sh
COPY demo/demo.tape /demo/demo.tape
RUN chmod +x /demo/setup.sh && /demo/setup.sh

WORKDIR /demo
VOLUME ["/output"]
CMD ["/demo/demo.tape"]
