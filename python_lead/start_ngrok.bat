@echo off
REM start_ngrok.bat - Helper script to start ngrok tunnel for Django webhook
REM 
REM Usage: start_ngrok.bat

echo.
echo 🚀 Starting ngrok tunnel for Lead Gateway...
echo.

REM Resolve local ngrok.exe (preferred)
set "NGROK_EXE=%~dp0ngrok.exe"
set "NGROK_EXE_PARENT=%~dp0..\ngrok.exe"

REM Check if ngrok is available (local file or PATH)
if exist "%NGROK_EXE%" (
    set "NGROK_CMD=%NGROK_EXE%"
) else if exist "%NGROK_EXE_PARENT%" (
    set "NGROK_CMD=%NGROK_EXE_PARENT%"
) else (
    where ngrok >nul 2>nul
    if %ERRORLEVEL% NEQ 0 (
        echo ❌ ngrok is not installed!
        echo.
        echo Place ngrok.exe in this folder or in the repo root, or install ngrok:
        echo   • Windows: choco install ngrok
        echo   • Or download from: https://ngrok.com/download
        echo.
        pause
        exit /b 1
    )
    set "NGROK_CMD=ngrok"
)

REM Check if Django is running on port 8004
netstat -an | find "8004" | find "LISTENING" >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ⚠️  Warning: Django doesn't appear to be running on port 8004
    echo.
    echo Make sure your Django app is running first:
    echo   docker-compose up -d
    echo   OR
    echo   python manage.py runserver 8004
    echo.
    set /p CONTINUE="Continue anyway? (y/n): "
    if /i not "%CONTINUE%"=="y" exit /b 1
)

echo Starting ngrok tunnel...
echo Port: 8004
echo Webhook endpoint: /webhooks/leads/
echo.
echo 📊 Inspect requests at: http://localhost:4041
echo.
echo When ngrok starts, copy the HTTPS URL and use it as your webhook endpoint:
echo   Example: https://abcd1234.ngrok.io/webhooks/leads/
echo.
echo Press Ctrl+C to stop the tunnel
echo ─────────────────────────────────────────────────────
echo.

REM Start ngrok with config file if it exists, otherwise use simple command
if exist "ngrok.yml" (
    "%NGROK_CMD%" start --all --config=ngrok.yml
) else (
    "%NGROK_CMD%" http 8004
)
