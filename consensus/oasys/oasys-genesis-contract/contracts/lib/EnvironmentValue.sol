// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { Constants } from "./Constants.sol";
import { IEnvironment } from "../IEnvironment.sol";

error ValidationError(string detail);

/**
 * @title EnvironmentValue
 */
library EnvironmentValue {
    function epoch(IEnvironment.EnvironmentValue storage value) internal view returns (uint256) {
        return value.startEpoch + (block.number - value.startBlock) / value.epochPeriod;
    }

    function nextStartBlock(IEnvironment.EnvironmentValue storage value, IEnvironment.EnvironmentValue memory newValue)
        internal
        view
        returns (uint256)
    {
        return value.startBlock + (newValue.startEpoch - value.startEpoch) * value.epochPeriod;
    }

    function started(IEnvironment.EnvironmentValue storage value, uint256 _block) internal view returns (bool) {
        return _block >= value.startBlock;
    }

    function validate(IEnvironment.EnvironmentValue memory value) internal pure {
        if (value.blockPeriod < Constants.MIN_BLOCK_PERIOD) {
            revert ValidationError("blockPeriod is too small.");
        }
        if (value.epochPeriod < Constants.MIN_EPOCH_PERIOD) {
            revert ValidationError("epochPeriod is too small.");
        }
        if (value.rewardRate > Constants.MAX_REWARD_RATE) {
            revert ValidationError("rewardRate is too large.");
        }
        if (value.commissionRate > Constants.MAX_COMMISSION_RATE) {
            revert ValidationError("commissionRate is too large.");
        }
        if (value.validatorThreshold < Constants.MIN_VALIDATOR_THRESHOLD) {
            revert ValidationError("validatorThreshold is too small.");
        }
        if (value.jailThreshold < Constants.MIN_JAIL_THRESHOLD) {
            revert ValidationError("jailThreshold is too small.");
        }
        if (value.jailPeriod < Constants.MIN_JAIL_PERIOD) {
            revert ValidationError("jailPeriod is too small.");
        }
    }
}
