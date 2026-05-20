# Bloom Filters: From Theory to Practice

> A comprehensive guide to Bloom filters with a clean, educational Go implementation.

A Bloom filter is a probabilistic data structure that answers a very specific question — *have I seen this thing before?* — while using almost no memory.

There is a trade-off, though: Bloom filters can be "wrong". A Bloom filter will **never** tell you something is absent when it is actually present, but it might occasionally claim something exists when it does not (a *false positive*).

In this guide, we explore Bloom filters end-to-end: from fundamental concepts and mathematics to advanced variants like counting and deletable Bloom filters, the nuances of hash functions, and real-world benchmarks and use cases.

## Table of Contents

- [Why Bloom Filters Matter](#why-bloom-filters-matter)
- [Our Go Implementation](#our-go-implementation)
- [The Fundamental Structure](#the-fundamental-structure)
- [Mathematics of False Positives](#mathematics-of-false-positives)
- [Hash Functions](#hash-functions)
  - [Cryptographic vs Non-Cryptographic Hashes](#cryptographic-vs-non-cryptographic-hashes)
  - [Double Hashing Optimization](#double-hashing-optimization)
- [Deletable Bloom Filters](#deletable-bloom-filters)
  - [Counting Bloom Filters](#counting-bloom-filters)
  - [Deletable Bloom Filter (DlBF)](#deletable-bloom-filter-dlbf)
- [Benchmarking Bloom Filters](#benchmarking-bloom-filters)
- [Bloom Filters in Databases](#bloom-filters-in-databases)
- [Use Cases](#use-cases)
- [Getting Started](#getting-started)

---

## Our Go Implementation

We provide a classic Bloom filter implementation in Go (`main.go`). It uses a `uint64` slice as the underlying bit array for efficient memory usage, and the fast, non-cryptographic `hash/fnv` package to map values.

```go
type BloomFilter struct {
	k    int        // Number of hash functions
	m    int        // Size of the bit array
	n    int        // Number of elements inserted
	data []uint64   // The bit array itself
}
```

### Adding an Element

To add an element to the filter, we iterate `k` times. For each iteration, we hash the element using the iteration index `i` as a seed (to simulate multiple independent hash functions), and then set the corresponding bit in our `data` slice.

```go
func (b *BloomFilter) add(id string) int {
	for i := 0; i < b.k; i++ {
		pos := b.hash(id, i)
		b.setBit(pos)
	}
	b.n++
	return b.n
}
```

### Checking for an Element

To check if an element is in the filter, we compute the same `k` hash positions. If any bit at those positions is `0` (`false`), we know for a fact the element was **never** added. If all are `1`, it is *probably* in the set (subject to false positives).

```go
func (b *BloomFilter) contains(id string) bool {
	for i := 0; i < b.k; i++ {
		pos := b.hash(id, i)
		if b.getBit(pos) == false {
			return false
		}
	}
	return true
}
```

---

## Why Bloom Filters Matter

Imagine you are building a recommendation engine for a content platform with millions of users. Each user has seen thousands of articles. When generating recommendations, you need to filter out articles the user has already seen.

Storing every article ID for every user in a hash set would consume enormous amounts of memory. Let's crunch the numbers...

**Assumptions:**

- 10 million active users
- Each user has seen an average of 5,000 articles
- Article IDs are 64-bit integers (8 bytes each)

**Storing article IDs directly in a hash set:**

Per user storage:
```
5,000 article IDs * 8 bytes = 40,000 bytes = 40 KB
```

Hash set overhead (typical 2× for load factor + pointers):
```
40 KB * 2 = 80 KB per user
```

Total for all users:
```
80 KB * 10,000,000 users = 800 GB
```

That is **800 GB of RAM** just for tracking which articles users have seen. Even with a more optimistic 1.5× overhead factor, you are looking at 600 GB.

A Bloom filter takes up roughly **60 GB** — about one-tenth of the space — by not storing actual article IDs, but rather storing existence in a compact bit array that can answer the question: *has this user probably seen this article?*

If the answer is **no**, you can confidently recommend it. If the answer is **yes**, you either skip it or do a more expensive lookup to confirm. In practice, this cuts memory usage by more than 90% without slowing down recommendation generation.

Variations of this idea show up in many large systems: Medium uses it in recommendations, Chrome uses it to flag unsafe URLs, and databases rely on Bloom filters to avoid unnecessary disk reads.

---

## The Fundamental Structure

A Bloom filter consists of two components:

1. A bit array of `m` bits, all initially set to zero
2. A collection of `k` independent hash functions

Each hash function maps an arbitrary input to one of the `m` array positions.

**To add an element to the filter:**

```python
def add(element):
    for i in 1 to k:
        position = hash_i(element) mod m
        bit_array[position] = 1
```

**To check if an element might exist in the filter:**

```python
def contains(element):
    for i in 1 to k:
        position = hash_i(element) mod m
        if bit_array[position] == 0:
            return false
    return true
```

If any bit at the computed positions is zero, the element was **definitely never added**. Those positions would have been set to one during insertion. However, if all bits are one, the element *might* have been added, or those positions might have been set by other elements. This is the source of false positives.

### A Concrete Example

Suppose you have a 10-bit array and two hash functions. You insert the strings `"apple"` and `"banana"`:

```
hash1("apple")  mod 10 = 2,   hash2("apple")  mod 10 = 5
hash1("banana") mod 10 = 3,   hash2("banana") mod 10 = 7
```

After these insertions, bits 2, 3, 5, and 7 are set to one. Now you check for `"cherry"`:

```
hash1("cherry") mod 10 = 2,   hash2("cherry") mod 10 = 3
```

Both positions are already one (set by previous insertions), so the filter incorrectly reports that `"cherry"` might be present. This is a **false positive**.

---

## Mathematics of False Positives

After inserting `n` elements using `k` hash functions into a filter with `m` bits, the probability that a specific bit remains zero is:

```
p₀ = (1 - 1/m)^(kn)
```

If you have `m` bits in your array and a hash function that outputs random positions, the probability of hitting any specific bit is `1/m`. Hence, the probability of **not** hitting a specific bit with one hash is `1 - 1/m`. The probability of that bit being zero with `n` insertions (each one setting `k` bits) will be `(1 - 1/m)^(kn)`.

For large `m`, this approximates to:

```
p₀ ≈ e^(-kn/m)
```

The probability of a false positive is the probability that all `k` bits are set for a random element not in the set:

```
p_fp = (1 - e^(-kn/m))^k
```

Here's an important trade-off: increasing `m` (more bits) reduces false positives but uses more memory. Increasing `k` (more hash functions) can either help or hurt depending on the relationship between `k` and `m/n`.

The optimal number of hash functions that minimizes the false positive rate is (via simple differentiation):

```
k_optimal = (m/n) * ln(2) ≈ 0.693 * (m/n)
```

At this optimal `k`, the false positive rate becomes:

```
p_fp ≈ (0.6185)^(m/n)
```

Working backwards, if you want a specific false positive rate `p`, you need approximately:

```
m/n = -ln(p) / (ln(2))² ≈ -1.44 * ln(p)
```

For a 1% false positive rate, you need about **9.6 bits per element**. For 0.1%, you need about **14.4 bits per element**.

Compare this to storing 1 million 64-bit integers directly, which would require 8 MB. The Bloom filter achieves roughly **7× space savings** while providing probabilistic membership testing.

---

## Hash Functions

Bloom filters require hash functions that are **fast**, produce **uniformly distributed** outputs, and behave **independently** of each other.

### Cryptographic vs Non-Cryptographic Hashes

Cryptographic hash functions, such as SHA-1 or MD5, offer good distribution properties and security guarantees; however, they are computationally expensive. Since Bloom filters do not require cryptographic security, using them wastes CPU cycles.

**Non-cryptographic hash functions** are the standard choice:

| Algorithm | Description |
|-----------|-------------|
| **MurmurHash3** | Widely considered the best general-purpose non-cryptographic hash. Excellent distribution, handles all input sizes well, and is very fast. Supports both 32-bit and 128-bit outputs. |
| **xxHash** | Extremely fast, especially on modern CPUs with SIMD support. The `xxh3` variant used in RocksDB is particularly optimized for short keys. |
| **FNV (Fowler–Noll–Vo)** | Simple to implement and performs well on short strings (under 20 characters). The FNV-1a variant is slightly better than FNV-1 for most use cases. |

### Double Hashing Optimization

A seminal paper by Kirsch and Mitzenmacher showed that you do not actually need `k` independent hash functions. Instead, you can compute just two hash functions and derive all `k` positions using a linear combination:

```
g_i(x) = h₁(x) + i * h₂(x)    for i = 0, 1, ..., k-1
```

This is called **double hashing** (or the *Kirsch–Mitzenmacher optimization*), and it reduces computational overhead dramatically while maintaining the same asymptotic false positive rate. Most production Bloom filter implementations use this approach.

```python
def compute_positions(element, k, m, hash1, hash2):
    """
    Compute k bit positions using double hashing.
    """
    h1 = hash1(element)
    h2 = hash2(element)
    
    positions = []
    for i in range(k):
        pos = (h1 + i * h2) % m
        positions.append(pos)
    
    return positions
```

A subtle but important implementation detail: if `h₂` can be zero or share a common factor with `m`, you may get degenerate cases where multiple positions collide. The **enhanced double hashing** variant addresses this by ensuring `h₂` is always odd (when `m` is a power of two) or relatively prime to `m`:

```python
def compute_positions_enhanced(element, k, m, hash_func):
    """
    Enhanced double hashing that avoids degenerate cases.
    """
    h = hash_func(element)
    h1 = h & 0xFFFFFFFF       # Lower 32 bits
    h2 = (h >> 32) | 1        # Upper 32 bits, ensure odd
    
    positions = []
    for i in range(k):
        pos = (h1 + i * h2) % m
        positions.append(pos)
    
    return positions
```

RocksDB discovered this issue in their Bloom filter implementation and fixed it by ensuring the delta value is always odd, which guarantees distinct positions when `m` is a power of two.

---

## Deletable Bloom Filters

Standard Bloom filters have a significant limitation: **you cannot remove elements**. Once a bit is set to one, there is no way to know if it was set by one element or multiple elements. Unsetting it could cause false negatives for other elements that share that bit position.

Several variants address this limitation.

### Counting Bloom Filters

The most straightforward approach replaces each bit with a small counter (typically 4 bits). Instead of setting bits to one, you increment counters. To delete an element, you decrement the counters at the corresponding positions.

```python
class CountingBloomFilter:
    def __init__(self, m, k, counter_bits=4):
        self.m = m
        self.k = k
        self.max_count = (1 << counter_bits) - 1
        self.counters = [0] * m
    
    def add(self, element):
        for pos in self._get_positions(element):
            if self.counters[pos] < self.max_count:
                self.counters[pos] += 1
    
    def remove(self, element):
        for pos in self._get_positions(element):
            if self.counters[pos] > 0:
                self.counters[pos] -= 1
    
    def contains(self, element):
        return all(self.counters[pos] > 0 
                   for pos in self._get_positions(element))
```

The trade-off is significant: using 4-bit counters means the filter uses **4× more memory** than a standard Bloom filter. Counter overflow is another concern. If too many elements hash to the same position, the counter saturates. Most implementations handle this by leaving saturated counters at maximum and never decrementing them, accepting a slight increase in false positives.

### Deletable Bloom Filter (DlBF)

The Deletable Bloom filter takes a different approach. Instead of counting, it splits the entire Bloom filter array into `r` logical regions, then tracks which bit regions have experienced collisions (in at least one of the bits), and maintains a separate collision bitmap.

**When inserting an element:**

1. Compute the `k` bit positions
2. For each position, if the bit is already set, mark that region as having a collision
3. Set the bits

**When deleting an element:**

1. Check if any of the element's bits are in collision-free regions
2. If at least one bit is in a collision-free region, that bit can be safely unset
3. If all bits are in collision regions, the element cannot be deleted without risking false negatives

The DlBF provides **probabilistic deletability**: most elements can be deleted, but some cannot. The advantage is a much lower memory overhead compared to counting filters.

---

## Benchmarking Bloom Filters

When evaluating Bloom filter implementations, several metrics matter. Most of these benchmarks can be found in my [**abloom**](https://github.com/idityage/abloom) repository.

### Insertion Throughput

Measures how many elements per second can be added to the filter. This depends primarily on:

- Hash function speed
- Number of hash functions

A Bloom filter should achieve hundreds of thousands to millions of insertions per second. Redis benchmarks show around 250,000 insertions per second with pipelining.

### Lookup Throughput

Typically similar to or faster than insertion, since lookups can short-circuit on the first zero bit. The double hashing optimization particularly benefits lookups by reducing hash computations.

### Actual False Positive Rate

The theoretical false positive rate assumes perfect hash functions. Real implementations should be benchmarked to verify.

```python
def benchmark_fp_rate(bf, test_size=100000):
    """
    Benchmark actual false positive rate.
    """
    # Add known elements
    known_elements = [f"element_{i}" for i in range(test_size)]
    for elem in known_elements:
        bf.add(elem)
    
    # Test with elements definitely not in the set
    false_positives = 0
    test_elements = [f"test_{i}" for i in range(test_size)]
    for elem in test_elements:
        if bf.contains(elem):
            false_positives += 1
    
    return false_positives / test_size
```

---

## Bloom Filters in Databases

Bloom filters are common in modern databases, particularly those using **Log-Structured Merge (LSM) trees**.

### LSM Trees

Databases like **RocksDB**, **Cassandra**, **LevelDB**, and **HBase** all use Bloom filters to optimize read paths. In an LSM tree:

1. Writes go to an in-memory MemTable
2. When full, the MemTable flushes to an immutable SSTable on disk
3. Reads must potentially check multiple SSTables to find a key

Without Bloom filters, every key lookup might require reading multiple SSTables from disk. With Bloom filters attached to each SSTable, the database can quickly determine if a key is **definitely not present**, avoiding expensive disk reads.

Cassandra exposes this as a tunable parameter. The `bloom_filter_fp_chance` setting controls the false positive rate per SSTable:

```sql
CREATE TABLE my_table (
    ...
) WITH bloom_filter_fp_chance = 0.01;
```

- **Lower values** (like 0.01) use more memory but reduce unnecessary disk reads
- **Higher values** (like 0.1) save memory at the cost of more false-positive disk accesses

---

## Use Cases

### Content Deduplication

Checking if content has been seen before is a classic Bloom filter application. Examples include:

- **Recommendation systems** filter previously shown items
- **Web crawlers** track visited URLs

The key insight is that false positives (occasionally re-checking something already processed) are acceptable, while false negatives (missing something new) are not.

### Cache Systems

CDNs use Bloom filters to track which objects are cached at edge nodes. Akamai pioneered the technique of using Bloom filters to avoid caching "one hit wonders" (objects accessed only once). By only caching objects seen at least twice, they dramatically improved cache efficiency.

**The pattern:**

1. **First request:** Add to Bloom filter, serve from origin
2. **Subsequent requests:** Check Bloom filter, cache if present

### Database Acceleration

#### PostgreSQL Bloom Indexes

Imagine a table with 10 columns and queries that filter on various combinations:

```sql
SELECT * FROM products WHERE color = 'red' AND size = 'large' AND brand = 'nike';
SELECT * FROM products WHERE category = 'shoes' AND price_range = 'mid';
SELECT * FROM products WHERE color = 'blue' AND category = 'shirts' AND material = 'cotton';
```

The traditional approach is to create B-tree indexes. But which combinations do you index?

```sql
CREATE INDEX idx1 ON products(color, size, brand);
CREATE INDEX idx2 ON products(category, price_range);
CREATE INDEX idx3 ON products(color, category, material);
-- And so on for every possible combination...
```

This explodes quickly. With 10 columns and arbitrary query patterns, you might need dozens of indexes, each consuming significant disk space and slowing down writes.

**PostgreSQL's Bloom index** creates a single compact index that encodes all specified columns into one Bloom filter per row (or per page):

```sql
CREATE EXTENSION bloom;
CREATE INDEX products_bloom ON products USING bloom (color, size, brand, category, price_range, material);
```

For each row:

1. Hash each column value and set bits in a small Bloom filter (say, 80 bits per row)
2. Store these mini Bloom filters in the index

At query time:

```sql
SELECT * FROM products WHERE color = 'red' AND size = 'large';
```

1. Hash `'red'` and `'large'` to get the expected bit positions
2. Scan the Bloom index, checking each row's filter
3. If any expected bit is 0, the row **definitely does not match** — skip it
4. If all bits are 1, the row **might match** — fetch and verify it

### Spark Broadcast Joins

You are joining a large table (1 billion rows, 500 GB) with a small table (100,000 rows, 10 MB):

```python
large_df = spark.read.parquet("events")  # 1B rows across 1000 partitions
small_df = spark.read.parquet("users")   # 100K rows

result = large_df.join(small_df, "user_id")
```

Without optimization, Spark would broadcast the small table to all nodes:

- Send the entire small table (10 MB) to every executor
- Each executor joins its partition of the large table locally
- No shuffle of the large table needed

But what if only 5% of rows in the large table actually have matching `user_id`s? You are still reading and processing 1 billion rows.

**Bloom filter optimization:**

```python
from pyspark.sql.functions import broadcast

# Spark builds a Bloom filter from small_df's user_id column
# and broadcasts it (just a few KB) to all executors

result = large_df.join(broadcast(small_df), "user_id")
```

What happens:

1. Build a Bloom filter containing all 100,000 `user_id`s from the small table (maybe 200 KB)
2. Broadcast this tiny Bloom filter to all nodes
3. Each executor filters its partition of the large table
4. Now only ~5% of rows (50 million instead of 1 billion) participate in the join

---

## Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.22 or later

### Running the Example

```bash
# Clone the repository
git clone https://github.com/idityage/bloom-filter.git
cd bloom-filter

# Run the example
go run main.go
```

**Expected output:**

```
Adding users...
Added: idityage
Added: idityaa
Added: its_adii
Added: aditya

Testing inclusion (Should be true):
Contains 'idityage': true
Contains 'idityaa': true
Contains 'its_adii': true
Contains 'aditya': true

Testing non-existent users (Should be mostly false, but false positives are possible):
Contains 'eve_hacker': false
Contains 'frank_sinatra': false
Contains 'grace_hopper': false
Contains 'alice124': false
Contains 'iditya': false
```

### Using the Bloom Filter in Your Code

```go
package main

import "fmt"

func main() {
	// Create a Bloom filter with 3 hash functions and 200 bits
	bf := NewBloomFilter(3, 200)

	// Add elements
	bf.add("hello")
	bf.add("world")

	// Check membership
	fmt.Println(bf.contains("hello"))  // true
	fmt.Println(bf.contains("world"))  // true
	fmt.Println(bf.contains("unknown")) // false (probably)
}
```
