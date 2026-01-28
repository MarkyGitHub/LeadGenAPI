@echo off
REM test_webhook_ngrok.bat - Test the webhook through ngrok
REM 
REM Usage: test_webhook_ngrok.bat [NGROK_URL]
REM Example: test_webhook_ngrok.bat https://abcd1234.ngrok.io

setlocal enabledelayedexpansion

REM Get ngrok URL from argument or prompt
if "%~1"=="" (
    echo Enter your ngrok URL (e.g., https://abcd1234.ngrok.io):
    set /p NGROK_URL=
) else (
    set NGROK_URL=%~1
)

REM Remove trailing slash if present
if "%NGROK_URL:~-1%"=="/" set NGROK_URL=%NGROK_URL:~0,-1%

echo.
echo 🚀 Testing Lead Gateway via ngrok
echo ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo Target: %NGROK_URL%/webhooks/leads/
echo.

REM Check if curl is available
where curl >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ❌ curl is not installed!
    echo Please install curl or use Git Bash to run test_webhook_ngrok.sh
    pause
    exit /b 1
)

REM Test 1: Valid lead
echo Test 1: Valid Lead (should succeed)
curl -s -X POST "%NGROK_URL%/webhooks/leads/" ^
  -H "Content-Type: application/json" ^
  -d "{\"email\": \"john.doe@example.com\", \"address\": {\"zip\": \"66123\", \"street\": \"Main Street 123\"}, \"house\": {\"is_owner\": true}}"
echo.
echo.
timeout /t 2 /nobreak >nul

REM Test 2: Invalid ZIP code
echo Test 2: Invalid ZIP (should be rejected)
curl -s -X POST "%NGROK_URL%/webhooks/leads/" ^
  -H "Content-Type: application/json" ^
  -d "{\"email\": \"invalid@example.com\", \"address\": {\"zip\": \"12345\", \"street\": \"Somewhere\"}, \"house\": {\"is_owner\": true}}"
echo.
echo.
timeout /t 2 /nobreak >nul

REM Test 3: Not a homeowner
echo Test 3: Not a Homeowner (should be rejected)
curl -s -X POST "%NGROK_URL%/webhooks/leads/" ^
  -H "Content-Type: application/json" ^
  -d "{\"email\": \"renter@example.com\", \"address\": {\"zip\": \"66123\", \"street\": \"Rental Street 1\"}, \"house\": {\"is_owner\": false}}"
echo.
echo.

echo ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo Testing Complete!
echo.
echo Next steps:
echo 1. Check ngrok dashboard: http://localhost:4040
echo 2. View Django logs: docker-compose logs web
echo 3. View Celery logs: docker-compose logs celery
echo.
pause
