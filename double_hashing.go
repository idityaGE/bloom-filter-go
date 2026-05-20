package main

import (
	"hash/fnv"
	"math"
)

type DoubleHashingBloomFilter struct {
	k    int
	m    int
	n    int
	data []uint64
}

func NewDoubleHashingBloomFilter(k, m int) *DoubleHashingBloomFilter {
	size := math.Ceil(float64(m) / 64)
	return &DoubleHashingBloomFilter{
		k:    k,
		m:    m,
		n:    0,
		data: make([]uint64, int(size)),
	}
}

func (b *DoubleHashingBloomFilter) add(id string) int {
	h1 := b.hash1(id)
	h2 := b.hash2(id)
	for i := range b.k {
		pos := (h1 + i*h2) % b.m
		b.setBit(pos)
	}
	b.n++
	return b.n
}

func (b *DoubleHashingBloomFilter) contains(id string) bool {
	h1 := b.hash1(id)
	h2 := b.hash2(id)
	for i := range b.k {
		pos := (h1 + i*h2) % b.m
		if b.getBit(pos) == false {
			return false
		}
	}
	return true
}

func (b *DoubleHashingBloomFilter) hash1(id string) int {
	h := fnv.New64a()
	h.Write([]byte(id))
	return int(h.Sum64() % uint64(b.m))
}

func (b *DoubleHashingBloomFilter) hash2(id string) int {
	h := fnv.New64a()
	h.Write([]byte{0xFF})
	h.Write([]byte(id))
	return int(h.Sum64() % uint64(b.m))
}

func (b *DoubleHashingBloomFilter) setBit(i int) {
	b.data[i/64] |= (1 << (i % 64))
}

func (b *DoubleHashingBloomFilter) getBit(i int) bool {
	return (b.data[i/64] & (1 << (i % 64))) != 0
}