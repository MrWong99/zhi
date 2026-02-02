package dev.zhi.plugin.javabean;

import dev.zhi.plugin.javabean.model.DatabaseConfig;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.protobuf.services.HealthStatusManager;

import java.io.IOException;
import java.net.ServerSocket;

/**
 * Entry point for the {@code zhi-config-javabean} plugin. Implements the
 * <a href="https://github.com/hashicorp/go-plugin">hashicorp/go-plugin</a>
 * startup protocol so the zhi host can launch and connect to this binary
 * over gRPC.
 *
 * <h2>go-plugin handshake protocol</h2>
 * <ol>
 *   <li>Verify the magic-cookie environment variable.</li>
 *   <li>Start a gRPC server on a random TCP port.</li>
 *   <li>Print the connection info to stdout:
 *       {@code <core_version>|<app_version>|tcp|<host:port>|grpc}.</li>
 *   <li>Monitor stdin &ndash; when the host closes it the plugin shuts down.</li>
 * </ol>
 */
public class Main {

    // These constants must match the values in pkg/zhiplugin/plugin.go.
    private static final String MAGIC_COOKIE_KEY   = "ZHI_PLUGIN";
    private static final String MAGIC_COOKIE_VALUE = "zhiplugin-v1";
    private static final int CORE_PROTOCOL_VERSION = 1;
    private static final int APP_PROTOCOL_VERSION  = 1;

    public static void main(String[] args) throws Exception {
        // ---- 1. Verify magic cookie -----------------------------------------
        String cookie = System.getenv(MAGIC_COOKIE_KEY);
        if (!MAGIC_COOKIE_VALUE.equals(cookie)) {
            System.err.println("This binary is a zhi plugin. It is not meant to be executed directly.");
            System.err.println("Please execute the 'zhi' binary instead.");
            System.exit(1);
        }

        // ---- 2. Set up the gRPC services ------------------------------------
        var reflector = new BeanReflector<>(DatabaseConfig.class, new DatabaseConfig());
        var configService = new ConfigServiceImpl(reflector);
        var healthManager = new HealthStatusManager();

        int port = findFreePort();
        Server server = ServerBuilder.forPort(port)
                .addService(configService)
                .addService(healthManager.getHealthService())
                .build()
                .start();

        // ---- 3. Write the handshake line ------------------------------------
        System.out.printf("%d|%d|tcp|127.0.0.1:%d|grpc%n",
                CORE_PROTOCOL_VERSION, APP_PROTOCOL_VERSION, port);
        System.out.flush();

        // ---- 4. Monitor stdin for parent death ------------------------------
        Thread stdinWatcher = new Thread(() -> {
            try {
                //noinspection StatementWithEmptyBody
                while (System.in.read() != -1) {
                    // consume until EOF
                }
            } catch (IOException ignored) {
                // stdin closed
            }
            server.shutdown();
        }, "stdin-watcher");
        stdinWatcher.setDaemon(true);
        stdinWatcher.start();

        server.awaitTermination();
    }

    private static int findFreePort() throws IOException {
        try (ServerSocket s = new ServerSocket(0)) {
            return s.getLocalPort();
        }
    }
}
