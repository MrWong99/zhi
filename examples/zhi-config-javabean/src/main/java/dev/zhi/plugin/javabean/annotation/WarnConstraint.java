package dev.zhi.plugin.javabean.annotation;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Meta-annotation for Jakarta Bean Validation constraints. When a constraint
 * annotation is itself annotated with {@code @WarnConstraint}, violations
 * produced by that constraint are reported with zhi severity
 * <strong>Warning</strong> (1) instead of the default
 * <strong>Blocking</strong> (2).
 *
 * <p>This lets you distinguish hard validation errors (missing required
 * values, out-of-range numbers) from softer recommendations (security best
 * practices, optional settings).
 */
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.ANNOTATION_TYPE)
public @interface WarnConstraint {
}
