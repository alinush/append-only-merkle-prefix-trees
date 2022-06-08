package main

import (
    "crypto/sha256"
    "fmt"
    "math/big"
    "os"
    "time"
)

/**
 * The tree has 257 levels, numbered from 0 to 256. Each level has 2^level nodes.
 * Leaf no's are numbered from 0 to ((2^256) - 1)).
 * Globally, all nodes are assigned a global number (GN) from 1 to ((2^257) - 1).
 * (by enumerating them from the root to the bottom level in left-to-right order)
 * However, at each level, nodes on that level are assigned a local number (LN) in left-to-right order from 0 to ((2^level) - 1).
 * Given GN and its level, we can obtain the level-local node # (LN), by subtracting 2^level from the tree-global node #.
 *
 * The tree is stored by levels (257 in total), from level 0 (the root node) to level 256 (the leaves).
 * Each level stores a map[[32]byte]Node dictionary, which maps the LN to the node's hash and whether it's a newly inserted/updated node.
 */

type TreeLevel struct {
    num  int                // the level's number, numbered from 0 to numLevels - 1
    node map[[32]byte]*Node // maps a LN to its node's Node struct
}

type Node struct {
    Hash [32]byte // the Merkle hash of this node

    /**
     * Marks this node as 'new' during a Tree::Insert(), so we can easily construct append-only proofs.
     * Only newly created nodes are marked as 'new', and not nodes whose hash is changed by the insert.
     * After a batch of Insert()'s, the new nodes are marked and the append-only proof can be easily computed.
     * We can then 'clear' thew 'new' flag for these nodes, insert a new batch, and repeat the append-only proof.
     */
    IsNew bool
}

type Tree struct {
    lvl       []*TreeLevel // the tree is just an array of 257 levels (256 + root node)
    numLevels int          // levels are numbered from 0 to numLevels - 1
    //numNodes int          // this is just 2^numLevels - 1

    EmptyHash [32]byte // the hash of a non-existing node (all zeros)
    RootNo    [32]byte // the LN of the root (all zeros)

    // We'll need big.Int's to represent the value 2^256 and the value 2, which we
    // use often in our calculations so it's better to cache them here rather than
    // allocate them all the time and trash the heap.
    MaxLeafs *big.Int
    Two      *big.Int
    One      *big.Int
}

/**
 * Creates a new level with no 'num' and of size '2^num' nodes
 */
func _newTreeLevel(num int) *TreeLevel {
    level := new(TreeLevel)
    level.num = num
    level.node = make(map[[32]byte]*Node)
    return level
}

/**
 * Creates a new, empty tree with a certain # of levels.
 */
func NewTree(numLevels int) *Tree {
    if numLevels != 257 {
        panic("This code can only handle trees which have 257 levels. Please change the array size in TreeLevel::node to accomodate bigger leaf numbers that would occur when using bigger trees. (Actually will have to change some other things too.)")
    }

    lastLevel := numLevels - 1
    tree := new(Tree)
    tree.numLevels = numLevels
    tree.lvl = make([]*TreeLevel, numLevels)

    tree.One = big.NewInt(1)
    tree.Two = big.NewInt(2)
    tree.MaxLeafs = new(big.Int)
    tree.MaxLeafs.Exp(tree.Two, big.NewInt(int64(lastLevel)), nil)

    for i, _ := range tree.lvl {
        tree.lvl[i] = _newTreeLevel(i)
    }

    for i := 0; i < 32; i++ {
        tree.EmptyHash[i] = 0x00
        tree.RootNo[i] = 0x00
    }

    return tree
}

/**
 * Returns the number of nodes in the tree. Used to get the append-only proof size!
 *
 * NOTE: In the worst case, this should return 2^257 - 1, but for now we are restricting
 * ourselves to 64 bits, since our tests won't add an exponential # of nodes :)
 */
func (tree *Tree) GetNumNodes() int64 {
    var treeSize int64 = 0
    var levelSize int64
    for level := tree.numLevels - 1; level >= 0; level-- {
        levelSize = int64(len(tree.lvl[level].node))
        //fmt.Printf("Level %v size: %v\n", level, levelSize)
        treeSize += levelSize
    }

    return treeSize
}

/**
 * Returns the root hash of the tree.
 */
