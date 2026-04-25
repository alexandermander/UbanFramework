# UagentDebugTestPkg

`UagentDebugTestPkg` is a small standalone UEFI test application for the custom `Uagent` debug protocol.

It does one thing:

- locates the `Uagent` custom protocol by GUID
- calls `SendDebugMessage()`
- prints the returned `EFI_STATUS` locally on the EFI console

## Purpose

Use this package to verify that:

- `UagentPkg` has installed the custom protocol
- the protocol is reachable from another EFI application
- the message is forwarded to the Go server over the active TCP session

## Build

From the EDK2 workspace root:

```sh
build -p UagentPrintTcpExPkg/UagentDebugTestPkg.dsc -m UagentPrintTcpExPkg/UagentDebugTest.inf -a X64 -t GCC5
```

Expected output:

`Build/UagentDebugTestPkg/DEBUG_GCC5/X64/UagentDebugTest.efi`

## Test flow

1. Build and start `Uagent.efi`.
2. Let it connect to the Go server and install its debug protocol.
3. Launch `UagentDebugTest.efi` on the same EFI system.
4. Check the EFI console for `SendDebugMessage returned: Success`.
5. Check the Go server for:

`Remote Output: HelloWorld from UagentDebugTestPkg`

## Notes

- The protocol only exists while `Uagent.efi` is still running and the remote session is active.
- If `Uagent.efi` is not active, `LocateProtocol()` should fail.

