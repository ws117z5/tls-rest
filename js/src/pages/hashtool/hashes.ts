// Pure-JS hash implementations for the Hash Tool page — no dependency needed
// (Web Crypto covers SHA-1/256/384/512 but not MD5, and doesn't have CRC32 or
// MurmurHash3 at all). Each is verified against known test vectors; see the
// tests referenced in the PR/commit that added this file.

function utf8Bytes(s: string): Uint8Array {
  return new TextEncoder().encode(s);
}

function rotl(x: number, n: number): number {
  return (x << n) | (x >>> (32 - n));
}

// MD5 (RFC 1321).
export function md5(message: string): string {
  const K = new Int32Array(64);
  for (let i = 0; i < 64; i++) {
    K[i] = Math.floor(Math.abs(Math.sin(i + 1)) * 4294967296) | 0;
  }
  const S = [
    7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
    5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
    4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
    6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
  ];

  const msg = utf8Bytes(message);
  const bitLenLow = (msg.length * 8) >>> 0;
  const bitLenHigh = Math.floor((msg.length * 8) / 4294967296) >>> 0;

  let padLen = msg.length + 1;
  while (padLen % 64 !== 56) padLen++;
  const padded = new Uint8Array(padLen + 8);
  padded.set(msg);
  padded[msg.length] = 0x80;
  const dv = new DataView(padded.buffer);
  dv.setUint32(padded.length - 8, bitLenLow, true);
  dv.setUint32(padded.length - 4, bitLenHigh, true);

  let a0 = 0x67452301 | 0, b0 = 0xefcdab89 | 0, c0 = 0x98badcfe | 0, d0 = 0x10325476 | 0;

  for (let chunkStart = 0; chunkStart < padded.length; chunkStart += 64) {
    const M = new Int32Array(16);
    for (let j = 0; j < 16; j++) M[j] = dv.getInt32(chunkStart + j * 4, true);

    let A = a0, B = b0, C = c0, D = d0;
    for (let i = 0; i < 64; i++) {
      let F: number, g: number;
      if (i < 16) { F = (B & C) | (~B & D); g = i; }
      else if (i < 32) { F = (D & B) | (~D & C); g = (5 * i + 1) % 16; }
      else if (i < 48) { F = B ^ C ^ D; g = (3 * i + 5) % 16; }
      else { F = C ^ (B | ~D); g = (7 * i) % 16; }
      F = (F + A + K[i] + M[g]) | 0;
      A = D; D = C; C = B;
      B = (B + rotl(F, S[i])) | 0;
    }
    a0 = (a0 + A) | 0; b0 = (b0 + B) | 0; c0 = (c0 + C) | 0; d0 = (d0 + D) | 0;
  }

  const toHexLE = (n: number) => {
    const buf = new ArrayBuffer(4);
    new DataView(buf).setInt32(0, n, true);
    return Array.from(new Uint8Array(buf)).map((b) => b.toString(16).padStart(2, "0")).join("");
  };
  return toHexLE(a0) + toHexLE(b0) + toHexLE(c0) + toHexLE(d0);
}

let crcTable: number[] | null = null;
function makeCrcTable(): number[] {
  const table = new Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? (0xedb88320 ^ (c >>> 1)) : c >>> 1;
    table[n] = c >>> 0;
  }
  return table;
}

// CRC32 (as used by zip/gzip).
export function crc32(input: string): string {
  if (!crcTable) crcTable = makeCrcTable();
  const bytes = utf8Bytes(input);
  let crc = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) crc = crcTable[(crc ^ bytes[i]) & 0xff] ^ (crc >>> 8);
  crc = (crc ^ 0xffffffff) >>> 0;
  return crc.toString(16).padStart(8, "0");
}

// MurmurHash3 (32-bit, x86 variant) — the same family papers.go uses to
// derive room hashes (github.com/ws117z5/mmh3), seed 0 by default.
export function murmur3_32(input: string, seed = 0): string {
  const bytes = utf8Bytes(input);
  let h1 = seed >>> 0;
  const c1 = 0xcc9e2d51, c2 = 0x1b873593;
  const len = bytes.length;
  const nblocks = len >> 2;

  for (let i = 0; i < nblocks; i++) {
    let k1 =
      (bytes[i * 4] & 0xff) |
      ((bytes[i * 4 + 1] & 0xff) << 8) |
      ((bytes[i * 4 + 2] & 0xff) << 16) |
      ((bytes[i * 4 + 3] & 0xff) << 24);

    k1 = Math.imul(k1, c1);
    k1 = (k1 << 15) | (k1 >>> 17);
    k1 = Math.imul(k1, c2);

    h1 ^= k1;
    h1 = (h1 << 13) | (h1 >>> 19);
    h1 = (Math.imul(h1, 5) + 0xe6546b64) | 0;
  }

  let k1 = 0;
  const tailIndex = nblocks * 4;
  switch (len & 3) {
    case 3: k1 ^= (bytes[tailIndex + 2] & 0xff) << 16; // falls through
    case 2: k1 ^= (bytes[tailIndex + 1] & 0xff) << 8; // falls through
    case 1:
      k1 ^= bytes[tailIndex] & 0xff;
      k1 = Math.imul(k1, c1);
      k1 = (k1 << 15) | (k1 >>> 17);
      k1 = Math.imul(k1, c2);
      h1 ^= k1;
  }

  h1 ^= len;
  h1 ^= h1 >>> 16;
  h1 = Math.imul(h1, 0x85ebca6b);
  h1 ^= h1 >>> 13;
  h1 = Math.imul(h1, 0xc2b2ae35);
  h1 ^= h1 >>> 16;

  return (h1 >>> 0).toString(16).padStart(8, "0");
}

// SHA-1/256/384/512 via the browser's native Web Crypto (not available for MD5).
export async function webCryptoDigest(algo: "SHA-1" | "SHA-256" | "SHA-384" | "SHA-512", input: string): Promise<string> {
  const buf = await crypto.subtle.digest(algo, utf8Bytes(input) as BufferSource);
  return Array.from(new Uint8Array(buf)).map((b) => b.toString(16).padStart(2, "0")).join("");
}

export interface HashResult {
  name: string;
  value: string;
}

// Computes every algorithm for one input, in a fixed display order.
export async function hashAll(input: string): Promise<HashResult[]> {
  const [sha1, sha256, sha384, sha512] = await Promise.all([
    webCryptoDigest("SHA-1", input),
    webCryptoDigest("SHA-256", input),
    webCryptoDigest("SHA-384", input),
    webCryptoDigest("SHA-512", input),
  ]);
  return [
    { name: "MD5", value: md5(input) },
    { name: "SHA-1", value: sha1 },
    { name: "SHA-256", value: sha256 },
    { name: "SHA-384", value: sha384 },
    { name: "SHA-512", value: sha512 },
    { name: "CRC32", value: crc32(input) },
    { name: "MurmurHash3 (32-bit)", value: murmur3_32(input) },
  ];
}

// A random string to hash (32 random bytes, hex-encoded) — "generate a
// random hash" means hashing random input, since every hash function here is
// deterministic.
export function randomInput(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes).map((b) => b.toString(16).padStart(2, "0")).join("");
}