func (tree *Tree) GetRootHash() [32]byte {
    rootNodes := tree.lvl[0].node
    if len(rootNodes) != 1 {
        panic("Expected tree to have exactly one node at level 0")
    }

    var rootNode *Node
    for _, v := range rootNodes {
        rootNode = v
    }

    return rootNode.Hash
}

/**
 * Computes the hash of a parent node, given its two children's hashes.
 * One hash is given directly, while another one is given as a sibling node pointer.
 *
 * 'dir' is true when the left hash is in 'prevHash' and 'prevSibling' is the right child
 * 'dir' is false when the right hash is in 'prevHash' and 'prevSibiling' is the left child node
 */
func (tree *Tree) _computeHash(prevHash [32]byte, prevSibling *Node, dir bool) [32]byte {
    var leftHash *[32]byte = &prevHash
    var rightHash *[32]byte

    if prevSibling == nil {
        rightHash = &tree.EmptyHash
    } else {
        rightHash = &prevSibling.Hash
    }

    if !dir {
        var t *[32]byte = leftHash
        leftHash = rightHash
        rightHash = t
    }

    return _merkleHash(*leftHash, *rightHash)
}

/**
 * Merkle hashes two children hashes.
 */
func _merkleHash(h1 [32]byte, h2 [32]byte) [32]byte {
    digest := sha256.New()

    digest.Write(h1[:])
    digest.Write(h2[:])

    //fmt.Printf("h(%s, %s) = ", hashStr(h1), hashStr(h2))

    var storage []byte = make([]byte, 0)
    var hash [32]byte
    storage = digest.Sum(storage)
    for i := 0; i < len(storage); i++ {
        hash[i] = storage[i]
    }

    //fmt.Printf("%s\n", hashStr(hash))

    return hash
}

/**
 * Goes through every node (and its sibling) along the path starting at the specified node and ending in the root.
 * The node is specified via its level 'level' and LN 'localIdx'.
 * Calls 'leafCheck' for the actual node.
 * Calls 'nodeFunc' for every node on the path, including the node itself.
 */
func (tree *Tree) _visitPath(
    localIdx [32]byte, // the LN of the node
    level int, // the level # of the node
    nodeFunc func(*TreeLevel, *big.Int, *big.Int, bool),
    leafCheck func([32]byte)) {
    localNo := hashToInt(localIdx)
    var siblingNo big.Int
    var dir bool = tree.GetSiblingNo(&siblingNo, localNo) // returns true if node is left child

    // Perform a user-specified node check, such as making sure it does not
    // exist in the tree already
    if leafCheck != nil {
        leafCheck(localIdx)
    }

    if level > tree.numLevels-1 {
        panic(fmt.Sprintf("Started at too low of a level: %d", level))
    }

    for levelNo := level; levelNo >= 0; levelNo-- {
        lvl := tree.lvl[levelNo]

        // Perform a user-specified action for the ancestor node
        nodeFunc(lvl, localNo, &siblingNo, dir)

        // Compute the next node's per-level local number
        localNo.Div(localNo, tree.Two) // parent's local no = child's local no / 2
        dir = tree.GetSiblingNo(&siblingNo, localNo)
    }
}

/**
 * Stores the LN of the sibling of 'nodeNo' in the 'siblingNo' bigInt.
 * Returns true, if 'nodeNo' was a left child, false otherwise.
 */
func (tree *Tree) GetSiblingNo(siblingNo *big.Int, nodeNo *big.Int) bool {
    // Is this node a left-child?
    dir := nodeNo.Bit(0) == 0

    // Compute this node's local sibling no by adding or
    // subtracting one from the node's local no
    siblingNo.Set(nodeNo)
    if dir {
        siblingNo.Add(siblingNo, tree.One)
    } else {
        siblingNo.Sub(siblingNo, tree.One)
    }

    return dir
}

/**
 * Given an LN as a big integer, returns the Node struct for that node.
 */
func (tree *Tree) getNode(lvl *TreeLevel, localNo *big.Int) *Node {
    return lvl.node[bigIntTo32Bytes(localNo)]
}

/**
 * Given an LN as byte array, returns the Node struct for that node.
 */
func (tree *Tree) getNodeByByteArray(lvl *TreeLevel, localNo *[32]byte) *Node {
    return lvl.node[*localNo]
}

/**
 * Inserts the specified 'dataHash' in the specified 'leafNo' in the last level of the tree.
 */
