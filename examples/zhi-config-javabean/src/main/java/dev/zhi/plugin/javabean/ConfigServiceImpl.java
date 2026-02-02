package dev.zhi.plugin.javabean;

import com.google.protobuf.ByteString;
import io.grpc.stub.StreamObserver;
import zhiplugin.v1.Config.GetRequest;
import zhiplugin.v1.Config.GetResponse;
import zhiplugin.v1.Config.ListRequest;
import zhiplugin.v1.Config.ListResponse;
import zhiplugin.v1.Config.SetRequest;
import zhiplugin.v1.Config.SetResponse;
import zhiplugin.v1.Config.ValidateRequest;
import zhiplugin.v1.Config.ValidateResponse;
import zhiplugin.v1.Config.ValidationResultMsg;
import zhiplugin.v1.ConfigServiceGrpc;

import java.util.HashMap;
import java.util.Map;

/**
 * gRPC implementation of the zhi {@code ConfigService}. Delegates all
 * operations to one or more {@link BeanReflector} instances.
 */
public class ConfigServiceImpl extends ConfigServiceGrpc.ConfigServiceImplBase {

    private final BeanReflector<?>[] reflectors;

    public ConfigServiceImpl(BeanReflector<?>... reflectors) {
        this.reflectors = reflectors;
    }

    @Override
    public void list(ListRequest request, StreamObserver<ListResponse> observer) {
        var builder = ListResponse.newBuilder();
        for (BeanReflector<?> r : reflectors) {
            builder.addAllPaths(r.listPaths());
        }
        observer.onNext(builder.build());
        observer.onCompleted();
    }

    @Override
    public void get(GetRequest request, StreamObserver<GetResponse> observer) {
        String path = request.getPath();
        for (BeanReflector<?> r : reflectors) {
            var entry = r.getValue(path);
            if (entry != null) {
                var builder = GetResponse.newBuilder()
                        .setFound(true)
                        .setValueJson(ByteString.copyFromUtf8(entry.valueJson()));
                if (entry.metadataJson() != null) {
                    builder.setMetadataJson(ByteString.copyFromUtf8(entry.metadataJson()));
                }
                observer.onNext(builder.build());
                observer.onCompleted();
                return;
            }
        }
        observer.onNext(GetResponse.newBuilder().setFound(false).build());
        observer.onCompleted();
    }

    @Override
    public void set(SetRequest request, StreamObserver<SetResponse> observer) {
        String path = request.getPath();
        String valueJson = request.getValueJson().toStringUtf8();
        String metadataJson = request.getMetadataJson().isEmpty()
                ? null : request.getMetadataJson().toStringUtf8();
        for (BeanReflector<?> r : reflectors) {
            // setValue is a no-op for paths the reflector does not own.
            r.setValue(path, valueJson, metadataJson);
        }
        observer.onNext(SetResponse.getDefaultInstance());
        observer.onCompleted();
    }

    @Override
    public void validate(ValidateRequest request, StreamObserver<ValidateResponse> observer) {
        String path = request.getPath();

        // Rebuild the tree as a simple path -> valueJson map.
        Map<String, String> treeValues = new HashMap<>();
        for (var entry : request.getTreeList()) {
            treeValues.put(entry.getPath(), entry.getValueJson().toStringUtf8());
        }

        var builder = ValidateResponse.newBuilder();
        for (BeanReflector<?> r : reflectors) {
            for (var result : r.validate(path, treeValues)) {
                builder.addResults(ValidationResultMsg.newBuilder()
                        .setSeverity(result.severity())
                        .setMessage(result.message())
                        .build());
            }
        }
        observer.onNext(builder.build());
        observer.onCompleted();
    }
}
