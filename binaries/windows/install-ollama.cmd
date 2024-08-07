@echo off
setlocal

REM Define variables
set DOWNLOAD_URL=https://ollama.com/download/OllamaSetup.exe
set INSTALLER_EXE_NAME=OllamaSetup.exe
set APP_NAME=ollama app.exe
set EXE_NAME=ollama.exe
set INSTALLER_PATH=%TEMP%\%INSTALLER_EXE_NAME%
set "APP_INSTALLATION_PATH=%LOCALAPPDATA%\Programs\Ollama\%APP_NAME%"
set "EXE_INSTALLATION_PATH=%LOCALAPPDATA%\Programs\Ollama\%EXE_NAME%"
set CHECK_URL=http://localhost:11434/api/tags

REM Check if http://localhost:11434/api/tags returns a valid response
echo Checking if %CHECK_URL% returns a valid response...
PowerShell -Command "try { $response = Invoke-WebRequest -Uri '%CHECK_URL%' -UseBasicParsing -ErrorAction Stop; if ($response.StatusCode -eq 200) { exit 0 } else { exit 1 } } catch { exit 1 }"

if %ERRORLEVEL% == 1 (
    echo The API check failed, checking if %APP_NAME% is available...
	if exist "%APP_INSTALLATION_PATH%" (
	    echo Ollama installation %APP_INSTALLATION_PATH% found on the machine, let's run it
		start "" "%APP_INSTALLATION_PATH%"
		%EXE_INSTALLATION_PATH% pull tinyllama
	) else (
	    echo Ollama installation not found on the machine, let's install it
	    if exist %INSTALLER_PATH% (
			echo Installer present on the machine.
	    ) else (
			REM Download the installer
			echo Ollama installation %APP_INSTALLATION_PATH% not found!
			echo Downloading %INSTALLER_EXE_NAME% from %DOWNLOAD_URL%
			PowerShell -Command "Invoke-WebRequest -Uri %DOWNLOAD_URL% -OutFile %INSTALLER_PATH%"
	    )
		REM Silent install
		echo Starting silent installation...
		start /wait %INSTALLER_PATH% /silent /norestart
		%EXE_INSTALLATION_PATH% pull tinyllama

		REM Check if installation was successful
		if %ERRORLEVEL% == 0 (
			echo Installation completed successfully.
		) else (
			echo Installation failed with error code %ERRORLEVEL%.
		)
		REM Cleanup
		echo Deleting installer...
		del /F /Q %INSTALLER_PATH%
	)
) else (
    echo Ollama seems to be running correctly.
)

endlocal
echo Done.
