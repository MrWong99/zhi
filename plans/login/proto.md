# Proto Message Definitions for Store Authentication

These messages are added to `api/proto/zhiplugin/v1/ui.proto` in the `UIControllerService`.

## New RPCs

```protobuf
service UIControllerService {
  // ... existing RPCs ...

  // --- Store Authentication -------------------------------------------------

  // StoreAuthMethods returns the auth methods supported by the configured store.
  rpc StoreAuthMethods(CtrlStoreAuthMethodsRequest) returns (CtrlStoreAuthMethodsResponse);
  // StoreLogin authenticates with the store using the given method and credentials.
  rpc StoreLogin(CtrlStoreLoginRequest) returns (CtrlStoreLoginResponse);
  // StoreAuthStatus returns the current authentication status.
  rpc StoreAuthStatus(CtrlStoreAuthStatusRequest) returns (CtrlStoreAuthStatusResponse);
  // StoreLogout clears the current authentication session.
  rpc StoreLogout(CtrlStoreLogoutRequest) returns (CtrlStoreLogoutResponse);
}
```

## New Messages

```protobuf
// --- Store Authentication messages -----------------------------------------

message CtrlStoreAuthFieldMsg {
  string name = 1;
  string description = 2;
  bool required = 3;
  bool secret = 4;
}

message CtrlStoreAuthMethodMsg {
  string type = 1;
  string description = 2;
  repeated CtrlStoreAuthFieldMsg fields = 3;
}

// -- StoreAuthMethods -------------------------------------------------------

message CtrlStoreAuthMethodsRequest {}

message CtrlStoreAuthMethodsResponse {
  repeated CtrlStoreAuthMethodMsg methods = 1;
}

// -- StoreLogin -------------------------------------------------------------

message CtrlStoreLoginRequest {
  string method = 1;
  map<string, string> credentials = 2;
}

message CtrlStoreLoginResponse {
  string session_id = 1;
  string status = 2;     // "none", "unauthenticated", "authenticated", "expired"
  string expires_at = 3; // RFC3339 or empty
  map<string, string> metadata = 4;
}

// -- StoreAuthStatus --------------------------------------------------------

message CtrlStoreAuthStatusRequest {}

message CtrlStoreAuthStatusResponse {
  string session_id = 1;
  string status = 2;
  string expires_at = 3;
  map<string, string> metadata = 4;
}

// -- StoreLogout ------------------------------------------------------------

message CtrlStoreLogoutRequest {}

message CtrlStoreLogoutResponse {}
```

## Wire Format Notes

- `credentials` is a `map<string, string>` matching the existing `LoginRequest` in `store.proto`. Secret values are transmitted in the clear over the gRPC channel, which is acceptable because the channel is a local Unix socket or loopback mTLS connection managed by hashicorp/go-plugin.
- `status` uses string encoding rather than an enum to allow forward-compatible additions without regenerating proto code for new status values.
- `metadata` is informational only and may contain fields like `"auth_method"`, `"username"`, or `"display_name"` depending on the store plugin.
