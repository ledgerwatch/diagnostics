package erigon_node

import (
	"context"
	"encoding/binary"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/ledgerwatch/diagnostics/internal"
)

type BlockNode struct {
	Height     uint64
	Hash       uint64
	ParentHash uint64
	Children   []*BlockNode
}

// Demonstration of the working with the Erigon database remotely on the example of getting information
// about past reorganisation of the chain

// FindDeepReorgs - Go through "Header" table and look for entries with the same block number and find lenght of deep reorgs on every block
// by using hash, parent hash, and block number
func (c *NodeClient) FindDeepReorgs(ctx context.Context,
	writer http.ResponseWriter,
	template *template.Template,
	requestChannel chan *internal.NodeRequest) {
	start := time.Now()
	var err error

	rc := NewRemoteCursor(c, requestChannel)
	if err = rc.Init(headersDb, headersTable, nil); err != nil {
		fmt.Fprintf(writer, "Create remote cursor: %v", err)
		return
	}

	total, wrongBlocks, errors := c.findDeepReorgsInternally(ctx, template, rc)
	for _, err := range errors {
		if err != nil {
			fmt.Fprintf(writer, "%v\n", err)
		}
	}

	for height, reorgLengthMap := range wrongBlocks {
		for length, count := range reorgLengthMap {
			fmt.Fprintf(writer, "Block height: %d, length of reorg: %d, count of deep reorgs: %d \n", height, length, count)
		}
	}

	fmt.Fprintf(writer, "Reorg iterator: %d, total scanned %s\n", total, time.Since(start))
	fmt.Fprintf(writer, "Reorg iterator: %d, wrong blocks\n", len(wrongBlocks))
}

func (c *NodeClient) findDeepReorgsInternally(ctx context.Context,
	template *template.Template,
	rc *RemoteCursor,
) (uint64, map[uint64]map[uint64]uint64, []error) {
	// mapping to store information of blocks on particular height
	deepReorgs := make(map[uint64][]*BlockNode)
	// store key and value fetched from DB
	var k []byte
	var v []byte
	var err error
	// stores maximum block height in order to find main chain
	var max uint64 = 0
	// store block-height, block-hash, and block's parent hash
	var height, hash, parentHash uint64
	// mapping to store block information corresponding to their block hash
	blocks := make(map[uint64]*BlockNode)
	// bool to avoid adding 1st node as children to a nil node
	var isFirst = true
	var countBlocks uint64 = 0
	// loop to iterate over all blocks as key value pairs and store their info
	for k, v, err = rc.Next(); err == nil && k != nil; k, v, err = rc.Next() {
		// fetch block height, hash, and parentHash from key vlaue pair
		height = binary.BigEndian.Uint64(k[:8])
		hash = binary.BigEndian.Uint64(k[8:40])
		parentHash = binary.BigEndian.Uint64(v[4:36])

		// initialize new block info
		newNode := &BlockNode{
			Height:     height,
			Hash:       hash,
			ParentHash: parentHash,
			Children:   []*BlockNode{},
		}

		// map block info corresponding to its hash
		blocks[hash] = newNode

		// condition to append block info to existing blockInfos according to their block height
		if _, found := deepReorgs[height]; found {
			// add block address as its parent's children
			blocks[newNode.ParentHash].Children = append(blocks[newNode.ParentHash].Children, newNode)
			deepReorgs[height] = append(deepReorgs[height], newNode)
		} else {
			if isFirst {
				isFirst = false
			} else {
				// add block address as its parent's children
				blocks[newNode.ParentHash].Children = append(blocks[newNode.ParentHash].Children, newNode)
			}
			deepReorgs[height] = []*BlockNode{newNode}
		}

		// helps in finding longest chain
		if height > max {
			max = height
		}

		// increment count of blocks
		countBlocks++
	}

	if err != nil {
		return 0, make(map[uint64]map[uint64]uint64), []error{err}
	}

	// returns empty mapping when no block info is available
	if len(blocks) == 0 {
		return 0, make(map[uint64]map[uint64]uint64), nil
	}

	// block to iterate over to find main chain
	var node *BlockNode
	var found bool = true
	// mapping to store main block corresponding to its height
	mainChain := make(map[uint64]*BlockNode)
	// maps main block according to its height
	for node = deepReorgs[max][0]; found; node, found = blocks[node.ParentHash] {
		mainChain[node.Height] = node
	}

	// mapping to store info for blockHeight of block from which reorgs are originating -> length of reorgs -> number of reorgs of corresponding length
	allReorgs := make(map[uint64]map[uint64]uint64)
	for _, node := range mainChain {
		// only iterate when there is a reorg initiating from that block
		if len(node.Children) > 1 {
			// initialize mapping for a particular node height
			allReorgs[node.Height] = make(map[uint64]uint64)
			// iterate over child blocks of node with reorg chains
			for _, block := range node.Children {
				// only iterate if it is not a part of main chain
				if mainChain[block.Height].Hash != block.Hash {
					allReorgs = appendDeepReorgInfo(1, block, allReorgs, node.Height)
				}
			}
		}
	}

	var errors []error
	/* for height, reorgLengthMap := range allReorgs {
		for length, count := range reorgLengthMap {
			// flush your data from here to your respective template, A blank template has been added named deep_reorg.html
		}
	} */
	return countBlocks, allReorgs, errors
}

// function to be called upon finding reorgs to store number of reorgs on that height corresponding to that reorg height
func appendDeepReorgInfo(reorgHeight uint64, node *BlockNode, allReorgs map[uint64]map[uint64]uint64, nodeHeight uint64) map[uint64]map[uint64]uint64 {
	// reorg count to be incremented only when node is a leaf node
	if len(node.Children) == 0 {
		// increment reorg count for that reorg height reorg height
		allReorgs[nodeHeight][reorgHeight]++
	} else {
		// iterate over all other blocks to find reorgs of increased length
		for _, block := range node.Children {
			appendDeepReorgInfo(reorgHeight+1, block, allReorgs, nodeHeight)
		}
	}
	return allReorgs
}
