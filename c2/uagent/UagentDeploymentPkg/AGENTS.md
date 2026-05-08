# TestDevPkg Agent Guide

## Purpose

`TestDevPkg` is a small EDK2 package for building standalone UEFI test applications that send remote-visible debug text through the `Uagent` custom debug protocol.

This package exists so an agent can quickly create or modify EFI test apps without needing to understand the full `UagentPkg` implementation.

## What The Agent Should Do Here

Use this package when the task is one of these:

- create a new EFI test app that sends a debug message to the active `Uagent` server session
- change the message text sent by a test app
- add another debug send
- add simple local `Print()` confirmation text for the operator
- wire a new test app into the EDK2 build through `.inf` and `.dsc`
- build, deploy, and run a test EFI through the existing `userver` service

## What The Agent Must Not Do

- do not change the `Uagent` packet format here
- do not invent a new network transport
- do not assume `Print()` or `DEBUG()` is visible to the Go server
- do not replace the `UAGENT_DEBUG_PROTOCOL` path with raw TCP code in this package
- do not change GUIDs unless the owning protocol definition changes intentionally
- do not create a custom uploader or runner when `userver` already provides `push` and `run`

## Dependency Contract

This package depends on `UagentPkg` providing the custom protocol defined in:

- `../UagentPkg/Uagent.h`

The key contract is:

- protocol GUID: `UAGENT_DEBUG_PROTOCOL_GUID`
- protocol type: `UAGENT_DEBUG_PROTOCOL`
- remote send method: `SendDebugMessage()`

This package does not own the transport. It only consumes the protocol.

## Core Rule: Remote Output

UEFI console output is not server output.

These only print locally:

- `Print()`
- `DEBUG()`

If text must be visible on the Go server, the EFI app must:

1. locate `UAGENT_DEBUG_PROTOCOL`
2. call `SendDebugMessage()`

That call is the approved path because `UagentPkg` converts it into a `TcpOutputText` packet for the server.

Agents must not treat local `Print()` success as proof that the message reached the server. Remote success means `SendDebugMessage()` was called and the resulting text is visible through `userver run` or `userver outputs`.

Current transport behavior:

- `UagentPkg` now sends `TcpConnectSession` and `TcpOutputText` payloads as raw `CHAR16` bytes instead of squeezing them through the old fixed ASCII buffer
- `userver` decodes those payloads for display
- if a payload is not readable text, `userver` shows it as `hex:...`

## Standard Implementation Recipe

When asked to make an EFI app in this package that sends a message to the server, do this:

1. Create or update a `.c` file with `UefiMain()`.
2. Include:
   - `<Uefi.h>`
   - `<Library/UefiApplicationEntryPoint.h>`
   - `<Library/UefiBootServicesTableLib.h>`
   - `<Library/UefiLib.h>`
   - `../UagentPkg/Uagent.h`
3. Define a local GUID variable from `UAGENT_DEBUG_PROTOCOL_GUID`.
4. Call `gBS->LocateProtocol()`.
5. If protocol lookup fails:
   - print the status locally with `Print()`
   - return the failure status
6. If lookup succeeds:
   - call `Debug->SendDebugMessage(Debug, L"...message...")`
   - print the returned status locally with `Print()`
   - return that status
7. Ensure the app has a valid `.inf`.
8. Ensure the package `.dsc` includes the component.

For any non-trivial test app, also send remote-visible marker messages:

- one clear start marker through `SendDebugMessage()`
- one clear success or completion marker through `SendDebugMessage()`
- one remote failure marker if the app aborts after protocol lookup succeeds

The marker text should be distinctive enough that an operator can recognize it immediately in `userver run`.

## Required File Pattern

For each new EFI test app, the expected files are:

- `Name.c`
- `Name.inf`

The package must also include the module in:

- `TestDevPkg.dsc`

If the request is only to change message text, usually only the `.c` file needs to change.

## Minimal Known-Good Template

Use this pattern unless the task requires something more specific:

