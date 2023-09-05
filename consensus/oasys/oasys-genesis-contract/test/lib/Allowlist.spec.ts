import { ethers } from 'hardhat'
import { Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { expect } from 'chai'

describe('Allowlist', () => {
  let allowlist: Contract
  let accounts: Account[]
  let account1: Account
  let account2: Account
  let account3: Account
  let account4: Account

  before(async () => {
    accounts = (await ethers.getSigners()).slice(0, 6)
    account1 = accounts[1]
    account2 = accounts[2]
    account3 = accounts[3]
    account4 = accounts[4]
  })

  beforeEach(async () => {
    allowlist = await (await ethers.getContractFactory('Allowlist')).deploy()
  })

  it('all methods', async () => {
    const expectAllowlist = async (expects: Account[]) => {
      const allows = expects.map((x) => x.address)
      const notAllows = accounts.filter((x) => !allows.includes(x.address)).map((x) => x.address)
      expect(await allowlist.getAllowlist()).to.eql(allows)
      for (let address of allows) {
        expect(await allowlist.containsAddress(address)).to.be.true
      }
      for (let address of notAllows) {
        expect(await allowlist.containsAddress(address)).to.be.false
      }
    }

    await expectAllowlist([])

    await allowlist.addAddress(account1.address)
    await expectAllowlist([account1])

    await allowlist.removeAddress(account1.address)
    await expectAllowlist([])

    await allowlist.addAddress(account1.address)
    await expectAllowlist([account1])

    await allowlist.addAddress(account2.address)
    await expectAllowlist([account1, account2])

    await allowlist.removeAddress(account2.address)
    await expectAllowlist([account1])

    await allowlist.removeAddress(account1.address)
    await expectAllowlist([])

    await allowlist.addAddress(account1.address)
    await expectAllowlist([account1])

    await allowlist.addAddress(account2.address)
    await expectAllowlist([account1, account2])

    await allowlist.addAddress(account3.address)
    await expectAllowlist([account1, account2, account3])

    await allowlist.removeAddress(account1.address)
    await expectAllowlist([account3, account2])

    await allowlist.removeAddress(account3.address)
    await expectAllowlist([account2])

    await allowlist.addAddress(account3.address)
    await expectAllowlist([account2, account3])

    await allowlist.addAddress(account4.address)
    await expectAllowlist([account2, account3, account4])

    await allowlist.removeAddress(account2.address)
    await expectAllowlist([account4, account3])

    await allowlist.removeAddress(account4.address)
    await expectAllowlist([account3])

    await allowlist.removeAddress(account3.address)
    await expectAllowlist([])

    await allowlist.addAddress(account1.address)
    await expectAllowlist([account1])
  })
})
