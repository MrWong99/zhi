package dev.zhi.plugin.javabean;

import dev.zhi.plugin.javabean.annotation.ConfigPrefix;
import dev.zhi.plugin.javabean.annotation.ConfigProperty;
import dev.zhi.plugin.javabean.annotation.WarnConstraint;
import jakarta.validation.ConstraintViolation;
import jakarta.validation.Validation;
import jakarta.validation.Validator;
import org.hibernate.validator.messageinterpolation.ParameterMessageInterpolator;

import java.lang.annotation.Annotation;
import java.lang.reflect.Field;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/**
 * Bridges an annotated Java bean to the zhi configuration model. Fields are
 * discovered via reflection and mapped to slash-delimited zhi paths. Jakarta
 * Bean Validation annotations on the bean drive the {@link #validate} method.
 *
 * <p>The {@link ParameterMessageInterpolator} is used instead of the default
 * EL-based interpolator so that no Expression Language implementation is
 * needed at runtime. This simplifies GraalVM native-image builds.
 *
 * @param <T> the bean type
 */
public class BeanReflector<T> {

    /** A single value entry ready for gRPC serialisation. */
    public record ValueEntry(String valueJson, String metadataJson) {}

    /** A validation result with zhi severity. */
    public record ValidationEntry(int severity, String message) {}

    private final Class<T> beanClass;
    private T instance;
    private final String prefix;
    private final Map<String, Field> pathToField;   // config path -> Field
    private final Map<String, String> fieldToPath;   // field name  -> config path
    private final Map<String, String> descriptions;  // config path -> description
    private final Validator validator;
    private final ReadWriteLock lock = new ReentrantReadWriteLock();

    public BeanReflector(Class<T> beanClass, T defaultInstance) {
        this.beanClass = beanClass;
        this.instance = defaultInstance;

        ConfigPrefix prefixAnn = beanClass.getAnnotation(ConfigPrefix.class);
        this.prefix = (prefixAnn != null) ? prefixAnn.value()
                : beanClass.getSimpleName().toLowerCase();

        this.pathToField = new LinkedHashMap<>();
        this.fieldToPath = new HashMap<>();
        this.descriptions = new HashMap<>();

        for (Field field : beanClass.getDeclaredFields()) {
            field.setAccessible(true);

            String segment;
            String description = null;

            ConfigProperty prop = field.getAnnotation(ConfigProperty.class);
            if (prop != null && !prop.path().isEmpty()) {
                segment = prop.path();
            } else {
                segment = toKebabCase(field.getName());
            }
            if (prop != null && !prop.description().isEmpty()) {
                description = prop.description();
            }

            String fullPath = prefix + "/" + segment;
            pathToField.put(fullPath, field);
            fieldToPath.put(field.getName(), fullPath);
            if (description != null) {
                descriptions.put(fullPath, description);
            }
        }

        this.validator = Validation.byDefaultProvider()
                .configure()
                .messageInterpolator(new ParameterMessageInterpolator())
                .buildValidatorFactory()
                .getValidator();
    }

    /** Returns all configuration paths managed by this bean, in declaration order. */
    public List<String> listPaths() {
        return new ArrayList<>(pathToField.keySet());
    }

    /** Reads the current value for the given path, or {@code null} if unknown. */
    public ValueEntry getValue(String path) {
        lock.readLock().lock();
        try {
            Field field = pathToField.get(path);
            if (field == null) {
                return null;
            }
            Object value = field.get(instance);
            String valueJson = toJson(value);

            String metadataJson = null;
            String desc = descriptions.get(path);
            if (desc != null) {
                metadataJson = "{\"description\":" + toJson(desc) + "}";
            }
            return new ValueEntry(valueJson, metadataJson);
        } catch (IllegalAccessException e) {
            throw new RuntimeException("failed to read field for path: " + path, e);
        } finally {
            lock.readLock().unlock();
        }
    }

    /** Stores a JSON-encoded value at the given path. */
    public void setValue(String path, String valueJson, String metadataJson) {
        lock.writeLock().lock();
        try {
            Field field = pathToField.get(path);
            if (field == null) {
                return;
            }
            Object value = fromJson(valueJson, field.getType());
            field.set(instance, value);
        } catch (IllegalAccessException e) {
            throw new RuntimeException("failed to set field for path: " + path, e);
        } finally {
            lock.writeLock().unlock();
        }
    }

