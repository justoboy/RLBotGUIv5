@echo off
setlocal

REM ============================================================================
REM RLBotGUIv5 one-click LAUNCH (the v5 counterpart to the v4 fork's runRLBotInsideEnv.bat).
REM
REM The v5 GUI is a Go/Wails/Svelte app, so "running the GUI" means running it in Wails dev mode
REM (live hot-reload of Go + Svelte). This is the dev-mode equivalent of v4's `python run.py`.
REM
REM It assumes setup_gui.bat has already run once (Go + wails3 + pnpm + frontend deps installed).
REM If the toolchain isn't set up yet, run setup_gui.bat first.
REM
REM For a one-off PRODUCTION build instead of dev hot-reload, run:  wails3 build
REM then launch the produced  bin\rlbotgui.exe
REM ============================================================================

REM cgo needs a C compiler. The go-interface flatbuffers package AND bob-diff are cgo-based, and
REM bob-diff's prebuilt static lib (bob-diff\libbob_diff_win.a) is built with mingw-w64 (Rust gnu
REM target), so cgo MUST use mingw gcc, NOT MSVC cl.exe.
REM
REM IMPORTANT: this machine has OS-level env vars CC=MSVC cl.exe and CGO_CFLAGS/CGO_LDFLAGS="-O2 -g".
REM cl.exe rejects gcc-style flags (D8021: invalid numeric argument '/Werror') and the spaced path
REM breaks Go's cgo arg-splitting. So we: (1) put a mingw gcc on PATH, (2) set CC=gcc (no spaces),
REM and (3) clear the inherited CGO_CFLAGS/CGO_LDFLAGS. This is the exact recipe that builds gui.exe.
set "CGO_ENABLED=1"
set "CGO_CFLAGS="
set "CGO_LDFLAGS="
where gcc >nul 2>&1
if not errorlevel 1 (
    echo [cc] using gcc already on PATH
) else (
    if exist "C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin\gcc.exe" (
        set "PATH=C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin;%PATH%"
        echo [cc] added CLion bundled mingw gcc to PATH
    ) else (
        echo WARNING: no gcc found. cgo build will fail. Install mingw-w64 gcc and add it to PATH.
    )
)
set "CC=gcc"

REM The Wails build pipeline (build/Taskfile.yml) runs Unix-style commands: a `sh:` precondition
REM (sh -c "pnpm --version"), `mkdir -p darwin`, and `rm -f *.syso`. Under cmd.exe these Unix
REM binaries are NOT on PATH by default, which is why the build failed with:
REM   task: [windows:common:generate:icons] mkdir -p darwin  ->  "mkdir": executable file not found
REM   "Looks like pnpm isn't installed."
REM Fix: put Git's sh/mkdir/rm and the npm-global pnpm shim on PATH for this session.
if exist "C:\Program Files\Git\bin\sh.exe" (
    set "PATH=C:\Program Files\Git\bin;C:\Program Files\Git\usr\bin;%PATH%"
    echo [sh] added Git bin/usr-bin to PATH (sh/mkdir/rm)
) else (
    echo WARNING: Git sh.exe not found at default path. Wails build steps that need sh/mkdir/rm will fail.
)
if exist "C:\Users\justo\AppData\Local\npm-global\pnpm.cmd" (
    set "PATH=C:\Users\justo\AppData\Local\npm-global;%PATH%"
    echo [pnpm] added npm-global to PATH (pnpm.cmd)
) else (
    echo WARNING: pnpm.cmd not found in npm-global. Install pnpm: npm install -g pnpm
)

REM ============================================================================
REM CORE SERVER (RLBotServer.exe) - the piece the GUI connects to on 127.0.0.1:23234.
REM
REM The v5 GUI is a pure TCP CLIENT. It NEVER starts the core server itself; it only dials
REM 127.0.0.1:23234. In the official setup the Rust launcher (RLBot/launcher) downloads
REM RLBotServer.exe from RLBot/core into %LOCALAPPDATA%\RLBot5\bin and spawns it before the GUI.
REM We don't use that launcher here, so this script replicates that job:
REM   1. Make sure RLBotServer.exe exists in %LOCALAPPDATA%\RLBot5\bin (download if missing).
REM   2. If nothing is already listening on 23234, start it ourselves (and remember we did).
REM   3. When the GUI exits, kill the server ONLY if we were the ones who started it.
REM
REM If you already run the official RLBot launcher (its server is already on 23234), this script
REM detects the open port, does NOT start a second server, and does NOT kill the launcher's server.
REM ============================================================================
set "SRV_DIR=%LOCALAPPDATA%\RLBot5\bin"
set "SRV_EXE=%SRV_DIR%\RLBotServer.exe"
set "SRV_URL=https://github.com/RLBot/core/releases/latest/download/RLBotServer.exe"
set "SRV_STARTED_BY_US=0"

if not exist "%SRV_DIR%" mkdir "%SRV_DIR%"

if not exist "%SRV_EXE%" (
    echo [server] RLBotServer.exe not found. Downloading from RLBot/core latest release...
    curl -L -s -o "%SRV_EXE%" "%SRV_URL%"
    if not exist "%SRV_EXE%" (
        echo ERROR: Could not download RLBotServer.exe from %SRV_URL%
        echo        Download it manually into "%SRV_DIR%" and re-run.
        pause
        exit /b 1
    )
)
echo [ok] core server binary: "%SRV_EXE%"

REM Is something already listening on 23234? (e.g. the official launcher's server)
REM
REM IMPORTANT: do NOT use a single findstr /r with spaces in the pattern. findstr /r splits its
REM search string on spaces into multiple OR-patterns, so ":23234[  ].*LISTENING" was being split
REM into ":23234[" OR "]" OR ".*LISTENING" - and ".*LISTENING" matched EVERY listening socket,
REM making the script think the port was always busy and skip starting our server. Fix: chain two
REM findstr with a literal /c: pattern (no spaces inside the pattern).
netstat -ano | findstr LISTENING | findstr /c:":23234" >nul 2>&1
if not errorlevel 1 (
    echo [server] Port 23234 already in use - assuming a server is already running. Not starting a new one.
) else (
    echo [server] Starting RLBotServer.exe in the background...
    start "" /b "%SRV_EXE%"
    set "SRV_STARTED_BY_US=1"
)

echo.
echo Starting RLBotGUIv5 in dev mode (hot reload)...
echo   - Go backend rebuilds on .go changes
echo   - Svelte frontend hot-reloads on frontend changes
echo Close this window (or Ctrl+C) to stop the GUI.
echo.

cd /d "%~dp0"
call wails3 dev
set "WAILS_RC=%errorlevel%"

REM Tear down the server ONLY if we started it.
if "%SRV_STARTED_BY_US%"=="1" (
    echo [server] Stopping the RLBotServer.exe we started...
    taskkill /im RLBotServer.exe /f >nul 2>&1
)

if not "%WAILS_RC%"=="0" (
    echo.
    echo ERROR: wails3 dev exited with an error. If the toolchain is missing, run setup_gui.bat.
    pause
    exit /b 1
)

endlocal
