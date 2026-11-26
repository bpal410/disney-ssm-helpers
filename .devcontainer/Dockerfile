FROM mcr.microsoft.com/devcontainers/go:2-1.27-trixie

# Runtime dependencies of the ssm binary: tmux (multi-instance sessions) and
# the AWS Session Manager plugin. The AWS CLI comes from a devcontainer feature.
RUN apt-get update \
    && apt-get install -y --no-install-recommends tmux \
    && case "$(dpkg --print-architecture)" in \
         amd64) SSM_ARCH=ubuntu_64bit ;; \
         arm64) SSM_ARCH=ubuntu_arm64 ;; \
         *) echo "unsupported architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
       esac \
    && curl -fsSL -o /tmp/session-manager-plugin.deb \
         "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/${SSM_ARCH}/session-manager-plugin.deb" \
    && dpkg -i /tmp/session-manager-plugin.deb \
    && rm -f /tmp/session-manager-plugin.deb \
    && rm -rf /var/lib/apt/lists/*
