import { ethers, network } from 'hardhat'
import { Contract } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { toWei, fromWei } from 'web3-utils'
import { expect } from 'chai'

import { zeroAddress } from '../helpers'

const getTimestamp = (dates: string): number => {
  const date = new Date(dates)
  return date.getTime() / 1000
}

const setBlockTimestamp = async (dates: string) => {
  await network.provider.send('evm_setNextBlockTimestamp', [getTimestamp(dates)])
}

describe('SOAS', () => {
  let soas: Contract
  let genesis: Account
  let user: Account

  before(async () => {
    const accounts = await ethers.getSigners()
    genesis = accounts[1]
    user = accounts[2]
  })

  beforeEach(async () => {
    await network.provider.send('hardhat_reset')
    soas = await (await ethers.getContractFactory('SOAS')).deploy(zeroAddress)
  })

  it('claim()', async () => {
    const expectBalance = async (expectOAS: string, expectSOAS: string) => {
      const actualOAS = fromWei((await user.getBalance()).toString())
      const actualSOAS = fromWei((await soas.balanceOf(user.address)).toString())
      expect(actualOAS).to.match(new RegExp(`^${expectOAS}`))
      expect(actualSOAS).to.match(new RegExp(`^${expectSOAS}`))
    }

    // initial balance.
    await expectBalance('10000', '0')

    // minting.
    await setBlockTimestamp('2100/01/01')
    await soas.connect(genesis).mint(user.address, getTimestamp('2100/07/01'), getTimestamp('2100/12/31'), {
      value: toWei('100', 'ether'),
    })

    // after minted.
    await expectBalance('10000', '100')

    // 1 month elapsed.
    await setBlockTimestamp('2100/07/31')
    await soas.connect(user).claim(toWei('16', 'ether'))
    await expectBalance('10016', '84')

    // 2 month elapsed.
    await setBlockTimestamp('2100/08/31')
    await soas.connect(user).claim(toWei('16', 'ether'))
    await expectBalance('10032', '68')

    // 3 month elapsed.
    await setBlockTimestamp('2100/09/31')
    await soas.connect(user).claim(toWei('16', 'ether'))
    await expectBalance('10048', '52')

    // 4 month elapsed.
    await setBlockTimestamp('2100/10/31')
    await soas.connect(user).claim(toWei('16', 'ether'))
    await expectBalance('10064', '36')

    // 5 month elapsed.
    await setBlockTimestamp('2100/11/31')
    await soas.connect(user).claim(toWei('16', 'ether'))
    await expectBalance('10080', '20')

    // 6 month elapsed.
    await setBlockTimestamp('2100/12/31')
    await soas.connect(user).claim(toWei('20', 'ether'))
    await expectBalance('10100', '0')

    // insufficient balance.
    await setBlockTimestamp('2101/01/01')
    await expect(soas.connect(user).claim(toWei('0.00001', 'ether'))).to.revertedWith('OverAmount()')
  })
})
