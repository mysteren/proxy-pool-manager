package main

import (
	"embed"
	"log"

	"proxy-pool-manager/internal/db"
	"proxy-pool-manager/internal/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Фронтенд собирается в frontend/dist и встраивается в бинарник.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Типизированные события: генератор биндингов даст фронтенду типизированный TS-API.
	application.RegisterEvent[string]("theme:changed")
	application.RegisterEvent[services.FetchResult]("source:fetched")
}

func main() {
	database, err := db.Open("")
	if err != nil {
		log.Fatalf("не удалось открыть БД: %v", err)
	}
	defer database.Close()

	storage := services.NewStorageService(database)
	themeService := services.NewThemeService()
	sourceService := services.NewSourceService(storage)

	app := application.New(application.Options{
		Name:        "Proxy Pool Manager",
		Description: "Сбор, проверка и управление пулом прокси",
		Services: []application.Service{
			application.NewService(themeService),
			application.NewService(sourceService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Proxy Pool Manager",
		Width:            1200,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(18, 18, 20),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