```c
#include <Uefi.h>

#include <Library/UefiApplicationEntryPoint.h>
#include <Library/UefiBootServicesTableLib.h>
#include <Library/UefiLib.h>

#include "../UagentPkg/Uagent.h"

STATIC EFI_GUID  mUagentDebugProtocolGuid = UAGENT_DEBUG_PROTOCOL_GUID;

EFI_STATUS
EFIAPI
UefiMain (
  IN EFI_HANDLE        ImageHandle,
  IN EFI_SYSTEM_TABLE  *SystemTable
  )
{
  EFI_STATUS                  Status;
  UAGENT_DEBUG_PROTOCOL  *Debug;

  Status = gBS->LocateProtocol (
                  &mUagentDebugProtocolGuid,
                  NULL,
                  (VOID **)&Debug
                  );
  if (EFI_ERROR (Status)) {
    Print (L"LocateProtocol failed: %r\n", Status);
    return Status;
  }

  Status = Debug->SendDebugMessage (
                    Debug,
                    L"Hello from TestDevPkg"
                    );
  Print (L"SendDebugMessage returned: %r\n", Status);
  return Status;
}
```

When adapting this template, prefer a distinctive remote message like:

```c
Status = Debug->SendDebugMessage (
                  Debug,
                  L"[VariableDumpToServer] start"
                  );
```

and send a matching completion line before returning success.

## `.inf` Requirements

Each module `.inf` should declare:

- `MODULE_TYPE = UEFI_APPLICATION`
- `ENTRY_POINT = UefiMain`
- the source `.c` file
- package dependency on `MdePkg/MdePkg.dec`

It should link these library classes unless the module has a clear reason not to:

- `UefiApplicationEntryPoint`
- `UefiBootServicesTableLib`
- `UefiLib`

## `.dsc` Requirements

The package `.dsc` must:

- define a valid EDK2 platform
- include `MdePkg/MdePkg.dec`
- provide the standard UEFI library implementations needed by the `.inf`
- list each EFI app under `[Components]`

If a new module is added and not listed in `[Components]`, the build will not include it.

## Build Recipe

For an easier shell setup, agents can source:

```sh
source /home/alexa/Documents/SanderStuff/aau/cyber2/edk2/TestDevPkg/agent_env.sh
```

That file exports:

- `TESTDEVPKG_DIR`
- `EDK2_SOURCE_DIR`
- `USERVER_DIR`

and defines helper functions:

- `testdevpkg_edksetup`
- `testdevpkg_build <ModuleName.inf>`
- `testdevpkg_build_sample`
- `testdevpkg_efi_path <ModuleBaseName>`

Before any EDK2 build, initialize the workspace environment.

From this package directory, the expected setup flow is:

```sh
source /home/alexa/Documents/SanderStuff/aau/cyber2/edk2/TestDevPkg/agent_env.sh
testdevpkg_edksetup
```

Agents must not skip this step. If `edksetup.sh` has not been sourced in the current shell, the `build` command may fail or use the wrong workspace context.

After the environment is set, run the build from the EDK2 workspace root:

```sh
build -p TestDevPkg/TestDevPkg.dsc -m TestDevPkg/<ModuleName>.inf -a X64 -t GCC5
```

If building the package's current sample app, the pattern is:

```sh
testdevpkg_build_sample
```

Expected output location pattern:

`Build/TestDevPkg/DEBUG_GCC5/X64/<ModuleName>.efi`

Expected behavior:

- local EFI console shows `SendDebugMessage returned: Success`
- Go server shows the remote text
- remote text may include Unicode-capable `SendDebugMessage()` content, not just short ASCII strings

## `userver` Workflow

When an agent needs to deploy and run a newly built EFI against the active remote `Uagent` session, use the existing Go control service instead of inventing another transport.

The `userver` project lives at:

- `/home/alexa/Documents/SanderStuff/UbanFramework/c2/userver/`

The binary path is:

- `/home/alexa/Documents/SanderStuff/UbanFramework/c2/userver/bin/userver`

The important control commands are:

- `./bin/userver list`
- `./bin/userver status`
- `./bin/userver use <id>`
- `./bin/userver push <path-to-efi>`
- `./bin/userver run`
- `./bin/userver outputs 50`
- `./bin/userver stop`

Do not create a custom uploader or a second control path when `userver` already exists.

Do not stop at "the EFI built successfully" or "the local console printed success". The agent must verify the remote side when the task is about server-visible output.

## Deployment Commands

From `/home/alexa/Documents/SanderStuff/UbanFramework/c2/userver`, the standard remote deployment flow is:

1. Build the EFI in EDK2.
2. Check for active remote sessions:

```sh
"${USERVER_DIR}/bin/userver" list
```

3. If more than one remote session is connected, select the intended target:

```sh
"${USERVER_DIR}/bin/userver" use <id>
```