func (tree *Tree) Insert(leafNo [32]byte, dataHash [32]byte, proofTree *Tree) {
    // This will be called before starting to iterate through the path to make sure the leaf's not been inserted before
    checkLeaf := func(leaf [32]byte) {
        if _, ok := tree.lvl[tree.numLevels-1].node[leaf]; ok {
            panic(fmt.Sprintf("Already set leaf '%s' at last level", hashStr(leafNo)))
        }
    }

    var prevSibling *Node = nil
    var prevHash [32]byte
    var prevDir bool = leafNo[31]&1 == 0

    // Our node function will compute the hashes of the internal nodes and set the inserted leaf's hash as well
    newNodes := 0
    insertNodeFunc := func(lvl *TreeLevel, ancestorNo *big.Int, siblingNo *big.Int, dir bool) {
        // Need to see if a node exists, and create it if not
        idx := bigIntTo32Bytes(ancestorNo)
        node, ok := lvl.node[idx]
        if !ok {
            //fmt.Printf("Creating new level %d node: %s\n", lvl.num, ancestorNo)
            // Don't set the new flag if we're not building consistency proofs
            isNew := proofTree != nil
            node = &Node{IsNew: isNew}
            lvl.node[idx] = node
            newNodes++
        }

        // Compute this node's hash from its children (for the leaf we just set the hash to 'dataHash')
        if lvl.num == tree.numLevels-1 {
            node.Hash = dataHash
        } else {
            node.Hash = tree._computeHash(prevHash, prevSibling, prevDir)
        }

        // Remember this node's hash
        prevHash = node.Hash
        // Get this node's sibling. Could be nil.
        prevSibling = tree.getNode(lvl, siblingNo)
        prevDir = dir
    }

    tree._visitPath(leafNo, tree.numLevels-1, insertNodeFunc, checkLeaf)

    // Incrementally build a consistency proof after each insertion
    if proofTree != nil {
        //fmt.Printf("Adding leaf %s to proof...\n", hashStr(leafNo))
        tree._proofAdd(leafNo, proofTree)
    }

    //fmt.Printf("Created %v nodes\n", newNodes)
}

/**
 * Checks an append-only proof.
 */
func VerifyAppendOnlyProof(proofTree *Tree, oldHash [32]byte, newHash [32]byte) bool {
    var hash [32]byte

    // Check that old nodes hash to old root hash
    hash = proofTree._hashProofTree(false)
    if hash != oldHash {
        fmt.Printf("ERROR: Old hash check failed\n")
        return false
    }

    // Check that old + new nodes hash to new root hash
    hash = proofTree._hashProofTree(true)
    if hash != newHash {
        fmt.Printf("ERROR: New hash check failed\n")
        return false
    }

    return true
}

/**
 * Hashes the proof tree, ignoring new nodes if isNew == true.
 * Obtains the old root if isNew == false, and new root otherwise.
 */
func (tree *Tree) _hashProofTree(isNew bool) [32]byte {
    var rootNo big.Int

    rootNo.SetUint64(0)
    return tree._hashProofTreeHelper(tree.lvl[0], &rootNo, isNew)
}

/**
 * Recursively computes the root hash in the proof tree, treating new nodes as empty nodes when isNew == false
 */
func (tree *Tree) _hashProofTreeHelper(lvl *TreeLevel, rootNo *big.Int, isNew bool) [32]byte {
    rootNode := lvl.node[bigIntTo32Bytes(rootNo)]

    if rootNode != nil {
        if isNew {
            return rootNode.Hash
        } else {
            if rootNode.IsNew {
                return tree.EmptyHash
            } else {
                return rootNode.Hash
            }
        }
    } else {
        if lvl.num == tree.numLevels-1 {
            //return tree.EmptyHash
            panic("Something's off: Reached leaf nil leaf node. Should've stopped descending earlier.")
        }

        var leftNo, rightNo big.Int
        leftNo.Set(rootNo)
        leftNo.Mul(&leftNo, tree.Two)
        rightNo.Set(&leftNo)
        rightNo.Add(&rightNo, tree.One)

        sublvl := tree.lvl[lvl.num+1]
        leftHash := tree._hashProofTreeHelper(sublvl, &leftNo, isNew)
        rightHash := tree._hashProofTreeHelper(sublvl, &rightNo, isNew)

        return _merkleHash(leftHash, rightHash)
    }
}

/**
 * Iterates through each level, from bottom-most to root level.
 * Calls 'levelFunc' for each level.
 * Calls 'nodeFunc' for each node on that level.
 */
