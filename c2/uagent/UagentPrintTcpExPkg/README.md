# RunAndGetDebugTestPkg

`RunAndGetDebugTestPkg` is a small standalone UEFI test application for the custom `RunAndGet` debug protocol.

It does one thing:

- locates the `RunAndGet` custom protocol by GUID
- calls `SendDebugMessage()`
- prints the returned `EFI_STATUS` locally on the EFI console

## Purpose

Use this package to verify that:

- `RunAndGetPkg` has installed the custom protocol
- the protocol is reachable from another EFI application
- the message is forwarded to the Go server over the active TCP session

## Build

From the EDK2 workspace root:

```sh
build -p RunAndGetDebugTestPkg/RunAndGetDebugTestPkg.dsc -m RunAndGetDebugTestPkg/RunAndGetDebugTest.inf -a X64 -t GCC5
```

Expected output:

`Build/RunAndGetDebugTestPkg/DEBUG_GCC5/X64/RunAndGetDebugTest.efi`

## Test flow

1. Build and start `RunAndGet.efi`.
2. Let it connect to the Go server and install its debug protocol.
3. Launch `RunAndGetDebugTest.efi` on the same EFI system.
4. Check the EFI console for `SendDebugMessage returned: Success`.
5. Check the Go server for:

`Remote Output: HelloWorld from RunAndGetDebugTestPkg`

## Notes

- The protocol only exists while `RunAndGet.efi` is still running and the remote session is active.
- If `RunAndGet.efi` is not active, `LocateProtocol()` should fail.
