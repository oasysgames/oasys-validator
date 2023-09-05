import web3 from 'web3'
import { ethers, network } from 'hardhat'
import { Contract, BigNumber } from 'ethers'
import { SignerWithAddress as Account } from '@nomiclabs/hardhat-ethers/signers'
import { toWei, fromWei, toDecimal } from 'web3-utils'
import { toBuffer } from 'ethereumjs-util'
import { expect } from 'chai'

interface EnvironmentValue {
  startBlock: number
  startEpoch: number
  blockPeriod: number
  epochPeriod: number
  rewardRate: number
  commissionRate: number
  validatorThreshold: string
  jailThreshold: number
  jailPeriod: number
}

interface ValidatorInfo {
  operator?: string
  active: boolean
  jailed: boolean
  candidate: boolean
  stakes: BigNumber
  commissionRate: BigNumber
}

interface StakerInfo {
  stakes: BigNumber
  unstakes: BigNumber
}

const gasPrice = 0

class Validator {
  constructor(private _contract: Contract, public owner: Account, public operator: Account) {}

  stake(token: number, validator: Validator, amount: string, sender?: Account) {
    const contract = this._contract.connect(sender || this.owner)
    amount = toWei(amount)
    if (token === 0) {
      return contract.stake(validator.owner.address, token, amount, { gasPrice, value: amount })
    } else {
      return contract.stake(validator.owner.address, token, amount, { gasPrice })
    }
  }

  unstake(token: number, validator: Validator, amount: string, sender?: Account) {
    return this._contract
      .connect(sender || this.owner)
      .unstake(validator.owner.address, token, toWei(amount), { gasPrice })
  }

  joinValidator(operator?: string) {
    return this._contract.connect(this.owner).joinValidator(operator || this.operator.address, { gasPrice })
  }

  updateOperator(newOperator: string, sender?: Account) {
    return this._contract.connect(sender || this.owner).updateOperator(newOperator, { gasPrice })
  }

  activateValidator(epochs: number[], sender?: Account) {
    return this._contract.connect(sender || this.operator).activateValidator(this.owner.address, epochs, { gasPrice })
  }

  deactivateValidator(epochs: number[], sender?: Account) {
    return this._contract.connect(sender || this.operator).deactivateValidator(this.owner.address, epochs, { gasPrice })
  }

  claimCommissions(sender?: Account, epochs?: number) {
    return this._contract.connect(sender || this.owner).claimCommissions(this.owner.address, epochs ?? 0, { gasPrice })
  }

  async getInfo(epoch?: number): Promise<ValidatorInfo> {
    return await this._contract.functions['getValidatorInfo(address,uint256)'](this.owner.address, epoch ?? 0)
  }

  async slash(validator: Validator, blocks: number) {
    return await this._contract.connect(this.operator).slash(validator.operator.address, blocks, { gasPrice })
  }

  async expectStakes(
    epoch: number,
    expectStakers: Staker[],
    expectEthers: string[],
    cursor = 0,
    howMany = 100,
    expectNewCursor?: number,
  ) {
    const { _stakers, stakes, newCursor } = await this._contract.getValidatorStakes(
      this.owner.address,
      epoch,
      cursor,
      howMany,
    )
    let _expectStakers = expectStakers.map((x) => x.address)

    expect(_stakers).to.eql(_expectStakers)
    expect(stakes).to.eql(expectEthers.map((x) => toBNWei(x)))
    expect(newCursor).to.equal(expectNewCursor ?? _stakers.length)
  }

  async expectCommissions(expectEther: string, epochs?: number) {
    const actual = await this._contract.getCommissions(this.owner.address, epochs || 0)
    expect(fromWei(actual.toString())).to.match(new RegExp(`^${expectEther}`))
  }

  async expectRewards(expectEther: string, epochs?: number) {
    const actual = await this._contract.getRewards(this.owner.address, epochs || 0)
    expect(actual).to.equal(toBNWei(expectEther))
  }

  async expectSlashes(epoch: number, expectBlocks: number, expectSlashes: number) {
    const { blocks, slashes } = await this._contract.getBlockAndSlashes(this.owner.address, epoch)
    expect(blocks).to.equal(expectBlocks)
    expect(slashes).to.equal(expectSlashes)
  }
}

class Staker {
  constructor(private _contract: Contract, public signer: Account) {}

  get address(): string {
    return this.signer.address
  }

  get contract(): Contract {
    return this._contract.connect(this.signer)
  }

  stake(token: number, validator: Validator, amount: string) {
    amount = toWei(amount)
    if (token === 0) {
      return this.contract.stake(validator.owner.address, token, amount, { gasPrice, value: amount })
    } else {
      return this.contract.stake(validator.owner.address, token, amount, { gasPrice })
    }
  }

  unstake(token: number, validator: Validator, amount: string) {
    return this.contract.unstake(validator.owner.address, token, toWei(amount), { gasPrice })
  }

  claimRewards(validator: Validator, epochs: number, sender?: Account) {
    return this._contract
      .connect(sender ?? this.signer)
      .claimRewards(this.address, validator.owner.address, epochs, { gasPrice })
  }