func (tree *Tree) _visitNodesByLevel(
    levelFunc func(*TreeLevel),
    nodeFunc func(*TreeLevel, [32]byte, *Node)) {
    for level := tree.numLevels - 1; level >= 0; level-- {
        lvl := tree.lvl[level]
        lvlNodes := lvl.node

        if levelFunc != nil {
            levelFunc(lvl)
        }

        if nodeFunc != nil {
            // WARNING: no fixed order for iterating through map
            for idx, _ := range lvlNodes {
                //fmt.Printf("idx=%v, ", hashToInt(idx))
                nodeFunc(lvl, idx, lvlNodes[idx])
            }
        }
    }
}
func (tree *Tree) _visitLeaves(
    nodeFunc func(*TreeLevel, [32]byte, *Node)) {
    level := tree.numLevels - 1
    lvl := tree.lvl[level]
    lvlNodes := lvl.node

    if nodeFunc != nil {
        // WARNING: no fixed order for iterating through map
        for idx, _ := range lvlNodes {
            //fmt.Printf("idx=%v, ", hashToInt(idx))
            nodeFunc(lvl, idx, lvlNodes[idx])
        }
    }
}

/**
 * Makes sure there are no nodes in the tree marked as 'new', so we can insert a
 * new batch of nodes in the tree and measure the append-only proof size.
 */
func (tree *Tree) _assertNoNewNodes() {
    tree._visitNodesByLevel(nil, func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
        if node.IsNew == true {
            panic("Something's off: some nodes' IsNew flag was not cleared")
        }
    })
}

/**
 * Gets the number of sibling nodes in an append-only proof that are 'empty'.
 */
func (tree *Tree) GetNumEmptySiblings() int64 {
    var count int64 = 0
    tree._visitNodesByLevel(nil, func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
        if node.Hash == tree.EmptyHash {
            count++
        }
    })

    return count
}

func (tree *Tree) _proofAdd(leafNo [32]byte, proofTree *Tree) {
    // Adds either a 'new' node or 'old' node to the proof, possibly updating hashes in the proof (since a new leaf was added before _proofAdd)
    include := func(level int, node *Node, nodeNo *big.Int, isNew bool) {
        lvlNodes := proofTree.lvl[level].node
        idx := bigIntTo32Bytes(nodeNo)

        // get the hash of the node from the tree, or empty hash if nil node
        nodeHash := tree.EmptyHash
        if node != nil {
            nodeHash = node.Hash
            if nodeHash == tree.EmptyHash {
                panic("Did not expect to find empty hash in non-nil node")
            }
        }

        // if the node was not yet added to the proof
        if prevNode, ok := lvlNodes[idx]; !ok {
            //fmt.Printf("Adding (isNew: %v', nodeNo: %s, level: %d, hash: %s) node to proof tree\n", isNew, nodeNo, level, hashStr(nodeHash))
            if node == nil && isNew {
                panic("Did not expect includeNew() to be called on nil node")
            }

            if nodeHash == tree.EmptyHash && isNew {
                panic("Did not expect to add 'new' node w/ empty hash")
            }

            lvlNodes[idx] = &Node{Hash: nodeHash, IsNew: isNew}
        } else {
            // Recall that _proofAdd is called after every inserted leaf, so some hashes up the tree might change
            // NOTE: An 'empty' node can turn into a 'new' node in the proof after appending a leaf to the tree.
            if prevNode.IsNew == false && isNew == true {
                if prevNode.Hash != tree.EmptyHash {
                    panic("Hm, I thought only empty nodes can go from 'old' to 'new'")
                } else {
                    //fmt.Printf("Empty node turning 'new'\n")
                }
                prevNode.IsNew = true
            }

            prevNode.Hash = nodeHash
        }
    }

    // Adds an 'old' node to the append-only proof
    includeExisting := func(level int, node *Node, nodeNo *big.Int) {
        include(level, node, nodeNo, false)
    }

    // Adds a 'new' node to the append-only proof
    includeNew := func(level int, node *Node, nodeNo *big.Int) {
        include(level, node, nodeNo, true)
    }

    // this is the node where the 'new path' of the newly added leaf (possibly a new subtree because of previously added new leaves)
    // intersects an 'old path' of a leaf added in a previous batch
    foundIntersectionNode := false
    tree._visitPath(
        leafNo,
        tree.numLevels-1,
        func(lvl *TreeLevel, nodeNo *big.Int, siblingNo *big.Int, dir bool) {
            // Nothing to do for level 0
            if lvl.num == 0 {
                return
            }

            sibling := tree.getNode(lvl, siblingNo)
            //fmt.Printf("Looking at level-%d sibling '%s': %v, foundIntersectionNode: %v\n", lvl.num, siblingNo, sibling, foundIntersectionNode)
            if !foundIntersectionNode {
                // NOTE: The intersection point of an 'old path' and a 'new path' is defined by a node with one 'old' child and one 'new' child.
                // That's why here we check that sibling.IsNew == false. Recall that the nodes along the path (i.e., nodeNo) are 'new.
                // Also note that if we had two sibling 'new' leaves, we would add an ancestor of both of them to the proof, rather than adding them individually.
                if sibling != nil && sibling.IsNew == false {
                    //fmt.Printf("Adding level-%d existing sibling '%s' and ancestor '%s'\n", lvl.num, siblingNo, nodeNo)
                    includeExisting(lvl.num, sibling, siblingNo)
                    includeNew(lvl.num, tree.getNode(lvl, nodeNo), nodeNo)
                    foundIntersectionNode = true
                }
            } else {
                //fmt.Printf("Adding level-%d existing sibling '%s'\n", lvl.num, siblingNo)
                // NOTE: This could be either an 'old' or a 'new' node that we're adding
                if sibling == nil {
                    // if there's no sibling (i.e., EmptyHash), we use includeExisting because we don't want to mark an empty hash as 'new'
                    includeExisting(lvl.num, sibling, siblingNo)
                } else {
                    // if there's a sibling, it could be either 'new' or 'old'
                    include(lvl.num, sibling, siblingNo, sibling.IsNew)
                }
            }
        },
        nil)

    // NOTE: Since _proofAdd is called after every appended leaf, it will add too many nodes to the proof.
    // Specifically, when adding sibling nodes, it could add two sibling nodes in the proof which are never
    // necessary because one of them can be computed from its children. We fix this with _compressProof()
}

