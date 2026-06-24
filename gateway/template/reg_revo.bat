cd /d %~dp0
if /i "%PROCESSOR_IDENTIFIER:~0,3%"=="X86" (
	echo system is x86
	copy .\*.dll %windir%\system32\
	copy .\*.ocx %windir%\system32\
	regsvr32 /s /c %windir%\system32\RealSvrOcxTcp.ocx
	regsvr32 /s /c %windir%\system32\AxInterop.RealSvrOcxTcpLib.dll
	regsvr32 /s /c %windir%\system32\Interop.RealSvrOcxTcpLib.dll
	) else (
		echo system is x64
		copy .\*.dll %windir%\SysWOW64\
		copy .\*.ocx %windir%\SysWOW64\
		regsvr32 /s /c %windir%\SysWOW64\RealSvrOcxTcp.ocx
		regsvr32 /s /c %windir%\SysWOW64\AxInterop.RealSvrOcxTcpLib.dll
		regsvr32 /s /c %windir%\SysWOW64\Interop.RealSvrOcxTcpLib.dll
	)
