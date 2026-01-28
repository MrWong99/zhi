# API Definitions of zhi

The `proto/zhiplugin/v1/` directory contains the Protocol Buffer service definitions for all plugin types (config, transform, store). These `.proto` files define the gRPC contracts between the zhi host and plugin processes. Run `make proto` from the project root to regenerate the Go stubs after editing.
