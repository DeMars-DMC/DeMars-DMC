package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"encoding/json"
	"time"
	"github.com/Demars-DMC/Demars-DMC/crypto/merkle"
	"github.com/Demars-DMC/Demars-DMC/crypto/tmhash"
	cmn "github.com/Demars-DMC/Demars-DMC/libs/common"
	"crypto/sha256"
	"github.com/Demars-DMC/Demars-DMC/abci/app/dmccoin"
	"github.com/Demars-DMC/Demars-DMC/libs/log"
)

// Block defines the atomic unit of a Demars-DMC blockchain.
// TODO: add Version byte
type Block struct {
	mtx        sync.Mutex
	*Header    `json:"header"`
	*Data      `json:"data"`
	Commit *Commit      `json:"commit"`
}

// MakeBlock returns a new block with an empty header, except what can be computed from itself.
// It populates the same set of fields validated by ValidateBasic
func MakeBlock(height int64, txs []Tx, commit *Commit, logger log.Logger) *Block {
	txBuckets := make(TxBuckets, 0)
	for i := 0; i < 0xff; i++ {
		txBucket := TxBucket{}
		txBucket.BucketId = fmt.Sprintf("%x", i)
		txBucket.Txs = make(Txs, 0)
		txBuckets = append(txBuckets, txBucket)
	}
	for i := 0; i < len(txs); i++ {
		segments := getBucketId(txs[i], height, logger)
		logger.Debug(fmt.Sprintf("Segment Id: %v Segment1 Id: %v", segments[0], segments[1]))
		for j := 0; j < 0xff; j++ {
			if txBuckets[j].BucketId == segments[0] || txBuckets[j].BucketId == segments[1] {
				txBuckets[j].Txs = append(txBuckets[j].Txs, txs[i])
				// Note we do not break here as we can add the same Tx to 2 buckets (input and output)
			}
		}
	}
	block := &Block{
		Header: &Header{
			Height: height,
			Time:   time.Now(),
			NumTxs: int64(len(txs)),
		},
		Commit: commit,
		Data: &Data{
			TxBuckets: txBuckets,
		},
	}
	block.fillHeader()
	return block
}

// getBucketId can return up to 2 bucket ids to which the transaction belongs
// FIXME: Demars-DMC should get Txs in a bucketed format from ABCI and not be able
// to parse them
func getBucketId(tx Tx, height int64, logger log.Logger) [2]string {
	utxoTx := dmccoin.TxUTXO{}
	dmcTx :=  dmccoin.DMCTx{}

	if height % 100 == 1 {
		// Parse UTXO tx
		json.Unmarshal(tx, &utxoTx)
		segment := string(utxoTx.Address)[:2]
		return [2]string{segment, ""}
	} else {
		// Parse normal tx
		json.Unmarshal(tx, &dmcTx)
		segment1 := fmt.Sprintf("%x", dmcTx.Input.Address[:])[:2]
		segment2 := fmt.Sprintf("%x", dmcTx.Output.Address[:])[:2]
		return [2]string{segment1, segment2}
	}
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("Nil blocks are invalid")
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	newTxs := b.Data.TotalTxsInBuckets()
	if b.NumTxs != newTxs {
		return fmt.Errorf("Wrong Block.Header.NumTxs. Expected %v, got %v", newTxs, b.NumTxs)
	}
	if b.Header.Height != 1 {
		if err := b.Commit.ValidateBasic(); err != nil {
			return err
		}
	}
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return fmt.Errorf("Wrong Block.Header.DataHash.  Expected %v, got %v", b.DataHash, b.Data.Hash())
	}
	return nil
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *Block) fillHeader() {
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
}

// Hash computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() cmn.HexBytes {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b == nil || b.Header == nil || b.Data == nil || b.Commit == nil {
		return nil
	}
	b.fillHeader()
	return b.Header.Hash()
}

// MakePartSet returns a PartSet containing parts of a serialized block.
// This is the form in which the block is gossipped to peers.
func (b *Block) MakePartSet(partSize int) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	// We prefix the byte length, so that unmarshaling
	// can easily happen via a reader.
	bz, err := cdc.MarshalBinary(b)
	if err != nil {
		panic(err)
	}
	return NewPartSetFromData(bz, partSize)
}

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size returns size of the block in bytes.
func (b *Block) Size() int {
	bz, err := cdc.MarshalBinaryBare(b)
	if err != nil {
		return 0
	}
	return len(bz)
}

// String returns a string representation of the block
func (b *Block) String() string {
	return b.StringIndented("")
}

// StringIndented returns a string representation of the block
func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Hash())
}

// StringShort returns a shortened string representation of the block
func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%v", b.Hash())
}

//-----------------------------------------------------------------------------

