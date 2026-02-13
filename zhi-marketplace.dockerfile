# syntax=docker/dockerfile:1

FROM busybox:musl AS dirs
RUN mkdir -p /var/zhi-marketplace

FROM scratch

# Don't run as root
USER 10668:10668

# Copy the pre-built binary from bin/
COPY --chown=10668:10668 bin/zhi-marketplace /usr/local/bin/zhi-marketplace

# Create directory for database
COPY --chown=10668:10668 --from=dirs /var/zhi-marketplace /var/zhi-marketplace
VOLUME ["/var/zhi-marketplace"]

# Expose default port (can be overridden)
EXPOSE 8080

# Default entrypoint and command
ENTRYPOINT ["/usr/local/bin/zhi-marketplace"]
CMD ["--listen", "0.0.0.0:8080", \
    "--api-keys", "", \
    "--db", "/var/zhi-marketplace/marketplace.db", \
    "--oci-registries", "ghcr.io"]
