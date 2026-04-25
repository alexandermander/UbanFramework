#include <Uefi.h>

#include <Library/BaseMemoryLib.h>
#include <Library/MemoryAllocationLib.h>
#include <Library/PrintLib.h>
#include <Library/UefiApplicationEntryPoint.h>
#include <Library/UefiBootServicesTableLib.h>
#include <Library/UefiLib.h>
#include <Library/UefiRuntimeServicesTableLib.h>

#include "../RunAndGetPkg/RunAndGet.h"

STATIC EFI_GUID  mRunAndGetDebugProtocolGuid = RUN_AND_GET_DEBUG_PROTOCOL_GUID;

#define VARIABLE_DUMP_START_MARKER    L"[VariableDumpToServer] start"
#define VARIABLE_DUMP_SUCCESS_MARKER  L"[VariableDumpToServer] Success"
#define VARIABLE_DUMP_FAILURE_FORMAT  L"[VariableDumpToServer] Failure: %r"

STATIC
EFI_STATUS
SendRemoteMessage (
  IN RUN_AND_GET_DEBUG_PROTOCOL  *Debug,
  IN CONST CHAR16                *Message
  )
{
  EFI_STATUS  Status;

  if ((Debug == NULL) || (Message == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  Status = Debug->SendDebugMessage (Debug, Message);
  if (EFI_ERROR (Status)) {
    Print (L"SendDebugMessage failed: %r\n", Status);
  }

  return Status;
}

STATIC
EFI_STATUS
SendRemoteFailure (
  IN RUN_AND_GET_DEBUG_PROTOCOL  *Debug,
  IN EFI_STATUS                  FailureStatus
  )
{
  CHAR16  Message[96];

  if (Debug == NULL) {
    return EFI_INVALID_PARAMETER;
  }

  UnicodeSPrint (
    Message,
    sizeof (Message),
    VARIABLE_DUMP_FAILURE_FORMAT,
    FailureStatus
    );

  return SendRemoteMessage (Debug, Message);
}

EFI_STATUS
EFIAPI
UefiMain (
  IN EFI_HANDLE        ImageHandle,
  IN EFI_SYSTEM_TABLE  *SystemTable
  )
{
  EFI_STATUS                  Status;
  EFI_STATUS                  SendStatus;
  RUN_AND_GET_DEBUG_PROTOCOL  *Debug;
  EFI_GUID                    VendorGuid;
  CHAR16                      *VariableName;
  CHAR16                      *RemoteMessage;
  UINTN                       VariableNameCapacity;
  UINTN                       VariableNameSize;
  UINTN                       RemoteMessageCapacity;
  UINTN                       RequiredMessageSize;
  UINTN                       VariableCount;
  BOOLEAN                     Started;

  Debug                 = NULL;
  VariableName          = NULL;
  RemoteMessage         = NULL;
  VariableNameCapacity  = sizeof (CHAR16);
  VariableNameSize      = sizeof (CHAR16);
  RemoteMessageCapacity = 0;
  VariableCount         = 0;
  Started               = FALSE;

  Status = gBS->LocateProtocol (
                  &mRunAndGetDebugProtocolGuid,
                  NULL,
                  (VOID **)&Debug
                  );
  if (EFI_ERROR (Status)) {
    Print (L"LocateProtocol failed: %r\n", Status);
    return Status;
  }

  Status = SendRemoteMessage (Debug, VARIABLE_DUMP_START_MARKER);
  if (EFI_ERROR (Status)) {
    return Status;
  }
  Started = TRUE;

  Print (L"VariableDumpToServer: start\n");

  VariableName = AllocateZeroPool (VariableNameCapacity);
  if (VariableName == NULL) {
    Status = EFI_OUT_OF_RESOURCES;
    Print (L"Variable name allocation failed: %r\n", Status);
    goto Exit;
  }

  VariableName[0] = L'\0';
  ZeroMem (&VendorGuid, sizeof (VendorGuid));

  while (TRUE) {
    VariableNameSize = VariableNameCapacity;
    Status           = gRT->GetNextVariableName (
                              &VariableNameSize,
                              VariableName,
                              &VendorGuid
                              );

    if (Status == EFI_BUFFER_TOO_SMALL) {
      CHAR16  *NewVariableName;

      NewVariableName = ReallocatePool (
                          VariableNameCapacity,
                          VariableNameSize,
                          VariableName
                          );
      if (NewVariableName == NULL) {
        Status = EFI_OUT_OF_RESOURCES;
        Print (L"Variable name resize failed: %r\n", Status);
        goto Exit;
      }

      VariableName         = NewVariableName;
      VariableNameCapacity = VariableNameSize;

      VariableNameSize = VariableNameCapacity;
      Status           = gRT->GetNextVariableName (
                                &VariableNameSize,
                                VariableName,
                                &VendorGuid
                                );
    }

    if (Status == EFI_NOT_FOUND) {
      Status = EFI_SUCCESS;
      break;
    }

    if (EFI_ERROR (Status)) {
      Print (L"GetNextVariableName failed: %r\n", Status);
      goto Exit;
    }

    RequiredMessageSize = StrSize (VariableName) + (64 * sizeof (CHAR16));
    if (RequiredMessageSize > RemoteMessageCapacity) {
      CHAR16  *NewRemoteMessage;

      NewRemoteMessage = ReallocatePool (
                           RemoteMessageCapacity,
                           RequiredMessageSize,
                           RemoteMessage
                           );
      if (NewRemoteMessage == NULL) {
        Status = EFI_OUT_OF_RESOURCES;
        Print (L"Remote message resize failed: %r\n", Status);
        goto Exit;
      }

      RemoteMessage         = NewRemoteMessage;
      RemoteMessageCapacity = RequiredMessageSize;
    }

    UnicodeSPrint (
      RemoteMessage,
      RemoteMessageCapacity,
      L"%s | %g",
      VariableName,
      &VendorGuid
      );

    Status = SendRemoteMessage (Debug, RemoteMessage);
    if (EFI_ERROR (Status)) {
      Print (L"Failed sending variable %s: %r\n", VariableName, Status);
      goto Exit;
    }

    VariableCount++;
    if ((VariableCount % 25) == 0) {
      Print (L"Sent %u variables\n", (UINT32)VariableCount);
    }
  }

  Print (L"VariableDumpToServer: sent %u variables\n", (UINT32)VariableCount);
  Status = SendRemoteMessage (Debug, VARIABLE_DUMP_SUCCESS_MARKER);

Exit:
  if (EFI_ERROR (Status) && Started) {
    SendStatus = SendRemoteFailure (Debug, Status);
    if (EFI_ERROR (SendStatus)) {
      Print (L"Failed to send remote failure marker: %r\n", SendStatus);
    }
  }

  if (RemoteMessage != NULL) {
    FreePool (RemoteMessage);
  }

  if (VariableName != NULL) {
    FreePool (VariableName);
  }

  return Status;
}
