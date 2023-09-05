// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

/**
 * @title Constants
 */
library Constants {
    uint256 public constant REWARD_PRECISION = 25;
    uint256 public constant SECONDS_PER_YEAR = 60 * 60 * 24 * 365;
    uint256 public constant MIN_BLOCK_PERIOD = 1;
    uint256 public constant MIN_EPOCH_PERIOD = 3;
    uint256 public constant MAX_REWARD_RATE = 100;
    uint256 public constant MIN_VALIDATOR_THRESHOLD = 1;
    uint256 public constant MIN_JAIL_THRESHOLD = 1;
    uint256 public constant MIN_JAIL_PERIOD = 1;
    uint256 public constant MAX_COMMISSION_RATE = 100;
}
