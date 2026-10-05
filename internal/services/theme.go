package services

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// ThemeService сообщает фронтенду системную тему и уведомляет её об изменениях.
// Ручной выбор темы (System / Light / Dark) хранит и применяет фронтенд;
// здесь — только источник истины для режима "system".
type ThemeService struct{}

func NewThemeService() *ThemeService {
	return &ThemeService{}
}

// GetSystemTheme возвращает "dark" или "light" по настройке ОС.
func (s *ThemeService) GetSystemTheme() string {
	return currentSystemTheme()
}

// ServiceStartup подписывается на системное событие смены темы и ретранслирует
// его на фронтенд как "theme:changed".
func (s *ThemeService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	app := application.Get()
	app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(_ *application.ApplicationEvent) {
		app.Event.Emit("theme:changed", currentSystemTheme())
	})
	return nil
}

func currentSystemTheme() string {
	if application.Get().Env.IsDarkMode() {
		return "dark"
	}
	return "light"
}
