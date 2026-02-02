plugins {
    java
    application
    id("com.google.protobuf") version "0.9.4"
    id("org.graalvm.buildtools.native") version "0.10.4"
}

group = "dev.zhi.plugin"
version = "0.1.0"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

repositories {
    mavenCentral()
}

val grpcVersion = "1.68.2"
val protobufVersion = "4.28.3"

dependencies {
    implementation(platform("io.grpc:grpc-bom:$grpcVersion"))
    implementation("io.grpc:grpc-netty")
    implementation("io.grpc:grpc-protobuf")
    implementation("io.grpc:grpc-stub")
    implementation("io.grpc:grpc-services") // gRPC health check

    implementation("com.google.protobuf:protobuf-java:$protobufVersion")

    implementation("jakarta.validation:jakarta.validation-api:3.1.0")
    implementation("org.hibernate.validator:hibernate-validator:8.0.1.Final")

    // Required at compile time for generated gRPC stubs (@Generated annotation)
    compileOnly("jakarta.annotation:jakarta.annotation-api:3.0.0")

    testImplementation("io.grpc:grpc-testing")
    testImplementation("io.grpc:grpc-inprocess")
    testImplementation("org.junit.jupiter:junit-jupiter:5.11.4")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

// Reference proto files from the repository root so we stay in sync with the
// Go host. Only config.proto is needed for a config plugin.
sourceSets {
    main {
        proto {
            srcDir("../../api/proto")
            include("zhiplugin/v1/config.proto")
        }
    }
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:$protobufVersion"
    }
    plugins {
        create("grpc") {
            artifact = "io.grpc:protoc-gen-grpc-java:$grpcVersion"
        }
    }
    generateProtoTasks {
        all().forEach { task ->
            task.plugins {
                create("grpc")
            }
        }
    }
}

application {
    mainClass.set("dev.zhi.plugin.javabean.Main")
}

tasks.test {
    useJUnitPlatform()
}

graalvmNative {
    metadataRepository {
        enabled.set(true)
    }
    binaries {
        named("main") {
            imageName.set("zhi-config-javabean")
            mainClass.set("dev.zhi.plugin.javabean.Main")
            buildArgs.addAll(
                "--no-fallback",
                "-H:+ReportExceptionStackTraces",
            )
        }
    }
}