// Header defines the structure of a Demars-DMC block header
// TODO: limit header size
// NOTE: changes to the Header should be duplicated in the abci Header
type Header struct {
	// basic block info
	ChainID string    `json:"chain_id"`
	Height  int64     `json:"height"`
	Time    time.Time `json:"time"`
	NumTxs  int64     `json:"num_txs"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`
	TotalTxs    int64   `json:"total_txs"`
	LastBucketHashes *LastBucketHashes `json:"last_bucket_hashes"`

	// hashes of block data
	DataHash       cmn.HexBytes `json:"data_hash"`        // transactions

	// hashes from the app output from the prev block
	ValidatorsHash  cmn.HexBytes `json:"validators_hash"`   // validators for the current block
	ConsensusHash   cmn.HexBytes `json:"consensus_hash"`    // consensus params for current block
	AppHash         cmn.HexBytes `json:"app_hash"`          // state after txs from the previous block
	LastResultsHash cmn.HexBytes `json:"last_results_hash"` // root hash of all results from the txs from the previous block
}

// Hash returns the hash of the header.
// Returns nil if ValidatorHash is missing,
// since a Header is not valid unless there is
// a ValidaotrsHash (corresponding to the validator set).
func (h *Header) Hash() cmn.HexBytes {
	if h == nil || len(h.ValidatorsHash) == 0 {
		return nil
	}
	return merkle.SimpleHashFromMap(map[string]merkle.Hasher{
		"ChainID":     aminoHasher(h.ChainID),
		"Height":      aminoHasher(h.Height),
		"Time":        aminoHasher(h.Time),
		"NumTxs":      aminoHasher(h.NumTxs),
		"TotalTxs":    aminoHasher(h.TotalTxs),
		"LastBlockID": aminoHasher(h.LastBlockID),
		"Data":        aminoHasher(h.DataHash),
		"Validators":  aminoHasher(h.ValidatorsHash),
		"App":         aminoHasher(h.AppHash),
		"Consensus":   aminoHasher(h.ConsensusHash),
		"Results":     aminoHasher(h.LastResultsHash),
	})
}

// StringIndented returns a string representation of the header
func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  ChainID:        %v
%s  Height:         %v
%s  Time:           %v
%s  NumTxs:         %v
%s  TotalTxs:       %v
%s  LastBlockID:    %v
%s  Data:           %v
%s  Validators:     %v
%s  App:            %v
%s  Consensus:       %v
%s  Results:        %v
%s}#%v`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Time,
		indent, h.NumTxs,
		indent, h.TotalTxs,
		indent, h.LastBlockID,
		indent, h.DataHash,
		indent, h.ValidatorsHash,
		indent, h.AppHash,
		indent, h.ConsensusHash,
		indent, h.LastResultsHash,
		indent, h.Hash())
}

//-------------------------------------

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"block_id"`
	Precommits []*Vote `json:"precommits"`

	// Volatile
	firstPrecommit *Vote
	hash           cmn.HexBytes
	bitArray       *cmn.BitArray
}

// FirstPrecommit returns the first non-nil precommit in the commit.
// If all precommits are nil, it returns an empty precommit with height 0.
func (commit *Commit) FirstPrecommit() *Vote {
	if len(commit.Precommits) == 0 {
		return nil
	}
	if commit.firstPrecommit != nil {
		return commit.firstPrecommit
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.firstPrecommit = precommit
			return precommit
		}
	}
	return &Vote{
		Type: VoteTypePrecommit,
	}
}

// Height returns the height of the commit
func (commit *Commit) Height() int64 {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Height
}

// Round returns the round of the commit
func (commit *Commit) Round() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Round
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

// Size returns the number of votes in the commit
func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Precommits)
}

// BitArray returns a BitArray of which validators voted in this commit
func (commit *Commit) BitArray() *cmn.BitArray {
	if commit.bitArray == nil {
		commit.bitArray = cmn.NewBitArray(len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			// TODO: need to check the BlockID otherwise we could be counting conflicts,
			// not just the one with +2/3 !
			commit.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return commit.bitArray
}

// GetByIndex returns the vote corresponding to a given validator index
func (commit *Commit) GetByIndex(index int) *Vote {
	return commit.Precommits[index]
}

// IsCommit returns true if there is at least one vote
func (commit *Commit) IsCommit() bool {
	return len(commit.Precommits) != 0
}

// ValidateBasic performs basic validation that doesn't involve state data.
func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
	if len(commit.Precommits) == 0 {
		return errors.New("No precommits in commit")
	}
	height, round := commit.Height(), commit.Round()

	// validate the precommits
	for _, precommit := range commit.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

// Hash returns the hash of the commit
func (commit *Commit) Hash() cmn.HexBytes {
	if commit.hash == nil {
		bs := make([]merkle.Hasher, len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			bs[i] = aminoHasher(precommit)
		}
		commit.hash = merkle.SimpleHashFromHashers(bs)
	}
	return commit.hash
}

// StringIndented returns a string representation of the commit
func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Commit{
%s  BlockID:    %v
%s  Precommits: %v
%s}#%v`,
		indent, commit.BlockID,
		indent, strings.Join(precommitStrings, "\n"+indent+"  "),
		indent, commit.hash)
}

