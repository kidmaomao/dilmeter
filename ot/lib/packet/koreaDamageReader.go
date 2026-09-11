package packet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math"
	"sync/atomic"
)

var block = atomic.Value{}
var iv = atomic.Value{}

func init() {
	keyStr := "0000000000000000000000000000000000000000000000000000000000000000"
	ivStr := "00000000000000000000000000000000"

	InitKoreaDamageBlock(keyStr, ivStr)
}

func InitKoreaDamageBlock(keyStr string, ivStr string) error {
	_key, err := hex.DecodeString(keyStr)
	if err != nil {
		return fmt.Errorf("key decode failed: %w", err)
	}

	_iv, err := hex.DecodeString(ivStr)
	if err != nil {
		return fmt.Errorf("iv decode failed: %w", err)
	}

	if len(_key) != 32 {
		return fmt.Errorf("key length must be 32 bytes (64 hex chars)")
	}

	if len(_iv) != 16 {
		return fmt.Errorf("iv length must be 16 bytes (32 hex chars)")
	}

	_block, err := aes.NewCipher(_key)
	if err != nil {
		return err
	}

	block.Store(_block)
	iv.Store(_iv)
	return nil
}

func koreaDamageDecrypt(enc []byte) (float32, float32, uint32, uint32) {
	_ = enc[15] // check length

	_block := block.Load().(cipher.Block)
	_iv := iv.Load().([]byte)

	dec := make([]byte, len(enc))
	blockN := _block.BlockSize()

	for i := 0; i < len(enc); i += blockN {
		_block.Decrypt(dec[i:i+blockN], enc[i:i+blockN])
	}

	subtle.XORBytes(dec[:blockN], dec[:blockN], _iv)
	subtle.XORBytes(dec[blockN:], dec[blockN:], enc[:len(enc)-blockN])

	safeFloat32 := func(v float32) float32 {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0
		}

		// hard limit
		if v < 0 || v > 1e15 {
			return 0
		}

		return v
	}

	damage := safeFloat32(math.Float32frombits(le.Uint32(dec[0:])))
	wound := safeFloat32(math.Float32frombits(le.Uint32(dec[4:])))
	manaDamage := le.Uint32(dec[8:])
	unk1 := le.Uint32(dec[12:])

	return damage, wound, manaDamage, unk1
}
