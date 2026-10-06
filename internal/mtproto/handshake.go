package mtproto

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"time"
)

const protoUDPFakeTLSLength = 517

var (
	protoAbridged = []byte{0xef, 0xef, 0xef, 0xef}
	protoSecure   = []byte{0xdd, 0xdd, 0xdd, 0xdd}
)

// buildFakeTLSClientHello собирает ClientHello с HMAC-подписью в поле digest
// и возвращает сам пакет и digest (с учётом XOR метки времени).
func buildFakeTLSClientHello(key []byte, domain string) ([]byte, []byte) {
	if domain == "" {
		domain = "www.google.com"
	}
	name := []byte(domain)
	parts := [][]byte{
		{0x16, 0x03, 0x01, 0x02, 0x00, 0x01, 0x00, 0x01, 0xfc, 0x03, 0x03},
		make([]byte, 32), // [11:43] — место под digest
		{0x20},
		randBytes(32), // session id
		{0x00, 0x22, 0x4a, 0x4a, 0x13, 0x01, 0x13, 0x02, 0x13, 0x03, 0xc0, 0x2b, 0xc0, 0x2f, 0xc0, 0x2c, 0xc0, 0x30, 0xcc, 0xa9},
		{0xcc, 0xa8, 0xc0, 0x13, 0xc0, 0x14, 0x00, 0x9c, 0x00, 0x9d, 0x00, 0x2f, 0x00, 0x35, 0x00, 0x0a, 0x01, 0x00, 0x01, 0x91},
		{0xda, 0xda, 0x00, 0x00, 0x00, 0x00},
		uint16be(len(name) + 5), uint16be(len(name) + 3), {0x00}, uint16be(len(name)), name,
		{0x00, 0x17, 0x00, 0x00, 0xff, 0x01, 0x00, 0x01, 0x00, 0x00, 0x0a, 0x00, 0x0a, 0x00, 0x08, 0xaa, 0xaa, 0x00, 0x1d, 0x00},
		{0x17, 0x00, 0x18, 0x00, 0x0b, 0x00, 0x02, 0x01, 0x00, 0x00, 0x23, 0x00, 0x00, 0x00, 0x10, 0x00, 0x0e, 0x00, 0x0c, 0x02},
		{0x68, 0x32, 0x08, 0x68, 0x74, 0x74, 0x70, 0x2f, 0x31, 0x2e, 0x31, 0x00, 0x05, 0x00, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x0d, 0x00, 0x14, 0x00, 0x12, 0x04, 0x03, 0x08, 0x04, 0x04, 0x01, 0x05, 0x03, 0x08, 0x05, 0x05, 0x01, 0x08, 0x06},
		{0x06, 0x01, 0x02, 0x01, 0x00, 0x12, 0x00, 0x00, 0x00, 0x33, 0x00, 0x2b, 0x00, 0x29, 0xaa, 0xaa, 0x00, 0x01, 0x00, 0x00},
		{0x1d, 0x00, 0x20}, randBytes(32),
		{0x00, 0x2d, 0x00, 0x02, 0x01, 0x01, 0x00, 0x2b, 0x00, 0x0b, 0x0a, 0xba, 0xba, 0x03, 0x04, 0x03, 0x03, 0x03, 0x02, 0x03},
		{0x01, 0x00, 0x1b, 0x00, 0x03, 0x02, 0x00, 0x02, 0x3a, 0x3a, 0x00, 0x01, 0x00, 0x00, 0x15},
	}
	msg := concat(parts...)
	padLen := protoUDPFakeTLSLength - len(msg) - 2
	msg = concat(msg, uint16be(padLen), make([]byte, padLen))

	digest := hmac256(key, msg) // digest-поле = нули
	ts := make([]byte, 4)
	binary.LittleEndian.PutUint32(ts, uint32(time.Now().Unix()))
	for i := 0; i < 4; i++ {
		digest[28+i] ^= ts[i]
	}
	copy(msg[11:43], digest)
	return msg, digest
}

// readTLSRecords собирает hello_pkt (до записи 0x17), как в эталоне.
func readTLSRecords(buf []byte) ([]byte, bool) {
	var recs [][]byte
	off := 0
	for off+5 <= len(buf) {
		typ := buf[off]
		length := int(binary.BigEndian.Uint16(buf[off+3 : off+5]))
		if off+5+length > len(buf) {
			return nil, false
		}
		recs = append(recs, buf[off:off+5+length])
		off += 5 + length
		if typ == 0x17 {
			return concat(recs...), true
		}
		if len(recs) >= 4 {
			return nil, false
		}
	}
	return nil, false
}

