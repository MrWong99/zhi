package dev.zhi.plugin.javabean.model;

import jakarta.validation.ConstraintValidator;
import jakarta.validation.ConstraintValidatorContext;

/**
 * Validates that SSL is enabled when the database host is not localhost.
 * The violation is attached to both the {@code host} and {@code sslEnabled}
 * properties so that it surfaces when either path is validated by zhi.
 */
public class SslRequiredForRemoteHostValidator
        implements ConstraintValidator<SslRequiredForRemoteHost, DatabaseConfig> {

    @Override
    public boolean isValid(DatabaseConfig config, ConstraintValidatorContext ctx) {
        if (config == null) {
            return true;
        }

        String host = config.getHost();
        boolean isLocal = "localhost".equals(host) || "127.0.0.1".equals(host);
        if (isLocal || config.isSslEnabled()) {
            return true;
        }

        // Disable the default violation (class-level, empty property path) and
        // attach the message to the two relevant fields instead.
        ctx.disableDefaultConstraintViolation();
        String msg = "SSL should be enabled when connecting to remote host '" + host + "'";
        ctx.buildConstraintViolationWithTemplate(msg)
                .addPropertyNode("sslEnabled")
                .addConstraintViolation();
        ctx.buildConstraintViolationWithTemplate(msg)
                .addPropertyNode("host")
                .addConstraintViolation();
        return false;
    }
}
