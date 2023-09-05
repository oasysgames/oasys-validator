import { ethers, network } from 'hardhat'
import { Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { toWei, fromWei } from 'web3-utils'
import { expect } from 'chai'

describe('WOAS', () => {
  let woas: Contract
  let account1: Account
  let account2: Account

  const expectBalance = async (account: Account, expectOAS: string, expectWOAS: string) => {
    const actualOAS = fromWei((await account.getBalance()).toString())
    const actualWOAS = fromWei((await woas.balanceOf(account.address)).toString())
    expect(actualOAS).to.match(new RegExp(`^${expectOAS}`))
    expect(actualWOAS).to.match(new RegExp(`^${expectWOAS}`))
  }

  before(async () => {
    const accounts = await ethers.getSigners()
    account1 = accounts[1]
    account2 = accounts[2]
  })

  beforeEach(async () => {
    await network.provider.send('hardhat_reset')
    woas = await (await ethers.getContractFactory('WOAS')).deploy()
  })

  it('deposit()', async () => {
    await expectBalance(account1, '10000', '0')

    await woas.connect(account1).deposit({ value: toWei('50') })
    await expectBalance(account1, '9950', '50')

    await woas.connect(account1).deposit({ value: toWei('50') })
    await expectBalance(account1, '9900', '100')
  })

  it('withdraw(uint256)', async () => {
    await expectBalance(account1, '10000', '0')

    await woas.connect(account1).deposit({ value: toWei('100') })
    await expectBalance(account1, '9900', '100')

    await woas.connect(account1)['withdraw(uint256)'](toWei('50'))
    await expectBalance(account1, '9950', '50')

    await woas.connect(account1)['withdraw(uint256)'](toWei('50'))
    await expectBalance(account1, '10000', '0')

    const tx = woas.connect(account1)['withdraw(uint256)'](toWei('1'))
    await expect(tx).to.be.revertedWith('over amount')
  })

  it('withdraw(uint256,address)', async () => {
    await expectBalance(account1, '10000', '0')
    await expectBalance(account2, '10000', '0')

    await woas.connect(account1).deposit({ value: toWei('100') })
    await expectBalance(account1, '9900', '100')
    await expectBalance(account2, '10000', '0')

    await woas.connect(account1)['withdraw(uint256,address)'](toWei('50'), account2.address)
    await expectBalance(account1, '9900', '50')
    await expectBalance(account2, '10050', '0')

    await woas.connect(account1)['withdraw(uint256,address)'](toWei('50'), account2.address)
    await expectBalance(account1, '9900', '0')
    await expectBalance(account2, '10100', '0')

    const tx = woas.connect(account1)['withdraw(uint256,address)'](toWei('1'), account2.address)
    await expect(tx).to.be.revertedWith('over amount')
  })
})
