package main

import (
	"embed"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

//go:embed static/*.html
var content embed.FS

// Открытие браузера по умолчанию
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Не удалось открыть браузер: %v\n", err)
	}
}

func serveHTML(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := content.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}
}

func main() {
	// Роуты
	http.HandleFunc("/", serveHTML("static/index.html"))
	http.HandleFunc("/minesweeper", serveHTML("static/minesweeper.html"))
	http.HandleFunc("/snake", serveHTML("static/snake.html"))
  http.HandleFunc("/tetris", serveHTML("static/tetris.html"))
  http.HandleFunc("/arkanoid", serveHTML("static/arkanoid.html"))
  http.HandleFunc("/spaceinvaders", serveHTML("static/spiceinvaders.html"))
  http.HandleFunc("/pacman", serveHTML("static/pacman.html"))
  http.HandleFunc("/bomberman", serveHTML("static/bomberman.html"))
  http.HandleFunc("/elasto", serveHTML("static/elastomania.html"))
  


	// Запускаем сервер в фоне
	go func() {
		fmt.Println("Развёрнут локальный сервер на порту 228")
		fmt.Println("Сервер: http://localhost:228")
		fmt.Println("Для выхода нажмите Ctrl+C")
		if err := http.ListenAndServe(":228", nil); err != nil {
			fmt.Printf("Ошибка сервера: %v\n", err)
		}
	}()

	// Даём серверу стартануть и открываем браузер
	time.Sleep(1 * time.Second)
	openBrowser("http://localhost:228")

	// Держим процесс
	select {}
}