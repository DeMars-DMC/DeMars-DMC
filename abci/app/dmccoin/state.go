package dmccoin

import (
	"github.com/Demars-DMC/Demars-DMC/libs/log"
)

type State struct {
	chainID    string
	store      KVS
	readCache  map[string][]byte // optional, for caching writes to store
	writeCache *KVCache          // optional, for caching writes w/o writing to store
	logger     log.Logger
}

func NewState(store KVS, logger log.Logger) *State {
	return &State{
		chainID:    "",
		store:      store,
		readCache:  make(map[string][]byte),
		writeCache: nil,
		logger:     logger,
	}
}

func (s *State) SetLogger(l log.Logger) {
	s.logger = l
}

func (s *State) SetChainID(chainID string) {
	s.chainID = chainID
	s.store.Set([]byte("base/chain_id"), []byte(chainID))
}

func (s *State) GetChainID() string {
	if s.chainID != "" {
		return s.chainID
	}
	s.chainID = string(s.store.Get([]byte("base/chain_id")))
	return s.chainID
}

func (s *State) Get(key []byte) (value []byte) {
	if s.readCache != nil { //if not a cachewrap
		value, ok := s.readCache[string(key)]
		if ok {
			return value
		}
	}
	return s.store.Get(key)
}

func (s *State) GetAll(key []byte, prefixSize int) (value []byteArray) {
	if s.readCache != nil { //if not a cachewrap
		i := 0
		for k, v := range s.readCache {
			if string(k[:prefixSize]) == string(key[:prefixSize]) {
				copy(value[i], v)
				i++
			}
		}
	}
	return value
}
func (s *State) Set(key []byte, value []byte) {
	if s.readCache != nil { //if not a cachewrap
		s.readCache[string(key)] = value
	}
	s.store.Set(key, value)
}

func (s *State) GetAccount(addr []byte) *Account {
	return GetAccount(s.store, addr)
}

func (s *State) GetAllAccountsInSegment(addr []byte, prefixSize int) []Account {
	return GetAllAccountsInSegment(s.store, addr, prefixSize)
}

func (s *State) GetAllAccounts() []Account {
	return GetAllAccounts(s.store)
}

func (s *State) SetAccount(addr []byte, acc *Account) {
	SetAccount(s.store, addr, acc)
}

// NOTE: errors if s is not from CacheWrap()
func (s *State) CacheSync() {
	s.writeCache.Sync()
}

func (s *State) Commit() string {
	return ""
}
