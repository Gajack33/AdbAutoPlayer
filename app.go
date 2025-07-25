package main

import (
	"adb-auto-player/internal"
	"adb-auto-player/internal/config"
	"adb-auto-player/internal/ipc"
	"adb-auto-player/internal/updater"
	"adb-auto-player/internal/utils"
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

type App struct {
	ctx                    context.Context
	pythonBinaryPath       *string
	games                  []ipc.GameGUI
	lastOpenGameConfigPath *string
	mainConfigPath         *string
	version                string
	isDev                  bool
	mainConfig             config.MainConfig
	updateManager          *updater.UpdateManager
}

func NewApp(version string, isDev bool, mainConfig config.MainConfig) *App {
	newApp := &App{
		version:          version,
		isDev:            isDev,
		mainConfig:       mainConfig,
		pythonBinaryPath: nil,
		games:            []ipc.GameGUI{},
	}
	return newApp
}

func (a *App) CheckForUpdates() (updater.UpdateInfo, error) {
	a.updateManager = updater.NewUpdateManager(a.ctx, a.version, a.isDev)
	return a.updateManager.CheckForUpdates(a.mainConfig.Update.AutoUpdate, a.mainConfig.Update.EnableAlphaUpdates)
}

func (a *App) GetChangelogs() []updater.Changelog {
	return a.updateManager.GetChangelogs()
}

func (a *App) DownloadUpdate(downloadURL string) error {
	a.updateManager.SetProgressCallback(func(progress float64) {
		runtime.EventsEmit(a.ctx, "download-progress", progress)
	})

	return a.updateManager.DownloadAndApplyUpdate(downloadURL)
}

func (a *App) setGamesFromPython() error {
	if a.pythonBinaryPath == nil {
		return errors.New("missing files: https://AdbAutoPlayer.github.io/AdbAutoPlayer/user-guide/troubleshoot.html#missing-files")
	}

	gamesString, err := internal.GetProcessManager().Exec(*a.pythonBinaryPath, "GUIGamesMenu", "--log-level=DISABLE")
	if err != nil {
		return err
	}
	var gameGUIs []ipc.GameGUI

	err = json.Unmarshal([]byte(gamesString), &gameGUIs)
	if err != nil {
		return err
	}

	a.games = gameGUIs

	return nil
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Shutdown(ctx context.Context) {
	a.ctx = ctx
	internal.GetProcessManager().KillProcess()
}

func (a *App) GetEditableMainConfig() map[string]interface{} {
	mainConfig, err := config.LoadMainConfig(a.getMainConfigPath())
	if err != nil {
		runtime.LogDebugf(a.ctx, "%v", err)
		tmp := config.NewMainConfig()
		mainConfig = &tmp
	}

	response := map[string]interface{}{
		"config":      mainConfig,
		"constraints": ipc.GetMainConfigConstraints(),
	}
	return response
}

func (a *App) SaveMainConfig(mainConfig config.MainConfig) error {
	if err := config.SaveConfig[config.MainConfig](a.getMainConfigPath(), &mainConfig); err != nil {
		return err
	}
	a.mainConfig = mainConfig
	runtime.EventsEmit(a.ctx, "log-clear")
	ipc.GetFrontendLogger().SetLogLevelFromString(mainConfig.Logging.Level)
	runtime.LogSetLogLevel(a.ctx, logger.LogLevel(ipc.GetLogLevelFromString(mainConfig.Logging.Level)))
	runtime.LogInfo(a.ctx, "Saved General Settings")
	return nil
}

func (a *App) GetEditableGameConfig(game ipc.GameGUI) (map[string]interface{}, error) {
	var gameConfig interface{}
	var err error

	workingDir, err := os.Getwd()
	if err != nil {
		runtime.LogErrorf(a.ctx, "Failed to get current working directory: %v", err)
		return nil, err
	}

	paths := []string{
		filepath.Join(workingDir, "games", game.ConfigPath),
		filepath.Join(workingDir, "python/adb_auto_player/games", game.ConfigPath),
	}
	if stdruntime.GOOS != "windows" {
		paths = append(paths, filepath.Join(workingDir, "../../python/adb_auto_player/games", game.ConfigPath))
	}
	configPath := utils.GetFirstPathThatExists(paths)

	if configPath == nil {
		a.lastOpenGameConfigPath = &paths[0]
		response := map[string]interface{}{
			"config":      map[string]interface{}{},
			"constraints": game.Constraints,
		}

		return response, nil
	}

	a.lastOpenGameConfigPath = configPath

	gameConfig, err = config.LoadConfig[map[string]interface{}](*configPath)
	if err != nil {

		return nil, err
	}

	response := map[string]interface{}{
		"config":      gameConfig,
		"constraints": game.Constraints,
	}
	return response, nil
}

func (a *App) GetTheme() string {
	mainConfig := config.NewMainConfig()
	loadedConfig, err := config.LoadMainConfig(a.getMainConfigPath())
	if err != nil {
		println(err.Error())
	} else {
		mainConfig = *loadedConfig
	}
	return mainConfig.UI.Theme
}

func (a *App) SaveGameConfig(gameConfig map[string]interface{}) error {
	if nil == a.lastOpenGameConfigPath {
		return errors.New("cannot save game config: no game config found")
	}

	if err := config.SaveConfig[map[string]interface{}](*a.lastOpenGameConfigPath, &gameConfig); err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, "Saved Game Settings")
	return nil
}

func (a *App) GetRunningSupportedGame(disableLogging bool) (*ipc.GameGUI, error) {
	if a.pythonBinaryPath == nil {
		err := a.setPythonBinaryPath()
		if err != nil {
			runtime.LogErrorf(a.ctx, "%v", err)
			return nil, err
		}
	}
	if len(a.games) == 0 {
		err := a.setGamesFromPython()
		if err != nil {
			runtime.LogErrorf(a.ctx, "%v", err)
			return nil, err
		}
	}

	runningGame := ""
	args := []string{"GetRunningGame"}
	if disableLogging {
		args = append(args, "--log-level=DISABLE")
	}
	output, err := internal.GetProcessManager().Exec(*a.pythonBinaryPath, args...)

	if err != nil {
		runtime.LogErrorf(a.ctx, "%v", err)
		return nil, err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var logMessage ipc.LogMessage
		if err = json.Unmarshal([]byte(line), &logMessage); err != nil {
			runtime.LogErrorf(a.ctx, "Failed to parse JSON log message: %v", err)
			continue
		}

		if strings.HasPrefix(logMessage.Message, "Running game: ") {
			runningGame = strings.TrimSpace(strings.TrimPrefix(logMessage.Message, "Running game: "))
			break
		}
		ipc.GetFrontendLogger().LogMessage(logMessage)
	}

	if runningGame == "" {
		return nil, nil
	}

	for _, game := range a.games {
		if runningGame == game.GameTitle {
			return &game, nil
		}
	}
	if a.pythonBinaryPath == nil {
		runtime.LogDebugf(a.ctx, "Python Binary Path: nil")
	} else {
		runtime.LogDebugf(a.ctx, "Python Binary Path: %s", *a.pythonBinaryPath)
	}
	runtime.LogDebugf(a.ctx, "Package: %s not supported", runningGame)
	return nil, nil
}

func (a *App) setPythonBinaryPath() error {
	workingDir, err := os.Getwd()
	if err != nil {
		runtime.LogErrorf(a.ctx, "%v", err)
		return err
	}

	if runtime.Environment(a.ctx).BuildType == "dev" {
		path := filepath.Join(workingDir, "python")
		a.pythonBinaryPath = &path
		internal.GetProcessManager().IsDev = true
		return nil
	}

	executable := "adb_auto_player.exe"
	if stdruntime.GOOS != "windows" {
		executable = "adb_auto_player_py_app"
	}

	paths := []string{
		filepath.Join(workingDir, "binaries", executable),
	}

	if stdruntime.GOOS != "windows" {
		paths = append(paths, filepath.Join(workingDir, "../../../python/main.dist/", executable))
		paths = append(paths, filepath.Join(workingDir, "../../python/main.dist/", executable))

	} else {
		paths = append(paths, filepath.Join(workingDir, "python/main.dist/", executable))
	}

	runtime.LogDebugf(a.ctx, "Paths: %s", strings.Join(paths, ", "))
	a.pythonBinaryPath = utils.GetFirstPathThatExists(paths)
	return nil
}

func (a *App) Debug() error {
	if a.pythonBinaryPath == nil {
		err := a.setPythonBinaryPath()
		if err != nil {
			runtime.LogErrorf(a.ctx, "%v", err)
			return err
		}
	}

	args := []string{"Debug"}

	if err := internal.GetProcessManager().StartProcess(a.pythonBinaryPath, args, 2); err != nil {
		runtime.LogErrorf(a.ctx, "Starting process: %v", err)

		return err
	}
	return nil
}

func (a *App) SaveDebugZip() {
	const debugDir = "debug"
	const zipName = "debug.zip"

	if _, err := os.Stat(debugDir); os.IsNotExist(err) {
		runtime.LogErrorf(a.ctx, "debug directory does not exist")
		return
	}

	zipFile, err := os.Create(zipName)
	if err != nil {
		runtime.LogErrorf(
			a.ctx,
			"%s",
			fmt.Errorf("failed to create zip file: %w", err),
		)
		return
	}
	defer func(zipFile *os.File) {
		err = zipFile.Close()
		if err != nil {
			runtime.LogErrorf(
				a.ctx,
				"%s",
				fmt.Errorf("%w", err),
			)
		}
	}(zipFile)

	zipWriter := zip.NewWriter(zipFile)
	defer func(zipWriter *zip.Writer) {
		err = zipWriter.Close()
		if err != nil {
			runtime.LogErrorf(
				a.ctx,
				"%s",
				fmt.Errorf("%w", err),
			)
		}
	}(zipWriter)

	err = filepath.Walk(debugDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(debugDir, filePath)
		if err != nil {
			return err
		}

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer func(file *os.File) {
			err = file.Close()
			if err != nil {
				runtime.LogErrorf(
					a.ctx,
					"%s",
					fmt.Errorf("%w", err),
				)
			}
		}(file)

		_, err = io.Copy(zipEntry, file)
		return err
	})

	if err != nil {
		runtime.LogErrorf(
			a.ctx,
			"%s",
			fmt.Errorf("failed to create zip archive: %w", err),
		)
		return
	}

	runtime.LogInfof(a.ctx, "debug.zip saved")
}

func (a *App) StartGameProcess(args []string) error {
	if err := internal.GetProcessManager().StartProcess(a.pythonBinaryPath, args); err != nil {
		runtime.LogErrorf(a.ctx, "Starting process: %v", err)
		return err
	}
	return nil
}

func (a *App) TerminateGameProcess() {
	internal.GetProcessManager().KillProcess()
}

func (a *App) IsGameProcessRunning() bool {
	return internal.GetProcessManager().IsProcessRunning()
}

func (a *App) getMainConfigPath() string {
	if a.mainConfigPath != nil {
		return *a.mainConfigPath
	}

	paths := []string{
		"config.toml",              // distributed
		"config/config.toml",       // dev
		"../../config/config.toml", // macOS dev no not a joke
	}

	configPath := utils.GetFirstPathThatExists(paths)

	a.mainConfigPath = configPath

	if a.mainConfigPath == nil {
		return paths[0]
	}
	return *configPath
}

func (a *App) RegisterGlobalHotkeys() {
	registerGlobalHotkeys(a)
}
