@echo off
REM test_trigger_flow.bat - Helper script to run the ngrok trigger E2E test
REM 
REM Usage: test_trigger_flow.bat [NGROK_URL]
REM Example: test_trigger_flow.bat https://abcd1234.ngrok.io

setlocal enabledelayedexpansion

echo.
echo ═══════════════════════════════════════════════════
echo   ngrok Trigger Flow E2E Test
echo ═══════════════════════════════════════════════════
echo.

REM Get ngrok URL from argument or prompt
if "%~1"=="" (
    echo Enter your ngrok URL (e.g., https://abcd1234.ngrok.io):
    set /p NGROK_URL=
) else (
    set NGROK_URL=%~1
)

REM Remove trailing slash if present
if "%NGROK_URL:~-1%"=="/" set NGROK_URL=%NGROK_URL:~0,-1%

REM Validate URL format
echo %NGROK_URL% | findstr /R "^https\?://" >nul
if %ERRORLEVEL% NEQ 0 (
    echo ❌ Error: Invalid URL format
    echo URL must start with http:// or https://
    pause
    exit /b 1
)

echo ✓ Using ngrok URL: %NGROK_URL%
echo.

REM Check if Django is running
echo Checking if Django is running...
netstat -an | find "8004" | find "LISTENING" >nul
if %ERRORLEVEL% NEQ 0 (
    echo ❌ Django is not running on port 8004
    echo.
    echo Start Django first:
    echo   docker-compose up -d
    echo   OR
    echo   python manage.py runserver 8004
    pause
    exit /b 1
)
echo ✓ Django is running
echo.

REM Check if pytest is installed
where pytest >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ❌ pytest is not installed
    echo.
    echo Install pytest:
    echo   pip install pytest
    pause
    exit /b 1
)

REM Check if psycopg2 is installed
python -c "import psycopg2" >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ⚠ psycopg2 not installed - database assertions will fail
    echo Install it with: pip install psycopg2-binary
    echo.
    set /p CONTINUE="Continue anyway? (y/n): "
    if /i not "!CONTINUE!"=="y" exit /b 1
)

REM Run the test
echo ═══════════════════════════════════════════════════
echo   Running E2E Trigger Flow Test
echo ═══════════════════════════════════════════════════
echo.
echo This test will:
echo   1. Call the external trigger endpoint
echo   2. Trigger sends lead to your ngrok URL
echo   3. Django receives and queues the lead
echo   4. Celery processes the lead asynchronously
echo   5. Test verifies lead in database
echo.
echo This may take 10-15 seconds...
echo.

set NGROK_URL=%NGROK_URL%

REM Run the test with verbose output
pytest leads/tests/test_e2e_trigger_flow.py -v -s

set TEST_EXIT_CODE=%ERRORLEVEL%

echo.
echo ═══════════════════════════════════════════════════

if %TEST_EXIT_CODE% EQU 0 (
    echo ✅ Test Passed!
    echo.
    echo Next steps:
    echo   • Check ngrok dashboard: http://localhost:4040
    echo   • View Django logs: docker-compose logs web
    echo   • View Celery logs: docker-compose logs celery
) else (
    echo ❌ Test Failed
    echo.
    echo Troubleshooting:
    echo   • Verify ngrok is running and URL is correct
    echo   • Check Django logs: docker-compose logs web
    echo   • Verify database is accessible
    echo   • Check ngrok dashboard: http://localhost:4040
)

echo ═══════════════════════════════════════════════════
echo.

pause
exit /b %TEST_EXIT_CODE%
