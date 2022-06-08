package main

import (
    "fmt"
    "runtime"
    "crypto/sha256"
    "crypto/rand"
    "math/big"
    "encoding/hex"
)

func minInt(a, b int) int {
    if a < b {
        return a
    } else {
        return b
    }
}

func maxInt(a, b int) int {
    if a > b {
        return a
    } else {
        return b
    }
}

func randomHash() *big.Int {
    bytes := make([]byte, 32)
    rand.Read(bytes)
    hash := sha256.Sum256(bytes)
    bint := big.NewInt(0)
    bint.SetBytes(hash[:])
    return bint
}

func hashToInt(hash [32]byte) *big.Int {
    num := big.NewInt(0)

    num.SetBytes(hash[:])

    if num.Cmp(big.NewInt(0)) < 0 {
        panic("WTF, I need unsigned integers!")
    }
    return num
}

func bigIntStr(num *big.Int) string {
    bytes := num.Bytes()
    if len(bytes) == 0 {
        bytes = []byte{0x00}
    }
    return hex.EncodeToString(bytes)
}

func bigIntTo32Bytes(num *big.Int) [32]byte {
    slice := num.Bytes()
    length := len(slice)

    if length > 32 {
        panic(fmt.Sprintf("Per-level local node number is greater than 32 bytes: %d bytes", length))
    }
    var bytes [32]byte
    for i, _ := range bytes {
        bytes[i] = 0x00
    }

    j := 0
    for i := 32 - length; i < 32; i++ {
        bytes[i] = slice[j]
        j++
    }

    return bytes
}

func hashStr(hash [32]byte) string {
    return hex.EncodeToString(hash[:])
}

func memGc() {
    fmt.Printf("Memory before GC: %d MB\n", memUsage())
    fmt.Println("Garbage collecting...")
    runtime.GC()
    fmt.Printf("Memory after GC: %d MB\n", memUsage())
}

// Returns the memory usage in MB
func memUsage() uint64 {
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)
    return m1.Alloc / (1024*1024)
}