  claimUnstakes(sender?: Account) {
    return this._contract.connect(sender ?? this.signer).claimUnstakes(this.address, { gasPrice })
  }

  async getStakes(
    epoch?: number,
    cursor = 0,
    howMany = 100,
  ): Promise<{ oasStakes: BigNumber[]; woasStakes: BigNumber[]; soasStakes: BigNumber[]; newCursor: BigNumber }> {
    return await this._contract.getStakerStakes(this.signer.address, epoch ?? 0, cursor, howMany)
  }

  async getUnstakes(): Promise<{ oasUnstakes: BigNumber; woasUnstakes: BigNumber; soasUnstakes: BigNumber }> {
    return await this._contract.getUnstakes(this.signer.address)
  }

  async expectRewards(expectEther: string, validator: Validator, epochs?: number) {
    const rewards = await this.contract.getRewards(this.address, validator.owner.address, epochs || 0)
    expect(fromWei(rewards.toString())).to.match(new RegExp(`^${expectEther}`))
  }

  async expectTotalStake(expectOAS: string, expectWOAS: string, expectSOAS: string) {
    const sum = (arr: BigNumber[]): BigNumber => {
      return arr.reduce((sum, element) => sum.add(element), BigNumber.from(0))
    }

    const { oasStakes, woasStakes, soasStakes } = await this.getStakes()
    expect(sum(oasStakes)).to.equal(toBNWei(expectOAS))
    expect(sum(woasStakes)).to.equal(toBNWei(expectWOAS))
    expect(sum(soasStakes)).to.equal(toBNWei(expectSOAS))
  }

  async expectStakes(
    epoch: number,
    expectValidators: Validator[],
    expectStakes: string[][],
    cursor = 0,
    howMany = 100,
    expectNewCursor?: number,
  ) {
    const { _validators, oasStakes, woasStakes, soasStakes, newCursor } = await this.contract.getStakerStakes(
      this.address,
      epoch,
      cursor,
      howMany,
    )

    expect(_validators).to.eql(expectValidators.map((x) => x.owner.address))
    expect(oasStakes).to.eql(expectStakes[0].map((x) => toBNWei(x)))
    expect(woasStakes).to.eql(expectStakes[1].map((x) => toBNWei(x)))
    expect(soasStakes).to.eql(expectStakes[2].map((x) => toBNWei(x)))
    expect(newCursor).to.equal(expectNewCursor ?? _validators.length)
  }
}

const getBlockNumber = async () => {
  const r = await network.provider.send('eth_getBlockByNumber', ['latest', false])
  return toDecimal(r.number)
}

const mining = async (targetBlockNumber: number) => {
  while (true) {
    if ((await getBlockNumber()) >= targetBlockNumber) return
    await network.provider.send('evm_mine')
  }
}

const fromEther = (ether: string) => BigNumber.from(toWei(ether))

const toBNWei = (ether: string) => BigNumber.from(toWei(ether))

const makeSignature = async (signer: Account, hash: string, chainid: number, expiration: number): Promise<string> => {
  const values = [
    { type: 'bytes32', value: hash },
    { type: 'uint256', value: String(chainid) },
    { type: 'uint64', value: String(expiration) },
  ]
  const msg = web3.utils.encodePacked(...values)
  return await signer.signMessage(toBuffer(msg!))
}

const makeHashWithNonce = (nonce: number, to: string, encodedSelector: string) => {
  const msg = web3.utils.encodePacked(
    { type: 'uint256', value: String(nonce) },
    { type: 'address', value: to },
    { type: 'bytes', value: encodedSelector },
  )
  return ethers.utils.keccak256(msg!)
}

const makeExpiration = (duration?: number): number => {
  const dt = new Date()
  return ~~(dt.getTime() / 1000) + (duration ?? 86400)
}

const chainid = network.config.chainId!

const zeroAddress = '0x0000000000000000000000000000000000000000'

const Token = { OAS: 0, wOAS: 1, sOAS: 2 }

