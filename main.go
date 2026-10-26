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
	app, err := NewApp(filepath.Join(configDir, "termimus"))
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Termimus",
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
				Title:   "Termimus",
				Message: "Lightweight open-source SSH client\nMIT License",
			},
		},
		Linux: &linux.Options{
			ProgramName: "termimus", // matches StartupWMClass in the .desktop file
		},
		// Lets files be dragged from Explorer into an SFTP tab.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
