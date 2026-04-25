#include <Uefi.h>

#include <Library/UefiApplicationEntryPoint.h>
#include <Library/UefiBootServicesTableLib.h>
#include <Library/UefiLib.h>

#include "../RunAndGetPkg/RunAndGet.h"

STATIC EFI_GUID  mRunAndGetDebugProtocolGuid = RUN_AND_GET_DEBUG_PROTOCOL_GUID;

EFI_STATUS
EFIAPI
UefiMain (
  IN EFI_HANDLE        ImageHandle,
  IN EFI_SYSTEM_TABLE  *SystemTable
  )
{
  EFI_STATUS                  Status;
  RUN_AND_GET_DEBUG_PROTOCOL  *Debug;

  Status = gBS->LocateProtocol (
                  &mRunAndGetDebugProtocolGuid,
                  NULL,
                  (VOID **)&Debug
                  );
  if (EFI_ERROR (Status)) {
    Print (L"LocateProtocol failed: %r\n", Status);
    return Status;
  }

  Status = Debug->SendDebugMessage (
                    Debug,
                    L"HelloWorld from RunAndGetDebugTestPkg"
                    );
  Print (L"SendDebugMessage returned: %r\n", Status);
  return Status;
}
