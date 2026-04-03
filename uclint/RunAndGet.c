#include "RunAndGet.h"

#include <Library/UefiApplicationEntryPoint.h>

EFI_GUID  gRunAndGetDebugProtocolGuid = RUN_AND_GET_DEBUG_PROTOCOL_GUID;

STATIC SOCKET_CLIENT  *mActiveClient        = NULL;
STATIC EFI_HANDLE     mDebugProtocolHandle  = NULL;

STATIC
EFI_STATUS
EFIAPI
RunAndGetSendDebugMessage (
  IN RUN_AND_GET_DEBUG_PROTOCOL  *This,
  IN CONST CHAR16                *Message
  )
{
  EFI_STATUS   Status;
  TCP_COMMAND  Command;

  if ((This == NULL) || (Message == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  if ((mActiveClient == NULL) || (mActiveClient->Tcp4 == NULL)) {
    return EFI_NOT_READY;
  }

  Command.Type        = TcpOutputText;
  Command.Text        = (CHAR16 *)Message;
  Command.Payload     = NULL;
  Command.PayloadSize = 0;

  Status = SendCommandPacket (mActiveClient, &Command);
  return Status;
}

STATIC RUN_AND_GET_DEBUG_PROTOCOL  mRunAndGetDebugProtocol = {
  RUN_AND_GET_DEBUG_PROTOCOL_REVISION,
  RunAndGetSendDebugMessage
};

EFI_STATUS
InstallRunAndGetDebugProtocol (
  VOID
  )
{
  EFI_STATUS  Status;

  if (mDebugProtocolHandle != NULL) {
    return EFI_ALREADY_STARTED;
  }

  Status = gBS->InstallProtocolInterface (
                  &mDebugProtocolHandle,
                  &gRunAndGetDebugProtocolGuid,
                  EFI_NATIVE_INTERFACE,
                  &mRunAndGetDebugProtocol
                  );
  return Status;
}

VOID
UninstallRunAndGetDebugProtocol (
  VOID
  )
{
  if (mDebugProtocolHandle == NULL) {
    return;
  }

  gBS->UninstallProtocolInterface (
         mDebugProtocolHandle,
         &gRunAndGetDebugProtocolGuid,
         &mRunAndGetDebugProtocol
         );
  mDebugProtocolHandle = NULL;
}

VOID
SetRunAndGetActiveClient (
  IN SOCKET_CLIENT  *Client
  )
{
  mActiveClient = Client;
}

EFI_STATUS
EFIAPI
UefiMain (
  IN EFI_HANDLE        ImageHandle,
  IN EFI_SYSTEM_TABLE  *SystemTable
  )
{
  EFI_STATUS  Status;

  gBS->SetWatchdogTimer (0, 0, 0, NULL);

  Print (L"version: %s\n", RUN_AND_GET_VERSION);

  Status = RunRemoteSession (ImageHandle);
  if (!EFI_ERROR (Status)) {
    return EFI_SUCCESS;
  }

  Print (L"Remote session failed: %r\n", Status);
  Print (L"Falling back to local shell.\n");
  return RunShell (ImageHandle);
}
