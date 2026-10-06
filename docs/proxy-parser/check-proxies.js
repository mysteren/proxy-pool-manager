#!/usr/bin/env node
// https://github.com/kort0881/telegram-proxy-collector
// Проверка прокси из всех .txt файлов в текущей директории + авто-загрузка списков с GitHub.
// Жёсткая проверка: реальное MTProto-рукопожатие (fake-TLS HMAC или obfuscated2 + req_pq к DC),
// а не только открытый TCP-порт. Рабочие пишутся в working_proxies.txt.

const fs = require('fs');
const net = require('net');
const https = require('https');
const crypto = require('crypto');

const TIMEOUT = Number(process.env.TIMEOUT ?? 5000);      // мс на проверку одного прокси
const CONCURRENCY = Number(process.env.CONCURRENCY ?? 50); // параллельных сокетов
const OUT_FILE = 'working_proxies.txt';

// Списки прокси для авто-загрузки (REMOTE=0 отключает загрузку)
const REMOTE = process.env.REMOTE !== '0';
const REMOTE_BASE = 'https://raw.githubusercontent.com/kort0881/telegram-proxy-collector/main/';
const REMOTE_FILES = [
  'proxy_ru.txt', 'proxy_eu.txt', 'proxy_us.txt', 'proxy_asia.txt',
  'proxy_all_mtproto.txt', 'proxy_all.txt', 'socks5.txt',
];

