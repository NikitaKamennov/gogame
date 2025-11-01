package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

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

// Весь интерфейс и логика — в одной HTML-странице.
// В localStorage храним В ОДНОЙ переменной ttt_state (JSON).
const htmlPage = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <title>Змейка — 600×600, 10 туров</title>
  <style>
    body { font-family: Arial, sans-serif; max-width: 1200px; margin: 20px auto; padding: 0 16px; color: #222; }
    h1 { margin: 0 0 12px; }
    .row { display: flex; gap: 24px; flex-wrap: wrap; }
    .game-panel { flex: 1 1 620px; border: 1px solid #ddd; border-radius: 8px; padding: 16px; background: #fff; }
    .game-panel h2 { margin: 0 0 10px; font-size: 18px; }
    canvas { border: 1px solid #ccc; background: #111; display: block; }
    .legend { margin-top: 8px; font-size: 14px; color: #555; }

    .panel { flex: 0 0 320px; border: 1px solid #ddd; border-radius: 8px; padding: 16px; background: #fafafa; }
    .panel h2 { margin: 0 0 10px; font-size: 18px; }
    .players { display: flex; flex-wrap: wrap; gap: 8px; }
    .player-btn { padding: 6px 10px; border: 1px solid #999; border-radius: 6px; background: #fff; cursor: pointer; }
    .player-btn:hover { background: #f0f0f0; }
    .player-btn.active { background: #007cba; color: #fff; border-color: #007cba; }
    .add-form { display: flex; gap: 8px; margin-top: 8px; }
    .add-form input { flex: 1; padding: 8px; border: 1px solid #ccc; border-radius: 6px; }
    .add-form button { padding: 8px 12px; background: #007cba; color: #fff; border: none; border-radius: 6px; cursor: pointer; }
    .add-form button:hover { background: #005a87; }
    .info { margin-top: 10px; font-size: 14px; color: #444; }
    .actions { margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap; }
    .btn { padding: 8px 12px; border-radius: 6px; cursor: pointer; border: 1px solid #ccc; background: #eee; }
    .btn:hover { background: #e0e0e0; }
    .btn.primary { background: #28a745; color: #fff; border: none; }
    .btn.primary:hover { background: #1e7e34; }
    .btn[disabled] { opacity: 0.6; cursor: not-allowed; }
    .status { margin-top: 8px; padding: 8px; background: #eef6ff; border: 1px solid #cfe2ff; border-radius: 6px; min-height: 20px; }
    .muted { color: #777; }

    /* Модалка */
    .modal { position: fixed; inset: 0; background: rgba(0,0,0,0.45); display: none; align-items: center; justify-content: center; z-index: 1000; }
    .modal.open { display: flex; }
    .modal-content { width: 90%; max-width: 420px; background: #fff; border-radius: 10px; padding: 20px; box-shadow: 0 10px 30px rgba(0,0,0,0.25); }
    .modal-content h3 { margin: 0 0 10px; }
    .modal-content p { margin: 0 0 16px; color: #444; }
    .modal-actions { display: flex; gap: 10px; justify-content: flex-end; }
    .modal-actions .btn { min-width: 100px; }
    .btn.secondary { background: #eee; border: 1px solid #ccc; }
  </style>
</head>
<body>
  <h1>Змейка — 600×600, 10 туров</h1>

  <div class="row">
    <!-- Поле слева -->
    <div class="game-panel">
      <h2>Поле 600×600</h2>
      <canvas id="game" width="600" height="600"></canvas>
      <div id="hud" class="legend"></div>
    </div>

    <!-- Настройки справа -->
    <div class="panel">
      <h2>Игроки</h2>
      <div id="players" class="players"></div>
      <div class="add-form">
        <input id="newPlayerName" placeholder="Имя нового игрока" />
        <button id="addPlayerBtn">Добавить</button>
      </div>

      <div id="playerInfo" class="info muted">Игрок не выбран</div>

      <div class="actions">
        <button id="startPauseBtn" class="btn primary">Старт</button>
        <button id="resetBtn" class="btn">Сбросить поле</button>
      </div>

      <div class="legend">
        Управление: стрелки или WASD.  
        Цель тура увеличивается: тур 1 — 15, тур 2 — 20, тур 3 — 25 и т.д.
      </div>

      <div id="status" class="status">Выберите игрока (клик по имени) или создайте нового, затем нажмите «Старт».</div>
    </div>
  </div>

  <!-- Модалка -->
  <div id="modal" class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
    <div class="modal-content">
      <h3 id="modalTitle">Тур пройден</h3>
      <p id="modalText">Описание</p>
      <div class="modal-actions">
        <button id="modalOk" class="btn primary">ОК</button>
      </div>
    </div>
  </div>

  <script>
    // --- Константы игрового поля ---
    const WIDTH = 600, HEIGHT = 600, CELL = 20;
    const COLS = WIDTH / CELL, ROWS = HEIGHT / CELL;

    // --- Хранилище (одна переменная) ---
    const STORAGE_KEY = 'snake_state';

    function loadState() {
      try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return { players: [], currentPlayerName: '' };
        const s = JSON.parse(raw);
        if (!Array.isArray(s.players)) s.players = [];
        if (typeof s.currentPlayerName !== 'string') s.currentPlayerName = '';
        return s;
      } catch (_) {
        return { players: [], currentPlayerName: '' };
      }
    }
    function saveState() {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    }

    // --- Глобальное состояние ---
    let state = loadState();

    // Игровые переменные
    let ctx, canvas;
    let snake = []; // массив сегментов: {x,y}
    let dir = { x: 1, y: 0 };       // текущее направление
    let nextDir = null;             // запрошенное направление
    let food = { x: 10, y: 10 };
    let running = false;
    let timerId = null;
    let level = 1;
    let apples = 0; // прогресс в текущем туре
    let tourStartAt = 0; // timestamp начала текущего тура
    const MAX_LEVEL = 10;

    // DOM
    const playersWrap = document.getElementById('players');
    const newPlayerName = document.getElementById('newPlayerName');
    const addPlayerBtn = document.getElementById('addPlayerBtn');
    const playerInfo = document.getElementById('playerInfo');
    const startPauseBtn = document.getElementById('startPauseBtn');
    const resetBtn = document.getElementById('resetBtn');
    const statusEl = document.getElementById('status');
    const hudEl = document.getElementById('hud');

    // Модалка DOM
    const modal = document.getElementById('modal');
    const modalTitle = document.getElementById('modalTitle');
    const modalText = document.getElementById('modalText');
    const modalOk = document.getElementById('modalOk');
    modalOk.onclick = function() { closeModal(); };

    // --- Прогресс по туру: 1 -> 15, 2 -> 20, 3 -> 25, ... ---
    function applesTargetForLevel(lvl) {
      // Формула: 10 + 5 * lvl
      return 10 + 5 * Math.max(1, Math.min(lvl, MAX_LEVEL));
    }

    function currentPlayer() {
      if (!state.currentPlayerName) return null;
      return state.players.find(p => p.name === state.currentPlayerName) || null;
    }

    function ensurePlayerDefaults(p) {
      if (typeof p.level !== 'number' || p.level < 1) p.level = 1;
      if (typeof p.apples !== 'number' || p.apples < 0) p.apples = 0;
      if (typeof p.completed !== 'boolean') p.completed = false;
      if (p.level > MAX_LEVEL) p.level = MAX_LEVEL;
      const target = applesTargetForLevel(p.level);
      if (p.apples > target) p.apples = target;
    }

    function renderPlayers() {
      playersWrap.innerHTML = '';
      state.players.forEach(p => {
        ensurePlayerDefaults(p);
        const btn = document.createElement('button');
        btn.className = 'player-btn' + (p.name === state.currentPlayerName ? ' active' : '');
        btn.textContent = p.name;
        const target = applesTargetForLevel(p.level);
        btn.title = 'Тур: ' + p.level + (p.completed ? ' (пройдено)' : '') + ', Прогресс: ' + p.apples + '/' + target;
        btn.onclick = function() {
          selectPlayer(p.name);
          updateInfo();
        };
        playersWrap.appendChild(btn);
      });
      if (!state.players.length) {
        const div = document.createElement('div');
        div.className = 'muted';
        div.textContent = 'Пока нет игроков. Создайте нового.';
        playersWrap.appendChild(div);
      }
      updateInfo();
    }

    function selectPlayer(name) {
      state.currentPlayerName = name;
      let p = state.players.find(x => x.name === name);
      if (!p) {
        p = { name: name, level: 1, apples: 0, completed: false };
        state.players.push(p);
      }
      ensurePlayerDefaults(p);
      saveState();
      renderPlayers();
    }

    function addPlayer() {
      const name = (newPlayerName.value || '').trim();
      if (!name) { setStatus('Введите имя игрока', true); return; }
      if (state.players.some(p => p.name.toLowerCase() === name.toLowerCase())) {
        setStatus('Такой игрок уже существует', true);
        return;
      }
      state.players.push({ name: name, level: 1, apples: 0, completed: false });
      state.currentPlayerName = name;
      saveState();
      newPlayerName.value = '';
      renderPlayers();
      setStatus('Игрок ' + name + ' создан. Нажмите «Старт» для начала игры.');
    }

    function updateInfo() {
      const p = currentPlayer();
      if (!p) {
        playerInfo.textContent = 'Игрок не выбран';
        hudEl.textContent = '';
        updateStartButton();
        return;
      }
      ensurePlayerDefaults(p);
      const speedMs = speedForLevel(p.level);
      const target = applesTargetForLevel(p.level);
      playerInfo.textContent = 'Игрок: ' + p.name + (p.completed ? ' (Игра пройдена)' : '') + ' — Тур: ' + p.level + '/' + MAX_LEVEL + ', Прогресс: ' + p.apples + '/' + target + ', Скорость: ' + speedMs + ' мс/шаг';
      hudEl.textContent = 'Управление: стрелки / WASD. Соберите ' + target + ' кубиков для перехода на следующий тур.';
      updateStartButton();
    }

    function updateStartButton() {
      const p = currentPlayer();
      if (running) {
        startPauseBtn.textContent = 'Пауза';
        startPauseBtn.disabled = false;
        return;
      }
      if (!p) {
        startPauseBtn.textContent = 'Старт';
        startPauseBtn.disabled = false;
        return;
      }
      if (p.completed) {
        startPauseBtn.textContent = 'Игра пройдена';
        startPauseBtn.disabled = true;
        return;
      }
      startPauseBtn.textContent = 'Старт: Тур ' + p.level;
      startPauseBtn.disabled = false;
    }

    function setStatus(msg, isError) {
      statusEl.textContent = msg;
      statusEl.style.color = isError ? '#b30000' : '#222';
    }

    // --- Скорость по туру (мс на шаг) ---
    function speedForLevel(lvl) {
      // Линейная шкала: 140мс (тур 1) -> 50мс (тур 10)
      const max = 140, min = 50;
      const t = (lvl - 1) / (MAX_LEVEL - 1); // 0..1
      return Math.round(max - t * (max - min));
    }

    // --- Игра ---
    function initCanvas() {
      canvas = document.getElementById('game');
      ctx = canvas.getContext('2d');
      draw(); // чистый кадр
    }

    function resetField() {
      // Змейка длиной 5 в центре направо
      const startX = Math.floor(COLS / 2), startY = Math.floor(ROWS / 2);
      snake = [];
      for (let i = 0; i < 5; i++) {
        snake.push({ x: startX - i, y: startY });
      }
      dir = { x: 1, y: 0 };
      nextDir = null;
      placeFood();
      draw();
    }

    function startGameOrPause() {
      const p = currentPlayer();
      if (!p) { setStatus('Сначала выберите игрока или создайте нового', true); return; }
      if (p.completed) { setStatus('Игра уже пройдена. Выберите другого игрока или сбросьте прогресс.', true); return; }

      if (!running) {
        // Запуск/продолжение тура
        ensurePlayerDefaults(p);
        level = p.level;
        apples = p.apples;
        resetField();
        startLoop();
        setStatus('Тур ' + level + ' начался! Прогресс ' + apples + '/' + applesTargetForLevel(level) + '.');
      } else {
        // Пауза
        stopLoop();
        setStatus('Пауза.');
      }
      updateStartButton();
    }

    function startLoop() {
      if (timerId) clearInterval(timerId);
      running = true;
      tourStartAt = Date.now();
      timerId = setInterval(tick, speedForLevel(level));
      updateStartButton();
    }

    function stopLoop() {
      running = false;
      if (timerId) {
        clearInterval(timerId);
        timerId = null;
      }
      updateStartButton();
    }

    function restartWithLevelSpeed() {
      if (!running) return;
      stopLoop();
      startLoop();
    }

    function placeFood() {
      // Случайная свободная клетка (не в теле змейки)
      do {
        food.x = Math.floor(Math.random() * COLS);
        food.y = Math.floor(Math.random() * ROWS);
      } while (snake.some(s => s.x === food.x && s.y === food.y));
    }

    function isOpposite(a, b) {
      return a && b && (a.x + b.x === 0) && (a.y + b.y === 0);
    }

    function onKey(e) {
      const code = e.code || e.key;
      let nd = null;
      if (code === 'ArrowUp' || code === 'KeyW') nd = { x: 0, y: -1 };
      else if (code === 'ArrowDown' || code === 'KeyS') nd = { x: 0, y: 1 };
      else if (code === 'ArrowLeft' || code === 'KeyA') nd = { x: -1, y: 0 };
      else if (code === 'ArrowRight' || code === 'KeyD') nd = { x: 1, y: 0 };
      if (nd) {
        if (!isOpposite(dir, nd)) {
          nextDir = nd;
        }
        e.preventDefault();
      }
      if (code === 'Space') {
        startGameOrPause();
        e.preventDefault();
      }
    }

    function tick() {
      // применяем отложенное направление
      if (nextDir && !isOpposite(dir, nextDir)) {
        dir = nextDir;
        nextDir = null;
      }

      const head = snake[0];
      const newHead = { x: head.x + dir.x, y: head.y + dir.y };

      // Столкновения со стенами
      if (newHead.x < 0 || newHead.x >= COLS || newHead.y < 0 || newHead.y >= ROWS) {
        return gameOver('Столкновение со стеной. Игра окончена.');
      }
      // Столкновение с собой
      if (snake.some(s => s.x === newHead.x && s.y === newHead.y)) {
        return gameOver('Укус за хвост. Игра окончена.');
      }

      // Двигаем змейку
      snake.unshift(newHead);
      if (newHead.x === food.x && newHead.y === food.y) {
        apples += 1;
        const p = currentPlayer();
        if (p) { p.apples = apples; p.level = level; saveState(); updateInfo(); }
        placeFood();

        // Переход тура по цели
        const target = applesTargetForLevel(level);
        if (apples >= target) {
          // Тур завершён
          stopLoop();
          const elapsed = Date.now() - tourStartAt;
          const secondsText = formatDuration(elapsed);

          if (level < MAX_LEVEL) {
            const completedLevel = level;
            level += 1;
            apples = 0;
            if (p) { p.level = level; p.apples = apples; saveState(); }
            showModal('Тур ' + completedLevel + ' пройден!', 'Вы прошли тур ' + completedLevel + ' за ' + secondsText + '. Нажмите «ОК», затем «Старт: Тур ' + level + '».');
            setStatus('Готовы к туру ' + level + '. Скорость увеличена.');
            updateInfo();
          } else {
            // Пройден 10 тур — финал
            if (p) { p.completed = true; p.apples = 0; saveState(); }
            showModal('Игра пройдена!', 'Поздравляем! Вы мастер змейки. Вы прошли игру за ' + secondsText + '.');
            setStatus('Игра пройдена. Вы мастер змейки!');
            updateInfo();
          }
          return;
        }
      } else {
        snake.pop();
      }

      draw();
    }

    function formatDuration(ms) {
      const totalSec = Math.round(ms / 1000);
      const m = Math.floor(totalSec / 60);
      const s = totalSec % 60;
      if (m > 0) return m + ' мин ' + s + ' сек';
      return s + ' сек';
    }

    function gameOver(message) {
      stopLoop();
      setStatus(message, true);
      // Прогресс не теряем — игрок может доиграть тур позже
    }

    function clearCanvas() {
      ctx.fillStyle = '#111';
      ctx.fillRect(0, 0, WIDTH, HEIGHT);
    }

    function drawGrid() {
      ctx.strokeStyle = '#1d1d1d';
      ctx.lineWidth = 1;
      for (let x = 0; x <= WIDTH; x += CELL) {
        ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, HEIGHT); ctx.stroke();
      }
      for (let y = 0; y <= HEIGHT; y += CELL) {
        ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(WIDTH, y); ctx.stroke();
      }
    }

    function drawSnake() {
      // Голова
      if (snake.length) {
        const h = snake[0];
        ctx.fillStyle = '#4caf50';
        ctx.fillRect(h.x * CELL, h.y * CELL, CELL, CELL);
      }
      // Тело
      ctx.fillStyle = '#2e7d32';
      for (let i = 1; i < snake.length; i++) {
        const s = snake[i];
        ctx.fillRect(s.x * CELL, s.y * CELL, CELL, CELL);
      }
    }

    function drawFood() {
      ctx.fillStyle = '#e53935';
      ctx.fillRect(food.x * CELL, food.y * CELL, CELL, CELL);
    }

    function drawHUD() {
      const p = currentPlayer();
      if (!p) return;
      ctx.fillStyle = '#fff';
      ctx.font = '14px Arial';
      const text = 'Игрок: ' + p.name + ' | Тур: ' + level + '/' + MAX_LEVEL + ' | Прогресс: ' + apples + '/' + applesTargetForLevel(level) + ' | Скорость: ' + speedForLevel(level) + 'мс';
      ctx.fillText(text, 10, HEIGHT - 10);
    }

    function draw() {
      clearCanvas();
      drawGrid();
      drawFood();
      drawSnake();
      drawHUD();
    }

    // --- Модалка ---
    function showModal(title, text) {
      modalTitle.textContent = title;
      modalText.textContent = text;
      modal.classList.add('open');
    }
    function closeModal() {
      modal.classList.remove('open');
    }

    // --- Сброс поля ---
    function resetOnlyField() {
      stopLoop();
      resetField();
      setStatus('Поле сброшено. Нажмите «Старт», чтобы начать текущий тур.');
      updateStartButton();
    }

    // --- Инициализация ---
    function init() {
      initCanvas();
      renderPlayers();
      resetField();
      document.addEventListener('keydown', onKey);
      addPlayerBtn.onclick = addPlayer;
      startPauseBtn.onclick = startGameOrPause;
      resetBtn.onclick = resetOnlyField;
      setStatus('Выберите игрока (клик по имени) или создайте нового, затем нажмите «Старт».');
    }

    window.addEventListener('load', init);
  </script>
</body>
</html>`

func main() {
	// Один обработчик корня — отдаём страницу
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := template.Must(template.New("ttt").Parse(htmlPage))
		_ = t.Execute(w, nil)
	})

	// Запускаем сервер в фоне
	go func() {
		fmt.Println("Тик-Так-То (крестики‑нолики) запущен.")
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