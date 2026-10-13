package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// The window has one fixed size: the form is laid out for it and scrolls when it grows.
const (
	windowWidth  = 580
	windowHeight = 800
)

func main() {
	finishUpdate()

	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = wails.Run(&options.App{
		Title:         "SVN Builder",
		Width:         windowWidth,
		Height:        windowHeight,
		DisableResize: true,
		// Matches --canvas in the dark theme so the window does not flash white while loading.
		BackgroundColour: &options.RGBA{R: 13, G: 15, B: 20, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
		// One copy per settings folder: a second launch brings the first window forward. The app
		// relies on this when it clears leftover temporary downloads at start.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               app.instanceID(),
			OnSecondInstanceLaunch: app.secondInstance,
		},
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