// Clears the IsNew flag from tree nodes after a batch is inserted, so we
// can be ready to compute consistency proofs for the next batch.
func (tree *Tree) clearNewFlag() {
    tree._visitLeaves(
        func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
            tree.clearNewFlagHelper(nodeIdx)
        })
}

func (tree *Tree) clearNewFlagHelper(leaf [32]byte) int {
    removedCount := 0

    tree._visitPath(leaf, tree.numLevels-1, func(lvl *TreeLevel,
        nodeNo *big.Int, siblingNo *big.Int, dir bool) {
        node := tree.getNode(lvl, nodeNo)
        if node == nil {
            panic(fmt.Sprintf("Expected node %v to exist at level %v",
                nodeNo, lvl.num))
        }

        if node.IsNew {
            node.IsNew = false
            removedCount++
        }
    }, nil)

    //fmt.Printf("Reset %v nodes\n", removedCount)
    return removedCount
}

/**
 * After _proofAdd is called after every newly added leaf, the proof tree will have an invariant:
 * The proof tree consists of:
 *  - for every 'intersection node', the children of that node: i.e., an 'old' node and a 'new' node
 *    + in theory, these could be the 'old' leaf and the 'new' leaf themselves, but in practice they never end up next to one another in the tree (requires a SHA2 collision almost)
 *  - sibling nodes that can be used to prove membership of the 'intersection nodes'
 *    + recall that we can hash an intersection node to an 'old' hash (by pretending its 'new' child doesn't exist) or to a 'new' hash (by incorporating its 'new' child)
 * However, the proof might have extra siblings that are not needed for verification. For example, we might have both the left and right child of the root node.
 * This function removes all extra siblings by identifying any siblings that are along a path from a leaf to the root (or from another sibling to the root).
 *
 * A first glance at this function might make one think it deletes everything from the proof tree.
 * However, the deletes always skips the node where a path to the root starts (by skipping the level).
 * Thus, the function only removes nodes along the path (not siblings!), leaving its leaves intact.
 * Note that siblings of one path will be removed when they show up along another path that is being walked up.
 */
