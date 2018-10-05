package dmccoin

import (
	"encoding/json"
	"fmt"
	"sort"

	crypto "github.com/Demars-DMC/Demars-DMC/crypto"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
)

type Account struct {
	PubKey  crypto.PubKey `json:"pub_key"` // May be nil, if not known.
	Height  int           `json:"height"`  // Hold the account balance at height
	Balance uint64        `json:"DMC"`
	Address data.Bytes    `json:"address"`
}

/**
 * Slice of accounts and function to sort them
 * for selecting the top k validators?
 */
type Accounts []Account

func TopkAccounts(acc []Account, k int) []Account {
	if acc == nil {
		return nil
	}
	accounts := acc
	sort.Slice(accounts, func(i int, j int) bool {
		return accounts[i].Balance < accounts[j].Balance
	})
	if k < len(accounts) {
		return accounts[0:k]
	}
	return accounts[:]
}

func (acc *Account) Copy() *Account {
	if acc == nil {
		return nil
	}
	accCopy := *acc
	return &accCopy
}

func (acc *Account) String() string {
	if acc == nil {
		return "nil-Account"
	}
	return fmt.Sprintf("Account{%v %v %v}",
		acc.PubKey, acc.Height, acc.Balance)
}

//----------------------------------------

type PrivAccount struct {
	crypto.PrivKey
	Account
}

//----------------------------------------

type AccountGetter interface {
	GetAccount(addr []byte) *Account
	GetAllAccountsInSegment(addr []byte, prefixSize int) []Account
}

type AccountSetter interface {
	SetAccount(addr []byte, acc *Account)
}

type AccountGetterSetter interface {
	GetAccount(addr []byte) *Account
	GetAllAccountsInSegment(addr []byte, prefixSize int) []Account
	SetAccount(addr []byte, acc *Account)
}

func AccountKey(addr []byte) []byte {
	return append([]byte("base/a/"), addr...)
}

func GetAccount(store KVS, addr []byte) *Account {
	data := store.Get(AccountKey(addr))
	if len(data) == 0 {
		return nil
	}
	acc := Account{}
	json.Unmarshal(data, &acc)
	return &acc
}

func GetAllAccountsInSegment(store KVS, addr []byte, prefixSize int) []Account {
	data := store.GetAll(AccountKey(addr), prefixSize)
	if len(data) == 0 {
		return nil
	}
	var acc = make([]Account, 0)
	for _, accountJSON := range data {
		currentAcc := Account{}
		json.Unmarshal(accountJSON, &currentAcc)
		acc = append(acc, currentAcc)
	}
	return acc
}

func GetAllAccounts(store KVS) []Account {
	data := store.GetAllAcc()
	if len(data) == 0 {
		return nil
	}
	var acc = make([]Account, 0)
	for _, accountJSON := range data {
		currentAcc := Account{}
		json.Unmarshal(accountJSON, &currentAcc)
		acc = append(acc, currentAcc)
	}
	return acc
}

func SetAccount(store KVS, addr []byte, acc *Account) {
	accBytes := wire.JSONBytes(acc)
	store.Set(AccountKey(addr), accBytes)
}
