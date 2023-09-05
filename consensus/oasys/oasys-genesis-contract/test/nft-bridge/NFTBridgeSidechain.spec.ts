import abi from 'web3-eth-abi'
import { ethers, network } from 'hardhat'
import { ContractFactory, Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { expect } from 'chai'
import { chainid as sidechainId, makeExpiration, makeHashWithNonce, makeSignature, zeroAddress } from '../helpers'
import { TransactionResponse } from '@ethersproject/abstract-provider'

const mainchainId = 33333
const mainchainERC721 = '0xbeAfbeafbEAFBeAFbeAFBEafBEAFbeaFBEAfbeaF'
const sidechainERC721 = '0x8398bCD4f633C72939F9043dB78c574A91C99c0A'
const tokenName = 'test token'
const tokenSymbol = 'tt'
const tokenId = 1
const depositIndex = 0
const withdrawalIndex = 0

const getCreateSidechainERC721Hash = (
  sidechainId: number,
  mainchainId: number,
  mainchainERC721: string,
  name: string,
  symbol: string,
) => {
  const types = 'uint256,uint256,address,string,string'
  const fsig = abi.encodeFunctionSignature(`createSidechainERC721(${types})`)
  const psig = abi.encodeParameters(types.split(','), [sidechainId, mainchainId, mainchainERC721, name, symbol])
  const hash = ethers.utils.keccak256(fsig + psig.slice(2))
  return hash
}

const getFinalizeDepositHash = (
  sidechainId: number,
  mainchainId: number,
  depositIndex: number,
  mainchainERC721: string,
  tokenId: number,
  mainFrom: string,
  sideTo: string,
) => {
  const types = 'uint256,uint256,uint256,address,uint256,address,address'
  const fsig = abi.encodeFunctionSignature(`finalizeDeposit(${types})`)
  const psig = abi.encodeParameters(types.split(','), [
    sidechainId,
    mainchainId,
    depositIndex,
    mainchainERC721,
    tokenId,
    mainFrom,
    sideTo,
  ])
  const hash = ethers.utils.keccak256(fsig + psig.slice(2))
  return hash
}

const getRejectWithdrawalHash = (sidechainId: number, withdrawalIndex: number) => {
  const fsig = abi.encodeFunctionSignature('rejectWithdrawal(uint256,uint256)')
  const psig = abi.encodeParameters(['uint256', 'uint256'], [sidechainId, withdrawalIndex])
  const hash = ethers.utils.keccak256(fsig + psig.slice(2))
  return hash
}

const getTransferSidechainRelayerHash = (nonce: number, to: string, sidechainId: number, newRelayer: string) => {
  const fsig = abi.encodeFunctionSignature('transferSidechainRelayer(uint256,address)')
  const psig = abi.encodeParameters(['uint256', 'address'], [sidechainId, newRelayer])
  const encodedSelector = fsig + psig.slice(2)
  return makeHashWithNonce(nonce, to, encodedSelector)
}

describe('NFTBridgeSidechain', () => {
  const expiration = makeExpiration()

  let accounts: Account[]
  let deployer: Account
  let signer: Account
  let user: Account
  let mainFrom: Account
  let mainTo: Account

  let bridgeFactory: ContractFactory
  let relayerFactory: ContractFactory
  let tokenFactory: ContractFactory

  let bridge: Contract
  let relayer: Contract
  let token: Contract

  let nonce: number

  const createSidechainERC721 = async (opts?: {
    sidechainId?: number
    mainchainId?: number
    mainchainERC721?: string
    tokenName?: string
    tokenSymbol?: string
  }) => {
    const hash = getCreateSidechainERC721Hash(
      opts?.sidechainId ?? sidechainId,
      opts?.mainchainId ?? mainchainId,
      opts?.mainchainERC721 ?? mainchainERC721,
      opts?.tokenName ?? tokenName,
      opts?.tokenSymbol ?? tokenSymbol,
    )
    const signatures = await makeSignature(signer, hash, sidechainId, expiration)
    return relayer.createSidechainERC721(
      opts?.sidechainId ?? sidechainId,
      opts?.mainchainId ?? mainchainId,
      opts?.mainchainERC721 ?? mainchainERC721,
      opts?.tokenName ?? tokenName,
      opts?.tokenSymbol ?? tokenSymbol,
      expiration,
      signatures,
    ) as Promise<TransactionResponse>
  }

  const finalizeDeposit = async (_sidechainId?: number, _depositIndex?: number) => {
    const hash = getFinalizeDepositHash(
      _sidechainId ?? sidechainId,
      mainchainId,
      _depositIndex ?? depositIndex,
      mainchainERC721,
      tokenId,
      mainFrom.address,
      user.address,
    )
    const signatures = await makeSignature(signer, hash, sidechainId, expiration)
    return relayer.finalizeDeposit(
      _sidechainId ?? sidechainId,
      mainchainId,
      _depositIndex ?? depositIndex,
      mainchainERC721,
      tokenId,
      mainFrom.address,
      user.address,
      expiration,
      signatures,
    )
  }

  const rejectWithdrawal = async (_sidechainId?: number) => {
    const hash = getRejectWithdrawalHash(_sidechainId ?? sidechainId, withdrawalIndex)
    const signatures = await makeSignature(signer, hash, sidechainId, expiration)
    return relayer.rejectWithdrawal(_sidechainId ?? sidechainId, withdrawalIndex, expiration, signatures)
  }

  before(async () => {
    accounts = await ethers.getSigners()
    deployer = accounts[1]
    signer = accounts[2]
    user = accounts[3]
    mainFrom = accounts[4]
    mainTo = accounts[5]

    bridgeFactory = await ethers.getContractFactory('NFTBridgeSidechain')
    relayerFactory = await ethers.getContractFactory('NFTBridgeRelayer')
    tokenFactory = await ethers.getContractFactory('SidechainERC721')

    token = tokenFactory.attach(sidechainERC721)
  })

  beforeEach(async () => {
    nonce = 0
    await network.provider.send('hardhat_reset')

    bridge = await bridgeFactory.connect(deployer).deploy()
    relayer = await relayerFactory.connect(deployer).deploy(zeroAddress, bridge.address, [signer.address], 1)

    await bridge.connect(deployer).transferSidechainRelayer(sidechainId, relayer.address)
  })

  it('getSidechainERC721()', async () => {
    await createSidechainERC721()
    expect(await bridge.getSidechainERC721(mainchainId, mainchainERC721)).to.equal(sidechainERC721)
  })

  it('getWithdrawalInfo()', async () => {
    await createSidechainERC721()
    await finalizeDeposit()
    await bridge.connect(user).withdraw(sidechainERC721, tokenId, mainTo.address)
    const actual = await bridge.getWithdrawalInfo(withdrawalIndex)
    expect(actual.sidechainERC721).to.equal(sidechainERC721)
    expect(actual.tokenId).to.equal(tokenId)
    expect(actual.sideFrom).to.equal(user.address)
    expect(actual.rejected).to.be.false
  })

  describe('createSidechainERC721()', () => {
    it('normally', async () => {
      const tx = await createSidechainERC721()
      await expect(tx)
        .to.emit(bridge, 'SidechainERC721Created')
        .withArgs(mainchainId, mainchainERC721, sidechainERC721, tokenName, tokenSymbol)
    })

    it('invalid chain id', async () => {
      const tx = createSidechainERC721({ sidechainId: 12345 })
      await expect(tx).to.be.revertedWith('Invalid side chain id')
    })

    it('same chain id', async () => {
      const tx = createSidechainERC721({ mainchainId: sidechainId })
      await expect(tx).to.be.revertedWith('Same chain id')
    })

    it('already exists', async () => {
      await createSidechainERC721()
      const tx = createSidechainERC721()
      await expect(tx).to.be.revertedWith('SideChainERC721 already exists')
    })
  })

  describe('finalizeDeposit()', () => {
    it('normally', async () => {
      await createSidechainERC721()
      expect(await token.balanceOf(user.address)).to.equal(0)

      const tx = await finalizeDeposit()
      expect(await token.balanceOf(user.address)).to.equal(1)
      expect(await token.ownerOf(tokenId)).to.equal(user.address)
      await expect(tx)
        .to.emit(bridge, 'DepositFinalized')
        .withArgs(mainchainId, depositIndex, mainchainERC721, sidechainERC721, tokenId, mainFrom.address, user.address)
    })

    it('invalid chain id', async () => {
      await createSidechainERC721()
      const tx = finalizeDeposit(12345)
      await expect(tx).to.be.revertedWith('Invalid side chain id')
    })

    it('sidechain erc721 not found', async () => {
      const tx = await finalizeDeposit()
      await expect(tx)
        .to.emit(bridge, 'DepositFailed')
        .withArgs(mainchainId, depositIndex, mainchainERC721, zeroAddress, tokenId, mainFrom.address, user.address)
    })

    it('already deposited', async () => {
      await createSidechainERC721()
      await finalizeDeposit()
      const tx = finalizeDeposit()
      await expect(tx).to.be.revertedWith('Already deposited')
    })

    it('already minted', async () => {
      await createSidechainERC721()
      await finalizeDeposit()

      const tx = await finalizeDeposit(undefined, depositIndex + 1)
      await expect(tx)
        .to.emit(bridge, 'DepositFailed')
        .withArgs(
          mainchainId,
          depositIndex + 1,
          mainchainERC721,
          sidechainERC721,
          tokenId,
          mainFrom.address,
          user.address,
        )
    })
  })

  describe('withdraw()', () => {
    it('normally', async () => {
      await createSidechainERC721()
      await finalizeDeposit()
      const tx = await bridge.connect(user).withdraw(sidechainERC721, tokenId, mainTo.address)
      expect(await token.balanceOf(user.address)).to.equal(0)
      await expect(token.ownerOf(tokenId)).to.be.revertedWith('ERC721: owner query for nonexistent token')
      await expect(tx)
        .to.emit(bridge, 'WithdrawalInitiated')
        .withArgs(0, mainchainId, depositIndex, mainchainERC721, sidechainERC721, tokenId, user.address, mainTo.address)
    })

    it('invalid sidechainERC721', async () => {
      const created = await tokenFactory.deploy(
        12345,
        '0x0000000000000000000000000000000000000001',
        'danger token',
        'dt',
      )
      const tx = bridge.withdraw(created.address, tokenId, mainTo.address)
      await expect(tx).to.be.revertedWith('Invalid sidechainERC721')
    })
  })

  describe('rejectWithdrawal()', () => {
    it(' normally', async () => {
      await createSidechainERC721()
      await finalizeDeposit()
      await bridge.connect(user).withdraw(sidechainERC721, tokenId, mainTo.address)
      await expect(token.ownerOf(tokenId)).to.be.revertedWith('ERC721: owner query for nonexistent token')

      const tx = await rejectWithdrawal()
      expect(await token.ownerOf(tokenId)).to.equal(user.address)
      await expect(tx).to.emit(bridge, 'WithdrawalRejected').withArgs(withdrawalIndex, mainchainId, depositIndex)

      const info = await bridge.getWithdrawalInfo(withdrawalIndex)
      expect(info.rejected).to.be.true
    })

    it('invalid chain id', async () => {
      await createSidechainERC721()
      await finalizeDeposit()
      await bridge.connect(user).withdraw(sidechainERC721, tokenId, mainTo.address)
      const tx = rejectWithdrawal(12345)
      await expect(tx).to.be.revertedWith('Invalid side chain id')
    })

    it('already rejected', async () => {
      await createSidechainERC721()
      await finalizeDeposit()
      await bridge.connect(user).withdraw(sidechainERC721, tokenId, mainTo.address)
      await rejectWithdrawal()
      const tx = rejectWithdrawal()
      await expect(tx).to.be.revertedWith('Already rejected')
    })
  })

  describe('transferSidechainRelayer()', () => {
    const newRelayer = '0xbeAfbeafbEAFBeAFbeAFBEafBEAFbeaFBEAfbeaF'

    const transferSidechainRelayerHash = async (override?: {
      nonce?: number
      contractAddress?: string
      sidechainId?: number
      newRelayer?: string
    }) => {
      const hash = getTransferSidechainRelayerHash(
        override?.nonce ?? nonce++,
        override?.contractAddress ?? relayer.address,
        override?.sidechainId ?? sidechainId,
        override?.newRelayer ?? newRelayer,
      )
      const signatures = await makeSignature(signer, hash, sidechainId, expiration)
      return relayer
        .connect(user)
        .transferSidechainRelayer(
          override?.sidechainId ?? sidechainId,
          override?.newRelayer ?? newRelayer,
          expiration,
          signatures,
        )
    }

    it('normally', async () => {
      expect(await relayer.nonce()).to.equal(0)
      expect(await bridge.owner()).to.equal(relayer.address)

      await transferSidechainRelayerHash()
      expect(await relayer.nonce()).to.equal(1)
      expect(await bridge.owner()).to.equal(newRelayer)
    })

    it('invalid nonce', async () => {
      await transferSidechainRelayerHash()

      const tx = transferSidechainRelayerHash({ nonce: 0 })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid to', async () => {
      await transferSidechainRelayerHash()

      const tx = transferSidechainRelayerHash({ contractAddress: zeroAddress })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid chain id', async () => {
      const tx = transferSidechainRelayerHash({ sidechainId: 12345 })
      await expect(tx).to.be.revertedWith('Invalid side chain id')
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
