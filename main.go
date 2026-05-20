package main

import (
	"fmt"
	"hash/fnv"
	"math"
)

type BloomFilterI interface {
	add(id string) int
	contains(id string) bool
	hash(id string, seed int) int
}

type BloomFilter struct {
	k    int
	m    int
	n    int
	data []uint64
}

func NewBloomFilter(k, m int) BloomFilterI {
	// Size is (m + 63) / 64 to round up to the nearest uint64
	size := math.Ceil(float64(m) / 64)
	return &BloomFilter{
		k:    k,
		m:    m,
		n:    0,
		data: make([]uint64, int(size)),
	}
}

func (b *BloomFilter) add(id string) int {
	for i := range b.k {
		pos := b.hash(id, i)
		b.setBit(pos)
	}
	b.n++
	return b.n
}

func (b *BloomFilter) contains(id string) bool {
	for i := range b.k {
		pos := b.hash(id, i)
		if b.getBit(pos) == false {
			return false
		}
	}
	return true
}

func (b *BloomFilter) hash(id string, seed int) int {
	h := fnv.New64a()           // non-cryptographic hash function
	h.Write([]byte{byte(seed)}) // seed differentiates the k hashes
	h.Write([]byte(id))
	return int(h.Sum64() % uint64(b.m))
}

func (b *BloomFilter) setBit(i int) {
	b.data[i/64] |= (1 << (i % 64))
}

func (b *BloomFilter) getBit(i int) bool {
	return (b.data[i/64] & (1 << (i % 64))) != 0
}

func main() {
	bf := NewBloomFilter(3, 200)

	usernames := []string{"idityage", "idityaa", "its_adii", "aditya"}

	fmt.Println("Adding users...")
	for _, user := range usernames {
		bf.add(user)
		fmt.Printf("Added: %s\n", user)
	}

	fmt.Println("\nTesting inclusion (Should be true):")
	for _, user := range usernames {
		fmt.Printf("Contains '%s': %t\n", user, bf.contains(user))
	}

	fmt.Println("\nTesting non-existent users (Should be mostly false, but false positives are possible):")
	fakeUsers := []string{"eve_hacker", "frank_sinatra", "grace_hopper", "alice124", "iditya"}
	for _, user := range fakeUsers {
		fmt.Printf("Contains '%s': %t\n", user, bf.contains(user))
	}
}
