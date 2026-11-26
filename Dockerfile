# Runtime image for the ssm binary. The binary itself is built outside Docker
# (by goreleaser, or `make docker`) and must be in the build context at
# <os>/<arch>/ssm, e.g. linux/amd64/ssm.

FROM debian:trixie-slim AS downloader

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# AWS CLI v2 and the Session Manager plugin, for the image's architecture
RUN case "$(dpkg --print-architecture)" in \
      amd64) AWS_ARCH=x86_64;  SSM_ARCH=ubuntu_64bit ;; \
      arm64) AWS_ARCH=aarch64; SSM_ARCH=ubuntu_arm64 ;; \
      *) echo "unsupported architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${AWS_ARCH}.zip" -o /tmp/awscliv2.zip \
    && unzip -q /tmp/awscliv2.zip -d /tmp \
    && /tmp/aws/install --install-dir /usr/local/aws-cli --bin-dir /usr/local/bin \
    && curl -fsSL "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/${SSM_ARCH}/session-manager-plugin.deb" -o /tmp/session-manager-plugin.deb \
    && dpkg -x /tmp/session-manager-plugin.deb /tmp/ssm-plugin

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    jq \
    less \
    locales \
    tmux \
    && sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen \
    && rm -rf /var/lib/apt/lists/*

ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8 LC_ALL=en_US.UTF-8

COPY --from=downloader /usr/local/aws-cli /usr/local/aws-cli
COPY --from=downloader /tmp/ssm-plugin/usr/local/sessionmanagerplugin/bin/session-manager-plugin /usr/local/sessionmanagerplugin/bin/session-manager-plugin
RUN ln -s /usr/local/aws-cli/v2/current/bin/aws /usr/local/bin/aws \
    && ln -s /usr/local/sessionmanagerplugin/bin/session-manager-plugin /usr/local/bin/session-manager-plugin

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/ssm /usr/local/bin/ssm

ENTRYPOINT ["/usr/local/bin/ssm"]