func (tree *Tree) _compressProofTree() {
    tree._visitNodesByLevel(
        nil,
        func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
            startLevel := lvl.num
            tree._visitPath(
                nodeIdx,
                startLevel,
                func(lvl *TreeLevel, nodeNo *big.Int, siblingNo *big.Int, dir bool) {
                    // We do nothing for previously taken care of levels
                    //fmt.Printf("Checking node '%s' at level %d (started at level %d)\n", nodeNo, lvl.num, startLevel)
                    if lvl.num == startLevel {
                        return
                    }

                    if lvl.num > startLevel {
                        panic("_visitPath should not have visited lower levels")
                    }

                    // Since this ancestor has descendants, we don't need it in the
                    // proof: we can recompute it => delete it from the tree
                    idx := bigIntTo32Bytes(nodeNo)
                    if _, ok := lvl.node[idx]; ok {
                        //fmt.Printf("Deleted node '%s' at level %d\n", nodeNo, lvl.num)
                        delete(lvl.node, idx)
                    }
                },
                nil)
        })

    if tree.GetNumNodes() == 0 {
        panic("Compressed proof tree too much, lol: size 0")
    }
}

/**
 * This function ensures I didn't mess up the proof code :)
 * It checks a simple invariant: if there's a hash in the proof at some node, there shouldn't be any hashes in that node's subtree!
 */
func (tree *Tree) _isCorrectlyConstructedProof() bool {
    var failed bool = false
    tree._visitNodesByLevel(
        nil,
        func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
            if !tree._hasEmptySubtree(lvl.num, hashToInt(nodeIdx)) {
                failed = true
            }
        })
    return !failed
}

func (tree *Tree) isAncestor(levelA int, ancestor *big.Int, levelD int, descendant *big.Int) bool {
    if levelD <= levelA {
        //return false
        //fmt.Printf(
        panic("Ancestor's level # is supposed to be less than descendant's\n")
    }

    var levelDiff int = levelD - levelA
    var ancestorExpected big.Int = *descendant
    for i := 0; i < levelDiff; i++ {
        ancestorExpected.Div(&ancestorExpected, tree.Two)
    }

    return ancestorExpected.Cmp(ancestor) == 0
}

func (tree *Tree) _hasEmptySubtree(level int, nodeNo *big.Int) bool {
    var failed bool = false
    // NOTE: starts from the bottom-most level (i.e., the leaves)
    tree._visitNodesByLevel(
        nil,
        func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
            if level < lvl.num {
                if tree.isAncestor(level, nodeNo, lvl.num, hashToInt(nodeIdx)) {
                    failed = true
                } else {
                    //fmt.Printf("Not an ancestor!\n")
                }
            } else {
                //fmt.Printf("Skipped because levelA=%v >= levelD=%v\n", level, lvl.num)
            }
        })

    return !failed
}

func (tree *Tree) Print(includeNew bool) {
    if includeNew {
        fmt.Printf("Printing with 'new' nodes")
    } else {
        fmt.Printf("Printing without 'new' nodes")
    }
    tree._visitNodesByLevel(func(lvl *TreeLevel) {
        if len(lvl.node) > 0 {
            fmt.Printf("\nLevel %d: ", lvl.num)
        }
    }, func(lvl *TreeLevel, nodeIdx [32]byte, node *Node) {
        var nodeNo big.Int
        nodeNo.SetBytes(nodeIdx[:])
        if !node.IsNew || (node.IsNew && includeNew) {
            fmt.Printf("\n\t%s -> hash %s, %v", &nodeNo, hashStr(node.Hash), node.IsNew)
        }
    })
    fmt.Println()
}

func (tree *Tree) PrintSummary() {
    maxCount := 4
    if tree.numLevels < maxCount {
        panic("Tree is too small in height. Make it bigger or make maxCount smaller.")
    }
    var count int
    for level := tree.numLevels - 1; level >= 0; level -= count {
        levelsLeft := level + 1
        count = minInt(levelsLeft, maxCount)

        for i := 0; i < count; i++ {
            fmt.Printf("Level %-3d: %6d nodes | ", level-i,
                len(tree.lvl[level-i].node))
        }
        fmt.Println()
    }

    fmt.Printf("Root node hash: %s\n", hashStr(tree.GetRootHash()))
}

/**
 * Simulates inserting numBatches batches of public keys in a sparse Merkle tree
 * of size 2^256 leaves (and ((2^257) - 1) nodes in total). Each batch has
 * batchSize public keys in it. The seed parameter is used to seed a (Secure?
 * Doesn't matter.) PRNG that is used to generate email addresses like
 * 'aliceX@wonderland.com' which are hashed to produce leaf numbers.
 */
