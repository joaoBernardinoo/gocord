package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	recordSize = 4096
	headerLen  = 16 + 4 + 1 + 65 // salt (16) + rs (4) + idlen (1) + keyid (65) = 86
)

var (
	ErrInvalidKeyLength  = errors.New("invalid public key length: expected 65 bytes uncompressed P-256 key")
	ErrInvalidAuthLength = errors.New("invalid auth secret length: expected 16 bytes")
	ErrInvalidPayload    = errors.New("invalid or malformed encrypted push payload")
)

// EncryptPayload encrypts plaintext for a Web Push subscription according to RFC 8291 (aes128gcm).
func EncryptPayload(uaPubBytes, authSecret, plaintext []byte) ([]byte, error) {
	if len(uaPubBytes) != 65 || uaPubBytes[0] != 0x04 {
		return nil, ErrInvalidKeyLength
	}
	if len(authSecret) != 16 {
		return nil, ErrInvalidAuthLength
	}

	curve := ecdh.P256()
	uaPubKey, err := curve.NewPublicKey(uaPubBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient public key: %w", err)
	}

	asPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate sender key: %w", err)
	}
	asPub := asPriv.PublicKey().Bytes() // 65 bytes

	ecdhSecret, err := asPriv.ECDH(uaPubKey)
	if err != nil {
		return nil, fmt.Errorf("compute ecdh secret: %w", err)
	}

	// keyInfo = "WebPush: info\x00" || uaPub || asPub
	keyInfo := make([]byte, 0, 13+len(uaPubBytes)+len(asPub))
	keyInfo = append(keyInfo, []byte("WebPush: info\x00")...)
	keyInfo = append(keyInfo, uaPubBytes...)
	keyInfo = append(keyInfo, asPub...)

	prkKey := hkdfExtract(authSecret, ecdhSecret)
	ikm := hkdfExpand(prkKey, keyInfo, 32)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	prk := hkdfExtract(salt, ikm)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm cipher: %w", err)
	}

	// Record formatting: plaintext + delimiter 0x02 (final record)
	record := make([]byte, len(plaintext)+1)
	copy(record, plaintext)
	record[len(plaintext)] = 0x02

	ciphertext := gcm.Seal(nil, nonce, record, nil)

	// Build RFC 8188 aes128gcm header
	out := make([]byte, headerLen+len(ciphertext))
	copy(out[0:16], salt)
	binary.BigEndian.PutUint32(out[16:20], recordSize)
	out[20] = byte(len(asPub)) // 65
	copy(out[21:86], asPub)
	copy(out[86:], ciphertext)

	return out, nil
}

// DecryptPayload decrypts an RFC 8291 aes128gcm payload using the recipient's private key and auth secret.
// This is primarily used for testing and validating the encryption.
func DecryptPayload(uaPriv *ecdh.PrivateKey, authSecret, encrypted []byte) ([]byte, error) {
	if len(encrypted) < headerLen+17 { // 86 header + min 1 byte plaintext + 1 byte delimiter + 16 byte tag
		return nil, ErrInvalidPayload
	}
	salt := encrypted[0:16]
	rs := binary.BigEndian.Uint32(encrypted[16:20])
	if rs != recordSize && rs != 4096 {
		// Allow standard record sizes
	}
	idlen := int(encrypted[20])
	if idlen != 65 || len(encrypted) < 21+idlen+16 {
		return nil, ErrInvalidPayload
	}
	asPubBytes := encrypted[21 : 21+idlen]
	ciphertext := encrypted[21+idlen:]

	curve := ecdh.P256()
	asPubKey, err := curve.NewPublicKey(asPubBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid sender public key in header: %w", err)
	}

	uaPubBytes := uaPriv.PublicKey().Bytes()
	ecdhSecret, err := uaPriv.ECDH(asPubKey)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	keyInfo := make([]byte, 0, 13+len(uaPubBytes)+len(asPubBytes))
	keyInfo = append(keyInfo, []byte("WebPush: info\x00")...)
	keyInfo = append(keyInfo, uaPubBytes...)
	keyInfo = append(keyInfo, asPubBytes...)

	prkKey := hkdfExtract(authSecret, ecdhSecret)
	ikm := hkdfExpand(prkKey, keyInfo, 32)

	prk := hkdfExtract(salt, ikm)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm cipher: %w", err)
	}

	plaintextRecord, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm decrypt: %w", err)
	}
	if len(plaintextRecord) == 0 {
		return nil, ErrInvalidPayload
	}

	// Remove padding / delimiter: delimiter 0x02 or 0x01
	lastIdx := len(plaintextRecord) - 1
	for lastIdx >= 0 && plaintextRecord[lastIdx] == 0x00 {
		lastIdx--
	}
	if lastIdx < 0 || (plaintextRecord[lastIdx] != 0x02 && plaintextRecord[lastIdx] != 0x01) {
		return nil, ErrInvalidPayload
	}

	return plaintextRecord[:lastIdx], nil
}

func hkdfExtract(salt, ikm []byte) []byte {
	if salt == nil {
		salt = make([]byte, 32)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	return mac.Sum(nil)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	var okm []byte
	var t []byte
	var counter byte = 1
	for len(okm) < length {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{counter})
		t = mac.Sum(nil)
		okm = append(okm, t...)
		counter++
	}
	return okm[:length]
}
