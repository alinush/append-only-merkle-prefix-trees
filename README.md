# A more-memory friendly append-only, sparse-prefix tree

This code was written in 2015 to estimate append-only proof sizes in Merkle prefix trees, and later used for a research project on _append-only authenticated dictionaries (AADs)_ in 2019[^TBPplus19].

The idea here was to not dynamically allocate every tree node with left, right and parent pointers.

Instead, for each tree level, we keep a Go map that maps a node's location on that level to its Merkle hash. This drastically reduced the memory consumption.

[^TBPplus19]: **Transparency Logs via Append-Only Authenticated Dictionaries**, by Tomescu, Alin and Bhupatiraju, Vivek and Papadopoulos, Dimitrios and Papamanthou, Charalampos and Triandopoulos, Nikos and Devadas, Srinivas, *in ACM CCS'19*, 2019, [[URL]](https://doi.org/10.1145/3319535.3345652)
