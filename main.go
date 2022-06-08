package main

import (
    "time"
    "os"
    "fmt"
    "strconv"
)

func main() {
    args := os.Args[1:] // exclude program"s name

    if len(args) < 2 {
        fmt.Printf("Usage: %s <prng-seed> <csv-output> [<size1> <size2> ... <size-n>]\n", os.Args[0])
        fmt.Printf("\n")
        return
    }

    var seed int64 = 1337
    n, err := strconv.Atoi(args[0])
    if err != nil {
        fmt.Printf("Error parsing PRNG seed: %v\n", err)
    }
    seed = int64(n)
    csvFile := args[1]

    
    var sizes []int
    if len(args) > 2 {
        args = args[2:] // exclude PRNG seed and csv file
        sizes = make([]int, len(args))
        for i, arg := range args {
            n, err := strconv.Atoi(arg)
            if err != nil {
                fmt.Printf("Error parsing batch size: %v\n", err)
            }
            sizes[i] = n
        }
    } else {
        sizes = []int{100, 200, 300, 400, 500}
    }
    
    fmt.Printf("Sizes: %v, seed: %v\n", sizes, seed)

    t := time.Now()
    hashsparse(sizes, seed, csvFile)
    fmt.Printf("Took %v\n", time.Since(t))
}