func hashsparse(sizes []int, seed int64, csvFile string) {
    memGc()

    // Initialize some bytes that we'll hash repeatedly to obtain leaf no's
    var randKey [32]byte = bigIntTo32Bytes(big.NewInt(seed))

    tree := NewTree(257)

    // We insert the dummy leaf 0, to make sure we have a non-empty tree, which
    // makes our consistency proof code easier to write
    tree.Insert(tree.RootNo, sha256.Sum256([]byte("Dummy leaf")), nil)
    if tree.GetNumNodes() != 257 {
        panic("Tree size is supposed to be 257 nodes")
    }

    f, err := os.Create(csvFile)
    if err != nil {
        panic("Error opening file: " + err.Error())
    }
    fmt.Fprintf(f, "dictSize,appendOnlyProofSize,verifyUsec,\n")

    prevSize := 1
    for i := 0; i < len(sizes); i++ {
        newSize := sizes[i]
        fmt.Printf("\nAppending new batch of size %v ...\n", newSize-prevSize)
        proofTree := NewTree(257)

        oldRootHash := tree.GetRootHash()

        startTime := time.Now()
        for j := 0; j < newSize-prevSize; j++ {
            randKey = sha256.Sum256(randKey[:])
            dataHash := sha256.Sum256([]byte(fmt.Sprintf("Data for leaf %v", hashStr(randKey))))

            //fmt.Println("Inserting key %s with value %s", hashStr(randKey), hashStr(dataHash))
            tree.Insert(randKey, dataHash, proofTree)
        }
        insertElapsed := time.Since(startTime)

        newRootHash := tree.GetRootHash()

        if oldRootHash == newRootHash {
            panic("Something's off: old and new root hash are the same")
        }
        fmt.Printf("Old root: %v\nNew root: %v\n", hashStr(oldRootHash), hashStr(newRootHash))

        // There will be some extra nodes in the proof that we can eliminate
        fmt.Printf("Getting number of nodes in proof... ")
        oldProofSize := proofTree.GetNumNodes()
        if oldProofSize == 0 {
            panic("Cannot have proof tree be of size 0")
        }
        fmt.Printf("Done.\n")

        //fmt.Printf("Proof (uncompressed) size: %v\n", oldProofSize)
        fmt.Printf("Compressing proof... ")
        proofTree._compressProofTree()
        fmt.Printf("Done.\n")

        //tree.Print()
        //proofTree.Print(false)
        //proofTree.Print(true)

        // Have to set IsNew flag to false once proof is computed
        fmt.Printf("Clearing 'new' flag... ")
        tree.clearNewFlag()
        fmt.Printf("Done.\n")
        //fmt.Printf("Asserting 'new' flag is cleared... ")
        //tree._assertNoNewNodes()
        //fmt.Printf("Done.\n")

        // NOTE: Slows us down, so commenting it out. Tested proof to be correctly computed in the past.
        //if !proofTree._isCorrectlyConstructedProof() {
        //    panic("Proof is not correctly computed. Check your code.");
        //}

        fmt.Printf("Verifying proof... ")
        startTime = time.Now()
        if VerifyAppendOnlyProof(proofTree, oldRootHash, newRootHash) == false {
            panic("Invalid consistency proof was generated")
        }
        proofVerifyTime := time.Since(startTime)
        fmt.Printf("Done.\n")

        numEmpty := proofTree.GetNumEmptySiblings()
        proofSize := proofTree.GetNumNodes()
        fmt.Printf(
            "# kv's: %v, "+
                "# tree nodes: %v, "+
                "proof size: %v "+
                "(uncompressed size: %v, # empty hashes: %d)\n"+
                "Insert time: %s, "+
                "proof verify time: %s usec\n",
            newSize,
            tree.GetNumNodes(),
            proofSize,
            oldProofSize, numEmpty,
            insertElapsed,
            proofVerifyTime)

        proofVerifyUsec := int64(proofVerifyTime / time.Microsecond)
        fmt.Fprintf(f, "%v, %v, %v,\n", newSize, proofSize, proofVerifyUsec)

        //if (i + 1) % 4000 == 0 {
        //    fmt.Println("Garbage collecting at i = %d...", i)
        //    memGc()
        //}

        prevSize = newSize
    }

    fmt.Printf("\n")
    memGc()
    //tree.PrintSummary()
}