func fakeTLSCheck(ctx context.Context, host string, port int, key []byte, domain string, timeout time.Duration) (int, bool) {
	msg, digest := buildFakeTLSClientHello(key, domain)
	return exchange(ctx, host, port, msg, timeout, func(out []byte) bool {
		hello, ok := readTLSRecords(out)
		if !ok || len(hello) < 43 {
			return false
		}
		recv := hello[11:43]
		zeroed := make([]byte, len(hello))
		copy(zeroed, hello)
		for i := 11; i < 43; i++ {
			zeroed[i] = 0
		}
		return bytes.Equal(recv, hmac256(key, digest, zeroed))
	}, nil)
}

var reservedNonceBeginnings = [][]byte{
	{0x48, 0x45, 0x41, 0x44}, // HEAD
	{0x50, 0x4f, 0x53, 0x54}, // POST
	{0x47, 0x45, 0x54, 0x20}, // "GET "
	{0xee, 0xee, 0xee, 0xee},
	{0xdd, 0xdd, 0xdd, 0xdd},
	{0x16, 0x03, 0x01, 0x02},
}

func newMessageID() uint64 {
	now := uint64(time.Now().Unix())
	var rb [4]byte
	copy(rb[:], randBytes(4))
	r := uint64(binary.LittleEndian.Uint32(rb[:])) & 0xffffff
	return (now << 32) | (r << 2)
}

func reqPqPacket(nonce []byte) []byte {
	body := concat([]byte{0xf1, 0x8e, 0x7e, 0xbe}, nonce) // req_pq_multi
	msgID := make([]byte, 8)
	binary.LittleEndian.PutUint64(msgID, newMessageID())
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(body)))
	return concat(make([]byte, 8), msgID, length, body) // auth_key_id = 0
}

func framePacket(packet, tag []byte) []byte {
	if bytes.Equal(tag, protoAbridged) {
		return concat([]byte{byte(len(packet) / 4)}, packet)
	}
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(packet)))
	return concat(length, packet)
}

func buildObfuscated2(key, tag []byte) ([]byte, cipher.Stream, cipher.Stream) {
	var rnd []byte
	for {
		rnd = randBytes(64)
		if rnd[0] == 0xef {
			continue
		}
		reserved := false
		for _, r := range reservedNonceBeginnings {
			if bytes.Equal(rnd[0:4], r) {
				reserved = true
				break
			}
		}
		if reserved || binary.LittleEndian.Uint32(rnd[4:8]) == 0 {
			continue
		}
		break
	}
	copy(rnd[56:60], tag)
	rnd[60] = 2 // dc index = 2 (little-endian)
	rnd[61] = 0

	encBlock, _ := aes.NewCipher(sha256sum(rnd[8:40], key))
	encStream := cipher.NewCTR(encBlock, rnd[40:56])

	rev := reverse(rnd[8:56])
	decBlock, _ := aes.NewCipher(sha256sum(rev[0:32], key))
	decStream := cipher.NewCTR(decBlock, rev[32:48])

	encAll := make([]byte, 64)
	encStream.XORKeyStream(encAll, rnd)
	handshake := concat(rnd[0:56], encAll[56:64])
	return handshake, encStream, decStream
}

func obfuscated2Check(ctx context.Context, host string, port int, key []byte, mode Mode, timeout time.Duration) (int, string, bool) {
	type candidate struct {
		tag  []byte
		name string
	}
	abridged := candidate{protoAbridged, "abridged"}
	secure := candidate{protoSecure, "secure"}
	candidates := []candidate{abridged, secure}
	if mode == ModeSecure {
		candidates = []candidate{secure, abridged}
	}

	for _, c := range candidates {
		nonce := randBytes(16)
		handshake, encStream, decStream := buildObfuscated2(key, c.tag)
		framed := framePacket(reqPqPacket(nonce), c.tag)
		encFramed := make([]byte, len(framed))
		encStream.XORKeyStream(encFramed, framed)
		needle := concat([]byte{0x63, 0x24, 0x16, 0x05}, nonce) // resPQ + наш nonce
		ping, ok := exchange(ctx, host, port, concat(handshake, encFramed), timeout, func(out []byte) bool {
			return bytes.Contains(out, needle)
		}, decStream)
		if ok {
			return ping, "mtproto/" + c.name, true
		}
	}
	return 0, "", false
}
