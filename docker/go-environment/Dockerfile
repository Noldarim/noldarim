# Build stage
FROM golang:1.25 AS builder

ARG GIT_DELTA_VERSION=0.18.2

# Install wget for downloading git-delta
RUN apt-get update && apt-get install -y --no-install-recommends \
  wget \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Download and extract git-delta
RUN ARCH=$(dpkg --print-architecture) && \
  wget "https://github.com/dandavison/delta/releases/download/${GIT_DELTA_VERSION}/git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" && \
  dpkg-deb -x "git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" /tmp/delta

# Build agent binary
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent ./cmd/agent

# Runtime stage
FROM golang:1.25

ARG TZ
ENV TZ="$TZ"

# Install basic development tools
RUN apt-get update && apt-get install -y --no-install-recommends \
  less \
  git \
  procps \
  sudo \
  fzf \
  zsh \
  man-db \
  unzip \
  gnupg2 \
  gh \
  dnsutils \
  jq \
  nano \
  lua5.4 \
  neovim \
  ripgrep \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN curl -LO https://github.com/ClementTsang/bottom/releases/download/0.11.0/bottom_0.11.0-1_arm64.deb && \
  dpkg -i bottom_0.11.0-1_arm64.deb && \
  rm bottom_0.11.0-1_arm64.deb

ARG USERNAME=noldarim
ARG USER_HOME=/home/$USERNAME

RUN go install github.com/jesseduffield/lazygit@latest

# Create the user and group first
RUN groupadd --gid 1000 $USERNAME \
  && useradd --uid 1000 --gid $USERNAME --shell /bin/bash --create-home $USERNAME

# Create directories and set permissions for user
RUN mkdir -p /usr/local/share/npm-global /workspace $USER_HOME/go && \
  chown -R $USERNAME:$USERNAME /usr/local/share /workspace $USER_HOME/go

# Copy git-delta binary from build stage (rarely changes)
COPY --from=builder /tmp/delta/usr/bin/delta /usr/local/bin/

# Set up non-root user
USER $USERNAME

# Set `DEVCONTAINER` environment variable to help with orientation
ENV DEVCONTAINER=true

# Persist bash history and create user directories
RUN mkdir -p $USER_HOME/.claude && \
  touch $USER_HOME/.bash_history

WORKDIR /workspace

# Install global packages
ENV NPM_CONFIG_PREFIX=/usr/local/share/npm-global
ENV PATH=$PATH:/usr/local/share/npm-global/bin

# Set up Go environment for user
ENV GOPATH=$USER_HOME/go
ENV GOBIN=$USER_HOME/go/bin
ENV PATH=$PATH:$GOBIN

# Set the default shell to zsh rather than sh
ENV SHELL=/bin/zsh

# Set the default editor and visual
ENV EDITOR=nano
ENV VISUAL=nano

# Default powerline10k theme - configure zsh for user
ARG ZSH_IN_DOCKER_VERSION=1.2.0
RUN sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v${ZSH_IN_DOCKER_VERSION}/zsh-in-docker.sh)" -- \
  -p git \
  -p fzf \
  -a "source /usr/share/doc/fzf/examples/key-bindings.zsh" \
  -a "source /usr/share/doc/fzf/examples/completion.zsh" \
  -a "export PROMPT_COMMAND='history -a' && export HISTFILE=$USER_HOME/.bash_history" \
  -a "export GOPATH=$USER_HOME/go" \
  -a "export GOBIN=$USER_HOME/go/bin" \
  -a "export PATH=\$PATH:\$GOBIN" \
  -x

# Install bun (JS runtime / package manager)
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH=$PATH:$USER_HOME/.bun/bin
RUN echo "export PATH=\$PATH:\$HOME/.bun/bin" >> $USER_HOME/.zshrc

# Install Claude for user
RUN curl -fsSL https://claude.ai/install.sh | bash -s latest
ENV PATH=$PATH:$USER_HOME/.local/bin
RUN echo "export PATH=\$PATH:\$HOME/.local/bin" >> $USER_HOME/.zshrc

RUN git clone --depth 1 https://github.com/AstroNvim/template ~/.config/nvim

# Copy agent binary last - changes frequently, so keep at end to maximize cache hits
# Switch back to root briefly to copy and set permissions
USER root
COPY --from=builder /build/agent /app/agent
RUN chmod +x /app/agent
USER noldarim
