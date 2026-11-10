package core

import (
	"crypto/sha256"
	"encoding/binary"
)

// FullCarRandom 确定性随机数：SHA256(seed‖domain‖BE32(index)) 前 8 字节大端。
// 与 play 服务 core.FullCarRandom 同款，用于活动转盘选槽的确定性派生
// （seed = 幂等键，同一参与幂等键必中同槽，可回放核对）。
func FullCarRandom(seed []byte, domain string, index uint32) uint64 {
	h := sha256.New()
	h.Write(seed)
	h.Write([]byte(domain))
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], index)
	h.Write(encoded[:])
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}
