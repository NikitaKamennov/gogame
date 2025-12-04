#!/usr/bin/env node
/* 
  Генератор manifest.json для медитационных треков.
  Использование:
    node scripts/generate-meditation-manifest.js --dir ./static/meditationmusic --out manifest.json --concurrency 4 --pretty

  По умолчанию:
    --dir ./static/meditationmusic
    --out <dir>/manifest.json
    --concurrency 4
    инкрементальная генерация на основе существующего manifest.json
*/

const fs = require("fs");
const fsp = require("fs/promises");
const path = require("path");
const mm = require("music-metadata");

function parseArgs() {
  const args = process.argv.slice(2);
  const opts = {
    dir: "./static/meditationmusic",
    out: null,
    concurrency: 4,
    pretty: false,
  };
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--dir") opts.dir = args[++i];
    else if (a === "--out") opts.out = args[++i];
    else if (a === "--concurrency")
      opts.concurrency = Number(args[++i] || 4) || 4;
    else if (a === "--pretty") opts.pretty = true;
    else if (a === "-p") opts.pretty = true;
    else if (a === "-h" || a === "--help") {
      console.log(`
Генератор manifest.json
  --dir <path>          Папка с mp3 (по умолчанию ./static/meditationmusic)
  --out <file>          Куда сохранить manifest.json (по умолчанию <dir>/manifest.json)
  --concurrency <n>     Сколько файлов обрабатывать параллельно (по умолчанию 4)
  --pretty | -p         Красивое форматирование JSON
`);
      process.exit(0);
    }
  }
  if (!opts.out) opts.out = path.join(opts.dir, "manifest.json");
  return opts;
}

function titleFromFile(file) {
  const base = file.replace(/\.mp3$/i, "");
  return decodeURIComponent(base).replace(/[_-]+/g, " ").trim() || base;
}

async function loadPrevManifest(file) {
  try {
    const raw = await fsp.readFile(file, "utf8");
    const json = JSON.parse(raw);
    const map = new Map();
    if (Array.isArray(json.tracks)) {
      for (const t of json.tracks) {
        if (t && typeof t.file === "string") {
          map.set(t.file, t);
        }
      }
    }
    return { json, map };
  } catch {
    return { json: null, map: new Map() };
  }
}

async function scanDir(dir) {
  const entries = await fsp.readdir(dir, { withFileTypes: true });
  return entries
    .filter((e) => e.isFile() && /\.mp3$/i.test(e.name))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b, "ru"));
}

async function statFile(abs) {
  const st = await fsp.stat(abs);
  return {
    size: st.size,
    mtimeMs: st.mtimeMs,
    mtimeISO: new Date(st.mtimeMs).toISOString(),
  };
}

async function readMeta(abs) {
  try {
    const meta = await mm.parseFile(abs, { duration: true });
    const title =
      meta.common && meta.common.title ? String(meta.common.title) : null;
    const durationSec =
      meta.format &&
      typeof meta.format.duration === "number" &&
      isFinite(meta.format.duration)
        ? Math.round(meta.format.duration)
        : 0;
    return { title, durationSec };
  } catch {
    return { title: null, durationSec: 0 };
  }
}

async function withConcurrency(items, limit, worker) {
  const results = new Array(items.length);
  let i = 0;
  let active = 0;
  let rejectFn;
  return await new Promise((resolve, reject) => {
    rejectFn = reject;
    function next() {
      if (i >= items.length && active === 0) return resolve(results);
      while (active < limit && i < items.length) {
        const idx = i++;
        active++;
        Promise.resolve(worker(items[idx], idx))
          .then((res) => {
            results[idx] = res;
            active--;
            next();
          })
          .catch((err) => {
            rejectFn(err);
          });
      }
    }
    next();
  });
}

async function main() {
  const opts = parseArgs();
  const absDir = path.resolve(process.cwd(), opts.dir);
  const absOut = path.resolve(process.cwd(), opts.out);

  if (!fs.existsSync(absDir)) {
    console.error("Папка не найдена:", absDir);
    process.exit(1);
  }

  const files = await scanDir(absDir);
  console.log(`Найдено mp3: ${files.length} в ${absDir}`);

  const { json: prevJson, map: prev } = await loadPrevManifest(absOut);

  const tracks = await withConcurrency(
    files,
    opts.concurrency,
    async (name) => {
      const abs = path.join(absDir, name);
      const st = await statFile(abs);

      // инкрементальность: если size/mtime совпадают — используем прежние данные
      const prevItem = prev.get(name);
      if (prevItem && Number(prevItem.size) === st.size) {
        const prevMtimeMs =
          typeof prevItem.mtimeMs === "number"
            ? prevItem.mtimeMs
            : prevItem.mtime
            ? Date.parse(prevItem.mtime)
            : null;
        if (prevMtimeMs && Math.abs(prevMtimeMs - st.mtimeMs) < 1) {
          return {
            file: name,
            title: prevItem.title || titleFromFile(name),
            durationSec: Number(prevItem.durationSec) || 0,
            size: st.size,
            mtime: st.mtimeISO,
            mtimeMs: st.mtimeMs,
          };
        }
      }

      // читаем метаданные
      const meta = await readMeta(abs);
      return {
        file: name,
        title: meta.title || titleFromFile(name),
        durationSec: meta.durationSec || 0,
        size: st.size,
        mtime: st.mtimeISO,
        mtimeMs: st.mtimeMs,
      };
    }
  );

  const manifest = {
    generatedAt: new Date().toISOString(),
    count: tracks.length,
    tracks,
  };

  const jsonString = opts.pretty
    ? JSON.stringify(manifest, null, 2)
    : JSON.stringify(manifest);
  await fsp.writeFile(absOut, jsonString, "utf8");
  console.log("Манифест сохранён:", absOut);
}

main().catch((err) => {
  console.error("Ошибка генерации манифеста:", err);
  process.exit(1);
});