const WOASAddress = '0x5200000000000000000000000000000000000001'
const SOASAddress = '0x5200000000000000000000000000000000000002'
const TestERC20Bytecode =
  '0x6080604052600436106100a75760003560e01c80633950935111610064578063395093511461016c57806370a082311461018c57806395d89b41146101c2578063a457c2d7146101d7578063a9059cbb146101f7578063dd62ed3e1461021757600080fd5b806306fdde03146100ac578063095ea7b3146100d75780631249c58b1461010757806318160ddd1461011157806323b872dd14610130578063313ce56714610150575b600080fd5b3480156100b857600080fd5b506100c161025d565b6040516100ce919061088d565b60405180910390f35b3480156100e357600080fd5b506100f76100f23660046108fe565b6102ef565b60405190151581526020016100ce565b61010f610307565b005b34801561011d57600080fd5b506002545b6040519081526020016100ce565b34801561013c57600080fd5b506100f761014b366004610928565b610313565b34801561015c57600080fd5b50604051601281526020016100ce565b34801561017857600080fd5b506100f76101873660046108fe565b610337565b34801561019857600080fd5b506101226101a7366004610964565b6001600160a01b031660009081526020819052604090205490565b3480156101ce57600080fd5b506100c1610376565b3480156101e357600080fd5b506100f76101f23660046108fe565b610385565b34801561020357600080fd5b506100f76102123660046108fe565b61041c565b34801561022357600080fd5b50610122610232366004610986565b6001600160a01b03918216600090815260016020908152604080832093909416825291909152205490565b60606003805461026c906109b9565b80601f0160208091040260200160405190810160405280929190818152602001828054610298906109b9565b80156102e55780601f106102ba576101008083540402835291602001916102e5565b820191906000526020600020905b8154815290600101906020018083116102c857829003601f168201915b5050505050905090565b6000336102fd81858561042a565b5060019392505050565b610311333461054e565b565b60003361032185828561062d565b61032c8585856106bf565b506001949350505050565b3360008181526001602090815260408083206001600160a01b03871684529091528120549091906102fd90829086906103719087906109f4565b61042a565b60606004805461026c906109b9565b3360008181526001602090815260408083206001600160a01b03871684529091528120549091908381101561040f5760405162461bcd60e51b815260206004820152602560248201527f45524332303a2064656372656173656420616c6c6f77616e63652062656c6f77604482015264207a65726f60d81b60648201526084015b60405180910390fd5b61032c828686840361042a565b6000336102fd8185856106bf565b6001600160a01b03831661048c5760405162461bcd60e51b8152602060048201526024808201527f45524332303a20617070726f76652066726f6d20746865207a65726f206164646044820152637265737360e01b6064820152608401610406565b6001600160a01b0382166104ed5760405162461bcd60e51b815260206004820152602260248201527f45524332303a20617070726f766520746f20746865207a65726f206164647265604482015261737360f01b6064820152608401610406565b6001600160a01b0383811660008181526001602090815260408083209487168084529482529182902085905590518481527f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925910160405180910390a3505050565b6001600160a01b0382166105a45760405162461bcd60e51b815260206004820152601f60248201527f45524332303a206d696e7420746f20746865207a65726f2061646472657373006044820152606401610406565b80600260008282546105b691906109f4565b90915550506001600160a01b038216600090815260208190526040812080548392906105e39084906109f4565b90915550506040518181526001600160a01b038316906000907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9060200160405180910390a35050565b6001600160a01b0383811660009081526001602090815260408083209386168352929052205460001981146106b957818110156106ac5760405162461bcd60e51b815260206004820152601d60248201527f45524332303a20696e73756666696369656e7420616c6c6f77616e63650000006044820152606401610406565b6106b9848484840361042a565b50505050565b6001600160a01b0383166107235760405162461bcd60e51b815260206004820152602560248201527f45524332303a207472616e736665722066726f6d20746865207a65726f206164604482015264647265737360d81b6064820152608401610406565b6001600160a01b0382166107855760405162461bcd60e51b815260206004820152602360248201527f45524332303a207472616e7366657220746f20746865207a65726f206164647260448201526265737360e81b6064820152608401610406565b6001600160a01b038316600090815260208190526040902054818110156107fd5760405162461bcd60e51b815260206004820152602660248201527f45524332303a207472616e7366657220616d6f756e7420657863656564732062604482015265616c616e636560d01b6064820152608401610406565b6001600160a01b038085166000908152602081905260408082208585039055918516815290812080548492906108349084906109f4565b92505081905550826001600160a01b0316846001600160a01b03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef8460405161088091815260200190565b60405180910390a36106b9565b600060208083528351808285015260005b818110156108ba5785810183015185820160400152820161089e565b818111156108cc576000604083870101525b50601f01601f1916929092016040019392505050565b80356001600160a01b03811681146108f957600080fd5b919050565b6000806040838503121561091157600080fd5b61091a836108e2565b946020939093013593505050565b60008060006060848603121561093d57600080fd5b610946846108e2565b9250610954602085016108e2565b9150604084013590509250925092565b60006020828403121561097657600080fd5b61097f826108e2565b9392505050565b6000806040838503121561099957600080fd5b6109a2836108e2565b91506109b0602084016108e2565b90509250929050565b600181811c908216806109cd57607f821691505b602082108114156109ee57634e487b7160e01b600052602260045260246000fd5b50919050565b60008219821115610a1557634e487b7160e01b600052601160045260246000fd5b50019056fea2646970667358221220c9b4e2e2bd5c3fd95e99a722b6e2ec90476d969d980d709b3308f56bbea45c4664736f6c63430008090033'

export {
  EnvironmentValue,
  ValidatorInfo,
  StakerInfo,
  Validator,
  Staker,
  getBlockNumber,
  mining,
  fromEther,
  toBNWei,
  makeSignature,
  makeHashWithNonce,
  makeExpiration,
  chainid,
  zeroAddress,
  Token,
  WOASAddress,
  SOASAddress,
  TestERC20Bytecode,
}