// --- Парсинг строки в объект прокси -----------------------------------------
function parseLine(raw) {
  const line = raw.trim();
  if (!line || line.startsWith('#')) return null;

  // tg://proxy?server=...&port=...&secret=...   (и t.me/proxy?...)
  if (/tg:\/\/(proxy|socks)/i.test(line) || /t\.me\/(proxy|socks)/i.test(line)) {
    const url = line.replace(/^t\.me\//i, 'tg://');
    const isSocks = /socks/i.test(url);
    const q = url.slice(url.indexOf('?') + 1);
    const p = new URLSearchParams(q);
    const host = p.get('server');
    const port = Number(p.get('port'));
    const secret = p.get('secret') || '';
    if (!host || !port) return null;
    return { host, port, secret, type: isSocks ? 'socks' : 'mtproto', raw: line, link: buildLink(host, port, secret, isSocks) };
  }

  // host:port:secret  |  host:port
  const parts = line.split(':');
  if (parts.length >= 2 && /^\d+$/.test(parts[1])) {
    const host = parts[0];
    const port = Number(parts[1]);
    const secret = parts[2] || '';
    return { host, port, secret, type: secret ? 'mtproto' : 'socks', raw: line, link: buildLink(host, port, secret, !secret) };
  }
  return null;
}

function buildLink(host, port, secret, socks) {
  if (socks) return `tg://socks?server=${host}&port=${port}`;
  return `tg://proxy?server=${host}&port=${port}&secret=${secret}`;
}

// --- Загрузка списков из GitHub-репозитория ----------------------------------
function httpGet(url, redirects = 5) {
  return new Promise(resolve => {
    https.get(url, res => {
      const { statusCode, headers } = res;
      if (statusCode >= 300 && statusCode < 400 && headers.location && redirects > 0) {
        res.resume();
        return resolve(httpGet(new URL(headers.location, url).href, redirects - 1));
      }
      if (statusCode !== 200) { res.resume(); return resolve(null); }
      let data = '';
      res.setEncoding('utf8');
      res.on('data', c => (data += c));
      res.on('end', () => resolve(data));
    }).on('error', () => resolve(null));
  });
}

async function downloadRemote() {
  const results = await Promise.all(REMOTE_FILES.map(async f => {
    const text = await httpGet(REMOTE_BASE + f);
    return text ? { name: `remote:${f}`, text } : null;
  }));
  return results.filter(Boolean);
}

// --- Сбор прокси из локальных .txt и удалённых списков ------------------------
function localSources() {
  return fs.readdirSync('.')
    .filter(f => f.toLowerCase().endsWith('.txt') && f !== OUT_FILE)
    .map(f => ({ name: f, text: fs.readFileSync(f, 'utf8') }));
}

function collectProxies(sources) {
  const seen = new Set();
  const proxies = [];
  for (const { name, text } of sources) {
    for (const line of text.split(/\r?\n/)) {
      const p = parseLine(line);
      if (!p) continue;
      const key = `${p.host}:${p.port}:${p.secret}`;
      if (seen.has(key)) continue;
      seen.add(key);
      p.file = name;
      proxies.push(p);
    }
  }
  return proxies;
}

// ============================================================================
// Секреты и крипто-примитивы
// ============================================================================
const PROTO_ABRIDGED = Buffer.from([0xef, 0xef, 0xef, 0xef]);
const PROTO_INTERMEDIATE = Buffer.from([0xee, 0xee, 0xee, 0xee]);
const PROTO_SECURE = Buffer.from([0xdd, 0xdd, 0xdd, 0xdd]);
const RESERVED_NONCE_BEGINNINGS = new Set([
  '48454144', '504f5354', '47455420', 'eeeeeeee', 'dddddddd', '16030102',
]);

function sha256(...parts) {
  const h = crypto.createHash('sha256');
  for (const p of parts) h.update(p);
  return h.digest();
}

function hmac256(key, ...parts) {
  const h = crypto.createHmac('sha256', key);
  for (const p of parts) h.update(p);
  return h.digest();
}

// Классический (hex) или base64url-секрет -> ключ 16 байт + режим.
function parseSecret(secretStr) {
  const s = (secretStr || '').trim();
  if (!s) return null;
  let bytes;
  if (/^[0-9a-fA-F]+$/.test(s) && s.length % 2 === 0) {
    bytes = Buffer.from(s, 'hex');
  } else if (/^[A-Za-z0-9_\-=]+$/.test(s)) {
    const b64 = s.replace(/-/g, '+').replace(/_/g, '/').replace(/=+$/, '');
    const pad = b64.length % 4 ? '='.repeat(4 - (b64.length % 4)) : '';
    bytes = Buffer.from(b64 + pad, 'base64');
  } else {
    return null;
  }
  if (bytes.length < 16) return null;
  if (bytes[0] === 0xee && bytes.length >= 17) {
    return { mode: 'tls', key: bytes.subarray(1, 17), domain: bytes.subarray(17).toString('latin1') };
  }
  if (bytes[0] === 0xdd && bytes.length >= 17) {
    return { mode: 'secure', key: bytes.subarray(1, 17) };
  }
  return { mode: 'classic', key: bytes.subarray(0, 16) };
}

// ============================================================================
// Обмен данными по TCP с оценкой пинга и потоковой расшифровкой (AES-CTR)
// ============================================================================
function exchange(host, port, payload, timeout, predicate, decipher) {
  return new Promise(resolve => {
    const start = Date.now();
    const sock = net.connect({ host, port });
    let out = Buffer.alloc(0);
    let done = false;
    const finish = ok => {
      if (done) return;
      done = true;
      clearTimeout(timer);
      sock.destroy();
      resolve(ok ? { ping: Date.now() - start } : null);
    };
    const timer = setTimeout(() => finish(false), timeout);
    sock.setNoDelay(true);
    sock.on('connect', () => sock.write(payload));
    sock.on('data', chunk => {
      out = Buffer.concat([out, decipher ? decipher.update(chunk) : chunk]);
      let ok = false;
      try { ok = predicate(out); } catch { ok = false; }
      if (!done && ok) finish(true);
    });
    sock.on('error', () => finish(false));
    sock.on('timeout', () => finish(false));
    sock.on('close', () => finish(false));
  });
}

// ============================================================================
// Fake-TLS (секрет "ee.."): шлём ClientHello с HMAC-подписью, сервер в ответе
// встраивает HMAC(secret, наш digest + его ServerHello) — только настоящий
// прокси, знающий секрет, сможет это посчитать. Проверяем эту подпись.
// ============================================================================
function buildFakeTlsClientHello(key, domain) {
  const name = Buffer.from(domain || 'www.google.com', 'latin1');
  const parts = [
    Buffer.from([0x16, 0x03, 0x01, 0x02, 0x00, 0x01, 0x00, 0x01, 0xfc, 0x03, 0x03]),
    Buffer.alloc(32),                 // [11:43] — место под digest
    Buffer.from([0x20]),
    crypto.randomBytes(32),           // session id
    Buffer.from([0x00, 0x22, 0x4a, 0x4a, 0x13, 0x01, 0x13, 0x02, 0x13, 0x03, 0xc0, 0x2b, 0xc0, 0x2f, 0xc0, 0x2c, 0xc0, 0x30, 0xcc, 0xa9]),
    Buffer.from([0xcc, 0xa8, 0xc0, 0x13, 0xc0, 0x14, 0x00, 0x9c, 0x00, 0x9d, 0x00, 0x2f, 0x00, 0x35, 0x00, 0x0a, 0x01, 0x00, 0x01, 0x91]),
    Buffer.from([0xda, 0xda, 0x00, 0x00, 0x00, 0x00]),
    uint16be(name.length + 5), uint16be(name.length + 3), Buffer.from([0x00]), uint16be(name.length), name,
    Buffer.from([0x00, 0x17, 0x00, 0x00, 0xff, 0x01, 0x00, 0x01, 0x00, 0x00, 0x0a, 0x00, 0x0a, 0x00, 0x08, 0xaa, 0xaa, 0x00, 0x1d, 0x00]),
    Buffer.from([0x17, 0x00, 0x18, 0x00, 0x0b, 0x00, 0x02, 0x01, 0x00, 0x00, 0x23, 0x00, 0x00, 0x00, 0x10, 0x00, 0x0e, 0x00, 0x0c, 0x02]),
    Buffer.from([0x68, 0x32, 0x08, 0x68, 0x74, 0x74, 0x70, 0x2f, 0x31, 0x2e, 0x31, 0x00, 0x05, 0x00, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00]),
    Buffer.from([0x00, 0x0d, 0x00, 0x14, 0x00, 0x12, 0x04, 0x03, 0x08, 0x04, 0x04, 0x01, 0x05, 0x03, 0x08, 0x05, 0x05, 0x01, 0x08, 0x06]),
    Buffer.from([0x06, 0x01, 0x02, 0x01, 0x00, 0x12, 0x00, 0x00, 0x00, 0x33, 0x00, 0x2b, 0x00, 0x29, 0xaa, 0xaa, 0x00, 0x01, 0x00, 0x00]),
    Buffer.from([0x1d, 0x00, 0x20]), crypto.randomBytes(32),
    Buffer.from([0x00, 0x2d, 0x00, 0x02, 0x01, 0x01, 0x00, 0x2b, 0x00, 0x0b, 0x0a, 0xba, 0xba, 0x03, 0x04, 0x03, 0x03, 0x03, 0x02, 0x03]),
    Buffer.from([0x01, 0x00, 0x1b, 0x00, 0x03, 0x02, 0x00, 0x02, 0x3a, 0x3a, 0x00, 0x01, 0x00, 0x00, 0x15]),
  ];
  let msg = Buffer.concat(parts);
  const padLen = 517 - msg.length - 2;
  msg = Buffer.concat([msg, uint16be(padLen), Buffer.alloc(padLen)]);

  const digest = Buffer.from(hmac256(key, msg));          // digest-поле = нули
  const ts = Buffer.alloc(4);
  ts.writeUInt32LE(Math.floor(Date.now() / 1000) >>> 0);
  for (let i = 0; i < 4; i++) digest[28 + i] ^= ts[i];
  digest.copy(msg, 11);
  return { buf: msg, digest };
}

function uint16be(n) {
  const b = Buffer.alloc(2);
  b.writeUInt16BE(n);
  return b;
}

async function fakeTlsCheck(proxy, { key, domain }) {
  const { buf, digest } = buildFakeTlsClientHello(key, domain);
  const res = await exchange(proxy.host, proxy.port, buf, TIMEOUT, out => {
    const hello = readTlsRecords(out);
    if (!hello || hello.length < 43) return false;
    const recv = hello.subarray(11, 43);
    const zeroed = Buffer.from(hello);
    zeroed.fill(0, 11, 43);
    return recv.equals(hmac256(key, digest, zeroed));
  });
  return { ok: !!res, ping: res ? res.ping : null, method: 'fake-tls' };
}

// Собирает hello_pkt = ServerHello + ChangeCipherSpec + AppData (пока не встретим 0x17).
function readTlsRecords(buf) {
  const recs = [];
  let off = 0;
  while (off + 5 <= buf.length) {
    const type = buf[off];
    const len = buf.readUInt16BE(off + 3);
    if (off + 5 + len > buf.length) return null;
    recs.push(buf.subarray(off, off + 5 + len));
    off += 5 + len;
    if (type === 0x17) return Buffer.concat(recs);
    if (recs.length >= 4) return null;
  }
  return null;
}

// ============================================================================
// Obfuscated2 (секреты "dd"/plain): рукопожатие + реальный req_pq к DC.
// Успешный ответ resPQ доказывает, что прокси живой и ходит в Telegram.
// ============================================================================
function newMessageId() {
  const nowSec = BigInt(Math.floor(Date.now() / 1000));
  const n = BigInt(Math.floor(Math.random() * 0xffffff)) & 0xffffffn;
  return (nowSec << 32n) | (n << 2n);
}

function reqPqPacket(nonce) {
  const body = Buffer.concat([Buffer.from([0xf1, 0x8e, 0x7e, 0xbe]), nonce]); // req_pq_multi
  const msgId = Buffer.alloc(8);
  msgId.writeBigUInt64LE(newMessageId());
  const len = Buffer.alloc(4);
  len.writeUInt32LE(body.length);
  return Buffer.concat([Buffer.alloc(8), msgId, len, body]); // auth_key_id = 0
}

function framePacket(packet, tag) {
  if (tag === PROTO_ABRIDGED) return Buffer.concat([Buffer.from([packet.length / 4]), packet]);
  const len = Buffer.alloc(4);
  len.writeUInt32LE(packet.length);
  return Buffer.concat([len, packet]);
}

function buildObfuscated2(key, tag) {
  let rnd;
  do {
    rnd = crypto.randomBytes(64);
  } while (
    rnd[0] === 0xef ||
    RESERVED_NONCE_BEGINNINGS.has(rnd.subarray(0, 4).toString('hex')) ||
    rnd.readUInt32LE(4) === 0
  );
  tag.copy(rnd, 56);
  rnd[60] = 2; rnd[61] = 0; // dc index = 2 (little-endian)

  // Шифрование клиент->сервер использует прямые байты, сервер->клиент — реверс.
  const encKey = sha256(rnd.subarray(8, 40), key);
  const encCipher = crypto.createCipheriv('aes-256-ctr', encKey, rnd.subarray(40, 56));

  const rev = Buffer.from(rnd.subarray(8, 56)).reverse();
  const decKey = sha256(rev.subarray(0, 32), key);
  const decCipher = crypto.createDecipheriv('aes-256-ctr', decKey, rev.subarray(32, 48));

  const encAll = encCipher.update(rnd); // расходуем 64 байта keystream
  const handshake = Buffer.concat([rnd.subarray(0, 56), encAll.subarray(56, 64)]);
  return { handshake, encCipher, decCipher };
}

async function obfuscated2Check(proxy, { key, mode }) {
  const tags = mode === 'secure' ? [PROTO_SECURE, PROTO_ABRIDGED] : [PROTO_ABRIDGED, PROTO_SECURE];
  for (const tag of tags) {
    const nonce = crypto.randomBytes(16);
    const { handshake, encCipher, decCipher } = buildObfuscated2(key, tag);
    const framed = encCipher.update(framePacket(reqPqPacket(nonce), tag));
    const needle = Buffer.concat([Buffer.from([0x63, 0x24, 0x16, 0x05]), nonce]); // resPQ + our nonce
    const res = await exchange(proxy.host, proxy.port, Buffer.concat([handshake, framed]), TIMEOUT,
      out => out.includes(needle), decCipher);
    if (res) {
      const name = tag === PROTO_ABRIDGED ? 'abridged' : tag === PROTO_SECURE ? 'secure' : 'intermediate';
      return { ok: true, ping: res.ping, method: `mtproto/${name}` };
    }
  }
  return { ok: false, ping: null, method: null };
}

// ============================================================================
// Проверка одного прокси
// ============================================================================
async function tcpCheck(proxy) {
  return new Promise(resolve => {
    const start = Date.now();
    const socket = new net.Socket();
    let done = false;
    const finish = ok => {
      if (done) return;
      done = true;
      socket.destroy();
      resolve({ ok, ping: ok ? Date.now() - start : null, method: ok ? 'tcp' : null });
    };
    socket.setTimeout(TIMEOUT, () => finish(false));
    socket.once('connect', () => finish(true));
    socket.once('error', () => finish(false));
    socket.connect(proxy.port, proxy.host);
  });
}

async function check(proxy) {
  const parsed = proxy.secret ? parseSecret(proxy.secret) : null;
  let r;
  if (!parsed || proxy.type === 'socks') {
    r = await tcpCheck(proxy); // SOCKS5 и непонятные секреты — только TCP
  } else if (parsed.mode === 'tls') {
    r = await fakeTlsCheck(proxy, parsed);
  } else {
    r = await obfuscated2Check(proxy, parsed);
  }
  return { ...proxy, ...r };
}

// --- Запуск с ограничением параллелизма --------------------------------------
async function run(proxies) {
  let index = 0;
  const working = [];

  async function worker() {
    while (index < proxies.length) {
      const proxy = proxies[index++];
      const r = await check(proxy);
      if (r.ok) {
        working.push(r);
        console.log(`✅ OK   ${String(r.ping).padStart(5)}ms  ${r.method.padEnd(16)}  ${r.link}   [${r.file}]`);
      } else {
        console.log(`🛑 DEAD              ${r.link}   [${r.file}]`);
      }
    }
  }

  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, proxies.length) }, worker));

  // Сортировка по возрастанию пинга (наименьший — первым)
  working.sort((a, b) => a.ping - b.ping);

  const lines = [`# Working proxies | checked: ${new Date().toISOString()}`];
  for (const r of working) {
    // ping добавляется как обычный GET-параметр в конец, не трогая server/port/secret
    const sep = r.link.includes('?') ? '&' : '?';
    lines.push(`${r.link}${sep}ping=${r.ping}`);
  }
  fs.writeFileSync(OUT_FILE, lines.join('\n') + '\n');

  console.log(`\n\nГотово: ${working.length}/${proxies.length} рабочих. ` +
    `Отсортированы по пингу и записаны в ${OUT_FILE}\n`);
  console.log('Топ по пингу (наименьший первым):');
  for (const r of working) console.log(`  ${String(r.ping).padStart(5)}ms  ${r.method.padEnd(16)}  ${r.link}`);
}

// --- main ---------------------------------------------------------------------
if (require.main === module) {
  (async () => {
    const sources = localSources();
    const localCount = sources.length;

    if (REMOTE) {
      const remote = await downloadRemote();
      sources.push(...remote);
      console.log(`Загружено удалённых списков: ${remote.length}/${REMOTE_FILES.length}` +
        (remote.length < REMOTE_FILES.length ? ' (часть недоступна)' : ''));
    }

    const proxies = collectProxies(sources);
    console.log(`Источников: ${localCount} локальных + ${sources.length - localCount} удалённых`);
    console.log(`Найдено прокси: ${proxies.length} (таймаут ${TIMEOUT}ms, параллельно ${CONCURRENCY})\n`);

    await run(proxies);
  })();
}

module.exports = { parseSecret, parseLine, buildFakeTlsClientHello, buildObfuscated2, reqPqPacket, framePacket, check, exchange, PROTO_ABRIDGED, PROTO_SECURE, PROTO_INTERMEDIATE };
