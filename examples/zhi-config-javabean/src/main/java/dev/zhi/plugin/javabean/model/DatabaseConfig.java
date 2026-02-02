package dev.zhi.plugin.javabean.model;

import dev.zhi.plugin.javabean.annotation.ConfigPrefix;
import dev.zhi.plugin.javabean.annotation.ConfigProperty;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;

/**
 * Example configuration bean for a database connection. Each field becomes a
 * zhi configuration path under the {@code database/} prefix. Jakarta Bean
 * Validation annotations drive the validation results returned to the host.
 *
 * <p>Managed paths:
 * <ul>
 *   <li>{@code database/host} &ndash; server hostname</li>
 *   <li>{@code database/port} &ndash; server port</li>
 *   <li>{@code database/name} &ndash; database name</li>
 *   <li>{@code database/username} &ndash; login user</li>
 *   <li>{@code database/max-connections} &ndash; connection pool size</li>
 *   <li>{@code database/ssl-enabled} &ndash; whether to use TLS</li>
 * </ul>
 */
@ConfigPrefix("database")
@SslRequiredForRemoteHost
public class DatabaseConfig {

    @NotBlank(message = "database host is required")
    @ConfigProperty(description = "Database server hostname")
    private String host = "localhost";

    @Min(value = 1, message = "port must be at least 1")
    @Max(value = 65535, message = "port must be at most 65535")
    @ConfigProperty(description = "Database server port")
    private int port = 5432;

    @NotBlank(message = "database name is required")
    @ConfigProperty(description = "Name of the database")
    private String name = "myapp";

    @NotBlank(message = "username is required")
    @ConfigProperty(description = "Database login user")
    private String username = "admin";

    @Min(value = 1, message = "max connections must be at least 1")
    @Max(value = 1000, message = "max connections must be at most 1000")
    @ConfigProperty(path = "max-connections", description = "Maximum number of database connections")
    private int maxConnections = 10;

    @ConfigProperty(path = "ssl-enabled", description = "Whether TLS is used for the database connection")
    private boolean sslEnabled = false;

    // -- accessors (used by the cross-value validator) -------------------------

    public String getHost() {
        return host;
    }

    public int getPort() {
        return port;
    }

    public String getName() {
        return name;
    }

    public String getUsername() {
        return username;
    }

    public int getMaxConnections() {
        return maxConnections;
    }

    public boolean isSslEnabled() {
        return sslEnabled;
    }
}
