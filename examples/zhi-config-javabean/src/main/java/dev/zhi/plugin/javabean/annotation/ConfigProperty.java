package dev.zhi.plugin.javabean.annotation;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Customises how a bean field is exposed as a zhi configuration path.
 *
 * <p>When {@link #path()} is empty the field name is converted to
 * {@code kebab-case} automatically (e.g. {@code maxConnections} becomes
 * {@code max-connections}).
 */
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.FIELD)
public @interface ConfigProperty {
    /** Override the path segment. Empty means derive from the field name. */
    String path() default "";

    /** Human-readable description exposed as metadata. */
    String description() default "";
}
