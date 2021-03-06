package forging

import (
	"math/big"
	"pandora-pay/blockchain/block"
	"pandora-pay/config"
	"pandora-pay/gui"
	"pandora-pay/helpers"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type ForgingWork struct {
	blkComplete *block.BlockComplete
	target      *big.Int
}

type ForgingSolution struct {
	timestamp uint64
	address   *ForgingWalletAddress
	work      *ForgingWork
}

type Forging struct {
	work     unsafe.Pointer
	solution unsafe.Pointer

	started int32

	wg sync.WaitGroup

	Wallet          ForgingWallet
	SolutionChannel chan *block.BlockComplete
}

func ForgingInit() (forging *Forging) {

	forging = &Forging{
		SolutionChannel: make(chan *block.BlockComplete),
		Wallet: ForgingWallet{
			addressesMap: make(map[string]*ForgingWalletAddress),
		},
	}

	gui.Log("Forging Init")
	if err := forging.Wallet.loadBalances(); err != nil {
		panic(err)
	}

	go forging.startForging(config.CPU_THREADS)

	return
}

func (forging *Forging) startForging(threads int) {

	if !atomic.CompareAndSwapInt32(&forging.started, 0, 1) {
		return
	}

	for atomic.LoadInt32(&forging.started) == 1 {

		workPointer := atomic.LoadPointer(&forging.work)
		if atomic.LoadPointer(&forging.work) == nil {
			// gui.Error("No block for staking..." )
			time.Sleep(10 * time.Millisecond)
			continue
		}
		work := (*ForgingWork)(workPointer)

		//distributing the wallets to each thread uniformly
		forging.Wallet.RLock()
		wallets := [][]*ForgingWalletAddress{{}}
		for i := 0; i < threads; i++ {
			wallets = append(wallets, []*ForgingWalletAddress{})
		}
		for i, walletAdr := range forging.Wallet.addresses {
			if walletAdr.account != nil || work.blkComplete.Block.Height == 0 {
				wallets[i%threads] = append(wallets[i%threads], &ForgingWalletAddress{
					delegatedPublicKey:  walletAdr.delegatedPublicKey,
					delegatedPrivateKey: walletAdr.delegatedPrivateKey,
					publicKeyHash:       walletAdr.publicKeyHash,
					account:             walletAdr.account,
				})
			}
		}
		forging.Wallet.RUnlock()

		for i := 0; i < threads; i++ {
			forging.wg.Add(1)
			go forge(forging, workPointer, work, wallets[i])
		}

		forging.wg.Wait()

		if atomic.LoadPointer(&forging.solution) != nil {
			err := forging.publishSolution()
			if err != nil {
				gui.Error("Error publishing solution", err)
				atomic.StorePointer(&forging.solution, nil)
			}
		}

	}

}

func (forging *Forging) StopForging() {
	atomic.StorePointer(&forging.work, nil)
	atomic.StorePointer(&forging.solution, nil)
	atomic.CompareAndSwapInt32(&forging.started, 1, 0)
}

//thread safe
func (forging *Forging) RestartForgingWorkers(blkComplete *block.BlockComplete, target *big.Int) {

	work := ForgingWork{
		blkComplete: blkComplete,
		target:      target,
	}
	atomic.StorePointer(&forging.work, unsafe.Pointer(&work))
	atomic.StorePointer(&forging.solution, nil)

}

func (forging *Forging) StopForgingWorkers() {
	atomic.StorePointer(&forging.work, nil)
	atomic.StorePointer(&forging.solution, nil)
}

//thread safe
func (forging *Forging) foundSolution(address *ForgingWalletAddress, timestamp uint64, work *ForgingWork) {

	solution := ForgingSolution{
		timestamp: timestamp,
		address:   address,
		work:      work,
	}

	atomic.StorePointer(&forging.solution, unsafe.Pointer(&solution))
	atomic.StorePointer(&forging.work, nil)
}

// thread not safe
func (forging *Forging) publishSolution() (err error) {

	defer func() {
		err = helpers.ConvertRecoverError(recover())
	}()

	solutionPointer := atomic.LoadPointer(&forging.solution)
	solution := (*ForgingSolution)(solutionPointer)

	work := solution.work

	work.blkComplete.Block.Forger = solution.address.publicKeyHash
	work.blkComplete.Block.DelegatedPublicKey = solution.address.delegatedPublicKey
	work.blkComplete.Block.Timestamp = solution.timestamp
	if work.blkComplete.Block.Height > 0 {
		work.blkComplete.Block.StakingAmount = solution.address.account.GetDelegatedStakeAvailable(work.blkComplete.Block.Height)
	}

	serializationForSigning := work.blkComplete.Block.SerializeForSigning()

	work.blkComplete.Block.Signature = solution.address.delegatedPrivateKey.Sign(serializationForSigning)

	//send message to blockchain
	forging.SolutionChannel <- work.blkComplete
	return
}

func (forging *Forging) Close() {
	forging.StopForging()
}
