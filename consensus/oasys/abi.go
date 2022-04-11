package oasys

const environmentAbi = `
[
  {
    "inputs": [],
    "name": "epoch",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "epochAndValues",
    "outputs": [
      {
        "internalType": "uint256[]",
        "name": "epochs",
        "type": "uint256[]"
      },
      {
        "components": [
          {
            "internalType": "uint256",
            "name": "startBlock",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "startEpoch",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "blockPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "epochPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "rewardRate",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "validatorThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailPeriod",
            "type": "uint256"
          }
        ],
        "internalType": "struct Environment.EnvironmentValue[]",
        "name": "_values",
        "type": "tuple[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "components": [
          {
            "internalType": "uint256",
            "name": "startBlock",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "startEpoch",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "blockPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "epochPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "rewardRate",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "validatorThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailPeriod",
            "type": "uint256"
          }
        ],
        "internalType": "struct Environment.EnvironmentValue",
        "name": "initialValue",
        "type": "tuple"
      }
    ],
    "name": "initialize",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "initialized",
    "outputs": [
      {
        "internalType": "bool",
        "name": "",
        "type": "bool"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "isFirstBlock",
    "outputs": [
      {
        "internalType": "bool",
        "name": "",
        "type": "bool"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "isLastBlock",
    "outputs": [
      {
        "internalType": "bool",
        "name": "",
        "type": "bool"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "nextValue",
    "outputs": [
      {
        "components": [
          {
            "internalType": "uint256",
            "name": "startBlock",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "startEpoch",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "blockPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "epochPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "rewardRate",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "validatorThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailPeriod",
            "type": "uint256"
          }
        ],
        "internalType": "struct Environment.EnvironmentValue",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "components": [
          {
            "internalType": "uint256",
            "name": "startBlock",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "startEpoch",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "blockPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "epochPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "rewardRate",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "validatorThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailPeriod",
            "type": "uint256"
          }
        ],
        "internalType": "struct Environment.EnvironmentValue",
        "name": "newValue",
        "type": "tuple"
      }
    ],
    "name": "updateValue",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "updates",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "value",
    "outputs": [
      {
        "components": [
          {
            "internalType": "uint256",
            "name": "startBlock",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "startEpoch",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "blockPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "epochPeriod",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "rewardRate",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "validatorThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailThreshold",
            "type": "uint256"
          },
          {
            "internalType": "uint256",
            "name": "jailPeriod",
            "type": "uint256"
          }
        ],
        "internalType": "struct Environment.EnvironmentValue",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "values",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "startBlock",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "startEpoch",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "blockPeriod",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "epochPeriod",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "rewardRate",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "validatorThreshold",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "jailThreshold",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "jailPeriod",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  }
]
`

const stakeManagerAbi = `
[
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "staker",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "amount",
        "type": "uint256"
      }
    ],
    "name": "Staked",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "staker",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "amount",
        "type": "uint256"
      }
    ],
    "name": "Unstaked",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "ValidatorActivated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "rate",
        "type": "uint256"
      }
    ],
    "name": "ValidatorCommissionRateUpdated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "ValidatorDeactivated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "epoch",
        "type": "uint256"
      }
    ],
    "name": "ValidatorJailed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "ValidatorSlashed",
    "type": "event"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "activateValidator",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epochs",
        "type": "uint256"
      }
    ],
    "name": "claimCommissions",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epochs",
        "type": "uint256"
      }
    ],
    "name": "claimRewards",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "claimUnstakes",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "currentValidators",
    "outputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "deactivateValidator",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "environment",
    "outputs": [
      {
        "internalType": "contract Environment",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epoch",
        "type": "uint256"
      }
    ],
    "name": "getBlockAndSlashes",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "blocks",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "slashes",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epochs",
        "type": "uint256"
      }
    ],
    "name": "getCommissions",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "commissions",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "getCurrentValidators",
    "outputs": [
      {
        "internalType": "address[]",
        "name": "owners",
        "type": "address[]"
      },
      {
        "internalType": "address[]",
        "name": "operators",
        "type": "address[]"
      },
      {
        "internalType": "uint256[]",
        "name": "stakes",
        "type": "uint256[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "staker",
        "type": "address"
      },
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epochs",
        "type": "uint256"
      }
    ],
    "name": "getRewards",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "rewards",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "staker",
        "type": "address"
      }
    ],
    "name": "getStakerInfo",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "stakes",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "unstakes",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "staker",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epoch",
        "type": "uint256"
      }
    ],
    "name": "getStakerStakes",
    "outputs": [
      {
        "internalType": "address[]",
        "name": "_validators",
        "type": "address[]"
      },
      {
        "internalType": "uint256[]",
        "name": "stakes",
        "type": "uint256[]"
      },
      {
        "internalType": "uint256[]",
        "name": "stakeRequests",
        "type": "uint256[]"
      },
      {
        "internalType": "uint256[]",
        "name": "unstakeRequests",
        "type": "uint256[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "page",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "perPage",
        "type": "uint256"
      }
    ],
    "name": "getStakers",
    "outputs": [
      {
        "internalType": "address[]",
        "name": "",
        "type": "address[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "epochs",
        "type": "uint256"
      }
    ],
    "name": "getTotalRewards",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "rewards",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "getValidatorInfo",
    "outputs": [
      {
        "internalType": "address",
        "name": "operator",
        "type": "address"
      },
      {
        "internalType": "bool",
        "name": "active",
        "type": "bool"
      },
      {
        "internalType": "uint256",
        "name": "stakes",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "commissionRate",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "jailEpoch",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "epoch",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "page",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "perPage",
        "type": "uint256"
      }
    ],
    "name": "getValidatorStakes",
    "outputs": [
      {
        "internalType": "address[]",
        "name": "_stakers",
        "type": "address[]"
      },
      {
        "internalType": "uint256[]",
        "name": "stakes",
        "type": "uint256[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "getValidators",
    "outputs": [
      {
        "internalType": "address[]",
        "name": "",
        "type": "address[]"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "contract Environment",
        "name": "_environment",
        "type": "address"
      }
    ],
    "name": "initialize",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "initialized",
    "outputs": [
      {
        "internalType": "bool",
        "name": "",
        "type": "bool"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "operator",
        "type": "address"
      }
    ],
    "name": "joinValidator",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "name": "operatorToOwner",
    "outputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "operator",
        "type": "address"
      }
    ],
    "name": "slash",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      }
    ],
    "name": "stake",
    "outputs": [],
    "stateMutability": "payable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "stakerSigners",
    "outputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "name": "stakers",
    "outputs": [
      {
        "internalType": "address",
        "name": "signer",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "validator",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "amount",
        "type": "uint256"
      }
    ],
    "name": "unstake",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "newRate",
        "type": "uint256"
      }
    ],
    "name": "updateCommissionRate",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "operator",
        "type": "address"
      }
    ],
    "name": "updateOperator",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address[]",
        "name": "operators",
        "type": "address[]"
      },
      {
        "internalType": "uint256[]",
        "name": "counts",
        "type": "uint256[]"
      }
    ],
    "name": "updateValidatorBlocks",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "updateValidators",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "validatorOwners",
    "outputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "validatorUpdates",
    "outputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "name": "validators",
    "outputs": [
      {
        "internalType": "address",
        "name": "owner",
        "type": "address"
      },
      {
        "internalType": "address",
        "name": "operator",
        "type": "address"
      },
      {
        "internalType": "bool",
        "name": "active",
        "type": "bool"
      },
      {
        "internalType": "uint256",
        "name": "jailEpoch",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "lastClaimCommission",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  }
]
`