//-----------------------------------------------------------------------------

// SignedHeader is a header along with the commits that prove it
type SignedHeader struct {
	Header *Header `json:"header"`
	Commit *Commit `json:"commit"`
}

//-----------------------------------------------------------------------------

// Data contains the set of transactions included in the block
type Data struct {

	// Txs that will be applied by state @ block.Height+1.
	// NOTE: not all txs here are valid.  We're just agreeing on the order first.
	// This means that block.AppHash does not include these txs.
	TxBuckets TxBuckets `json:"txBuckets"`

	// Volatile
	hash cmn.HexBytes
}

// Txs is a slice of Tx.
type TxBuckets []TxBucket

func (buckets TxBuckets) Hash() cmn.HexBytes {
	startingHash := sha256.Sum256(make([]byte, 1))
	var combinedHash []byte

	for i := 0; i < len(buckets); i++ {
		bucket := buckets[i]
		hashTx := bucket.Txs.Hash()
		if i == 0 {
			combinedHash = startingHash[:]
		} else {
			combinedHash = merkle.SimpleHashFromTwoHashes(combinedHash[:], hashTx)
		}
	}
	return combinedHash
}

// Txs is a slice of Tx.
type TxBucket struct {
	BucketId string `json: bucket_id`
	Txs Txs `json: "txs"`
}

type BucketHash cmn.HexBytes

type LastBucketHashes struct {
	BucketHashes map[string]BucketHash `json: bucket_hashes`
}

// Hash returns the hash of the data
func (data *Data) Hash() cmn.HexBytes {
	if data == nil {
		return (Txs{}).Hash()
	}
	if data.hash == nil {
		data.hash = data.TxBuckets.Hash() // NOTE: leaves of merkle tree are TxIDs
	}
	return data.hash
}

// StringIndented returns a string representation of the transactions
func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}

	TxBuckets := data.TxBuckets
	txStrings := make([]string, cmn.MinInt64(data.TotalTxsInBuckets(), 21))

	for _, TxBucket := range TxBuckets {
		for i, tx := range TxBucket.Txs {
			if i == 20 {
				txStrings[i] = fmt.Sprintf("... (%v total)", len(TxBucket.Txs))
				break
			}
			txStrings[i] = fmt.Sprintf("%X (%d bytes)", tx.Hash(), len(tx))
		}
	}

	return fmt.Sprintf(`Data{
%s  %v
%s}#%v`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}
// TotalTxsInBuckets returns the total number of transactions in all the buckets in this block
func (data *Data) TotalTxsInBuckets() int64 {
	if data == nil {
		return 0
	}

	totalTxs := int64(0)
	for _, TxBucket := range data.TxBuckets {
		totalTxs += int64(len(TxBucket.Txs))
	}

	return totalTxs
}

//--------------------------------------------------------------------------------

// BlockID defines the unique ID of a block as its Hash and its PartSetHeader
type BlockID struct {
	Hash        cmn.HexBytes  `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

// IsZero returns true if this is the BlockID for a nil-block
func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 && blockID.PartsHeader.IsZero()
}

// Equals returns true if the BlockID matches the given BlockID
func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash, other.Hash) &&
		blockID.PartsHeader.Equals(other.PartsHeader)
}

// Key returns a machine-readable string representation of the BlockID
func (blockID BlockID) Key() string {
	bz, err := cdc.MarshalBinaryBare(blockID.PartsHeader)
	if err != nil {
		panic(err)
	}
	return string(blockID.Hash) + string(bz)
}

// String returns a human readable string representation of the BlockID
func (blockID BlockID) String() string {
	return fmt.Sprintf(`%v:%v`, blockID.Hash, blockID.PartsHeader)
}

//-------------------------------------------------------

type hasher struct {
	item interface{}
}

func (h hasher) Hash() []byte {
	hasher := tmhash.New()
	if h.item != nil && !cmn.IsTypedNil(h.item) && !cmn.IsEmpty(h.item) {
		bz, err := cdc.MarshalBinaryBare(h.item)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(bz)
		if err != nil {
			panic(err)
		}
	}
	return hasher.Sum(nil)

}

func aminoHash(item interface{}) []byte {
	h := hasher{item}
	return h.Hash()
}

func aminoHasher(item interface{}) merkle.Hasher {
	return hasher{item}
}
