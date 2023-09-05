import web3 from 'web3'
import abi from 'web3-eth-abi'
import { ethers } from 'hardhat'
import { ContractFactory, Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { expect } from 'chai'
import { chainid, makeExpiration, makeSignature, makeHashWithNonce, zeroAddress } from '../helpers'

const getAddSignerHash = (nonce: number, to: string, signer: string) => {
  const funcSig = abi.encodeFunctionSignature('addSigner(address,uint64,bytes)')
  const funcParamsSig = abi.encodeParameters(['address'], [signer])
  const encodedSelector = funcSig + funcParamsSig.slice(2)
  return makeHashWithNonce(nonce, to, encodedSelector)
}

const getRemoveSignerHash = (nonce: number, to: string, signer: string) => {
  const funcSig = abi.encodeFunctionSignature('removeSigner(address,uint64,bytes)')
  const funcParamsSig = abi.encodeParameters(['address'], [signer])
  const encodedSelector = funcSig + funcParamsSig.slice(2)
  return makeHashWithNonce(nonce, to, encodedSelector)
}

const getUpdateThresholdHash = (nonce: number, to: string, threshold: number) => {
  const funcSig = abi.encodeFunctionSignature('updateThreshold(uint256,uint64,bytes)')
  const funcParamsSig = abi.encodeParameters(['uint256'], [threshold])
  const encodedSelector = funcSig + funcParamsSig.slice(2)
  return makeHashWithNonce(nonce, to, encodedSelector)
}

describe('Signers', () => {
  let expiration = makeExpiration()

  let accounts: Account[]
  let signer1: Account
  let signer2: Account
  let factory: ContractFactory
  let nonce: number

  const getContract = async (initialSigners: Account[], initialThreshold?: number): Promise<Contract> =>
    await factory.deploy(
      initialSigners.map((x) => x.address),
      initialThreshold ?? initialSigners.length,
    )

  before(async () => {
    accounts = await ethers.getSigners()
    signer1 = accounts[18]
    signer2 = accounts[19]
    factory = await ethers.getContractFactory('Signers')
  })

  beforeEach(() => {
    nonce = 0
  })

  describe('constructor()', () => {
    it('normally', async () => {
      await getContract([accounts[0], accounts[1]])
    })

    it('signer is zero address', async () => {
      const tx = factory.deploy([zeroAddress], 1)
      await expect(tx).to.be.revertedWith('Signer is zero address.')
    })

    it('duplicate signer', async () => {
      const tx = getContract([accounts[0], accounts[0]])
      await expect(tx).to.be.revertedWith('Duplicate signer.')
    })

    it('threshold is zero', async () => {
      const tx = getContract([accounts[0]], 0)
      await expect(tx).to.be.revertedWith('Threshold is zero.')
    })
  })

  describe('verifySignatures()', () => {
    const getSignature = async (
      contract: Contract,
      signers: Account[],
      override?: { expiration?: number; chainid?: number },
    ): Promise<Uint8Array> => {
      const hash = getAddSignerHash(nonce++, contract.address, adding.address)
      const signatures = await Promise.all(
        signers.map((signer) =>
          makeSignature(signer, hash, override?.chainid ?? chainid, override?.expiration ?? expiration),
        ),
      )
      return ethers.utils.concat(signatures)
    }

    let adding: Account

    before(() => {
      adding = accounts[1]
    })

    it('signature expired', async () => {
      const contract = await getContract([signer1])
      const signatures = await getSignature(contract, [signer1])
      const tx = contract.addSigner(adding.address, 0, signatures)
      await expect(tx).to.be.revertedWith('Signature expired')
    })

    it('invalid signatures length', async () => {
      const contract = await getContract([signer1])
      const signatures = await getSignature(contract, [signer1])
      const tx = contract.addSigner(adding.address, expiration, ethers.utils.concat([signatures, '0xff']))
      await expect(tx).to.be.revertedWith('Invalid signatures length')
    })

    it('invalid chain id', async () => {
      const contract = await getContract([signer1])
      const signatures = await getSignature(contract, [signer1], { chainid: chainid + 1 })
      const tx = contract.addSigner(adding.address, expiration, signatures)
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid expiration', async () => {
      const contract = await getContract([signer1])
      const signatures = await getSignature(contract, [signer1], { expiration: 0 })
      const tx = contract.addSigner(adding.address, expiration, signatures)
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('below Threshold', async () => {
      let contract = await getContract([signer1, signer2])
      let signatures = await getSignature(contract, [signer1])
      const tx = contract.addSigner(adding.address, expiration, signatures)
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })
  })

  describe('addSigner()', async () => {
    const addSigner = async (override?: { nonce?: number; contractAddress?: string; adding?: string }) => {
      const hash = getAddSignerHash(
        override?.nonce ?? nonce++,
        override?.contractAddress ?? contract.address,
        override?.adding ?? adding.address,
      )
      const signatures = await makeSignature(signer1, hash, chainid, expiration)
      return contract.addSigner(override?.adding ?? adding.address, expiration, signatures)
    }

    let adding: Account
    let contract: Contract

    beforeEach(async () => {
      adding = accounts[1]
      contract = await getContract([signer1])
    })

    it('normally', async () => {
      expect(await contract.nonce()).to.equal(0)
      expect(await contract.getSigners()).to.eql([signer1.address])

      await addSigner()
      expect(await contract.nonce()).to.equal(1)
      expect(await contract.getSigners()).to.eql([signer1.address, adding.address])
    })

    it('signer is zero address', async () => {
      await addSigner()

      const tx = addSigner({ adding: zeroAddress })
      await expect(tx).to.be.revertedWith('Signer is zero address')
    })

    it('invalid nonce', async () => {
      await addSigner()

      const tx = addSigner({ nonce: 0 })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid to', async () => {
      await addSigner()

      const tx = addSigner({ contractAddress: zeroAddress })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('already added', async () => {
      await addSigner()

      const tx = addSigner()
      await expect(tx).to.be.revertedWith('already added')
    })
  })

  describe('removeSigner()', () => {
    const removeSigner = async (override?: { nonce?: number; contractAddress?: string; removing?: string }) => {
      const hash = getRemoveSignerHash(
        override?.nonce ?? nonce++,
        override?.contractAddress ?? contract.address,
        override?.removing ?? removing.address,
      )
      const signatures = await makeSignature(signer1, hash, chainid, expiration)
      return contract.removeSigner(override?.removing ?? removing.address, expiration, signatures)
    }

    let removing: Account
    let contract: Contract

    beforeEach(async () => {
      removing = signer2
      contract = await getContract([signer1, removing], 1)
    })

    it('normally', async () => {
      expect(await contract.nonce()).to.equal(0)
      expect(await contract.getSigners()).to.eql([signer1.address, removing.address])

      await removeSigner()
      expect(await contract.nonce()).to.equal(1)
      expect(await contract.getSigners()).to.eql([signer1.address])
    })

    it('invalid nonce', async () => {
      await removeSigner()

      const tx = removeSigner({ nonce: 0 })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid to', async () => {
      await removeSigner()

      const tx = removeSigner({ contractAddress: zeroAddress })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('signer shortage', async () => {
      await removeSigner()

      const tx = removeSigner({ removing: signer1.address })
      await expect(tx).to.be.revertedWith('Signer shortage.')
    })
  })

  describe('updateThreshold()', () => {
    const updateThreshold = async (
      threshold: number,
      override?: { nonce?: number; contractAddress?: string; signers?: Account[] },
    ) => {
      const hash = getUpdateThresholdHash(
        override?.nonce ?? nonce++,
        override?.contractAddress ?? contract.address,
        threshold,
      )

      const signers = override?.signers ?? [signer1]
      const signatures = ethers.utils.concat(
        await Promise.all(signers.map((signer) => makeSignature(signer, hash, chainid, expiration))),
      )

      return contract.updateThreshold(threshold, expiration, signatures)
    }

    let contract: Contract

    beforeEach(async () => {
      contract = await getContract([signer1, signer2], 1)
    })

    it('normally', async () => {
      expect(await contract.nonce()).to.equal(0)
      expect(await contract.threshold()).to.equal(1)

      await updateThreshold(2)
      expect(await contract.nonce()).to.equal(1)
      expect(await contract.threshold()).to.equal(2)
    })

    it('threshold is zero', async () => {
      await updateThreshold(2)

      const tx = updateThreshold(0)
      await expect(tx).to.be.revertedWith('Threshold is zero.')
    })

    it('invalid nonce', async () => {
      await updateThreshold(2)

      const tx = updateThreshold(1)
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('invalid to', async () => {
      await updateThreshold(2)

      const tx = updateThreshold(1, { contractAddress: zeroAddress })
      await expect(tx).to.be.revertedWith('Invalid signatures')
    })

    it('signer shortage', async () => {
      await updateThreshold(2)

      const tx = updateThreshold(3, { signers: [signer2, signer1] })
      await expect(tx).to.be.revertedWith('Signer shortage.')
    })
  })
})
