// Package core 抽奖引擎。
// deterministic.go：确定性开奖引擎，字节级对齐 server_v2 的 Rust 引擎
// （F:\workspace\src\server_v2\app\engine\draw\src\lib.rs）。
//
// 随机源 = SHA256 拒绝采样（unbiased，域名 "luckybox-draw-v2"），
// 审计哈希 = SHA256（域名 "luckybox-audit-v2"），覆盖 seed/奖池/结果，
// 同一 (seed, pool, entries, count) 结果确定，可离线复算核对公平性。
package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// DeterministicEntry 引擎条目（对齐 Rust Entry { entry_id, weight, remaining }）。
// remaining 为 u32：谢谢参与/不限量由调用方以 0xFFFFFFFF（=int64 -1 直接转 u32）表示。
type DeterministicEntry struct {
	EntryID   uint64
	Weight    uint64
	Remaining uint32
}

// DeterministicOutcome 引擎结果（对齐 Rust Outcome { sequence, entry_id, random_counter }）。
type DeterministicOutcome struct {
	Sequence      uint32
	EntryID       uint64
	RandomCounter uint64
}

var (
	errDeterministicSeed     = errors.New("deterministic draw: seed must contain at least 32 bytes")
	errDeterministicCount    = errors.New("deterministic draw: count must be positive")
	errDeterministicDup      = errors.New("deterministic draw: duplicate entry id")
	errDeterministicOverflow = errors.New("deterministic draw: weight overflow")
	errDeterministicExhaust  = errors.New("deterministic draw: pool exhausted")
)

// DrawDeterministic 确定性开奖（count 抽），返回结果与审计哈希（hex）。
// 入参 entries 必须是「可抽取」条目（调用方负责排除售罄项）；seed 至少 32 字节。
func DrawDeterministic(entries []DeterministicEntry, seed []byte, poolID, poolVersion uint64,
	requestID string, count uint32, quantityWeighted bool) ([]DeterministicOutcome, string, error) {
	if len(seed) < 32 {
		return nil, "", errDeterministicSeed
	}
	if count == 0 {
		return nil, "", errDeterministicCount
	}
	seen := make(map[uint64]struct{}, len(entries))
	for _, e := range entries {
		if _, ok := seen[e.EntryID]; ok {
			return nil, "", fmt.Errorf("%w: %d", errDeterministicDup, e.EntryID)
		}
		seen[e.EntryID] = struct{}{}
	}

	// work 为抽取过程中的剩余库存副本；audit hash 用原始 entries（对齐 Rust req.entries）。
	work := make([]DeterministicEntry, len(entries))
	copy(work, entries)

	outcomes := make([]DeterministicOutcome, 0, count)
	var counter uint64
	for sequence := uint32(0); sequence < count; sequence++ {
		var total uint64
		for _, e := range work {
			w := selectionWeight(e, quantityWeighted)
			if total > ^uint64(0)-w {
				return nil, "", errDeterministicOverflow
			}
			total += w
		}
		if total == 0 {
			return nil, "", errDeterministicExhaust
		}
		ticket := unbiased(seed, poolID, poolVersion, requestID, &counter, total)

		chosen := -1
		var cumulative uint64
		for i := range work {
			cumulative += selectionWeight(work[i], quantityWeighted)
			if ticket < cumulative {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			return nil, "", errDeterministicExhaust
		}
		if work[chosen].Remaining > 0 {
			work[chosen].Remaining--
		}
		outcomes = append(outcomes, DeterministicOutcome{
			Sequence:      sequence,
			EntryID:       work[chosen].EntryID,
			RandomCounter: counter - 1,
		})
	}
	return outcomes, auditHash(entries, seed, poolID, poolVersion, requestID, count, quantityWeighted, outcomes), nil
}

// selectionWeight 对齐 Rust selection_weight：remaining==0 → 0；数量加权取 remaining，否则取 weight。
func selectionWeight(e DeterministicEntry, quantityWeighted bool) uint64 {
	if e.Remaining == 0 {
		return 0
	}
	if quantityWeighted {
		return uint64(e.Remaining)
	}
	return e.Weight
}

// unbiased 对齐 Rust unbiased：SHA256(域名‖seed‖pool_id‖version‖request_id_len‖request_id‖counter)
// 前 16 字节大端解释为 u128，拒绝采样 zone = u128::MAX - (u128::MAX % total)，返回 x % total。
func unbiased(seed []byte, poolID, version uint64, requestID string, counter *uint64, total uint64) uint64 {
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)) // 2^128 - 1
	totalBig := new(big.Int).SetUint64(total)
	zone := new(big.Int).Sub(max, new(big.Int).Mod(max, totalBig))

	var b8 [8]byte
	var b4 [4]byte
	for {
		h := sha256.New()
		h.Write([]byte("luckybox-draw-v2"))
		h.Write(seed)
		binary.BigEndian.PutUint64(b8[:], poolID)
		h.Write(b8[:])
		binary.BigEndian.PutUint64(b8[:], version)
		h.Write(b8[:])
		binary.BigEndian.PutUint32(b4[:], uint32(len(requestID)))
		h.Write(b4[:])
		h.Write([]byte(requestID))
		binary.BigEndian.PutUint64(b8[:], *counter)
		h.Write(b8[:])
		d := h.Sum(nil)
		x := new(big.Int).SetBytes(d[:16]) // u128 from first 16 bytes BE
		*counter++
		if x.Cmp(zone) < 0 {
			return new(big.Int).Mod(x, totalBig).Uint64()
		}
	}
}

