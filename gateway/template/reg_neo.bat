@echo off
"%ProgramFiles%"\"Microsoft SDKs"\Windows\v7.0A\bin\gacutil /i Riss.Devices.dll
%SystemRoot%\Microsoft.NET\Framework\v2.0.50727\regasm Riss.Devices.dll /tlb:Riss.Devices.tlb