    /**
     * Validates the value at {@code path} using Jakarta Bean Validation. The
     * full tree snapshot ({@code treeValues}) is used to populate a temporary
     * bean so that cross-value constraints can fire.
     *
     * @param path       the config path being validated
     * @param treeValues map of config path to JSON-encoded value (full tree)
     * @return validation results filtered to the requested path
     */
    public List<ValidationEntry> validate(String path, Map<String, String> treeValues) {
        lock.readLock().lock();
        try {
            Field targetField = pathToField.get(path);
            if (targetField == null) {
                return Collections.emptyList();
            }

            // Create a temporary bean populated from the tree snapshot.
            T temp = beanClass.getDeclaredConstructor().newInstance();
            for (Map.Entry<String, String> entry : treeValues.entrySet()) {
                Field field = pathToField.get(entry.getKey());
                if (field != null) {
                    Object value = fromJson(entry.getValue(), field.getType());
                    field.set(temp, value);
                }
            }

            Set<ConstraintViolation<T>> violations = validator.validate(temp);

            List<ValidationEntry> results = new ArrayList<>();
            for (ConstraintViolation<T> v : violations) {
                String propPath = v.getPropertyPath().toString();
                String violationConfigPath = fieldToPath.get(propPath);

                // Include violation if it targets the requested path, or if
                // the property path is empty (class-level constraint).
                if (path.equals(violationConfigPath) || propPath.isEmpty()) {
                    int severity = severityFor(v);
                    results.add(new ValidationEntry(severity, v.getMessage()));
                }
            }
            return results;
        } catch (ReflectiveOperationException e) {
            return List.of(new ValidationEntry(2, "validation error: " + e.getMessage()));
        } finally {
            lock.readLock().unlock();
        }
    }

    // ---- severity mapping ---------------------------------------------------

    /**
     * Determines the zhi severity for a constraint violation. If the
     * constraint annotation is meta-annotated with {@link WarnConstraint} the
     * severity is Warning (1); otherwise it is Blocking (2).
     */
    private int severityFor(ConstraintViolation<T> violation) {
        Annotation constraintAnnotation = violation.getConstraintDescriptor().getAnnotation();
        if (constraintAnnotation.annotationType().isAnnotationPresent(WarnConstraint.class)) {
            return 1; // Warning
        }
        return 2; // Blocking
    }

    // ---- minimal JSON helpers (no external dependency) ----------------------

    static String toJson(Object value) {
        if (value == null) return "null";
        if (value instanceof String s) return "\"" + escapeJson(s) + "\"";
        if (value instanceof Number || value instanceof Boolean) return value.toString();
        return "\"" + escapeJson(value.toString()) + "\"";
    }

    static Object fromJson(String json, Class<?> type) {
        if (json == null || json.isEmpty() || "null".equals(json)) {
            return defaultFor(type);
        }
        String trimmed = json.trim();

        if (type == String.class) {
            if (trimmed.startsWith("\"") && trimmed.endsWith("\"")) {
                return unescapeJson(trimmed.substring(1, trimmed.length() - 1));
            }
            return trimmed;
        }
        if (type == int.class || type == Integer.class) {
            return (int) Double.parseDouble(trimmed);
        }
        if (type == long.class || type == Long.class) {
            return (long) Double.parseDouble(trimmed);
        }
        if (type == double.class || type == Double.class) {
            return Double.parseDouble(trimmed);
        }
        if (type == float.class || type == Float.class) {
            return (float) Double.parseDouble(trimmed);
        }
        if (type == boolean.class || type == Boolean.class) {
            return Boolean.parseBoolean(trimmed);
        }

        // Fallback: treat as string.
        if (trimmed.startsWith("\"") && trimmed.endsWith("\"")) {
            return unescapeJson(trimmed.substring(1, trimmed.length() - 1));
        }
        return trimmed;
    }

    private static Object defaultFor(Class<?> type) {
        if (type == int.class)     return 0;
        if (type == long.class)    return 0L;
        if (type == double.class)  return 0.0;
        if (type == float.class)   return 0.0f;
        if (type == boolean.class) return false;
        return null;
    }

    private static String escapeJson(String s) {
        return s.replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t");
    }

    private static String unescapeJson(String s) {
        return s.replace("\\\"", "\"")
                .replace("\\\\", "\\")
                .replace("\\n", "\n")
                .replace("\\r", "\r")
                .replace("\\t", "\t");
    }

    /** Converts a camelCase name to kebab-case (e.g. maxConnections -> max-connections). */
    static String toKebabCase(String camelCase) {
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < camelCase.length(); i++) {
            char c = camelCase.charAt(i);
            if (Character.isUpperCase(c)) {
                if (i > 0) sb.append('-');
                sb.append(Character.toLowerCase(c));
            } else {
                sb.append(c);
            }
        }
        return sb.toString();
    }
}
