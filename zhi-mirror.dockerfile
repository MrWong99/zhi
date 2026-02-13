# syntax=docker/dockerfile:1

FROM busybox:musl AS dirs
RUN mkdir -p /etc/zhi-mirror /var/zhi-mirror

FROM scratch

# Don't run as root
USER 10669:10669

# Copy the pre-built binary from bin/
COPY --chown=10669:10669 bin/zhi-mirror /usr/local/bin/zhi-mirror

# Create directories for policy and storage
COPY --chown=10669:10669 --from=dirs /etc/zhi-mirror /etc/zhi-mirror
COPY --chown=10669:10669 --from=dirs /var/zhi-mirror /var/zhi-mirror
VOLUME ["/etc/zhi-mirror", "/var/zhi-mirror"]

# Expose default port (can be overridden)
EXPOSE 8080

# Default entrypoint and command
ENTRYPOINT ["/usr/local/bin/zhi-mirror"]
CMD ["serve", \
    "--listen", "0.0.0.0:8080", \
    "--policy", "/etc/zhi-mirror/policy.yaml", \
    "--storage", "/var/zhi-mirror/store", \
    "--upstream-marketplace", "https://marketplace.zhi.dev", \
    "--upstream-registry", "ghcr.io"]
