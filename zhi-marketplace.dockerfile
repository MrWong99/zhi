# syntax=docker/dockerfile:1

FROM scratch

# Copy the pre-built binary from bin/
COPY bin/zhi-marketplace /usr/local/bin/zhi-marketplace

# Create directory for database
# Note: In scratch images, we rely on volume mounts for persistence
VOLUME ["/var/zhi-marketplace"]

# Expose default port (can be overridden)
EXPOSE 8080

# Default entrypoint and command
ENTRYPOINT ["/usr/local/bin/zhi-marketplace"]
CMD ["--listen", "0.0.0.0:8080", \
     "--api-keys", "", \
     "--db", "/var/zhi-marketplace/marketplace.db", \
     "--oci-registries", "ghcr.io"]
