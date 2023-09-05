import { ethers, network } from 'hardhat'
import { Contract, BigNumber } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { toWei } from 'web3-utils'
import { expect } from 'chai'

import { mining, EnvironmentValue, getBlockNumber } from './helpers'

const initialValue: EnvironmentValue = {
  startBlock: 0,
  startEpoch: 0,
  blockPeriod: 10,
  epochPeriod: 100,
  rewardRate: 10,
  commissionRate: 0,
  validatorThreshold: toWei('500', 'ether'),
  jailThreshold: 50,
  jailPeriod: 2,
}

const expectValues = (actual: EnvironmentValue, expect_: EnvironmentValue) => {
  expect(actual.blockPeriod).to.equal(expect_.blockPeriod)
  expect(actual.epochPeriod).to.equal(expect_.epochPeriod)
  expect(actual.rewardRate).to.equal(expect_.rewardRate)
  expect(actual.commissionRate).to.equal(expect_.commissionRate)
  expect(actual.validatorThreshold).to.equal(expect_.validatorThreshold)
  expect(actual.jailThreshold).to.equal(expect_.jailThreshold)
  expect(actual.jailPeriod).to.equal(expect_.jailPeriod)
}

describe('Environment', () => {
  let accounts: Account[]
  let environment: Contract
  let currentBlock = 0

  const toNextEpoch = async () => {
    currentBlock += initialValue.epochPeriod
    await mining(currentBlock - 2)
    await mining(currentBlock)
  }

  const initialize = async () => {
    await environment.initialize(initialValue)
    await toNextEpoch()
  }

  before(async () => {
    accounts = await ethers.getSigners()
  })

  beforeEach(async () => {
    await network.provider.send('hardhat_reset')
    await network.provider.send('hardhat_setCoinbase', [accounts[0].address])
    environment = await (await ethers.getContractFactory('Environment')).deploy()
    currentBlock = 0
  })

  it('initialize()', async () => {
    await initialize()
    await expect(initialize()).to.revertedWith('AlreadyInitialized()')
  })

  describe('updateValue()', async () => {
    it('startEpoch is past', async () => {
      await initialize()

      const tx = environment.updateValue(initialValue)
      await expect(tx).to.revertedWith('PastEpoch()')
    })
  })

  describe('epoch()', () => {
    const updateValue = async (startEpoch: number, epochPeriod: number) => {
      const value = { ...initialValue, startEpoch, epochPeriod }
      return await environment.updateValue(value)
    }

    const expectEpoch = async (start: number, end: number, expect_: number) => {
      for (let i = start; i <= end; i++) {
        await mining(i)
        expect(await environment.epoch()).to.equal(expect_)
      }
    }

    beforeEach(async () => {
      await initialize()
    })

    it('simple case', async () => {
      await expectEpoch(100, 199, 2)

      await updateValue(4, 150)

      await expectEpoch(200, 299, 3)
      await expectEpoch(300, 449, 4)

      await updateValue(6, 50)

      await expectEpoch(450, 599, 5)
      await expectEpoch(600, 649, 6)
      await expectEpoch(650, 699, 7)
    })

    it('update in last block of epoch', async () => {
      await expectEpoch(100, 198, 2)
      await expect(updateValue(3, 150)).to.revertedWith('OnlyNotLastBlock()')
    })

    it('update in first block of epoch', async () => {
      await expectEpoch(100, 199, 2)

      await updateValue(4, 150)

      await expectEpoch(200, 299, 3)
      await expectEpoch(300, 349, 4)
    })

    it('overwriting the same epoch', async () => {
      await expectEpoch(100, 199, 2)

      await updateValue(4, 150)

      await expectEpoch(200, 250, 3)

      await updateValue(4, 200)

      await expectEpoch(200, 299, 3)

      await expectEpoch(300, 499, 4)
      await expectEpoch(500, 699, 5)
    })

    it('overwriting the same epoch', async () => {
      await expectEpoch(100, 199, 2)

      await updateValue(4, 150)

      await expectEpoch(200, 250, 3)

      await updateValue(5, 200)

      await expectEpoch(250, 299, 3)

      await expectEpoch(300, 399, 4)
      await expectEpoch(400, 599, 5)
    })
  })

  it('value() and nextValue()', async () => {
    type expect = {
      start: number
      end: number
      method: () => Promise<EnvironmentValue>
      value: EnvironmentValue
    }

    const miningAndExpect = async (end: number, expects: expect[]) => {
      const start = (await getBlockNumber()) + 1
      expect(end > start).to.be.true

      for (let block = start; block <= end; block++) {
        await mining(block)

        for (let expect of expects) {
          if (block >= expect.start && block <= expect.end) {
            expectValues(await expect.method(), expect.value)
          }
        }
      }
    }

    await initialize()

    let newValue1 = { ...initialValue }
    newValue1.startEpoch = 4
    newValue1.epochPeriod = 50
    newValue1.rewardRate += 1
    await environment.updateValue(newValue1)

    await miningAndExpect(299, [
      { start: 0, end: 299, method: environment.value, value: initialValue },
      { start: 0, end: 199, method: environment.nextValue, value: initialValue },
      { start: 200, end: 299, method: environment.nextValue, value: newValue1 },
    ])

    const newValue2 = { ...newValue1 }
    newValue2.startEpoch = 6
    newValue2.epochPeriod = 25
    newValue2.rewardRate += 1
    await environment.updateValue(newValue2)

    await miningAndExpect(399, [
      { start: 300, end: 399, method: environment.value, value: newValue1 },
      { start: 300, end: 349, method: environment.nextValue, value: newValue1 },
      { start: 350, end: 399, method: environment.nextValue, value: newValue2 },
    ])

    const newValue3 = { ...newValue2 }
    newValue3.startEpoch = 10
    newValue3.rewardRate += 1
    await environment.updateValue(newValue3)

    await miningAndExpect(499, [
      { start: 400, end: 499, method: environment.value, value: newValue2 },
      { start: 400, end: 474, method: environment.nextValue, value: newValue2 },
      { start: 475, end: 499, method: environment.nextValue, value: newValue3 },
    ])

    const newValue4 = { ...newValue3 }
    newValue4.startEpoch = 12
    newValue4.rewardRate += 1
    await environment.updateValue(newValue4)

    await miningAndExpect(539, [
      { start: 500, end: 539, method: environment.value, value: newValue3 },
      { start: 500, end: 524, method: environment.nextValue, value: newValue3 },
      { start: 525, end: 539, method: environment.nextValue, value: newValue4 },
    ])

    const newValue5 = { ...newValue4 }
    newValue5.startEpoch = 14
    newValue5.rewardRate += 1
    await environment.updateValue(newValue5)

    await miningAndExpect(599, [
      { start: 540, end: 599, method: environment.value, value: newValue3 },
      { start: 540, end: 574, method: environment.nextValue, value: newValue3 },
      { start: 575, end: 599, method: environment.nextValue, value: newValue5 },
    ])

    await miningAndExpect(649, [
      { start: 600, end: 649, method: environment.value, value: newValue5 },
      { start: 600, end: 649, method: environment.nextValue, value: newValue5 },
    ])
  })

  it('findValue()', async () => {
    const value1 = { ...initialValue, startEpoch: 4, rewardRate: 1 }
    const value2 = { ...initialValue, startEpoch: 7, rewardRate: 2 }
    const value3 = { ...initialValue, startEpoch: 10, rewardRate: 3 }

    await initialize()

    await environment.updateValue(value1)
    await mining(300)

    await environment.updateValue(value2)
    await mining(700)

    await environment.updateValue(value3)
    await mining(1000)

    expectValues(await environment.findValue(1), initialValue)
    expectValues(await environment.findValue(2), initialValue)
    expectValues(await environment.findValue(3), initialValue)

    expectValues(await environment.findValue(4), value1)
    expectValues(await environment.findValue(5), value1)
    expectValues(await environment.findValue(6), value1)

    expectValues(await environment.findValue(7), value2)
    expectValues(await environment.findValue(8), value2)
    expectValues(await environment.findValue(9), value2)

    expectValues(await environment.findValue(10), value3)
    expectValues(await environment.findValue(11), value3)
    expectValues(await environment.findValue(12), value3)
    expectValues(await environment.findValue(13), value3)
  })
})
