param(
  [string]$Target = "127.0.0.1:9162",
  [string]$Community = "public",
  [string]$SysName = "lab-switch",
  [int]$Temp = 26,
  [string]$TrapOID = ".1.3.6.1.6.3.1.1.5.1",
  [string]$ValueOID = ".1.3.6.1.4.1.9.9.13.1.3.1.3.0"
)

# Requires Net-SNMP snmptrap.exe in PATH
$hostPort = $Target.Split(":")
if ($hostPort.Count -ne 2) { throw "Target must be host:port" }

$exe = "snmptrap"
$cmd = @(
  "-v", "2c",
  "-c", $Community,
  "$($hostPort[0]):$($hostPort[1])",
  "",
  $TrapOID,
  ".1.3.6.1.2.1.1.5.0", "s", $SysName,
  $ValueOID, "i", $Temp
)

& $exe @cmd
