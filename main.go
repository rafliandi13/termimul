package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"termimul/internal/services"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Soft cap so the GC returns memory instead of letting the heap drift up.
	debug.SetMemoryLimit(96 << 20)

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dataDir := filepath.Join(configDir, "termimul")
	migrateDataDir(filepath.Join(configDir, "termimus"), dataDir)
	svc, err := services.New(dataDir)
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Termimul",
		Width:     1200,
		Height:    760,
		MinWidth:  760,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 27, B: 27, A: 1},
		// xterm.js uses its DOM renderer here, so skipping the GPU process costs nothing visible.
		Windows: &windows.Options{
			WebviewGpuIsDisabled: true,
			Theme:                windows.Dark,
		},
		Mac: &mac.Options{
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "Termimul",
				Message: "Lightweight open-source SSH client\nMIT License",
			},
		},
		Linux: &linux.Options{
			ProgramName: "termimul", // matches StartupWMClass in the .desktop file
		},
		// Lets files be dragged from the file manager into an SFTP tab.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:  svc.Startup,
		OnShutdown: svc.Shutdown,
		Bind:       svc.Bind(),
	})
	if err != nil {
		log.Fatal(err)
	}
}

// migrateDataDir moves data from the app's former name (Termimus, before
// v0.4.0) so existing hosts, keys and known hosts carry over. It only acts
// when the new folder doesn't exist yet, so it never overwrites anything.
func migrateDataDir(oldDir, newDir string) {
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		return
	}
	if _, err := os.Stat(oldDir); err != nil {
		return
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		log.Printf("could not move %s to %s: %v", oldDir, newDir, err)
	}
}
