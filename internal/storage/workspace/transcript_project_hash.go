package workspace

import (
	"encoding/binary"
	"math/bits"
	"strconv"
)

const (
	transcriptWyhashSecret0 uint64 = 0xa0761d6478bd642f
	transcriptWyhashSecret1 uint64 = 0xe7037ed1a0b428db
	transcriptWyhashSecret2 uint64 = 0x8ebc6af09c88c6e3
	transcriptWyhashSecret3 uint64 = 0x589965cc75374cc3
)

func transcriptProjectHashSuffix(path string) string {
	return strconv.FormatUint(transcriptZigWyhash([]byte(path), 0), 36)
}

// transcriptZigWyhash 复现 Zig std.hash.Wyhash.hash(seed, input)，也就是 Bun.hash 的底层规则。
func transcriptZigWyhash(input []byte, seed uint64) uint64 {
	state0 := seed ^ transcriptWyhashMix(seed^transcriptWyhashSecret0, transcriptWyhashSecret1)
	state1 := state0
	state2 := state0
	var a uint64
	var b uint64

	if len(input) <= 16 {
		a, b = transcriptWyhashSmallKey(input)
	} else {
		index := 0
		if len(input) >= 48 {
			for index+48 < len(input) {
				state0 = transcriptWyhashMix(transcriptRead64(input[index:])^transcriptWyhashSecret1, transcriptRead64(input[index+8:])^state0)
				state1 = transcriptWyhashMix(transcriptRead64(input[index+16:])^transcriptWyhashSecret2, transcriptRead64(input[index+24:])^state1)
				state2 = transcriptWyhashMix(transcriptRead64(input[index+32:])^transcriptWyhashSecret3, transcriptRead64(input[index+40:])^state2)
				index += 48
			}
			state0 ^= state1 ^ state2
		}
		for offset := index; offset+16 < len(input); offset += 16 {
			state0 = transcriptWyhashMix(transcriptRead64(input[offset:])^transcriptWyhashSecret1, transcriptRead64(input[offset+8:])^state0)
		}
		a = transcriptRead64(input[len(input)-16:])
		b = transcriptRead64(input[len(input)-8:])
	}

	a ^= transcriptWyhashSecret1
	b ^= state0
	low, high := transcriptWyhashMum(a, b)
	return transcriptWyhashMix(low^transcriptWyhashSecret0^uint64(len(input)), high^transcriptWyhashSecret1)
}

func transcriptWyhashSmallKey(input []byte) (uint64, uint64) {
	if len(input) >= 4 {
		end := len(input) - 4
		quarter := (len(input) >> 3) << 2
		a := transcriptRead32(input) << 32
		a |= transcriptRead32(input[quarter:])
		b := transcriptRead32(input[end:]) << 32
		b |= transcriptRead32(input[end-quarter:])
		return a, b
	}
	if len(input) > 0 {
		a := uint64(input[0]) << 16
		a |= uint64(input[len(input)>>1]) << 8
		a |= uint64(input[len(input)-1])
		return a, 0
	}
	return 0, 0
}

func transcriptWyhashMix(a uint64, b uint64) uint64 {
	low, high := transcriptWyhashMum(a, b)
	return low ^ high
}

func transcriptWyhashMum(a uint64, b uint64) (uint64, uint64) {
	high, low := bits.Mul64(a, b)
	return low, high
}

func transcriptRead32(input []byte) uint64 {
	return uint64(binary.LittleEndian.Uint32(input[:4]))
}

func transcriptRead64(input []byte) uint64 {
	return binary.LittleEndian.Uint64(input[:8])
}
