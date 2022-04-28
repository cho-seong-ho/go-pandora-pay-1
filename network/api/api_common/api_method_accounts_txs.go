package api_common

import (
	"errors"
	"net/http"
	"pandora-pay/config"
	"pandora-pay/helpers/generics"
	"pandora-pay/network/api/api_common/api_types"
	"pandora-pay/store"
	"pandora-pay/store/store_db/store_db_interface"
	"strconv"
)

type APIAccountTxsRequest struct {
	api_types.APIAccountBaseRequest
	Start uint64 `json:"start,omitempty" msgpack:"start,omitempty"`
	Dsc   bool   `json:"dsc,omitempty" msgpack:"dsc,omitempty"`
}

type APIAccountTxsReply struct {
	Count uint64   `json:"count,omitempty" msgpack:"count,omitempty"`
	Txs   [][]byte `json:"txs,omitempty" msgpack:"txs,omitempty"`
}

func (api *APICommon) GetAccountTxs(r *http.Request, args *APIAccountTxsRequest, reply *APIAccountTxsReply) (err error) {

	publicKey, err := args.GetPublicKey(true)
	if err != nil {
		return
	}

	publicKeyStr := string(publicKey)

	return store.StoreBlockchain.DB.View(func(reader store_db_interface.StoreDBTransactionInterface) (err error) {

		data := reader.Get("addrTxsCount:" + publicKeyStr)
		if data == nil {
			return nil
		}

		if reply.Count, err = strconv.ParseUint(string(data), 10, 64); err != nil {
			return
		}

		s := generics.Min(generics.Max(args.Start, 0), reply.Count)
		if args.Dsc {
			s = generics.Max(args.Start-config.API_ACCOUNT_MAX_TXS, 0)
		}
		n := generics.Min(s+config.API_ACCOUNT_MAX_TXS, reply.Count)

		reply.Txs = make([][]byte, n-s)
		for i := s; i < n; i++ {
			hash := reader.Get("addrTx:" + publicKeyStr + ":" + strconv.FormatUint(i, 10))
			if hash == nil {
				return errors.New("Error reading address transaction")
			}
			reply.Txs[s-i-1] = hash
		}

		return
	})
}
