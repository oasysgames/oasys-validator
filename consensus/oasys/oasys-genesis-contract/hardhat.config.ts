import { HardhatUserConfig } from 'hardhat/types'
import '@nomiclabs/hardhat-waffle'
import 'solidity-coverage'

const config: HardhatUserConfig = {
  solidity: {
    compilers: [
      {
        version: '0.5.17',
        settings: { optimizer: { enabled: true, runs: 200 } },
      },
      {
        version: '0.8.12',
        settings: { optimizer: { enabled: true, runs: 200 } },
      },
    ],
  },
  networks: {
    hardhat: {
      initialBaseFeePerGas: 0,
      gasPrice: 0,
    },
  },
  mocha: {
    timeout: 1000 * 60 * 3, // 3 minutes
  },
}

export default config
