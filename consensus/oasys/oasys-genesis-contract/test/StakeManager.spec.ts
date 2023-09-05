import { ethers, network } from 'hardhat'
import { Contract, BigNumber } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { toWei, fromWei } from 'web3-utils'
import { expect } from 'chai'

import {
  EnvironmentValue,
  Validator,
  Staker,
  mining,
  zeroAddress,
  Token,
  WOASAddress,
  SOASAddress,
  TestERC20Bytecode,
} from './helpers'

const initialEnv: EnvironmentValue = {
  startBlock: 0,
  startEpoch: 0,
  blockPeriod: 15,
  epochPeriod: 240,
  rewardRate: 10,
  commissionRate: 0,
  validatorThreshold: toWei('500'),
  jailThreshold: 50,
  jailPeriod: 2,
}

const gasPrice = 0

describe('StakeManager', () => {
  let accounts: Account[]
  let stakeManager: Contract
  let environment: Contract
  let allowlist: Contract
  let woas: Contract
  let soas: Contract

  let deployer: Account

  let validator1: Validator
  let validator2: Validator
  let validator3: Validator
  let validator4: Validator
  let fixedValidator: Validator
  let validators: Validator[]

  let staker1: Staker
  let staker2: Staker
  let staker3: Staker
  let staker4: Staker
  let staker5: Staker
  let staker6: Staker
  let stakers: Staker[]

  let currentBlock = 0

  const expectCurrentValidators = async (
    expValidators: Validator[],
    expCandidates: boolean[],
    expStakes?: string[],
  ) => {
    await expectValidators(await getEpoch(0), expValidators, expCandidates, expStakes)
  }

  const expectNextValidators = async (expValidators: Validator[], expCandidates: boolean[], expStakes?: string[]) => {
    await expectValidators(await getEpoch(1), expValidators, expCandidates, expStakes)
  }

  const expectValidators = async (
    epoch: number,
    expValidators: Validator[],
    expCandidates: boolean[],
    expStakes?: string[],
    cursor = 0,
    howMany = 100,
    expNewCursor?: number,
  ) => {
    const { owners, operators, stakes, candidates, newCursor } = await stakeManager.getValidators(
      epoch,
      cursor,
      howMany,
    )
    expect(owners).to.eql(expValidators.map((x) => x.owner.address))
    expect(operators).to.eql(expValidators.map((x) => x.operator.address))
    expect(candidates).to.eql(expCandidates)
    if (expStakes) {
      expect(stakes.map((x: any) => fromWei(x.toString()))).to.eql(expStakes)
    }
    expect(newCursor).to.equal(expNewCursor ?? owners.length)
  }

  const expectBalance = async (
    holder: Contract | Account,
    expectOAS: string,
    expectWOAS: string,
    expectSOAS: string,
  ) => {
    const actualOAS = fromWei((await stakeManager.provider.getBalance(holder.address)).toString())
    const actualWOAS = fromWei((await woas.balanceOf(holder.address)).toString())
    const actualSOAS = fromWei((await soas.balanceOf(holder.address)).toString())
    expect(actualOAS).to.match(new RegExp(`^${expectOAS}`))
    expect(actualWOAS).to.match(new RegExp(`^${expectWOAS}`))
    expect(actualSOAS).to.match(new RegExp(`^${expectSOAS}`))
  }

  const allowAddress = async (validator: Validator) => {
    await allowlist.connect(deployer).addAddress(validator.owner.address, { gasPrice })
  }

  const initializeContracts = async () => {
    await environment.initialize(initialEnv, { gasPrice })
    await stakeManager.initialize(environment.address, allowlist.address, { gasPrice })
  }

  const initializeValidators = async () => {
    await Promise.all(validators.map((x) => allowAddress(x)))
    await Promise.all(validators.map((x) => x.joinValidator()))
    await fixedValidator.stake(Token.OAS, fixedValidator, '500')
  }

  const initialize = async () => {
    await initializeContracts()
    await initializeValidators()
  }

  const toNextEpoch = async () => {
    currentBlock += initialEnv.epochPeriod
    await mining(currentBlock)
  }

  const getEpoch = async (incr: number) => {
    return (await environment.epoch()).toNumber() + incr
  }

  const setCoinbase = async (address: string) => {
    const current = await network.provider.send('eth_coinbase')
    await network.provider.send('hardhat_setCoinbase', [address])
    return async () => await network.provider.send('hardhat_setCoinbase', [current])
  }

  const updateEnvironment = async (diff: object) => {
    const restoreCoinbase = await setCoinbase(fixedValidator.operator.address)
    await environment
      .connect(fixedValidator.operator)
      .updateValue({ ...(await environment.value()), ...diff }, { gasPrice })
    await restoreCoinbase()
  }

  const slash = async (validator: Validator, target: Validator, count: number) => {
    const env = await environment.value()
    const { operators, candidates } = await stakeManager.getValidators(await getEpoch(0), 0, 100)
    const blocks = ~~(env.epochPeriod / operators.filter((_: any, i: number) => candidates[i]).length)

    const restoreCoinbase = await setCoinbase(validator.operator.address)
    await Promise.all([...Array(count).keys()].map((_) => validator.slash(target, blocks)))
    await restoreCoinbase()
  }

  before(async () => {
    accounts = await ethers.getSigners()
    deployer = accounts[0]
  })

  beforeEach(async () => {
    await network.provider.send('hardhat_reset')
    await network.provider.send('hardhat_setCoinbase', [accounts[0].address])
    await network.provider.send('hardhat_setCode', [WOASAddress, TestERC20Bytecode])
    await network.provider.send('hardhat_setCode', [SOASAddress, TestERC20Bytecode])

    environment = await (await ethers.getContractFactory('Environment')).connect(deployer).deploy()
    allowlist = await (await ethers.getContractFactory('Allowlist')).connect(deployer).deploy()
    stakeManager = await (await ethers.getContractFactory('StakeManager')).connect(deployer).deploy()

    validator1 = new Validator(stakeManager, accounts[1], accounts[2])
    validator2 = new Validator(stakeManager, accounts[3], accounts[4])
    validator3 = new Validator(stakeManager, accounts[5], accounts[6])
    validator4 = new Validator(stakeManager, accounts[7], accounts[8])
    fixedValidator = new Validator(stakeManager, accounts[9], accounts[10])
    validators = [validator1, validator2, validator3, validator4, fixedValidator]

    staker1 = new Staker(stakeManager, accounts[11])
    staker2 = new Staker(stakeManager, accounts[12])
    staker3 = new Staker(stakeManager, accounts[13])
    staker4 = new Staker(stakeManager, accounts[14])
    staker5 = new Staker(stakeManager, accounts[15])
    staker6 = new Staker(stakeManager, accounts[16])
    stakers = [staker1, staker2, staker3, staker4, staker5, staker6]

    woas = (await ethers.getContractFactory('TestERC20')).attach(WOASAddress)
    soas = (await ethers.getContractFactory('TestERC20')).attach(SOASAddress)
    await Promise.all(
      stakers.map(
        (x) =>
          new Promise(async (resolve) => {
            const value = toWei('1000')
            await woas.connect(x.signer).mint({ gasPrice, value })
            await woas.connect(x.signer).approve(stakeManager.address, value, { gasPrice })
            await soas.connect(x.signer).mint({ gasPrice, value })
            await soas.connect(x.signer).approve(stakeManager.address, value, { gasPrice })
            resolve(true)
          }),
      ),
    )

    currentBlock = 0
  })

  it('initialize()', async () => {
    await initialize()
    await expect(initialize()).to.revertedWith('AlreadyInitialized()')
  })

  describe('validator owner or operator functions', () => {
    let validator: Validator
    let owner: Account
    let operator: Account
    let attacker: Account

    before(() => {
      owner = accounts[accounts.length - 1]
      operator = accounts[accounts.length - 2]
      attacker = accounts[accounts.length - 3]
    })

    beforeEach(async () => {
      validator = new Validator(stakeManager, owner, operator)
      await initializeContracts()
    })

    it('joinValidator()', async () => {
      let tx = validator.joinValidator(zeroAddress)
      await expect(tx).to.revertedWith('UnauthorizedValidator()')

      await allowAddress(validator)

      tx = validator.joinValidator(zeroAddress)
      await expect(tx).to.revertedWith('EmptyAddress()')

      tx = validator.joinValidator(owner.address)
      await expect(tx).to.revertedWith('SameAsOwner()')

      await validator.joinValidator()

      tx = validator.joinValidator()
      await expect(tx).to.revertedWith('AlreadyJoined()')
    })

    it('updateOperator()', async () => {
      const newOperator = accounts[accounts.length - 3]

      await allowAddress(validator)
      await validator.joinValidator()

      let tx = validator.updateOperator(zeroAddress)
      await expect(tx).to.revertedWith('EmptyAddress()')

      tx = validator.updateOperator(owner.address)
      await expect(tx).to.revertedWith('SameAsOwner()')

      // from owner
      await validator.updateOperator(newOperator.address)
      expect((await validator.getInfo()).operator).to.equal(newOperator.address)

      // from operator
      tx = validator.updateOperator(operator.address, operator)
      await expect(tx).to.revertedWith('ValidatorDoesNotExist()')

      // from attacker
      tx = validator.updateOperator(attacker.address, attacker)
      await expect(tx).to.revertedWith('ValidatorDoesNotExist()')
    })

    it('deactivateValidator() and activateValidator()', async () => {
      await allowAddress(validator)
      await validator.joinValidator()
      await staker1.stake(Token.OAS, validator, '500')

      await expectValidators(await getEpoch(0), [validator], [false], ['0'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      // from owner
      await validator.deactivateValidator([await getEpoch(1)], owner)

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [false], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.false

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await validator.deactivateValidator([await getEpoch(2), await getEpoch(3), await getEpoch(5)], owner)
      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      await expectValidators(await getEpoch(2), [validator], [false], ['500'])
      await expectValidators(await getEpoch(3), [validator], [false], ['500'])
      await expectValidators(await getEpoch(4), [validator], [true], ['500'])
      await expectValidators(await getEpoch(5), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await validator.activateValidator([await getEpoch(3)])
      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      await expectValidators(await getEpoch(2), [validator], [false], ['500'])
      await expectValidators(await getEpoch(3), [validator], [true], ['500'])
      await expectValidators(await getEpoch(4), [validator], [true], ['500'])
      await expectValidators(await getEpoch(5), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [false], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.false

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [false], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.false

      await toNextEpoch()

      // from operator
      await validator.deactivateValidator([await getEpoch(1)], operator)

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [false], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [false], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.false

      await toNextEpoch()

      await expectValidators(await getEpoch(0), [validator], [true], ['500'])
      await expectValidators(await getEpoch(1), [validator], [true], ['500'])
      expect((await validator.getInfo()).active).to.be.true

      // from attacker
      let tx = validator.deactivateValidator([await getEpoch(1)], attacker)
      await expect(tx).to.revertedWith('UnauthorizedSender()')

      tx = validator.activateValidator([await getEpoch(1)], attacker)
      await expect(tx).to.revertedWith('UnauthorizedSender()')
    })

    it('claimCommissions()', async () => {
      await allowAddress(validator)
      await validator.joinValidator()
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 50 })

      await staker1.stake(Token.OAS, validator, '500')
      await staker1.stake(Token.wOAS, validator, '250')
      await staker1.stake(Token.sOAS, validator, '250')

      await expectBalance(stakeManager, '500', '250', '250')
      await expectBalance(validator.owner, '10000', '0', '0')

      await toNextEpoch()
      await toNextEpoch()

      // from owner
      await validator.claimCommissions(owner)
      await expectBalance(stakeManager, '499.994292237442922375', '250', '250')
      await expectBalance(validator.owner, '10000.005707762557077625', '0', '0')

      await toNextEpoch()
      await toNextEpoch()

      // from operator
      await validator.claimCommissions(operator)
      await expectBalance(stakeManager, '499.982876712328767125', '250', '250')
      await expectBalance(validator.owner, '10000.017123287671232875', '0', '0')

      await toNextEpoch()
      await toNextEpoch()

      // from outsider
      await validator.claimCommissions(attacker)
      await expectBalance(stakeManager, '499.971461187214611875', '250', '250')
      await expectBalance(validator.owner, '10000.028538812785388125', '0', '0')
    })
  })

  describe('staker functions', () => {
    beforeEach(async () => {
      await initialize()
    })

    it('stake()', async () => {
      await expectBalance(stakeManager, '500', '0', '0')
      await expectBalance(staker1.signer, '8000', '1000', '1000')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator1, '5')
      await staker1.stake(Token.wOAS, validator1, '2.5')
      await staker1.stake(Token.sOAS, validator1, '2.5')

      await expectBalance(stakeManager, '505', '2.5', '2.5')
      await expectBalance(staker1.signer, '7995', '997.5', '997.5')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await expectBalance(stakeManager, '505', '2.5', '2.5')
      await expectBalance(staker1.signer, '7995', '997.5', '997.5')
      await staker1.expectTotalStake('5', '2.5', '2.5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['5', '0', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator1, '20')
      await staker1.stake(Token.OAS, validator1, '10')
      await staker1.stake(Token.wOAS, validator1, '10')
      await staker1.stake(Token.sOAS, validator1, '10')

      await expectBalance(stakeManager, '535', '12.5', '12.5')
      await expectBalance(staker1.signer, '7965', '987.5', '987.5')
      await staker1.expectTotalStake('5', '2.5', '2.5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['5', '0', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await expectBalance(stakeManager, '535', '12.5', '12.5')
      await expectBalance(staker1.signer, '7965', '987.5', '987.5')
      await staker1.expectTotalStake('35', '12.5', '12.5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['35', '0', '0', '0', '0'],
          ['12.5', '0', '0', '0', '0'],
          ['12.5', '0', '0', '0', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator1, '20')
      await staker1.stake(Token.wOAS, validator1, '10')
      await staker1.stake(Token.sOAS, validator1, '10')
      await staker1.stake(Token.OAS, validator2, '20')
      await staker1.stake(Token.wOAS, validator2, '10')
      await staker1.stake(Token.sOAS, validator2, '10')

      await toNextEpoch()

      await expectBalance(stakeManager, '575', '32.5', '32.5')
      await expectBalance(staker1.signer, '7925', '967.5', '967.5')
      await staker1.expectTotalStake('75', '32.5', '32.5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['55', '20', '0', '0', '0'],
          ['22.5', '10', '0', '0', '0'],
          ['22.5', '10', '0', '0', '0'],
        ],
      )
    })

    it('unstake() and claimUnstakes()', async () => {
      await expectBalance(stakeManager, '500', '0', '0')
      await expectBalance(staker1.signer, '8000', '1000', '1000')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator1, '5')
      await staker1.stake(Token.wOAS, validator1, '5')
      await staker1.stake(Token.OAS, validator2, '10')
      await staker1.stake(Token.sOAS, validator2, '10')

      await toNextEpoch()

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('15', '5', '10')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['5', '10', '0', '0', '0'],
          ['5', '0', '0', '0', '0'],
          ['0', '10', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator1, '2.5')
      await staker1.unstake(Token.wOAS, validator1, '2.5')
      await staker1.unstake(Token.sOAS, validator1, '2.5')

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('15', '5', '10')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['5', '10', '0', '0', '0'],
          ['5', '0', '0', '0', '0'],
          ['0', '10', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('12.5', '2.5', '10')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['2.5', '10', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
          ['0', '10', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator1, '1')
      await staker1.unstake(Token.OAS, validator1, '1')
      await staker1.unstake(Token.wOAS, validator1, '1')
      await staker1.unstake(Token.wOAS, validator1, '1')

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('12.5', '2.5', '10')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['2.5', '10', '0', '0', '0'],
          ['2.5', '0', '0', '0', '0'],
          ['0', '10', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('10.5', '0.5', '10')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0.5', '10', '0', '0', '0'],
          ['0.5', '0', '0', '0', '0'],
          ['0', '10', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator1, '9999')
      await staker1.unstake(Token.wOAS, validator1, '9999')
      await staker1.unstake(Token.OAS, validator2, '5')
      await staker1.unstake(Token.sOAS, validator2, '5')

      await toNextEpoch()

      await expectBalance(stakeManager, '515', '5', '10')
      await expectBalance(staker1.signer, '7985', '995', '990')
      await staker1.expectTotalStake('5', '0', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      await staker1.claimUnstakes()

      await expectBalance(stakeManager, '505', '0', '5')
      await expectBalance(staker1.signer, '7995', '1000', '995')
      await staker1.expectTotalStake('5', '0', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      // testing for immediate refunds
      await staker1.stake(Token.OAS, validator2, '5')
      await staker1.stake(Token.wOAS, validator2, '10')
      await staker1.stake(Token.sOAS, validator2, '15')

      await expectBalance(stakeManager, '510', '10', '20')
      await expectBalance(staker1.signer, '7990', '990', '980')
      await staker1.expectTotalStake('5', '0', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator2, '10')
      await staker1.unstake(Token.wOAS, validator2, '15')
      await staker1.unstake(Token.sOAS, validator2, '20')

      await expectBalance(stakeManager, '505', '0', '5')
      await expectBalance(staker1.signer, '7995', '1000', '995')
      await staker1.expectTotalStake('5', '0', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      // cannot be claim until the next epoch
      await staker1.claimUnstakes()

      await expectBalance(stakeManager, '505', '0', '5')
      await expectBalance(staker1.signer, '7995', '1000', '995')
      await staker1.expectTotalStake('5', '0', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      await toNextEpoch()
      await staker1.claimUnstakes()

      await expectBalance(stakeManager, '500', '0', '0')
      await expectBalance(staker1.signer, '8000', '1000', '1000')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      // check for double claim
      await staker1.claimUnstakes()
      await toNextEpoch()
      await staker1.claimUnstakes()

      await expectBalance(stakeManager, '500', '0', '0')
      await expectBalance(staker1.signer, '8000', '1000', '1000')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator2, '10')
      await staker1.stake(Token.wOAS, validator2, '15')
      await staker1.stake(Token.sOAS, validator2, '20')

      await expectBalance(stakeManager, '510', '15', '20')
      await expectBalance(staker1.signer, '7990', '985', '980')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator2, '5')
      await staker1.unstake(Token.wOAS, validator2, '10')
      await staker1.unstake(Token.sOAS, validator2, '15')

      await expectBalance(stakeManager, '505', '5', '5')
      await expectBalance(staker1.signer, '7995', '995', '995')
      await staker1.expectTotalStake('0', '0', '0')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await expectBalance(stakeManager, '505', '5', '5')
      await expectBalance(staker1.signer, '7995', '995', '995')
      await staker1.expectTotalStake('5', '5', '5')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator2, '5')

      await expectBalance(stakeManager, '505', '5', '5')
      await expectBalance(staker1.signer, '7995', '995', '995')
      await expectBalance(staker2.signer, '8000', '1000', '1000')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '5', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )
      await staker2.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      // claim from outsider
      await toNextEpoch()
      await staker1.claimUnstakes(staker2.signer)

      await expectBalance(stakeManager, '500', '5', '5')
      await expectBalance(staker1.signer, '8000', '995', '995')
      await expectBalance(staker2.signer, '8000', '1000', '1000')
      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
          ['0', '5', '0', '0', '0'],
        ],
      )
      await staker2.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )
    })
  })

  describe('rewards and commissions', () => {
    beforeEach(async () => {
      await initialize()
      await updateEnvironment({ startEpoch: await getEpoch(1), jailThreshold: 120 })
      await toNextEpoch()
    })

    it('when operating ratio is 100%', async () => {
      const startingEpoch = await getEpoch(0)

      await staker1.stake(Token.OAS, validator1, '1000')
      await staker2.stake(Token.OAS, validator1, '2000')
      await toNextEpoch()

      await staker1.claimRewards(validator1, 0)
      await staker2.claimRewards(validator1, 0)
      await validator1.claimCommissions()

      await toNextEpoch() // 1
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 10 })
      await staker1.stake(Token.wOAS, validator1, '500')
      await staker2.stake(Token.sOAS, validator1, '500')
      await toNextEpoch() // 2
      await toNextEpoch() // 3
      await toNextEpoch() // 4
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 50 })
      await staker1.stake(Token.wOAS, validator1, '250')
      await staker1.stake(Token.sOAS, validator1, '250')
      await staker2.stake(Token.wOAS, validator1, '250')
      await staker2.stake(Token.sOAS, validator1, '250')
      await toNextEpoch() // 5
      await toNextEpoch() // 6
      await toNextEpoch() // 7
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 100 })
      await toNextEpoch() // 8
      await toNextEpoch() // 9
      await staker1.unstake(Token.OAS, validator1, '500')
      await staker1.unstake(Token.wOAS, validator1, '500')
      await staker2.unstake(Token.OAS, validator1, '500')
      await staker2.unstake(Token.sOAS, validator1, '500')
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 10 })
      await toNextEpoch() // 10
      await updateEnvironment({ startEpoch: startingEpoch + 12, rewardRate: 50 })

      await staker1.expectRewards('0.01141552', validator1, 1)
      await staker2.expectRewards('0.02283105', validator1, 1)
      await validator1.expectCommissions('0', 1)

      await staker1.expectRewards('0.02283105', validator1, 2)
      await staker2.expectRewards('0.04566210', validator1, 2)
      await validator1.expectCommissions('0', 2)

      await staker1.expectRewards('0.03824200', validator1, 3)
      await staker2.expectRewards('0.07134703', validator1, 3)
      await validator1.expectCommissions('0.00456621', 3)

      await staker1.expectRewards('0.05365296', validator1, 4)
      await staker2.expectRewards('0.09703196', validator1, 4)
      await validator1.expectCommissions('0.00913242', 4)

      await staker1.expectRewards('0.06906392', validator1, 5)
      await staker2.expectRewards('0.12271689', validator1, 5)
      await validator1.expectCommissions('0.01369863', 5)

      await staker1.expectRewards('0.08047945', validator1, 6)
      await staker2.expectRewards('0.13984018', validator1, 6)
      await validator1.expectCommissions('0.04223744', 6)

      await staker1.expectRewards('0.09189497', validator1, 7)
      await staker2.expectRewards('0.15696347', validator1, 7)
      await validator1.expectCommissions('0.07077625', 7)

      await staker1.expectRewards('0.10331050', validator1, 8)
      await staker2.expectRewards('0.17408675', validator1, 8)
      await validator1.expectCommissions('0.09931506', 8)

      await staker1.expectRewards('0.10331050', validator1, 9)
      await staker2.expectRewards('0.17408675', validator1, 9)
      await validator1.expectCommissions('0.15639269', 9)

      await staker1.expectRewards('0.10331050', validator1, 10)
      await staker2.expectRewards('0.17408675', validator1, 10)
      await validator1.expectCommissions('0.21347031', 10)

      await staker1.expectRewards('0.10331050', validator1, 0)
      await staker2.expectRewards('0.17408675', validator1, 0)
      await validator1.expectCommissions('0.21347031', 0)

      await staker1.expectRewards('0.10331050', validator1, 99)
      await staker2.expectRewards('0.17408675', validator1, 99)
      await validator1.expectCommissions('0.21347031', 99)

      await staker1.claimRewards(validator1, 5)
      await staker2.claimRewards(validator1, 5)
      await validator1.claimCommissions(undefined, 5)

      const check1 = async () => {
        await expectBalance(staker1.signer, '7000.06906392', '250', '750')
        await expectBalance(staker2.signer, '6000.12271689', '750', '250')
        await expectBalance(validator1.owner, '10000.01369863', '0', '0')

        await staker1.expectRewards('0.01141552', validator1, 1)
        await staker2.expectRewards('0.01712328', validator1, 1)
        await validator1.expectCommissions('0.02853881', 1)

        await staker1.expectRewards('0.02283105', validator1, 2)
        await staker2.expectRewards('0.03424657', validator1, 2)
        await validator1.expectCommissions('0.05707762', 2)

        await staker1.expectRewards('0.03424657', validator1, 3)
        await staker2.expectRewards('0.05136986', validator1, 3)
        await validator1.expectCommissions('0.08561643', 3)

        await staker1.expectRewards('0.03424657', validator1, 4)
        await staker2.expectRewards('0.05136986', validator1, 4)
        await validator1.expectCommissions('0.14269406', 4)

        await staker1.expectRewards('0.03424657', validator1, 5)
        await staker2.expectRewards('0.05136986', validator1, 5)
        await validator1.expectCommissions('0.19977168', 5)
      }

      await check1()

      await toNextEpoch() // 11 (6)
      await toNextEpoch() // 12 (7)
      await toNextEpoch() // 13 (8)

      await check1()

      await staker1.expectRewards('0.04452054', validator1, 6)
      await staker2.expectRewards('0.07191780', validator1, 6)
      await validator1.expectCommissions('0.20319634', 6)

      await staker1.expectRewards('0.0958904', validator1, 7)
      await staker2.expectRewards('0.17465753', validator1, 7)
      await validator1.expectCommissions('0.22031963', 7)

      await staker1.expectRewards('0.14726027', validator1, 8)
      await staker2.expectRewards('0.27739726', validator1, 8)
      await validator1.expectCommissions('0.23744292', 8)

      const check2 = async () => {
        await staker1.claimRewards(validator1, 0)
        await staker2.claimRewards(validator1, 0)
        await validator1.claimCommissions(undefined, 0)

        await expectBalance(staker1.signer, '7000.21632420', '250', '750')
        await expectBalance(staker2.signer, '6000.40011415', '750', '250')
        await expectBalance(validator1.owner, '10000.25114155', '0', '0')

        await staker1.expectRewards('0', validator1, 0)
        await staker2.expectRewards('0', validator1, 0)
        await validator1.expectCommissions('0', 0)
      }

      await check2()

      // check for double claim
      await check2()

      await toNextEpoch()

      // claim from outsider
      await staker1.claimRewards(validator1, 0, staker2.signer)
      await expectBalance(staker1.signer, '7000.26769406', '250', '750')
      await expectBalance(staker2.signer, '6000.40011415', '750', '250')
    })

    it('when operating ratio is 50%', async () => {
      await staker1.stake(Token.OAS, validator1, '1000')
      await staker2.stake(Token.OAS, validator1, '2000')
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 10 })
      await toNextEpoch()

      await staker1.claimRewards(validator1, 0)
      await staker2.claimRewards(validator1, 0)
      await validator1.claimCommissions()

      await toNextEpoch() // 1
      await slash(validator1, validator1, 60)
      await toNextEpoch() // 2
      await toNextEpoch() // 3
      await slash(validator1, validator1, 60)
      await toNextEpoch() // 4

      await staker1.expectRewards('0.01027397', validator1, 1)
      await staker2.expectRewards('0.02054794', validator1, 1)
      await validator1.expectCommissions('0.00342465', 1)

      await staker1.expectRewards('0.01541095', validator1, 2)
      await staker2.expectRewards('0.03082191', validator1, 2)
      await validator1.expectCommissions('0.00513698', 2)

      await staker1.expectRewards('0.02568493', validator1, 3)
      await staker2.expectRewards('0.05136986', validator1, 3)
      await validator1.expectCommissions('0.00856164', 3)

      await staker1.expectRewards('0.03082191', validator1, 4)
      await staker2.expectRewards('0.06164383', validator1, 4)
      await validator1.expectCommissions('0.01027397', 4)
    })

    it('when operating ratio is 0% and jailed', async () => {
      await staker1.stake(Token.OAS, validator1, '1000')
      await staker2.stake(Token.OAS, validator1, '2000')
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 10 })
      await toNextEpoch()

      await staker1.claimRewards(validator1, 0)
      await staker2.claimRewards(validator1, 0)
      await validator1.claimCommissions()

      await toNextEpoch() // 1
      await slash(validator1, validator1, 120)
      await toNextEpoch() // 2 (operating ratio is 0%)
      await toNextEpoch() // 3 (jailed)
      await toNextEpoch() // 4 (jailed)
      await toNextEpoch() // 5
      await toNextEpoch() // 6
      await slash(validator1, validator1, 120)
      await toNextEpoch() // 7 (operating ratio is 0%)
      await toNextEpoch() // 8 (jailed)
      await toNextEpoch() // 9 (jailed)
      await toNextEpoch() // 10

      await staker1.expectRewards('0.01027397', validator1, 1)
      await staker2.expectRewards('0.02054794', validator1, 1)
      await validator1.expectCommissions('0.00342465', 1)

      await staker1.expectRewards('0.01027397', validator1, 2)
      await staker2.expectRewards('0.02054794', validator1, 2)
      await validator1.expectCommissions('0.00342465', 2)

      await staker1.expectRewards('0.01027397', validator1, 3)
      await staker2.expectRewards('0.02054794', validator1, 3)
      await validator1.expectCommissions('0.00342465', 3)

      await staker1.expectRewards('0.01027397', validator1, 4)
      await staker2.expectRewards('0.02054794', validator1, 4)
      await validator1.expectCommissions('0.00342465', 4)

      await staker1.expectRewards('0.02054794', validator1, 5)
      await staker2.expectRewards('0.04109589', validator1, 5)
      await validator1.expectCommissions('0.0068493', 5)

      await staker1.expectRewards('0.03082191', validator1, 6)
      await staker2.expectRewards('0.06164383', validator1, 6)
      await validator1.expectCommissions('0.01027397', 6)

      await staker1.expectRewards('0.03082191', validator1, 7)
      await staker2.expectRewards('0.06164383', validator1, 7)
      await validator1.expectCommissions('0.01027397', 7)

      await staker1.expectRewards('0.03082191', validator1, 8)
      await staker2.expectRewards('0.06164383', validator1, 8)
      await validator1.expectCommissions('0.01027397', 8)

      await staker1.expectRewards('0.03082191', validator1, 9)
      await staker2.expectRewards('0.06164383', validator1, 9)
      await validator1.expectCommissions('0.01027397', 9)

      await staker1.expectRewards('0.04109589', validator1, 10)
      await staker2.expectRewards('0.08219178', validator1, 10)
      await validator1.expectCommissions('0.01369863', 10)
    })

    it('when inactive', async () => {
      await staker1.stake(Token.OAS, validator1, '1000')
      await staker2.stake(Token.OAS, validator1, '2000')
      await updateEnvironment({ startEpoch: await getEpoch(1), commissionRate: 10 })
      await toNextEpoch()

      await staker1.claimRewards(validator1, 0)
      await staker2.claimRewards(validator1, 0)
      await validator1.claimCommissions()

      const epoch = await getEpoch(0)
      await validator1.deactivateValidator([
        epoch + 1,
        epoch + 2,
        epoch + 3,
        epoch + 4,
        epoch + 5,
        epoch + 6,
        epoch + 7,
        epoch + 8,
      ])
      await validator1.activateValidator([epoch + 4, epoch + 5])

      await toNextEpoch() // 1
      await toNextEpoch() // 2 (inactive)
      await toNextEpoch() // 3 (inactive)
      await toNextEpoch() // 4 (inactive)
      await toNextEpoch() // 5
      await toNextEpoch() // 6
      await toNextEpoch() // 7 (inactive)
      await toNextEpoch() // 8 (inactive)
      await toNextEpoch() // 9 (inactive)
      await toNextEpoch() // 10

      await staker1.expectRewards('0.01027397', validator1, 1)
      await staker2.expectRewards('0.02054794', validator1, 1)
      await validator1.expectCommissions('0.00342465', 1)

      await staker1.expectRewards('0.01027397', validator1, 2)
      await staker2.expectRewards('0.02054794', validator1, 2)
      await validator1.expectCommissions('0.00342465', 2)

      await staker1.expectRewards('0.01027397', validator1, 3)
      await staker2.expectRewards('0.02054794', validator1, 3)
      await validator1.expectCommissions('0.00342465', 3)

      await staker1.expectRewards('0.01027397', validator1, 4)
      await staker2.expectRewards('0.02054794', validator1, 4)
      await validator1.expectCommissions('0.00342465', 4)

      await staker1.expectRewards('0.02054794', validator1, 5)
      await staker2.expectRewards('0.04109589', validator1, 5)
      await validator1.expectCommissions('0.0068493', 5)

      await staker1.expectRewards('0.03082191', validator1, 6)
      await staker2.expectRewards('0.06164383', validator1, 6)
      await validator1.expectCommissions('0.01027397', 6)

      await staker1.expectRewards('0.03082191', validator1, 7)
      await staker2.expectRewards('0.06164383', validator1, 7)
      await validator1.expectCommissions('0.01027397', 7)

      await staker1.expectRewards('0.03082191', validator1, 8)
      await staker2.expectRewards('0.06164383', validator1, 8)
      await validator1.expectCommissions('0.01027397', 8)

      await staker1.expectRewards('0.03082191', validator1, 9)
      await staker2.expectRewards('0.06164383', validator1, 9)
      await validator1.expectCommissions('0.01027397', 9)

      await staker1.expectRewards('0.04109589', validator1, 10)
      await staker2.expectRewards('0.08219178', validator1, 10)
      await validator1.expectCommissions('0.01369863', 10)
    })
  })

  describe('current validator functions', () => {
    beforeEach(async () => {
      await initialize()
    })

    it('slash()', async () => {
      const startingEpoch = await getEpoch(0)
      await updateEnvironment({
        startEpoch: startingEpoch + 1,
        jailThreshold: 50,
      })
      await toNextEpoch()

      await staker1.stake(Token.OAS, validator1, '500')
      await staker1.stake(Token.OAS, validator2, '500')
      await staker1.stake(Token.OAS, validator3, '500')

      await expectCurrentValidators(validators, [false, false, false, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, true, true, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])

      await slash(validator1, validator2, 49)

      await expectCurrentValidators(validators, [true, true, true, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, true, true, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])

      await slash(validator1, validator2, 50)

      await expectCurrentValidators(validators, [true, true, true, false, true])
      await expectNextValidators(validators, [true, false, true, false, true])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, false, true, false, true])
      await expectNextValidators(validators, [true, false, true, false, true])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, false, true, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, true, true, false, true])
      await expectNextValidators(validators, [true, true, true, false, true])
    })
  })

  describe('view functions', () => {
    beforeEach(async () => {
      await initialize()
    })

    it('getValidators()', async () => {
      await expectCurrentValidators(validators, [false, false, false, false, false], ['0', '0', '0', '0', '0'])
      await expectNextValidators(validators, [false, false, false, false, true], ['0', '0', '0', '0', '500'])

      await staker1.stake(Token.OAS, validator1, '501')
      await staker1.stake(Token.OAS, validator2, '499')
      await staker1.stake(Token.OAS, validator3, '502')

      await expectCurrentValidators(validators, [false, false, false, false, false], ['0', '0', '0', '0', '0'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      await validator1.deactivateValidator([await getEpoch(1)])

      await expectCurrentValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      await slash(validator1, validator1, 50)

      await expectCurrentValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [false, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      await toNextEpoch()

      await expectCurrentValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])
      await expectNextValidators(validators, [true, false, true, false, true], ['501', '499', '502', '0', '500'])

      // check pagination
      // howMany = 2
      await expectValidators(await getEpoch(0), [validator1, validator2], [true, false], ['501', '499'], 0, 2, 2)
      await expectValidators(await getEpoch(0), [validator3, validator4], [true, false], ['502', '0'], 2, 2, 4)
      await expectValidators(await getEpoch(0), [fixedValidator], [true], ['500'], 4, 2, 5)

      // howMany = 4
      await expectValidators(
        await getEpoch(0),
        [validator1, validator2, validator3, validator4],
        [true, false, true, false],
        ['501', '499', '502', '0'],
        0,
        4,
        4,
      )
      await expectValidators(await getEpoch(0), [fixedValidator], [true], ['500'], 4, 2, 5)

      // howMany = 10
      await expectValidators(
        await getEpoch(0),
        [validator1, validator2, validator3, validator4, fixedValidator],
        [true, false, true, false, true],
        ['501', '499', '502', '0', '500'],
        0,
        10,
        5,
      )
    })

    it('getValidatorOwners()', async () => {
      const _expect = (
        result: { owners: string[]; newCursor: BigNumber },
        expectOwners: Validator[],
        expectNewCursor: number,
      ) => {
        expect(result.owners).to.eql(expectOwners.map((x) => x.owner.address))
        expect(result.newCursor).to.equal(expectNewCursor)
      }

      // howMany = 2
      _expect(await stakeManager.getValidatorOwners(0, 2), [validator1, validator2], 2)
      _expect(await stakeManager.getValidatorOwners(2, 2), [validator3, validator4], 4)
      _expect(await stakeManager.getValidatorOwners(4, 2), [fixedValidator], 5)

      // howMany = 3
      _expect(await stakeManager.getValidatorOwners(0, 3), [validator1, validator2, validator3], 3)
      _expect(await stakeManager.getValidatorOwners(3, 3), [validator4, fixedValidator], 5)

      // howMany = 10
      _expect(
        await stakeManager.getValidatorOwners(0, 10),
        [validator1, validator2, validator3, validator4, fixedValidator],
        5,
      )
    })

    it('getStakers()', async () => {
      const _expect = (
        result: { _stakers: string[]; newCursor: BigNumber },
        expectStakers: Account[],
        expectNewCursor: number,
      ) => {
        expect(result._stakers).to.eql(expectStakers.map((x) => x.address))
        expect(result.newCursor).to.equal(expectNewCursor)
      }

      await staker1.stake(Token.OAS, validator1, '1')
      await staker2.stake(Token.OAS, validator1, '2')
      await staker3.stake(Token.OAS, validator1, '3')
      await staker4.stake(Token.OAS, validator1, '4')
      await staker5.stake(Token.OAS, validator1, '5')

      // howMany = 2
      _expect(await stakeManager.getStakers(0, 2), [fixedValidator.owner, staker1.signer], 2)
      _expect(await stakeManager.getStakers(2, 2), [staker2.signer, staker3.signer], 4)
      _expect(await stakeManager.getStakers(4, 2), [staker4.signer, staker5.signer], 6)
      _expect(await stakeManager.getStakers(6, 2), [], 6)

      // howMany = 3
      _expect(await stakeManager.getStakers(0, 3), [fixedValidator.owner, staker1.signer, staker2.signer], 3)
      _expect(await stakeManager.getStakers(3, 3), [staker3.signer, staker4.signer, staker5.signer], 6)
      _expect(await stakeManager.getStakers(6, 3), [], 6)

      // howMany = 10
      _expect(
        await stakeManager.getStakers(0, 10),
        [fixedValidator.owner, staker1.signer, staker2.signer, staker3.signer, staker4.signer, staker5.signer],
        6,
      )
    })

    it('getValidatorInfo()', async () => {
      const checker = async (active: boolean, jailed: boolean, candidate: boolean, stakes: string, epoch?: number) => {
        const acutal = await validator1.getInfo(epoch)
        if (!epoch) {
          expect(acutal.operator).to.equal(validator1.operator.address)
        }
        expect(acutal.active).to.equal(active)
        expect(acutal.jailed).to.equal(jailed)
        expect(acutal.candidate).to.equal(candidate)
        expect(fromWei(acutal.stakes.toString())).to.eql(stakes)
      }

      await checker(true, false, false, '0') // epoch 1
      await checker(true, false, false, '0', 1)
      await checker(true, false, false, '0', 2)
      await checker(true, false, false, '0', 3)

      await staker1.stake(Token.OAS, validator1, '500')
      await staker2.stake(Token.OAS, validator1, '250')

      await checker(true, false, false, '0') // epoch 1
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)

      await toNextEpoch()

      await checker(true, false, true, '750') // epoch 2
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, true, '750', 4)

      await toNextEpoch()

      await checker(true, false, true, '750') // epoch 3
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, true, '750', 4)
      await checker(true, false, true, '750', 5)

      await staker1.unstake(Token.OAS, validator1, '200')
      await staker2.unstake(Token.OAS, validator1, '100')

      await checker(true, false, true, '750') // epoch 3
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, false, '450', 5)

      await toNextEpoch()

      await checker(true, false, false, '450') // epoch 4
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, false, '450', 5)
      await checker(true, false, false, '450', 6)

      await staker1.stake(Token.OAS, validator1, '50')

      await checker(true, false, false, '450') // epoch 4
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(true, false, true, '500', 6)

      await toNextEpoch()

      await checker(true, false, true, '500') // epoch 5
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(true, false, true, '500', 6)
      await checker(true, false, true, '500', 7)

      // mark
      const epoch: number = await getEpoch(0)
      await validator1.deactivateValidator([epoch + 1, epoch + 2])

      await checker(true, false, true, '500') // epoch 5
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)

      await toNextEpoch()

      await checker(false, false, false, '500') // epoch 6
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)

      await toNextEpoch()

      await checker(false, false, false, '500') // epoch 7
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, false, true, '500', 9)

      await toNextEpoch()

      await checker(true, false, true, '500') // epoch 8
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, false, true, '500', 9)
      await checker(true, false, true, '500', 10)

      await slash(validator1, validator1, 50)

      await checker(true, false, true, '500') // epoch 8
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, true, false, '500', 9)
      await checker(true, true, false, '500', 10)
      await checker(true, false, true, '500', 11)

      await toNextEpoch()

      await checker(true, true, false, '500') // epoch 9
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, true, false, '500', 9)
      await checker(true, true, false, '500', 10)
      await checker(true, false, true, '500', 11)

      await toNextEpoch()

      await checker(true, true, false, '500') // epoch 10
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, true, false, '500', 9)
      await checker(true, true, false, '500', 10)
      await checker(true, false, true, '500', 11)

      await toNextEpoch()

      await checker(true, false, true, '500') // epoch 11
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, true, false, '500', 9)
      await checker(true, true, false, '500', 10)
      await checker(true, false, true, '500', 11)
      await checker(true, false, true, '500', 12)

      await toNextEpoch()

      await checker(true, false, true, '500') // epoch 12
      await checker(true, false, false, '0', 1)
      await checker(true, false, true, '750', 2)
      await checker(true, false, true, '750', 3)
      await checker(true, false, false, '450', 4)
      await checker(true, false, true, '500', 5)
      await checker(false, false, false, '500', 6)
      await checker(false, false, false, '500', 7)
      await checker(true, false, true, '500', 8)
      await checker(true, true, false, '500', 9)
      await checker(true, true, false, '500', 10)
      await checker(true, false, true, '500', 11)
      await checker(true, false, true, '500', 12)
    })

    it('getUnstakes()', async () => {
      const check = async (expOAS: number, expWOAS: number, expSOAS: number) => {
        const acutal = await staker1.getUnstakes()
        expect(fromWei(acutal.oasUnstakes.toString())).to.eql('' + expOAS)
        expect(fromWei(acutal.woasUnstakes.toString())).to.eql('' + expWOAS)
        expect(fromWei(acutal.soasUnstakes.toString())).to.eql('' + expSOAS)
      }

      await check(0, 0, 0)

      await staker1.stake(Token.OAS, validator1, '10')
      await staker1.stake(Token.wOAS, validator1, '20')
      await staker1.stake(Token.sOAS, validator1, '30')

      await toNextEpoch()
      await check(0, 0, 0)

      await staker1.unstake(Token.OAS, validator1, '1')

      await toNextEpoch()
      await check(1, 0, 0)

      await staker1.unstake(Token.wOAS, validator1, '2')

      await toNextEpoch()
      await check(1, 2, 0)

      await staker1.unstake(Token.sOAS, validator1, '3')

      await toNextEpoch()
      await check(1, 2, 3)

      await staker1.claimUnstakes()
      await check(0, 0, 0)
    })

    it('getValidatorStakes()', async () => {
      await staker1.stake(Token.OAS, validator1, '10')
      await toNextEpoch()

      await staker2.stake(Token.OAS, validator1, '20')
      await toNextEpoch()

      await staker1.stake(Token.OAS, validator1, '30')
      await staker3.stake(Token.OAS, validator1, '30')
      await toNextEpoch()

      await staker4.stake(Token.OAS, validator1, '40')
      await staker5.stake(Token.OAS, validator1, '50')
      await staker6.stake(Token.OAS, validator1, '60')
      await toNextEpoch()

      await staker2.unstake(Token.OAS, validator1, '20')
      await toNextEpoch()

      await validator1.expectStakes(
        1,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['0', '0', '0', '0', '0', '0'],
      )
      await validator1.expectStakes(
        2,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['10', '0', '0', '0', '0', '0'],
      )
      await validator1.expectStakes(
        3,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['10', '20', '0', '0', '0', '0'],
      )
      await validator1.expectStakes(
        4,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['40', '20', '30', '0', '0', '0'],
      )
      await validator1.expectStakes(
        5,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['40', '20', '30', '40', '50', '60'],
      )
      await validator1.expectStakes(
        6,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['40', '0', '30', '40', '50', '60'],
      )
      await validator1.expectStakes(
        0,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['40', '0', '30', '40', '50', '60'],
      )

      // check pagination
      // howMany = 2
      await validator1.expectStakes(0, [staker1, staker2], ['40', '0'], 0, 2, 2)
      await validator1.expectStakes(0, [staker3, staker4], ['30', '40'], 2, 2, 4)
      await validator1.expectStakes(0, [staker5, staker6], ['50', '60'], 4, 2, 6)
      await validator1.expectStakes(0, [], [], 6, 2, 6)

      // howMany = 3
      await validator1.expectStakes(0, [staker1, staker2, staker3], ['40', '0', '30'], 0, 3, 3)
      await validator1.expectStakes(0, [staker4, staker5, staker6], ['40', '50', '60'], 3, 3, 6)
      await validator1.expectStakes(0, [], [], 6, 3, 6)

      // howMany = 10
      await validator1.expectStakes(
        0,
        [staker1, staker2, staker3, staker4, staker5, staker6],
        ['40', '0', '30', '40', '50', '60'],
        0,
        10,
        6,
      )
    })

    it('getStakerStakes()', async () => {
      await staker1.stake(Token.OAS, validator1, '5')
      await staker1.stake(Token.OAS, validator1, '5')
      await staker1.stake(Token.OAS, validator2, '10')
      await staker1.stake(Token.wOAS, validator2, '10')
      await staker1.stake(Token.OAS, validator4, '15')
      await staker1.stake(Token.wOAS, validator4, '15')
      await staker1.stake(Token.OAS, validator4, '20')
      await staker1.stake(Token.sOAS, validator4, '20')

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await toNextEpoch()

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['10', '10', '0', '35', '0'],
          ['0', '10', '0', '15', '0'],
          ['0', '0', '0', '20', '0'],
        ],
      )

      await staker1.unstake(Token.OAS, validator1, '1')
      await staker1.unstake(Token.OAS, validator1, '1')
      await staker1.unstake(Token.wOAS, validator1, '1')
      await staker1.unstake(Token.sOAS, validator1, '1')
      await staker1.unstake(Token.OAS, validator2, '2')
      await staker1.unstake(Token.wOAS, validator2, '1')
      await staker1.unstake(Token.OAS, validator4, '3')
      await staker1.unstake(Token.sOAS, validator4, '1')

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['10', '10', '0', '35', '0'],
          ['0', '10', '0', '15', '0'],
          ['0', '0', '0', '20', '0'],
        ],
      )

      await toNextEpoch()

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '32', '0'],
          ['0', '9', '0', '15', '0'],
          ['0', '0', '0', '19', '0'],
        ],
      )

      await staker1.stake(Token.wOAS, validator1, '1')
      await staker1.stake(Token.sOAS, validator2, '2')
      await staker1.stake(Token.OAS, validator4, '3')
      await staker1.stake(Token.OAS, validator1, '10')
      await staker1.stake(Token.wOAS, validator2, '20')
      await staker1.stake(Token.sOAS, validator4, '30')
      await staker1.unstake(Token.OAS, validator1, '10')
      await staker1.unstake(Token.wOAS, validator2, '20')
      await staker1.unstake(Token.sOAS, validator4, '30')

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '32', '0'],
          ['0', '9', '0', '15', '0'],
          ['0', '0', '0', '19', '0'],
        ],
      )

      await toNextEpoch()

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '35', '0'],
          ['1', '9', '0', '15', '0'],
          ['0', '2', '0', '19', '0'],
        ],
      )

      await staker1.stake(Token.OAS, validator1, '10')
      await staker1.stake(Token.wOAS, validator2, '20')
      await staker1.stake(Token.sOAS, validator4, '30')

      await staker1.unstake(Token.OAS, validator1, '5')
      await staker1.unstake(Token.wOAS, validator2, '10')
      await staker1.unstake(Token.sOAS, validator4, '15')

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '35', '0'],
          ['1', '9', '0', '15', '0'],
          ['0', '2', '0', '19', '0'],
        ],
      )

      await toNextEpoch()

      await staker1.expectStakes(
        0,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['13', '8', '0', '35', '0'],
          ['1', '19', '0', '15', '0'],
          ['0', '2', '0', '34', '0'],
        ],
      )

      await staker1.expectStakes(
        1,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
          ['0', '0', '0', '0', '0'],
        ],
      )

      await staker1.expectStakes(
        2,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['10', '10', '0', '35', '0'],
          ['0', '10', '0', '15', '0'],
          ['0', '0', '0', '20', '0'],
        ],
      )

      await staker1.expectStakes(
        3,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '32', '0'],
          ['0', '9', '0', '15', '0'],
          ['0', '0', '0', '19', '0'],
        ],
      )

      await staker1.expectStakes(
        4,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '35', '0'],
          ['1', '9', '0', '15', '0'],
          ['0', '2', '0', '19', '0'],
        ],
      )

      // check pagination
      // howMany = 2
      await staker1.expectStakes(
        4,
        [validator1, validator2],
        [
          ['8', '8'],
          ['1', '9'],
          ['0', '2'],
        ],
        0,
        2,
        2,
      )
      await staker1.expectStakes(
        4,
        [validator3, validator4],
        [
          ['0', '35'],
          ['0', '15'],
          ['0', '19'],
        ],
        2,
        2,
        4,
      )
      await staker1.expectStakes(4, [fixedValidator], [['0'], ['0'], ['0']], 4, 2, 5)

      // howMany = 3
      await staker1.expectStakes(
        4,
        [validator1, validator2, validator3],
        [
          ['8', '8', '0'],
          ['1', '9', '0'],
          ['0', '2', '0'],
        ],
        0,
        3,
        3,
      )
      await staker1.expectStakes(
        4,
        [validator4, fixedValidator],
        [
          ['35', '0'],
          ['15', '0'],
          ['19', '0'],
        ],
        3,
        3,
        5,
      )

      // howMany = 10
      await staker1.expectStakes(
        4,
        [validator1, validator2, validator3, validator4, fixedValidator],
        [
          ['8', '8', '0', '35', '0'],
          ['1', '9', '0', '15', '0'],
          ['0', '2', '0', '19', '0'],
        ],
        0,
        10,
        5,
      )
    })

    it('getBlockAndSlashes()', async () => {
      await staker1.stake(Token.OAS, validator1, '500')
      await staker1.stake(Token.OAS, validator2, '500')

      await toNextEpoch()
      await toNextEpoch()

      await slash(validator1, validator2, 10)

      await toNextEpoch()
      await toNextEpoch()
      await toNextEpoch()

      await slash(validator1, validator2, 20)

      await toNextEpoch()

      await validator2.expectSlashes(1, 0, 0)
      await validator2.expectSlashes(2, 0, 0)
      await validator2.expectSlashes(3, 80, 10)
      await validator2.expectSlashes(4, 0, 0)
      await validator2.expectSlashes(5, 0, 0)
      await validator2.expectSlashes(6, 80, 20)
      await validator2.expectSlashes(7, 0, 0) // current epoch
    })

    it('getTotalStake()', async () => {
      const checker = async (epoch: number, expect_: string) => {
        const actual = await stakeManager.getTotalStake(epoch)
        expect(fromWei(actual.toString())).to.eql(expect_)
      }

      await checker(0, '0')
      await checker(2, '500')

      await staker1.stake(Token.OAS, validator1, '10')

      await checker(0, '0')
      await checker(2, '510')

      await toNextEpoch()

      await checker(0, '510')
      await checker(3, '510')

      await staker1.stake(Token.OAS, validator1, '20')

      await checker(0, '510')
      await checker(3, '530')

      await toNextEpoch()

      await checker(0, '530')
      await checker(4, '530')

      await staker1.unstake(Token.OAS, validator1, '1')

      await checker(0, '530')
      await checker(4, '529')

      await toNextEpoch()

      await checker(0, '529')
      await checker(5, '529')

      await staker1.unstake(Token.OAS, validator1, '2')

      await checker(0, '529')
      await checker(5, '527')

      await toNextEpoch()

      await checker(0, '527')
      await checker(6, '527')

      await staker1.stake(Token.OAS, validator1, '30')

      await checker(0, '527')
      await checker(6, '557')

      await staker1.unstake(Token.OAS, validator1, '3')

      await checker(0, '527')
      await checker(6, '554')

      await toNextEpoch()

      await checker(0, '554')
      await checker(7, '554')
    })

    it('getTotalRewards()', async () => {
      const checker = async (validators: Validator[], epochs: number, expectEther: string) => {
        let actual: BigNumber = await stakeManager.getTotalRewards(
          validators.map((x) => x.owner.address),
          epochs,
        )
        expect(fromWei(actual.toString())).to.match(new RegExp(`^${expectEther}`))
      }

      await staker1.stake(Token.OAS, validator1, '1000')
      await staker2.stake(Token.OAS, validator2, '2000')
      await toNextEpoch()

      await toNextEpoch() // 1

      await staker1.stake(Token.OAS, validator1, '500')
      await staker2.stake(Token.OAS, validator2, '500')

      await toNextEpoch() // 3
      await toNextEpoch() // 4

      await staker1.unstake(Token.OAS, validator1, '250')
      await staker2.unstake(Token.OAS, validator2, '250')

      await toNextEpoch() // 5
      await toNextEpoch() // 6

      await checker([fixedValidator], 1, '0.00570776')
      await checker([fixedValidator], 2, '0.01141552')
      await checker([fixedValidator], 3, '0.01712328')
      await checker([fixedValidator], 4, '0.02283105')
      await checker([fixedValidator], 5, '0.02853881')

      await checker([fixedValidator, validator1, validator2], 1, '0.0456621')
      await checker([fixedValidator, validator1, validator2], 2, '0.0970319')
      await checker([fixedValidator, validator1, validator2], 3, '0.1484018')
      await checker([fixedValidator, validator1, validator2], 4, '0.1883561')
      await checker([fixedValidator, validator1, validator2], 5, '0.2283105')
    })
  })
})
