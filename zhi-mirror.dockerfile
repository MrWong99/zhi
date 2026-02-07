# syntax=docker/dockerfile:1

FROM scratch

# Copy the pre-built binary from bin/
COPY bin/zhi-mirror /usr/local/bin/zhi-mirror

# Create directories for policy and storage
# Note: In scratch images, we rely on volume mounts or ConfigMaps to populate these
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
