package dev.zhi.plugin.javabean.model;

import dev.zhi.plugin.javabean.annotation.WarnConstraint;
import jakarta.validation.Constraint;
import jakarta.validation.Payload;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Cross-value constraint that checks whether SSL is enabled when the database
 * host is not localhost. Violations are reported as warnings (not blocking)
 * because of the {@link WarnConstraint} meta-annotation.
 */
@WarnConstraint
@Constraint(validatedBy = SslRequiredForRemoteHostValidator.class)
@Target(ElementType.TYPE)
@Retention(RetentionPolicy.RUNTIME)
public @interface SslRequiredForRemoteHost {
    String message() default "SSL should be enabled when connecting to a remote host";

    Class<?>[] groups() default {};

    Class<? extends Payload>[] payload() default {};
}
