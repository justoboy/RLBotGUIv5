@echo off
setlocal

REM ============================================================================
REM RLBotGUIv5 one-click SETUP (the v5 counterpart to the v4 fork's setup_venv.bat).
REM
REM The v5 GUI is NOT a Python app - it is Go + Wails v3 + Svelte. So instead of a Python venv, this
REM script installs the BUILD TOOLCHAIN the GUI needs and the frontend dependencies:
REM   - Go          (compiler; must be preinstalled - see https://go.dev/dl)
REM   - wails3 CLI  (installed here via `go install` if missing)
REM   - pnpm        (frontend package manager; installed here via npm if missing)
REM   - mingw gcc   (cgo compiler for the go-interface flatbuffers cgo bits)
REM   - frontend/node_modules (pnpm install)
REM   - generated Wails bindings (wails3 generate bindings)
REM
REM After this runs once, use run_gui.bat to launch the GUI.
REM ============================================================================

echo ========================================
echo RLBotGUIv5 Toolchain Setup
echo ========================================
echo.

REM --- Go (required, cannot auto-install here) ---
where go >nul 2>&1
if errorlevel 1 (
    echo ERROR: Go is not installed or not on PATH.
    echo Install it from https://go.dev/dl then re-run this script.
    pause
    exit /b 1
)
echo [ok] go: 
go version

REM --- git (needed for go install / go mod) ---
where git >nul 2>&1
if errorlevel 1 (
    echo ERROR: git is not installed or not on PATH. Install Git for Windows and re-run.
    pause
    exit /b 1
)
echo [ok] git found.

REM --- mingw gcc (cgo compiler) + cgo env setup ---
REM The go-interface flatbuffers package AND bob-diff are cgo-based, and bob-diff's prebuilt static
REM lib (bob-diff\libbob_diff_win.a) is built with mingw-w64 (Rust gnu target), so cgo MUST use mingw
REM gcc, NOT MSVC cl.exe. This machine has OS-level CC=MSVC cl.exe and CGO_CFLAGS/CGO_LDFLAGS="-O2 -g"
REM which break cgo (spaced path + cl.exe rejects gcc flags). We override them for this session:
REM prepend a mingw gcc to PATH, set CC=gcc (no spaces), and clear the inherited CGO flags.
set "CGO_ENABLED=1"
set "CGO_CFLAGS="
set "CGO_LDFLAGS="
where gcc >nul 2>&1
if not errorlevel 1 (
    echo [ok] gcc already on PATH.
) else (
    if exist "C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin\gcc.exe" (
        set "PATH=C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin;%PATH%"
        echo [ok] added CLion bundled mingw gcc to PATH.
    ) else (
        echo ERROR: gcc not found on PATH. The go-interface package uses cgo and needs a C compiler.
        echo Install mingw-w64 gcc and add it to PATH, then re-run.
        pause
        exit /b 1
    )
)
set "CC=gcc"
echo [ok] cgo CC=gcc CGO_ENABLED=1

REM --- Unix-ish tools + npm-global on PATH (for the Wails build pipeline) ---
REM The Wails build pipeline (build/Taskfile.yml) runs Unix-style commands: a `sh:` precondition
REM (sh -c "pnpm --version"), `mkdir -p darwin`, and `rm -f *.syso`. Under cmd.exe these are not on
REM PATH by default. Add Git's sh/mkdir/rm and the npm-global pnpm shim to PATH for this session.
if exist "C:\Program Files\Git\bin\sh.exe" (
    set "PATH=C:\Program Files\Git\bin;C:\Program Files\Git\usr\bin;%PATH%"
    echo [ok] added Git bin/usr-bin to PATH (sh/mkdir/rm).
) else (
    echo WARNING: Git sh.exe not found at default path. Wails build steps needing sh/mkdir/rm may fail.
)
if exist "C:\Users\justo\AppData\Local\npm-global\pnpm.cmd" (
    set "PATH=C:\Users\justo\AppData\Local\npm-global;%PATH%"
    echo [ok] added npm-global to PATH (pnpm.cmd).
)

REM --- pnpm (frontend package manager) ---
where pnpm >nul 2>&1
if errorlevel 1 (
    echo.
    echo pnpm not found. Installing via npm...
    where npm >nul 2>&1
    if errorlevel 1 (
        echo ERROR: npm not found. Install Node.js from https://nodejs.org then re-run.
        pause
        exit /b 1
    )
    call npm install -g pnpm
    if errorlevel 1 (
        echo ERROR: Failed to install pnpm.
        pause
        exit /b 1
    )
)
echo [ok] pnpm: 
call pnpm --version

REM --- wails3 CLI ---
where wails3 >nul 2>&1
if errorlevel 1 (
    echo.
    echo wails3 CLI not found. Installing (this compiles the CLI, may take a minute)...
    go install github.com/wailsapp/wails/v3/cmd/wails3@latest
    if errorlevel 1 (
        echo ERROR: Failed to install wails3 CLI.
        pause
        exit /b 1
    )
    echo NOTE: wails3 was installed to your Go bin. If 'wails3' is still not found, ensure
    echo       your Go bin directory is on PATH, then open a NEW terminal and re-run.
)
echo [ok] wails3: 
call wails3 version 2>nul

REM --- frontend deps ---
echo.
echo Installing frontend dependencies (pnpm install)...
pushd "%~dp0frontend"
call pnpm install
if errorlevel 1 (popd & echo ERROR: pnpm install failed. & pause & exit /b 1)
popd

REM --- generate bindings ---
echo.
echo Generating Wails bindings...
call wails3 generate bindings -clean=true
if errorlevel 1 (
    echo WARNING: wails3 generate bindings failed. You can retry after fixing the above.
)

REM --- core server binary (RLBotServer.exe) ---
REM The GUI is a pure TCP client that dials 127.0.0.1:23234; it never starts the server itself.
REM The official Rust launcher normally downloads RLBotServer.exe and spawns it. We don't use that
REM launcher, so we pre-download the binary here into the same path the launcher uses. run_gui.bat
REM will then start it (only if nothing is already on 23234) before launching the GUI.
set "SRV_DIR=%LOCALAPPDATA%\RLBot5\bin"
set "SRV_EXE=%SRV_DIR%\RLBotServer.exe"
set "SRV_URL=https://github.com/RLBot/core/releases/latest/download/RLBotServer.exe"
if not exist "%SRV_DIR%" mkdir "%SRV_DIR%"
if exist "%SRV_EXE%" (
    echo [ok] core server already present: "%SRV_EXE%"
) else (
    echo.
    echo Downloading core server RLBotServer.exe from RLBot/core latest release...
    curl -L -s -o "%SRV_EXE%" "%SRV_URL%"
    if exist "%SRV_EXE%" (echo [ok] core server downloaded.) else (echo WARNING: could not download RLBotServer.exe. run_gui.bat will retry on launch.)
)

echo.
echo ========================================
echo Setup complete!
echo Next: double-click run_gui.bat to launch the GUI.
echo ========================================
echo.
pause
endlocal