// auditHash 对齐 Rust audit_hash：SHA256(域名‖quantity_weighted‖count‖pool_id‖pool_version‖
// request_id_len‖request_id‖SHA256(seed)‖entries‖outcomes)。
func auditHash(entries []DeterministicEntry, seed []byte, poolID, poolVersion uint64,
	requestID string, count uint32, quantityWeighted bool, outcomes []DeterministicOutcome) string {
	h := sha256.New()
	h.Write([]byte("luckybox-audit-v2"))

	// quantity_weighted: 1 字节
	if quantityWeighted {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}

	var b8 [8]byte
	var b4 [4]byte
	// count: u32 BE
	binary.BigEndian.PutUint32(b4[:], count)
	h.Write(b4[:])
	// pool_id: u64 BE
	binary.BigEndian.PutUint64(b8[:], poolID)
	h.Write(b8[:])
	// pool_version: u64 BE
	binary.BigEndian.PutUint64(b8[:], poolVersion)
	h.Write(b8[:])
	// request_id.len: u32 BE
	binary.BigEndian.PutUint32(b4[:], uint32(len(requestID)))
	h.Write(b4[:])
	// request_id bytes
	h.Write([]byte(requestID))
	// SHA256(seed)
	seedHash := sha256.Sum256(seed)
	h.Write(seedHash[:])

	// entries: entry_id BE8 ‖ weight BE8 ‖ remaining BE4
	for _, e := range entries {
		binary.BigEndian.PutUint64(b8[:], e.EntryID)
		h.Write(b8[:])
		binary.BigEndian.PutUint64(b8[:], e.Weight)
		h.Write(b8[:])
		binary.BigEndian.PutUint32(b4[:], e.Remaining)
		h.Write(b4[:])
	}
	// outcomes: sequence BE4 ‖ entry_id BE8 ‖ random_counter BE8
	for _, o := range outcomes {
		binary.BigEndian.PutUint32(b4[:], o.Sequence)
		h.Write(b4[:])
		binary.BigEndian.PutUint64(b8[:], o.EntryID)
		h.Write(b8[:])
		binary.BigEndian.PutUint64(b8[:], o.RandomCounter)
		h.Write(b8[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}