4. Push the built EFI to the active remote session:

```sh
"${USERVER_DIR}/bin/userver" push /full/path/to/<ModuleName>.efi
```

5. Execute the uploaded EFI and watch the remote output:

```sh
"${USERVER_DIR}/bin/userver" run
```

6. If additional inspection is needed after execution, fetch buffered output:

```sh
"${USERVER_DIR}/bin/userver" outputs 50
```

Important behavioral rules:

- `push` sends the file basename to the remote side, so the `.efi` filename matters.
- `run` executes the currently uploaded EFI on the active remote connection.
- `run` sends the execute request and returns transport success or failure.
- `outputs` shows buffered `TcpOutputText` messages already received by the Go service.
- `stop` shuts down the background `userver` service.
- if the task is to send text to the server, the agent should expect to see a distinctive remote marker line during `run`
- if no distinctive remote marker exists in the EFI code yet, the agent should add one

## Remote Execution Contract

The expected remote text path is:

1. the EFI app calls `SendDebugMessage()`
2. `UagentPkg` converts that into `TcpOutputText`
3. for `TcpConnectSession` and `TcpOutputText`, `UagentPkg` sends the text payload as raw `CHAR16` bytes
4. `userver` decodes the payload and exposes it through live `run` output and `outputs`

This means:

- `Print()` is for the local EFI console
- `SendDebugMessage()` is for the remote Go server
- `SendDebugMessage()` is no longer limited by the old fixed ASCII payload path used for local control text commands

If the user asks the agent to "deploy the EFI", "send it to the server", "run it remotely", or "test it through Uagent", the agent should assume the correct path is:

1. build the `.efi`
2. use `userver push`
3. use `userver run`
4. verify that remote output contains the expected distinctive `SendDebugMessage()` text

If the task is specifically about proving that output reached the server, the EFI should emit:

- a clear start message
- the main remote payload message or messages
- a clear completion or success message

## Common Failure Modes

`LocateProtocol failed`

- `Uagent.efi` is not running
- the protocol was not installed
- the session ended before the test app was launched

`SendDebugMessage returned: Not Ready`

- the debug protocol exists, but `UagentPkg` does not currently have an active TCP client

Message appears locally but not on server

- the app used `Print()` only
- the app never called `SendDebugMessage()`
- the app called `SendDebugMessage()` but did not emit any distinctive remote marker, so the operator could not tell what was sent

Build succeeds but `.efi` is missing

- wrong `.dsc`
- component missing from `[Components]`
- wrong `.inf` path in build command

`./bin/userver list` shows no connections

- the background `userver` service may not be running
- the remote `Uagent` client may not be connected
- the wrong control socket or machine may be in use

`./bin/userver push ...` fails

- the `.efi` path is wrong
- no active remote connection is selected
- the background service is not reachable

`./bin/userver run` produces no useful remote output

- the uploaded EFI may only be using `Print()`
- the app may not be locating `UAGENT_DEBUG_PROTOCOL`
- the remote session may no longer be active
- the app may be sending remote text, but not with a recognizable start or completion marker
- the payload may not be readable text, in which case `userver` will show it as `hex:...`

Remote output appears but the wrong system received the command

- multiple connections may exist
- check `./bin/userver list`
- explicitly select the target with `./bin/userver use <id>`

## Agent Decision Rules

When the user asks for remote-visible text:

- use `SendDebugMessage()`

When the user asks for local operator feedback:

- use `Print()`

When the user asks for both:

- use both, for different audiences

When the user asks to "make an EDK app that prints to the server":

- create a UEFI application in this package
- consume `UAGENT_DEBUG_PROTOCOL`
- send the text through `SendDebugMessage()`
- include a distinctive remote start or completion marker
- make sure `.inf` and `.dsc` are correct

When the user asks to deploy or run the built EFI remotely:

- use `userver push`
- use `userver run`
- use `userver outputs` if follow-up inspection is needed
- use `userver use <id>` when more than one remote session exists
- confirm that the expected remote marker text actually appeared on the server

## Default Success Criteria

A change in this package is successful when:

- the module builds as a UEFI application
- it can locate `UAGENT_DEBUG_PROTOCOL`
- it calls `SendDebugMessage()` successfully
- the message becomes visible on the Go server
- the agent can deploy and run the resulting `.efi` through `userver` when the task requires remote execution
- the remote output is distinctive enough for an operator to verify that the correct EFI actually ran

