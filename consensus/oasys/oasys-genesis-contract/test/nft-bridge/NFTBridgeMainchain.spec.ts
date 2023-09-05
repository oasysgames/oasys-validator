import abi from 'web3-eth-abi'
import { ethers } from 'hardhat'
import { ContractFactory, Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { expect } from 'chai'
import { chainid as mainchainId, makeExpiration, makeSignature, makeHashWithNonce, zeroAddress } from '../helpers'

const sidechainId = 33333
const tokenId = 1
const depositIndex = 0
const withdrawalIndex = 0

const getRejectDepositHash = (mainchainId: number, depositIndex: number) => {
  const fsig = abi.encodeFunctionSignature('rejectDeposit(uint256,uint256)')
  const psig = abi.encodeParameters(['uint256', 'uint256'], [mainchainId, depositIndex])
  const hash = ethers.utils.keccak256(fsig + psig.slice(2))
  return hash
}

const getFinalizeWithdrawalHash = (
  mainchainId: number,
  depositIndex: number,
  sidechainId: number,
  withdrawalIndex: number,
  sideFrom: string,
  mainTo: string,
) => {
  const types = 'uint256,uint256,uint256,uint256,address,address'
  const fsig = abi.encodeFunctionSignature(`finalizeWithdrawal(${types})`)
  const psig = abi.encodeParameters(types.split(','), [
    mainchainId,
    depositIndex,
    sidechainId,
    withdrawalIndex,
    sideFrom,
    mainTo,
  ])
  const hash = ethers.utils.keccak256(fsig + psig.slice(2))
  return hash
}

const getTransferMainchainRelayerHash = (nonce: number, to: string, mainchainId: number, newRelayer: string) => {
  const fsig = abi.encodeFunctionSignature('transferMainchainRelayer(uint256,address)')
  const psig = abi.encodeParameters(['uint256', 'address'], [mainchainId, newRelayer])
  const encodedSelector = fsig + psig.slice(2)
  return makeHashWithNonce(nonce, to, encodedSelector)
}

describe('NFTBridgeMainchain', () => {
  const expiration = makeExpiration()

  let accounts: Account[]
  let deployer: Account
  let signer: Account
  let user: Account
  let sideTo: Account

  let bridgeFactory: ContractFactory
  let relayerFactory: ContractFactory
  let tokenFactory: ContractFactory

  let bridge: Contract
  let relayer: Contract
  let token: Contract

  let nonce: number

  const mint = async () => {
    return token.connect(deployer).mint(user.address, tokenId)
  }

  const deposit = async (_sideTo?: string) => {
    await token.connect(user).approve(bridge.address, tokenId)
    return bridge.connect(user).deposit(token.address, tokenId, sidechainId, _sideTo ?? sideTo.address)
  }

  before(async () => {
    accounts = await ethers.getSigners()
    deployer = accounts[1]
    signer = accounts[2]
    user = accounts[3]
    sideTo = accounts[3]

    bridgeFactory = await ethers.getContractFactory('NFTBridgeMainchain')
    relayerFactory = await ethers.getContractFactory('NFTBridgeRelayer')
    tokenFactory = await ethers.getContractFactory('TestERC721')
  })

  beforeEach(async () => {
    nonce = 0

    bridge = await bridgeFactory.connect(deployer).deploy()
    relayer = await relayerFactory.connect(deployer).deploy(bridge.address, zeroAddress, [signer.address], 1)
    token = await tokenFactory.connect(deployer).deploy('test token', 'tt')

    await bridge.connect(deployer).transferMainchainRelayer(mainchainId, relayer.address)
  })

  it('depositInfos()', async () => {
    await mint()
    await deposit()

    const actual = await bridge.getDepositInfo(0)
    expect(actual.mainchainERC721).to.equal(token.address)
    expect(actual.tokenId).to.equal(tokenId)
    expect(actual.mainFrom).to.equal(user.address)
    expect(actual.mainTo).to.equal(zeroAddress)
  })

  describe('deposit()', () => {
    it('normally', async () => {
      await mint()
      expect(await token.ownerOf(tokenId)).to.equal(user.address)

      const tx = await deposit()
      expect(await token.ownerOf(tokenId)).to.equal(bridge.address)
      await expect(tx)
        .to.emit(bridge, 'DepositInitiated')
        .withArgs(0, token.address, tokenId, sidechainId, user.address, user.address)
    })

    it('sideTo is zero address', async () => {
      await mint()
      const tx = deposit(zeroAddress)
      await expect(tx).to.be.revertedWith('sideTo is zero address.')
    })
  })

  describe('rejectDeposit()', () => {
    const rejectDeposit = async (_mainchainId?: number) => {
      const hash = getRejectDepositHash(_mainchainId ?? mainchainId, depositIndex)
      const signatures = await makeSignature(signer, hash, mainchainId, expiration)
      return relayer.connect(user).rejectDeposit(_mainchainId ?? mainchainId, depositIndex, expiration, signatures)
    }

    it('normally', async () => {
      await mint()
      await deposit()
      expect(await token.ownerOf(tokenId)).to.equal(bridge.address)

      const tx = await rejectDeposit()
      expect(await token.ownerOf(tokenId)).to.equal(user.address)
      await expect(tx).to.emit(bridge, 'DepositRejected').withArgs(depositIndex)
    })

    it('invalid chain id', async () => {
      await mint()
      await deposit()
      const tx = rejectDeposit(12345)
      await expect(tx).to.be.revertedWith('Invalid main chain id.')
    })

    it('already rejected', async () => {
      await mint()
      await deposit()
      await rejectDeposit()
      const tx = rejectDeposit()
      await expect(tx).to.be.revertedWith('already rejected')
    })
  })

  describe('finalizeWithdrawal()', () => {
    const finalizeWithdrawal = async (_mainchainId?: number, mainTo?: string) => {
      const hash = getFinalizeWithdrawalHash(
        _mainchainId ?? mainchainId,
        depositIndex,
        sidechainId,
        withdrawalIndex,
        user.address,
        mainTo ?? user.address,
      )
      const signatures = await makeSignature(signer, hash, mainchainId, expiration)
      return relayer.finalizeWithdrawal(
        _mainchainId ?? mainchainId,
        depositIndex,
        sidechainId,
        withdrawalIndex,
        user.address,
        mainTo ?? user.address,
        expiration,
        signatures,
      )
    }

    it('normally', async () => {
      await mint()
      await deposit()
      const tx = await finalizeWithdrawal()
      expect(await token.ownerOf(tokenId)).to.equal(user.address)
      await expect(tx)
        .to.emit(bridge, 'WithdrawalFinalized')
        .withArgs(depositIndex, sidechainId, withdrawalIndex, token.address, user.address, user.address)
    })

    it('invalid chain id', async () => {
      await mint()
      await deposit()
      const tx = finalizeWithdrawal(12345)
      await expect(tx).to.be.revertedWith('Invalid main chain id.')
    })

    it('mainTo is zero address', async () => {
      await mint()
      await deposit()
      const tx = finalizeWithdrawal(undefined, zeroAddress)
      await expect(tx).to.be.revertedWith('mainTo is zero address.')
    })

    it('already withdraw', async () => {
      await mint()
      await deposit()
      await finalizeWithdrawal()
      const tx = finalizeWithdrawal()
      await expect(tx).to.be.revertedWith('already withdraw')
    })

    it('failed token transfer', async () => {
      const to = '0xbeAfbeafbEAFBeAFbeAFBEafBEAFbeaFBEAfbeaF'

      await mint()
      await deposit()

      await token.forceTransfer(to, tokenId)
      expect(await token.ownerOf(tokenId)).to.equal(to)

      const tx = await finalizeWithdrawal()
      expect(await token.ownerOf(tokenId)).to.equal(to)
      await expect(tx)
        .to.emit(bridge, 'WithdrawalFailed')
        .withArgs(depositIndex, sidechainId, withdrawalIndex, token.address, user.address, user.address)
    })
  })

  describe('transferMainchainRelayer()', () => {
    const newRelayer = '0xbeAfbeafbEAFBeAFbeAFBEafBEAFbeaFBEAfbeaF'

    const transferMainchainRelayer = async (override?: {
      nonce?: number
      contractAddress?: string
      mainchainId?: number
      newRelayer?: string
    }) => {
      const hash = getTransferMainchainRelayerHash(
        override?.nonce ?? nonce++,
        override?.contractAddress ?? relayer.address,
        override?.mainchainId ?? mainchainId,
        override?.newRelayer ?? newRelayer,
      )
      const signatures = await makeSignature(signer, hash, mainchainId, expiration)
      return relayer
        .connect(user)
        .transferMainchainRelayer(
          override?.mainchainId ?? mainchainId,
          override?.newRelayer ?? newRelayer,
          expiration,
          signatures,
        )
    }

    it('normally', async () => {
      expect(await relayer.nonce()).to.equal(0)
      expect(await bridge.owner()).to.equal(relayer.address)

      await transferMainchainRelayer()
      expect(await relayer.nonce()).to.equal(1)
      expect(await bridge.owner()).to.equal(newRelayer)
    })

    it('invalid nonce', async () => {
      await transferMainchainRelayer()

      const tx = transferMainchainRelayer({ nonce: 0 })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid to', async () => {
      await transferMainchainRelayer()

      const tx = transferMainchainRelayer({ contractAddress: zeroAddress })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid chain id', async () => {
      const tx = transferMainchainRelayer({ mainchainId: 12345 })
      await expect(tx).to.be.revertedWith('Invalid main chain id.')
    })
  })

  it('transferOwnership()', async () => {
    const tx = bridge.transferOwnership(zeroAddress)
    await expect(tx).to.be.revertedWith('Transfer is prohibited.')
  })

  it('renounceOwnership()', async () => {
    const tx = bridge.renounceOwnership()
    await expect(tx).to.be.revertedWith('Not renounceable.')
  })
})
