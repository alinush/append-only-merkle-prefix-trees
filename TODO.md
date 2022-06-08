[DONE] 65536 hashes, on average, each hash matched 11,000 intervals.

[DONE] Try sorting batches of 1024, see what happens
    Each random hash matched in pretty much every sorted batch

[DONE] Test if byte-keyed maps work

    func testByteKeyedMap() {
        var max *big.Int = new(big.Int)
        var m = make(map[[32]]byte]bool)
    
        max.Exp(big.NewInt(2), big.NewInt(256), nil)
        var i *big.Int;
        i, _ = rand.Int(rand.Reader, max)
        var h1 [32]byte = sha256.Sum256(i.Bytes())
    
        fmt.Printf("Adding h1 @ %p to map\n", &h1)
        m[h1] = true
    
        var h2 [32]byte = sha256.Sum256(i.Bytes())
        fmt.Printf("Checking for the value of h1 using h2 @ %p\n", &h2)
        fmt.Printf("Found: %v\n", m[h2])
    
        j, _ := rand.Int(rand.Reader, max)
        var h3 [32]byte = sha256.Sum256(j.Bytes())
        fmt.Printf("Found: %v\n", m[h3])
    }

Running out of memory on 46GB machine when running 32 batches of size 2^16 =
65536 items each:

    Total items: 851968, tree size: 202266106, proof size: 333217 (old size: 457046, # empty: 59921), insert time: 1m59.955766848s, proof verify time: 2m48.699047044s
    fatal error: runtime: out of memory

    goroutine 1 [running]:
    runtime.throw(0x5e09f7)
        /usr/lib/go/src/pkg/runtime/panic.c:464 +0x69 fp=0x7fe0f8114500
    runtime.SysMap(0xcdb4f40000, 0x5400000, 0x5ede18)
        /usr/lib/go/src/pkg/runtime/mem_linux.c:131 +0xfe fp=0x7fe0f8114530
    runtime.MHeap_SysAlloc(0x5f7d60, 0x5400000)
        /usr/lib/go/src/pkg/runtime/malloc.goc:473 +0x10a fp=0x7fe0f8114570
    MHeap_Grow(0x5f7d60, 0x5400)
        /usr/lib/go/src/pkg/runtime/mheap.c:241 +0x5d fp=0x7fe0f81145b0
    MHeap_AllocLocked(0x5f7d60, 0x5400, 0x0)
        /usr/lib/go/src/pkg/runtime/mheap.c:126 +0x305 fp=0x7fe0f81145f0
    runtime.MHeap_Alloc(0x5f7d60, 0x5400, 0x100000000, 0x1)
        /usr/lib/go/src/pkg/runtime/mheap.c:95 +0x7b fp=0x7fe0f8114618
    runtime.mallocgc(0x5400000, 0x4ce381, 0x0)
        /usr/lib/go/src/pkg/runtime/malloc.goc:89 +0x484 fp=0x7fe0f8114688
    cnew(0x4ce380, 0x40000, 0x1)
        /usr/lib/go/src/pkg/runtime/malloc.goc:718 +0xc1 fp=0x7fe0f81146a8
    runtime.cnewarray(0x4ce380, 0x40000)
        /usr/lib/go/src/pkg/runtime/malloc.goc:731 +0x3a fp=0x7fe0f81146c8
    hash_grow(0x4a98a0, 0xc210038960)
        /usr/lib/go/src/pkg/runtime/hashmap.c:448 +0x71 fp=0x7fe0f81146f8
    hash_insert(0x4a98a0, 0xc210038960, 0x7fe0f8114800, 0x7fe0f8114820)
        /usr/lib/go/src/pkg/runtime/hashmap.c:647 +0x3a9 fp=0x7fe0f8114790
    runtime.mapassign(0x4a98a0, 0xc210038960, 0x7fe0f8114800, 0x7fe0f8114820)
        /usr/lib/go/src/pkg/runtime/hashmap.c:1114 +0x88 fp=0x7fe0f81147b8
    runtime.mapassign1(0x4a98a0, 0xc210038960, 0x0, 0x0, 0x0, ...)
        /usr/lib/go/src/pkg/runtime/hashmap.c:1146 +0x80 fp=0x7fe0f81147f0
    main.func·002(0xc21001e3e0, 0xc924427720, 0xc924427760, 0xc924427701)
        /home/ubuntu/repos/hashperiments/sparse.go:233 +0x1a4 fp=0x7fe0f81148f8
    main.(*Tree)._iteratePath(0xc21004ef60, 0x6ede46f2c0bbf29, 0xbf974bcd569545e, 0x1047bd5b4631c898, 0x6ea9b68e2791154a, ...)
        /home/ubuntu/repos/hashperiments/sparse.go:171 +0x224 fp=0x7fe0f8114988
    main.(*Tree).Insert(0xc21004ef60, 0x6ede46f2c0bbf29, 0xbf974bcd569545e, 0x1047bd5b4631c898, 0x6ea9b68e2791154a, ...)
        /home/ubuntu/repos/hashperiments/sparse.go:252 +0x164 fp=0x7fe0f8114a60
    main.hashsparse(0x20, 0x10000, 0x539)
        /home/ubuntu/repos/hashperiments/sparse.go:565 +0x578 fp=0x7fe0f8114dd0
    main.main()
        /home/ubuntu/repos/hashperiments/main.go:76 +0xb24 fp=0x7fe0f8114f48
    runtime.main()

[DONE] Consistency proofs
-------------------------

How to compute consistency proofs while inserting a batch in a sparse Merkle
Tree? We are given the current `tree` and an array of `newLeafs`. We can store
the consistency proof in a new `proofTree`.

Note that the consistency proof tree is really split in two parts: (1) the proof
tree that hashes to the old root node, (2) the proof tree that hashes to the new
root node. Nodes are included in the 1st and 2nd tree using `includeExisting`.
New nodes are included in the 2nd tree only using `includeNew`. The `include*`
calls will ignore the node if it's already been included.

    // 'empty' -> node had no data before insert and no data after (unaffected)
    // 'new' -> node was 'empty' before insert and now has data (affected)
    // 'set' -> node had data before insert (could have been affected/unaffected
    //          by insert

    // TODO: think of consistency proof between version 0 and version 1
    // TODO: check that proof tree doesn't ever contain two sibling nodes set
    //       unless one node is 'set' and one is 'new' (i.e. was 'empty' before 
    //       insertion and 'set' after)
    // TODO: must be able to exclude nodes, like when a node's children are 
    //       included
    

    computeProof(tree, newLeafs):
        for l in newLeafs:
            // sibling can be either 'set', 'empty' or 'new'
            // INVARIANT: ancestor is always an ancestor of the leaf l
            // INVARIANT: sibling and ancestor are always sibling nodes after
            // assignment
            ancestor = l
            sibling = getSibling(l)

            while isNotSet(sibling):
                // ancestor will be either 'set' (because 'set' sibling implies its
                // parent is 'set') or 'empty' (because 'empty' or 'new' sibling 
                // implies parent is 'empty')
                ancestor = getParent(ancestor)
                // sibling will be either 'set' or 'empty' or 'new'
                sibling = getSibling(ancestor)

            // INVARIANT: sibling is 'set' => ancestor is 'new'. Why? (1) Ancestor
            // cannot be 'empty' because its descendant leaf is 'new' => ancestor
            // is at least 'new'.
            // (2) If ancestor were 'set', then that implies one of its two
            // children in the previous iteration (i.e. sibling and ancestor) was
            // 'set'. If the previous sibling was 'set' then we would have found it
            // in an earlier iteration and not in this one. If the previous
            // ancestor were 'set' we can recursively argue that it implies either
            // a previous sibling was set or the leaf was 'set' which cannot happen
            // because the leaf was 'new'

            includeExisting(sibling)    // sibling is 'set'
            // this is always a 'new' node (will be 'set' in next tree)
            includeNew(ancestor)

            // now we need to include the uncle nodes along the path to the root
            parent = getParent(ancestor)
            while isNotRoot(parent):
                sibling = getSibling(parent)
                // this will either be 'set' or 'new' or 'empty'
                includeExisting(sibling)        

                parent = getParent(parent)

We can also write it as an iterator algorithm:

    var foundSetNode = false
    var proofTree = ...

    tree._iteratePath(leafNo, func(lvl *TreeLevel, ancestorNo *big.Int,
        siblingNo *big.Int) {
        // Nothing to do for level 0
        if lvl.num == 0 {
            return
        }

        if !foundSetNode {
            sibling = getNode(lvl, siblingNo)

            if isSet(sibling) {
                includeExisting(lvl.num, sibling, siblingNo)
                includeNew(lvl.num, getNode(lvl, ancestorNo), ancestorNo)
                foundSetNode = true
            }
        } else {
            includeExisting(lvl.num, sibling, siblingNo)
        }
    })
    
    // TODO: include root node in proofTree as well? or on the side?
    
    isSet = func(node *Node) bool {
        return node != nil && node.IsNew == false
    }

    include = func(level int, node *Node, nodeNo *big.Int, isNew bool) {
        lvlNodes := proofTree.lvl[level].node
        idx := bigIntTo32Bytes(nodeNo)

        if prevNode, ok := lvlNodes[idx]; !ok {
            lvlNodes[idx] = &Node{Hash: node.Hash, IsNew: isNew}
        } else {
            if prevNode.IsNew != isNew {
                panic(fmt.Sprintf("Expected prevNode.IsNew to be '%v', not '%v'", 
                    isNew, prevNode.IsNew))
            }
        }
    }
    includeExisting = func(level int, node *Node, nodeNo *big.Int) {
        return include(level, node, nodeNo, false)
    }
    includeNew = func(level int, node *Node, nodeNo *big.Int) {
        return include(level, node, nodeNo, true)
    }

The problem with this algorithm so far is that it will include extra nodes along
a path. So we can either change the algorithm to filter out extra nodes along a
path (could be hard because when we include an uncle/sibling node we would have
to be able to tell if there is anything below or above it on the path: above is
easy, but below is hard), or we can do an additional compression step at the
end, where, for each leaf in the `proofTree` (including internal nodes in the
sparse Merkle Tree), we go up its path and exclude any nodes included)

    compressProof(proofTree):
        for l in getLeafs(proofTree):
            parent = getParent(l)
            while isNotRoot(parent):
                if isIncluded(parent):
                    exclude(parent)
                parent = getParent(parent)

Finally, we would like to be able to hash a `proofTree` and compare it to the
root hash. We can either hash the old tree or the new tree, so we need a
`version` parameter.

    hashProofTree(proofTree, version):
        if proofTree.left is 'included'
            hash1 = getHash(proofTree.left, version)
        else:
            hash1 = hashProofTree(proofTree.left, version)

        if proofTree.right is 'included'
            hash2 = getHash(proofTree.right, version)
        else:
            hash2 = hashProofTree(proofTree.right, version)

        return SHA(hash1, hash2)

    getHash(proofTree, version):
        if version == 'new'
            return proofTree.root.hash
        else if proofTree.root.isNew == true:
            return emptyHash()
