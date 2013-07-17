// Copyright 2012 Apcera, Inc. All rights reserved.

package murmur3

import "hash"

// The default seed provided by the algorithm.
const DefaultSeed = uint32(0x9747b28c)

// Constant defined by the Murmur3 algorithm
const C1 = uint32(0xcc9e2d51)
const C2 = uint32(0x1b873593)

// Inner loop of the murmur3 hash.
func inner32(h1 uint32, data []byte, offset int) uint32 {
	k1 := uint32(data[offset])
	k1 |= uint32(data[offset+1]) << 8
	k1 |= uint32(data[offset+2]) << 16
	k1 |= uint32(data[offset+3]) << 24

	k1 *= C1
	k1 = (k1 << 15) | (k1 >> 17)
	k1 *= C2

	h1 ^= k1
	h1 = (h1 << 13) | (h1 >> 19)
	return h1*5 + 0xe6546b64
}

// Tail function of the murmur3 hash.
func tail32(h1 uint32, data []byte, offset, length int) uint32 {
	var k1 uint32
	switch length {
	case 3:
		k1 |= uint32(data[offset+2]) << 16
		fallthrough
	case 2:
		k1 |= uint32(data[offset+1]) << 8
		fallthrough
	case 1:
		k1 |= uint32(data[offset])
		k1 *= C1
		k1 = (k1 << 15) | (k1 >> 17)
		k1 *= C2
		h1 ^= k1
	}
	return h1
}

// Finalization function of the murmur3 hash.
func finalization32(h1 uint32, length int) uint32 {
	h1 ^= uint32(length)

	h1 ^= (h1 >> 16)
	h1 *= uint32(0x85ebca6b)
	h1 ^= (h1 >> 13)
	h1 *= uint32(0xc2b2ae35)
	h1 ^= (h1 >> 16)
	return h1
}

// Used to implement hash.Hash.
type sum32 struct {
	h1         uint32
	seed       uint32
	length     int
	buffer_len int
	buffer     [4]byte
}

// Creates a new 32 bit Murmur3 hash object.
func New32() hash.Hash32 {
	s := sum32{}
	s.h1 = DefaultSeed
	s.seed = DefaultSeed
	return &s
}

// Creates a new 32 bit Murmur3 hash object with the given seed.
func New32Seed(seed uint32) hash.Hash32 {
	s := sum32{}
	s.h1 = seed
	s.seed = seed
	return &s
}

// Resets the hash generator back to the initial state.
func (s *sum32) Reset() { s.h1 = s.seed }

// Returns a 32 bit integer for the data already written rather than
// a byte array like Sum()
func (s *sum32) Sum32() uint32 {
	h1 := tail32(s.h1, s.buffer[:], 0, s.buffer_len)
	h1 = finalization32(h1, s.length)
	return h1
}

func (s *sum32) Write(data []byte) (int, error) {
	s.length += len(data)
	i := 0
	switch s.buffer_len {
	case 3:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
		fallthrough
	case 2:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
		fallthrough
	case 1:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
		s.h1 = inner32(s.h1, s.buffer[:], 0)
		s.buffer_len = 0
	}

	end := len(data) - (len(data) % 4)
	for ; i < end; i += 4 {
		s.h1 = inner32(s.h1, data, i)
	}

	s.buffer_len = 0
	switch len(data) - i {
	case 3:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
		fallthrough
	case 2:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
		fallthrough
	case 1:
		s.buffer[s.buffer_len] = data[i]
		s.buffer_len++
		i++
	}

	return len(data), nil
}

// Takes a byte array, or nil and writes the resulting hash to it.
func (s *sum32) Sum(in []byte) []byte {
	h1 := tail32(s.h1, s.buffer[:], 0, s.buffer_len)
	h1 = finalization32(h1, s.length)
	in = append(in, byte(h1>>24))
	in = append(in, byte(h1>>16))
	in = append(in, byte(h1>>8))
	in = append(in, byte(h1))
	return in
}

// Returns the size of the resulting output.
func (s *sum32) Size() int {
	return 4
}

// Returns the size of the blocks that the hash function reads at a time.
func (s *sum32) BlockSize() int {
	return 4
}

// Generates a Murmur3 hash from the given data without generating any
// intermediate objects.
func Hash(data []byte, seed uint32) uint32 {
	h1 := seed
	i := 0
	end := len(data) - (len(data) % 4)

	for ; i < end; i += 4 {
		h1 = inner32(h1, data, i)
	}

	h1 = tail32(h1, data, i, len(data)-i)
	h1 = finalization32(h1, len(data))
	return h1
}
