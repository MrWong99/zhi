package dev.zhi.plugin.javabean;

import com.google.protobuf.ByteString;
import dev.zhi.plugin.javabean.model.DatabaseConfig;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import zhiplugin.v1.Config.GetRequest;
import zhiplugin.v1.Config.ListRequest;
import zhiplugin.v1.Config.SetRequest;
import zhiplugin.v1.Config.TreeEntry;
import zhiplugin.v1.Config.ValidateRequest;
import zhiplugin.v1.Config.ValidateResponse;
import zhiplugin.v1.ConfigServiceGrpc;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests the Java config plugin through the full gRPC round-trip using an
 * in-process transport (no network, no subprocess).
 */
class ConfigServiceImplTest {

    private Server server;
    private ManagedChannel channel;
    private ConfigServiceGrpc.ConfigServiceBlockingStub stub;

    @BeforeEach
    void setUp() throws Exception {
        String serverName = InProcessServerBuilder.generateName();

        var reflector = new BeanReflector<>(DatabaseConfig.class, new DatabaseConfig());
        var service = new ConfigServiceImpl(reflector);

        server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(service)
                .build()
                .start();

        channel = InProcessChannelBuilder.forName(serverName)
                .directExecutor()
                .build();

        stub = ConfigServiceGrpc.newBlockingStub(channel);
    }

    @AfterEach
    void tearDown() {
        channel.shutdownNow();
        server.shutdownNow();
    }

    @Test
    void listReturnsSixPaths() {
        var resp = stub.list(ListRequest.getDefaultInstance());
        assertEquals(6, resp.getPathsCount());
        assertTrue(resp.getPathsList().contains("database/host"));
        assertTrue(resp.getPathsList().contains("database/port"));
        assertTrue(resp.getPathsList().contains("database/name"));
        assertTrue(resp.getPathsList().contains("database/username"));
        assertTrue(resp.getPathsList().contains("database/max-connections"));
        assertTrue(resp.getPathsList().contains("database/ssl-enabled"));
    }

    @Test
    void getReturnsDefaultValues() {
        var resp = stub.get(GetRequest.newBuilder().setPath("database/host").build());
        assertTrue(resp.getFound());
        assertEquals("\"localhost\"", resp.getValueJson().toStringUtf8());

        resp = stub.get(GetRequest.newBuilder().setPath("database/port").build());
        assertTrue(resp.getFound());
        assertEquals("5432", resp.getValueJson().toStringUtf8());

        resp = stub.get(GetRequest.newBuilder().setPath("database/ssl-enabled").build());
        assertTrue(resp.getFound());
        assertEquals("false", resp.getValueJson().toStringUtf8());
    }

    @Test
    void getReturnsMetadata() {
        var resp = stub.get(GetRequest.newBuilder().setPath("database/host").build());
        assertTrue(resp.getFound());
        String meta = resp.getMetadataJson().toStringUtf8();
        assertTrue(meta.contains("description"));
        assertTrue(meta.contains("Database server hostname"));
    }

    @Test
    void getUnknownPathReturnsNotFound() {
        var resp = stub.get(GetRequest.newBuilder().setPath("unknown/path").build());
        assertFalse(resp.getFound());
    }

    @Test
    void setUpdatesValue() {
        stub.set(SetRequest.newBuilder()
                .setPath("database/host")
                .setValueJson(ByteString.copyFromUtf8("\"db.example.com\""))
                .build());

        var resp = stub.get(GetRequest.newBuilder().setPath("database/host").build());
        assertTrue(resp.getFound());
        assertEquals("\"db.example.com\"", resp.getValueJson().toStringUtf8());
    }

    @Test
    void validatePassesForValidDefaults() {
        var tree = defaultTree();
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/host")
                .addAllTree(tree)
                .build());
        assertTrue(resp.getResultsList().isEmpty(),
                "default host should pass validation");
    }

    @Test
    void validateBlocksBlankHost() {
        var tree = defaultTreeWith("database/host", "\"\"");
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/host")
                .addAllTree(tree)
                .build());
        assertFalse(resp.getResultsList().isEmpty());
        assertEquals(2, resp.getResults(0).getSeverity(), "should be blocking");
        assertTrue(resp.getResults(0).getMessage().contains("required"));
    }

    @Test
    void validateBlocksPortOutOfRange() {
        var tree = defaultTreeWith("database/port", "99999");
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/port")
                .addAllTree(tree)
                .build());
        assertFalse(resp.getResultsList().isEmpty());
        assertEquals(2, resp.getResults(0).getSeverity());
        assertTrue(resp.getResults(0).getMessage().contains("65535"));
    }

    @Test
    void validateWarnsAboutMissingSsl() {
        // Remote host without SSL should produce a Warning.
        var tree = defaultTreeWith("database/host", "\"db.example.com\"");
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/host")
                .addAllTree(tree)
                .build());
        assertHasWarning(resp, "SSL");
    }

    @Test
    void validateNoSslWarningForLocalhost() {
        var tree = defaultTree();
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/host")
                .addAllTree(tree)
                .build());
        for (var r : resp.getResultsList()) {
            assertFalse(r.getMessage().contains("SSL"),
                    "localhost should not trigger SSL warning");
        }
    }

    @Test
    void validateNoSslWarningWhenSslEnabled() {
        var tree = defaultTree();
        // Override host to remote and enable SSL.
        tree = replaceEntry(tree, "database/host", "\"db.example.com\"");
        tree = replaceEntry(tree, "database/ssl-enabled", "true");
        var resp = stub.validate(ValidateRequest.newBuilder()
                .setPath("database/host")
                .addAllTree(tree)
                .build());
        for (var r : resp.getResultsList()) {
            assertFalse(r.getMessage().contains("SSL"),
                    "SSL enabled should suppress SSL warning");
        }
    }

    // --- helpers -------------------------------------------------------------

    private static java.util.List<TreeEntry> defaultTree() {
        return java.util.List.of(
                entry("database/host", "\"localhost\""),
                entry("database/port", "5432"),
                entry("database/name", "\"myapp\""),
                entry("database/username", "\"admin\""),
                entry("database/max-connections", "10"),
                entry("database/ssl-enabled", "false")
        );
    }

    private static java.util.List<TreeEntry> defaultTreeWith(String path, String valueJson) {
        return replaceEntry(defaultTree(), path, valueJson);
    }

    private static java.util.List<TreeEntry> replaceEntry(
            java.util.List<TreeEntry> tree, String path, String valueJson) {
        var result = new java.util.ArrayList<TreeEntry>();
        for (var e : tree) {
            if (e.getPath().equals(path)) {
                result.add(entry(path, valueJson));
            } else {
                result.add(e);
            }
        }
        return result;
    }

    private static TreeEntry entry(String path, String valueJson) {
        return TreeEntry.newBuilder()
                .setPath(path)
                .setValueJson(ByteString.copyFromUtf8(valueJson))
                .build();
    }

    private static void assertHasWarning(ValidateResponse resp, String substring) {
        boolean found = resp.getResultsList().stream()
                .anyMatch(r -> r.getSeverity() == 1 && r.getMessage().contains(substring));
        assertTrue(found,
                "expected a warning containing '" + substring + "' in " + resp.getResultsList());
    }
}
