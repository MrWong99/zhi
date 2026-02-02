package dev.zhi.plugin.javabean.annotation;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Declares the path prefix for all configuration values managed by the
 * annotated bean. The prefix becomes the first segment of every zhi
 * configuration path (e.g. {@code "database"} produces paths like
 * {@code database/host}).
 *
 * <p>If this annotation is absent the simple class name in lower-case is used.
 */
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.TYPE)
public @interface ConfigPrefix {
    /** The path prefix (must satisfy the zhi path segment rules). */
    String value();
}
