#!/usr/bin/env node
/**
 * Простая компрессия MP3 через ffmpeg (ffmpeg-static + fluent-ffmpeg).
 * Примеры:
 *  node scripts/compress-mp3.js --in ./static/meditationmusic --mode vbr --q 5
 *  node scripts/compress-mp3.js --in ./static/meditationmusic --mode cbr --kbps 128 --mono
 *
 * Опции:
 *  --in <dir>         Входная папка с .mp3 (обязательная)
 *  --out <dir>        Выходная папка (по умолчанию <in>_compressed)
 *  --mode vbr|cbr     Режим кодирования (по умолчанию vbr)
 *  --q <0..9>         Качество VBR для LAME (0 лучше/больше, 9 хуже/меньше), по умолчанию 5
 *  --kbps <number>    Битрейт для CBR (например 128), по умолчанию 128
 *  --mono             Принудительно моно (экономит ~в 2 раза для голоса)
 *  --overwrite        Перезаписывать, если файл уже есть
 *  --concurrency <n>  Параллелизм (по умолчанию 2)
 */

const fs = require("fs");
const fsp = require("fs/promises");
const path = require("path");
const ffmpegPath = require("ffmpeg-static");
const ffmpeg = require("fluent-ffmpeg");

if (!ffmpegPath) {
  console.error("ffmpeg-static не нашёл бинарник ffmpeg. Проверьте установку.");
  process.exit(1);
}
ffmpeg.setFfmpegPath(ffmpegPath);

function parseArgs() {
  const args = process.argv.slice(2);
  const opts = {
    inDir: null,
    outDir: null,
    mode: "vbr",
    q: 5,
    kbps: 128,
    mono: false,
    overwrite: false,
    concurrency: 2,
  };
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--in") opts.inDir = args[++i];
    else if (a === "--out") opts.outDir = args[++i];
    else if (a === "--mode") opts.mode = (args[++i] || "vbr").toLowerCase();
    else if (a === "--q")
      opts.q = Math.max(0, Math.min(9, Number(args[++i] || 5)));
    else if (a === "--kbps") opts.kbps = Math.max(8, Number(args[++i] || 128));
    else if (a === "--mono") opts.mono = true;
    else if (a === "--overwrite") opts.overwrite = true;
    else if (a === "--concurrency")
      opts.concurrency = Math.max(1, Number(args[++i] || 2));
    else if (a === "--help" || a === "-h") {
      console.log(`Usage:
  node scripts/compress-mp3.js --in <dir> [--out <dir>] [--mode vbr|cbr] [--q 0..9] [--kbps 128] [--mono] [--overwrite] [--concurrency 2]
`);
      process.exit(0);
    }
  }
  if (!opts.inDir) {
    console.error("Укажите --in <dir> с .mp3 файлами");
    process.exit(1);
  }
  if (!opts.outDir) {
    const absIn = path.resolve(process.cwd(), opts.inDir);
    const base = path.basename(absIn);
    opts.outDir = path.join(path.dirname(absIn), base + "_compressed");
  }
  if (opts.mode !== "vbr" && opts.mode !== "cbr") {
    console.error("Неверный --mode. Допустимо: vbr | cbr");
    process.exit(1);
  }
  return opts;
}

async function ensureDir(dir) {
  await fsp.mkdir(dir, { recursive: true });
}

async function scanMp3(dir) {
  const items = await fsp.readdir(dir, { withFileTypes: true });
  const files = [];
  for (const it of items) {
    if (it.isFile() && /\.mp3$/i.test(it.name)) {
      files.push(path.join(dir, it.name));
    }
  }
  return files.sort((a, b) => a.localeCompare(b, "ru"));
}

function outputFor(inputAbs, inDirAbs, outDirAbs) {
  const rel = path.relative(inDirAbs, inputAbs);
  const out = path.join(outDirAbs, rel);
  return out;
}

function transcodeMp3(input, output, opts) {
  return new Promise((resolve, reject) => {
    const cmd = ffmpeg(input)
      .noVideo()
      .audioCodec("libmp3lame")
      .outputOptions(["-map_metadata 0", "-id3v2_version 3"]);

    if (opts.mono) {
      cmd.audioChannels(1);
    }

    if (opts.mode === "vbr") {
      // VBR: q 0..9 (0 best quality/larger, 9 lower quality/smaller)
      cmd.audioQuality(opts.q);
    } else {
      // CBR: фиксированный битрейт
      cmd.audioBitrate(`${opts.kbps}k`);
    }

    ensureDir(path.dirname(output))
      .then(() => {
        if (!opts.overwrite && fs.existsSync(output)) {
          return resolve({ input, output, skipped: true, reason: "exists" });
        }
        cmd
          .on("error", (err) => reject(err))
          .on("end", () => resolve({ input, output, skipped: false }))
          .save(output);
      })
      .catch(reject);
  });
}

async function withConcurrency(items, limit, worker) {
  const results = [];
  let i = 0;
  const runners = new Array(Math.min(limit, items.length))
    .fill(0)
    .map(async () => {
      while (i < items.length) {
        const idx = i++;
        const it = items[idx];
        try {
          const res = await worker(it, idx);
          results[idx] = res;
        } catch (e) {
          results[idx] = {
            error: String(e && e.message ? e.message : e),
            item: it,
          };
        }
      }
    });
  await Promise.all(runners);
  return results;
}

async function main() {
  const opts = parseArgs();
  const inDirAbs = path.resolve(process.cwd(), opts.inDir);
  const outDirAbs = path.resolve(process.cwd(), opts.outDir);

  if (!fs.existsSync(inDirAbs)) {
    console.error("Папка не найдена:", inDirAbs);
    process.exit(1);
  }
  await ensureDir(outDirAbs);

  const files = await scanMp3(inDirAbs);
  if (!files.length) {
    console.log("MP3 не найдены в", inDirAbs);
    return;
  }
  console.log(`Найдено MP3: ${files.length}`);
  console.log(
    `Режим: ${
      opts.mode === "vbr" ? `VBR q=${opts.q}` : `CBR ${opts.kbps} kbps`
    }${opts.mono ? " (mono)" : ""}`
  );
  console.log(`Выход: ${outDirAbs}`);

  const results = await withConcurrency(
    files,
    opts.concurrency,
    async (input) => {
      const output = outputFor(input, inDirAbs, outDirAbs);
      return await transcodeMp3(input, output, opts);
    }
  );

  const ok = results.filter((r) => r && !r.error && !r.skipped).length;
  const skipped = results.filter((r) => r && r.skipped).length;
  const failed = results.filter((r) => r && r.error).length;
  if (failed) {
    console.log("Ошибки:");
    for (const r of results) {
      if (r && r.error) {
        console.log(" -", r.item, "=>", r.error);
      }
    }
  }
  console.log(`Готово: ok=${ok}, skipped=${skipped}, failed=${failed}`);
}

main().catch((e) => {
  console.error("Ошибка:", e);
  process.exit(1);
});
